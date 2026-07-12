//go:build !windows

package main

import "os/exec"

// hideWindow Unix 系统无需隐藏窗口（无控制台窗口概念）
func hideWindow(cmd *exec.Cmd) {}
