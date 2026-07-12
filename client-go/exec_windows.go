//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// CREATE_NO_WINDOW 阻止子进程创建控制台窗口（0x08000000）
// 作用: 即使父进程是 GUI 程序（-H windowsgui），cmd/powershell 等子进程
// 默认仍会弹出新控制台窗口；设置此标志后子进程完全无窗口
const createNoWindow = 0x08000000

// hideWindow 为 exec.Cmd 设置 Windows 专属属性，确保子进程不弹出控制台黑框
// 同时设置 HideWindow（对 console 子系统程序生效）和 CREATE_NO_WINDOW（更彻底，
// 直接不分配控制台）。两者并用兼容性最好。
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
