//go:build !windows

package main

import "fmt"

// cameraPhotoWindows 非 Windows 平台的 stub（不会被执行）
// task_media.go 中已通过 runtime.GOOS == "windows" 判断，Linux 走 ffmpeg 分支
func cameraPhotoWindows() ([]byte, error) {
	return nil, fmt.Errorf("cameraPhotoWindows: 仅 Windows 支持 WIA")
}

// cameraRecordWindows 非 Windows 平台的 stub
func cameraRecordWindows(duration int) ([]byte, error) {
	return nil, fmt.Errorf("cameraRecordWindows: 仅 Windows 支持 WIA")
}
