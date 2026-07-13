package main

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// ============ 原生 WebSocket（替代 Socket.IO）============
// 与 app.py socketio.emit('client_update'/'task_update') 对齐
// 前端 app-core.js 从 io() 改为 new WebSocket('/ws')

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源（与 Socket.IO cors_allowed_origins="*" 对齐）
	},
}

// wsClient 包装一个 WebSocket 连接
type wsClient struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// 全局 WebSocket 客户端管理
var (
	wsClients   = map[*wsClient]bool{}
	wsClientsMu sync.RWMutex
)

// handleWebSocket WebSocket 升级处理器
// 鉴权: 升级前校验 token（query 参数 ?token=xxx 或 Header）
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 鉴权: 未登录的连接直接拒绝，防止信息泄露
	token := getTokenFromRequest(r)
	if _, ok := validateToken(token); !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // 升级失败，gorilla/websocket 会自动返回错误响应
	}

	client := &wsClient{conn: conn}
	wsClientsMu.Lock()
	wsClients[client] = true
	wsClientsMu.Unlock()

	defer func() {
		conn.Close()
		wsClientsMu.Lock()
		delete(wsClients, client)
		wsClientsMu.Unlock()
	}()

	// 读取循环（保持连接活跃，处理 ping/pong）
	for {
		msgType, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
		// 处理 ping 消息（自动回复 pong）
		if msgType == websocket.PingMessage {
			client.mu.Lock()
			conn.WriteMessage(websocket.PongMessage, nil)
			client.mu.Unlock()
		}
	}
}

// broadcastEvent 向所有 WebSocket 客户端广播事件
// 与 Socket.IO 的 emit 等价，前端通过 event 字段区分消息类型
func broadcastEvent(event string) {
	msg := map[string]interface{}{"event": event}
	broadcastJSON(msg)
}

// broadcastJSON 向所有客户端广播 JSON 消息
func broadcastJSON(msg interface{}) {
	wsClientsMu.RLock()
	clients := make([]*wsClient, 0, len(wsClients))
	for c := range wsClients {
		clients = append(clients, c)
	}
	wsClientsMu.RUnlock()

	for _, c := range clients {
		c.mu.Lock()
		err := c.conn.WriteJSON(msg)
		c.mu.Unlock()
		if err != nil {
			// 写失败，移除客户端
			wsClientsMu.Lock()
			delete(wsClients, c)
			wsClientsMu.Unlock()
			c.conn.Close()
		}
	}
}

// broadcastClientUpdate 广播客户端更新事件（与 socketio.emit('client_update') 对齐）
func broadcastClientUpdate() {
	broadcastEvent("client_update")
}

// broadcastTaskUpdate 广播任务更新事件（与 socketio.emit('task_update') 对齐）
func broadcastTaskUpdate() {
	broadcastEvent("task_update")
}

// getWebSocketClientCount 获取当前 WebSocket 连接数
func getWebSocketClientCount() int {
	wsClientsMu.RLock()
	defer wsClientsMu.RUnlock()
	return len(wsClients)
}
