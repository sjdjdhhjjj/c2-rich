package main

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// decodeWindowsOutput 在 Windows 上将 GBK 编码的命令输出解码为 UTF-8
// Windows cmd/tasklist/ipconfig 等默认输出 GBK (CP936)，直接当 UTF-8 用会乱码
func decodeWindowsOutput(b []byte) string {
	if runtime.GOOS != "windows" {
		return string(b)
	}
	// 先尝试 UTF-8 直接解析（部分命令可能输出 UTF-8）
	if isValidUTF8(b) {
		return string(b)
	}
	// 回退: GBK → UTF-8 解码
	r := transform.NewReader(bytes.NewReader(b), simplifiedchinese.GBK.NewDecoder())
	decoded, err := io.ReadAll(r)
	if err == nil && len(decoded) > 0 {
		return string(decoded)
	}
	return string(b)
}

// isValidUTF8 检查字节流是否为合法 UTF-8
func isValidUTF8(b []byte) bool {
	for i := 0; i < len(b); {
		c := b[i]
		size := 0
		switch {
		case c < 0x80:
			size = 1
		case c < 0xC2:
			return false // 非法 UTF-8 起始字节
		case c < 0xE0:
			size = 2
		case c < 0xF0:
			size = 3
		case c < 0xF5:
			size = 4
		default:
			return false
		}
		if i+size > len(b) {
			return false
		}
		for j := 1; j < size; j++ {
			if b[i+j]&0xC0 != 0x80 {
				return false
			}
		}
		i += size
	}
	return true
}

// runCommand 直接执行命令（无 shell），返回 stdout + 合并 stderr
func runCommand(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	hideWindow(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := decodeWindowsOutput(stdout.Bytes())
	if stderr.Len() > 0 {
		errStr := decodeWindowsOutput(stderr.Bytes())
		if out != "" {
			out += "\n[STDERR]\n"
		} else {
			out = "[STDERR]\n"
		}
		out += errStr
	}
	return out, err
}

// runCommandSeparate 直接执行命令，分别返回 stdout 和 stderr
func runCommandSeparate(name string, args ...string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	hideWindow(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return decodeWindowsOutput(stdout.Bytes()), decodeWindowsOutput(stderr.Bytes()), err
}

// runShellCmd 通过 shell 执行命令（对应 Python subprocess.run(shell=True)）
// shell 参数: "cmd"(默认) / "powershell" / "bash"
// 与 agent.py task_exec_cmd 逻辑一致
func runShellCmd(cmdLine, shell string, timeoutSec int) string {
	if timeoutSec <= 0 {
		timeoutSec = 60
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		switch shell {
		case "powershell":
			cmd = exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", cmdLine)
		default: // "cmd" 或空
			cmd = exec.CommandContext(ctx, "cmd", "/c", cmdLine)
		}
	} else {
		switch shell {
		case "bash":
			cmd = exec.CommandContext(ctx, "bash", "-c", cmdLine)
		default:
			// 使用 /bin/sh 兼容（更通用）
			cmd = exec.CommandContext(ctx, "sh", "-c", cmdLine)
		}
	}
	hideWindow(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	var output string
	if stdout.Len() > 0 {
		output = decodeWindowsOutput(stdout.Bytes())
	}
	if stderr.Len() > 0 {
		errStr := decodeWindowsOutput(stderr.Bytes())
		if output != "" {
			output += "\n[STDERR]\n"
		} else {
			output = "[STDERR]\n"
		}
		output += errStr
	}
	if ctx.Err() == context.DeadlineExceeded {
		return "[ERROR] Command timeout"
	}
	if err != nil && output == "" {
		return "[ERROR] " + err.Error()
	}
	if output == "" {
		return "(no output)"
	}
	return strings.TrimRight(output, "\r\n")
}
