package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
)

// ============ HTTP 响应辅助函数 ============

// jsonOK 返回 JSON 成功响应
func jsonOK(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

// jsonError 返回 JSON 错误响应
func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{"error": msg})
}

// decodeJSON 解码请求体 JSON
func decodeJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// serverDir 服务端目录（main.go 中设置）
var serverDir string

// uploadDir 上传目录
func uploadDir() string {
	return filepath.Join(serverDir, "uploads")
}

// payloadDir Payload 输出目录
func payloadDir() string {
	return filepath.Join(serverDir, "payloads")
}

// ensureDirs 确保必要目录存在
func ensureDirs() {
	dirs := []string{
		uploadDir(),
		payloadDir(),
		getTmpDir(),
		filepath.Join(getTmpDir(), "screenshots"),
		filepath.Join(getTmpDir(), "recordings"),
		filepath.Join(getTmpDir(), "audio"),
		filepath.Join(getTmpDir(), "downloads"),
		filepath.Join(getTmpDir(), "uploads"),
	}
	for _, d := range dirs {
		os.MkdirAll(d, 0755)
	}
}

// fileExists 检查文件是否存在
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
