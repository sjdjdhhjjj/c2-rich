package main

import (
	"net"
	"sync"

	"github.com/gorilla/websocket"
)

// ============ WS/TCP 长连接客户端注册表 ============
// 用于服务端主动推送任务给 WS/TCP 客户端（无需客户端轮询）
// key: client_id, value: 连接信息
// 当 handleSendTask 下发任务时，如果目标客户端在此注册表中，直接推送任务，无需等待 pull

// AgentConn 长连接客户端连接信息
type AgentConn struct {
	ClientID  string
	ConnType  string // "websocket" 或 "tcp"
	WSConn    *websocket.Conn
	TCPConn   net.Conn
	WriteMu   sync.Mutex // 写锁，确保推送和响应不交叉
	EncAlgo   string     // 客户端使用的加密算法（响应加密用）
	EncPwd    string     // 客户端使用的加密密码
}

// agentConnRegistry 长连接客户端注册表
var (
	agentConns   = make(map[string]*AgentConn) // client_id → conn
	agentConnsMu sync.RWMutex
)

// registerAgentConn 注册长连接客户端
func registerAgentConn(clientID, connType, encAlgo, encPwd string, wsConn *websocket.Conn, tcpConn net.Conn) *AgentConn {
	agentConnsMu.Lock()
	defer agentConnsMu.Unlock()
	// 如果已存在旧连接，不关闭（由各自 handler 管理生命周期），仅覆盖注册
	ac := &AgentConn{
		ClientID: clientID,
		ConnType: connType,
		WSConn:   wsConn,
		TCPConn:  tcpConn,
		EncAlgo:  encAlgo,
		EncPwd:   encPwd,
	}
	agentConns[clientID] = ac
	return ac
}

// unregisterAgentConn 注销长连接客户端
func unregisterAgentConn(clientID string) {
	agentConnsMu.Lock()
	defer agentConnsMu.Unlock()
	delete(agentConns, clientID)
}

// getAgentConn 获取长连接客户端
func getAgentConn(clientID string) *AgentConn {
	agentConnsMu.RLock()
	defer agentConnsMu.RUnlock()
	return agentConns[clientID]
}

// pushTaskToAgent 通过长连接主动推送任务给客户端
// 返回 true 表示推送成功，false 表示客户端不在线或非长连接模式
func pushTaskToAgent(clientID string, taskData map[string]interface{}) bool {
	ac := getAgentConn(clientID)
	if ac == nil {
		return false
	}
	ac.WriteMu.Lock()
	defer ac.WriteMu.Unlock()

	// 构造推送消息（与 pull 响应格式一致: {"tasks": [...]}）
	pushPayload := map[string]interface{}{
		"tasks": []map[string]interface{}{taskData},
	}

	// 加密推送内容（用客户端的算法和密码）
	encrypted, _, err := encEncrypt(pushPayload, ac.EncAlgo, ac.EncPwd)
	if err != nil {
		return false
	}

	if ac.ConnType == "websocket" && ac.WSConn != nil {
		if err := ac.WSConn.WriteMessage(websocket.TextMessage, []byte(encrypted)); err != nil {
			return false
		}
		return true
	}
	if ac.ConnType == "tcp" && ac.TCPConn != nil {
		if err := writeTCPFrame(ac.TCPConn, []byte(encrypted)); err != nil {
			return false
		}
		return true
	}
	return false
}
