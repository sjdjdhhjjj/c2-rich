package main

import (
	"os"
	"os/user"
	"runtime"
	"strings"
)

// getSystemInfo 基础系统信息（心跳用），与 agent.py get_system_info 对齐
// 字段: client_id(由调用方填) / hostname / os / os_version / arch / username
func getSystemInfo() map[string]interface{} {
	hostname, _ := os.Hostname()
	info := map[string]interface{}{
		"hostname":   hostname,
		"os":         goosToPython(),
		"os_version": getOSVersion(),
		"arch":       goarchToMachine(),
		"username":   getUsername(),
	}
	return info
}

// goosToPython 映射为 Python platform.system() 的返回值 (Windows/Linux/Darwin)
func goosToPython() string {
	switch runtime.GOOS {
	case "windows":
		return "Windows"
	case "linux":
		return "Linux"
	case "darwin":
		return "Darwin"
	default:
		return runtime.GOOS
	}
}

// goarchToMachine 映射为 Python platform.machine() 的返回值
func goarchToMachine() string {
	switch runtime.GOOS {
	case "windows":
		switch runtime.GOARCH {
		case "amd64":
			return "AMD64"
		case "arm64":
			return "ARM64"
		case "386":
			return "x86"
		}
		return runtime.GOARCH
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return "x86_64"
		case "arm64":
			return "aarch64"
		case "386":
			return "i686"
		}
		return runtime.GOARCH
	}
	return runtime.GOARCH
}

// getUsername 获取当前用户名，与 agent.py 一致: USERNAME 或 USER 环境变量
func getUsername() string {
	if v := os.Getenv("USERNAME"); v != "" {
		return v
	}
	if v := os.Getenv("USER"); v != "" {
		return v
	}
	if u, err := user.Current(); err == nil {
		// user.Current().Username 在 Windows 上返回 "HOST\user" 形式，取最后一段
		if idx := strings.LastIndex(u.Username, "\\"); idx >= 0 {
			return u.Username[idx+1:]
		}
		return u.Username
	}
	return "unknown"
}

// getOSVersion 获取操作系统版本字符串（对应 Python platform.version()）
// Windows: 形如 "10.0.19041"；Linux: 内核版本如 "6.5.0-..."
func getOSVersion() string {
	if runtime.GOOS == "windows" {
		// 用 ver 命令获取版本（避免 cgo）
		if out, err := runCmdQuiet("cmd", "/c", "ver"); err == nil {
			// 输出形如: Microsoft Windows [Version 10.0.19041.1]
			s := strings.TrimSpace(out)
			if idx := strings.Index(s, "[Version "); idx >= 0 {
				return strings.TrimSuffix(s[idx+9:], "]")
			}
			return s
		}
		return "unknown"
	}
	if out, err := runCmdQuiet("uname", "-r"); err == nil {
		return strings.TrimSpace(out)
	}
	return "unknown"
}

// runCmdQuiet 静默运行命令并返回 stdout（无 shell，避免注入）
func runCmdQuiet(name string, args ...string) (string, error) {
	out, err := runCommand(name, args...)
	return out, err
}
