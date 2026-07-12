//go:build !windows

package main

import "fmt"

// captureScreenWindows 非 Windows 平台 stub（实际通过 runtime.GOOS 分支不会调用到此）
func captureScreenWindows(targetH, quality int) ([]byte, error) {
	return nil, fmt.Errorf("Windows 截屏仅在 Windows 平台可用")
}
