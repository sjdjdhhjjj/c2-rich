//go:build windows

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// avicap32 message constants
const (
	wmUser               = 0x400
	wmCapDriverConnect   = wmUser + 10
	wmCapDriverDisconnect = wmUser + 11
	wmCapSetPreview      = wmUser + 50
	wmCapSetPreviewRate  = wmUser + 52
	wmCapSetScale        = wmUser + 53
	wmCapGrabFrameNoStop = wmUser + 61
	wmCapFileSaveDIBA    = wmUser + 25
)

// cameraPhotoWindows 用多种方式尝试拍照
// 优先级: WinRT MediaCapture → ffmpeg dshow → WIA → avicap32.dll (VFW)
// 错误信息用英文避免非 UTF-8 控制台乱码
func cameraPhotoWindows() ([]byte, error) {
	var errs []string

	// 1. 优先 WinRT MediaCapture（UWP API，走不同权限路径，能绕过 WIA 限制）
	if data, err := cameraPhotoWinRT(); err == nil && len(data) > 0 {
		return data, nil
	} else if err != nil {
		errs = append(errs, "WinRT: "+err.Error())
	}

	// 2. ffmpeg dshow（自动检测设备名）
	if toolExists("ffmpeg") {
		if data, err := cameraPhotoFFmpegAuto(); err == nil && len(data) > 0 {
			return data, nil
		} else if err != nil {
			errs = append(errs, "ffmpeg: "+err.Error())
		}
	}

	// 3. WIA
	if data, err := cameraPhotoWIA(); err == nil && len(data) > 0 {
		return data, nil
	} else if err != nil {
		errs = append(errs, "WIA: "+err.Error())
	}

	// 4. avicap32.dll (VFW API，32 位 PowerShell)
	if data, err := cameraPhotoAviCap(); err == nil && len(data) > 0 {
		return data, nil
	} else if err != nil {
		errs = append(errs, "AviCap: "+err.Error())
	}

	return nil, fmt.Errorf("all camera methods failed: %s", strings.Join(errs, "; "))
}

// cameraPhotoWinRT 用 Windows.Media.Capture.MediaCapture (WinRT API) 拍照
// 这是 UWP 摄像头 API，走与 WIA 不同的权限路径
// 在某些机器上 WIA 被隐私设置阻止但 WinRT 仍可工作
// 通过 PowerShell + C# 调用 Windows.Runtime.Interop API
func cameraPhotoWinRT() ([]byte, error) {
	jpgPath := filepath.Join(tmpDir(), fmt.Sprintf("_cam_%d.jpg", time.Now().UnixNano()))
	defer os.Remove(jpgPath)

	// PowerShell 调用 WinRT MediaCapture API
	// 需要 Windows 10+ 和 System.Runtime.WindowsRuntime.dll
	// 注意: Go 原始字符串(反引号)不能包含反引号，所以用 __BACKTICK__ 占位符替换
	psScript := `$ErrorActionPreference='Stop'
[Windows.Foundation.Metadata.ApiInformation, Windows.Foundation.Metadata, ContentType=WindowsRuntime] | Out-Null
$jpgPath = '__JPG__'
if (Test-Path $jpgPath) { Remove-Item $jpgPath -Force }

# 加载 WinRT 类型
$asq = ([System.AppDomain]::CurrentDomain.GetAssemblies() | Where-Object { $_.FullName -match 'System.Runtime.WindowsRuntime' })
if (-not $asq) {
  Add-Type -AssemblyName System.Runtime.WindowsRuntime
}
$asTaskGeneric = ([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object { $_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1 -and $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation__BACKTICK__1' })[0]

function Await($op, $t) {
  $task = $asTaskGeneric.MakeGenericMethod($t).Invoke($null, @($op))
  $task.Wait(10000) | Out-Null
  $task.Result
}

try {
  # 创建 MediaCapture
  $mc = New-Object Windows.Media.Capture.MediaCapture
  $init = $mc.InitializeAsync()
  Await $init ([Windows.Media.Capture.MediaCaptureInitializationResult])

  # 创建 InMemoryRandomAccessStream
  $stream = New-Object Windows.Storage.Streams.InMemoryRandomAccessStream
  $props = New-Object Windows.Media.MediaProperties.ImageEncodingProperties
  $props.Subtype = 'JPEG'
  $props.Width = 1280
  $props.Height = 720

  # 拍照
  $capOp = $mc.CapturePhotoToStreamAsync($props, $stream)
  Await $capOp ([Windows.Foundation.IMediaCapture])

  # 读取流数据
  $stream.Seek(0) | Out-Null
  $reader = New-Object Windows.Storage.Streams.DataReader($stream.GetInputStreamAt(0))
  $loadOp = $reader.LoadAsync($stream.Size)
  Await $loadOp ([uint32])
  $bytes = New-Object byte[] $stream.Size
  $reader.ReadBytes($bytes)

  [System.IO.File]::WriteAllBytes($jpgPath, $bytes)
  $mc.Dispose()
  Write-Output 'OK'
} catch {
  Write-Error "WinRT: $($_.Exception.Message)"
  exit 1
}
`
	psScript = strings.ReplaceAll(psScript, "__JPG__", jpgPath)
	psScript = strings.ReplaceAll(psScript, "__BACKTICK__", "`")

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
	hideWindow(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("WinRT capture failed: %v %s", err, stderr.String())
	}
	return os.ReadFile(jpgPath)
}

// cameraPhotoAviCap 用 avicap32.dll (Video for Windows) 拍照
// Windows 原生视频捕获 API，通过 PowerShell + C# P/Invoke 调用
// 关键点:
//   1. avicap32 是 32 位 DLL，64 位进程调用会失败，必须用 32 位 PowerShell (SysWOW64)
//   2. avicap32 是消息驱动 API，必须有 Windows 消息泵(DoEvents)才能抓帧
// 流程: 创建捕获窗口 → 连接驱动 → 启用预览 → 消息泵预热 → 抓帧 → 保存BMP → 转JPEG
func cameraPhotoAviCap() ([]byte, error) {
	bmpPath := filepath.Join(tmpDir(), fmt.Sprintf("_cam_%d.bmp", time.Now().UnixNano()))
	jpgPath := filepath.Join(tmpDir(), fmt.Sprintf("_cam_%d.jpg", time.Now().UnixNano()))
	defer os.Remove(bmpPath)
	defer os.Remove(jpgPath)

	// 用 __BMP__ / __JPG__ 占位符 + ReplaceAll 避免 fmt.Sprintf 参数计数错误
	// 关键: 加 [System.Windows.Forms.Application]::DoEvents() 消息泵
	//       avicap32 的帧捕获回调需要消息循环才能执行，否则 GRAB_FRAME 和 FILE_SAVEDIBA 都会失败
	psScript := `$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.Drawing, System.Windows.Forms
try {
  Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;
public class AviCap {
  [DllImport("avicap32.dll", EntryPoint="capCreateCaptureWindowA", CharSet=CharSet.Ansi)]
  public static extern IntPtr capCreateCaptureWindowA(string lpszWindowName, int dwStyle, int x, int y, int nWidth, int nHeight, IntPtr hwndParent, int nID);
  [DllImport("user32.dll", EntryPoint="SendMessage", CharSet=CharSet.Ansi)]
  public static extern int SendMessageStr(IntPtr hWnd, int Msg, int wParam, [MarshalAs(UnmanagedType.LPStr)] string lParam);
  [DllImport("user32.dll", EntryPoint="SendMessage")]
  public static extern int SendMessage(IntPtr hWnd, int Msg, int wParam, int lParam);
  [DllImport("user32.dll", EntryPoint="DestroyWindow")]
  public static extern bool DestroyWindow(IntPtr hWnd);
}
"@
} catch {}

$bmpPath = '__BMP__'
$jpgPath = '__JPG__'
if (Test-Path $bmpPath) { Remove-Item $bmpPath -Force }
if (Test-Path $jpgPath) { Remove-Item $jpgPath -Force }

$WM_USER = 0x400
$WM_CAP_DRIVER_CONNECT = $WM_USER + 10
$WM_CAP_DRIVER_DISCONNECT = $WM_USER + 11
$WM_CAP_SET_PREVIEW = $WM_USER + 50
$WM_CAP_SET_PREVIEWRATE = $WM_USER + 52
$WM_CAP_SET_SCALE = $WM_USER + 53
$WM_CAP_GRAB_FRAME_NOSTOP = $WM_USER + 61
$WM_CAP_FILE_SAVEDIBA = $WM_USER + 25

$hwnd = [AviCap]::capCreateCaptureWindowA("Capture", 0, 0, 0, 640, 480, [IntPtr]::Zero, 0)
if ($hwnd -eq [IntPtr]::Zero) { Write-Error 'AviCap: failed to create capture window'; exit 1 }

$connected = $false
for ($i = 0; $i -lt 10; $i++) {
  $ret = [AviCap]::SendMessage($hwnd, $WM_CAP_DRIVER_CONNECT, $i, 0)
  if ($ret -ne 0) { $connected = $true; break }
  Start-Sleep -Milliseconds 50
}
if (-not $connected) {
  [AviCap]::DestroyWindow($hwnd)
  Write-Error 'AviCap: no driver connected (tried index 0-9)'
  exit 1
}

[AviCap]::SendMessage($hwnd, $WM_CAP_SET_SCALE, 1, 0) | Out-Null
[AviCap]::SendMessage($hwnd, $WM_CAP_SET_PREVIEWRATE, 30, 0) | Out-Null
[AviCap]::SendMessage($hwnd, $WM_CAP_SET_PREVIEW, 1, 0) | Out-Null

# CRITICAL: pump Windows messages for 2 seconds so the capture window can render frames
# Without DoEvents, avicap32 never processes WM_TIMER/WM_PAINT and no frames are captured
$endTime = [System.DateTime]::Now.AddMilliseconds(2000)
while ([System.DateTime]::Now -lt $endTime) {
  [System.Windows.Forms.Application]::DoEvents()
  Start-Sleep -Milliseconds 30
}

$saved = $false
for ($attempt = 0; $attempt -lt 5; $attempt++) {
  [AviCap]::SendMessage($hwnd, $WM_CAP_GRAB_FRAME_NOSTOP, 0, 0) | Out-Null
  # Pump messages so the grab callback can execute
  $subEnd = [System.DateTime]::Now.AddMilliseconds(300)
  while ([System.DateTime]::Now -lt $subEnd) {
    [System.Windows.Forms.Application]::DoEvents()
    Start-Sleep -Milliseconds 20
  }
  $ret = [AviCap]::SendMessageStr($hwnd, $WM_CAP_FILE_SAVEDIBA, 0, $bmpPath)
  if ($ret -ne 0 -and (Test-Path $bmpPath)) {
    $f = Get-Item $bmpPath
    if ($f.Length -gt 0) { $saved = $true; break }
  }
  Start-Sleep -Milliseconds 200
}

[AviCap]::SendMessage($hwnd, $WM_CAP_DRIVER_DISCONNECT, 0, 0) | Out-Null
[AviCap]::DestroyWindow($hwnd)

if (-not $saved) {
  Write-Error 'AviCap: failed to save frame after 5 attempts'
  exit 1
}

$img = [System.Drawing.Image]::FromFile($bmpPath)
$img.Save($jpgPath, [System.Drawing.Imaging.ImageFormat]::Jpeg)
$img.Dispose()
Write-Output 'OK'
`
	psScript = strings.ReplaceAll(psScript, "__BMP__", bmpPath)
	psScript = strings.ReplaceAll(psScript, "__JPG__", jpgPath)

	// CRITICAL: avicap32.dll is a 32-bit DLL. On 64-bit Windows, the 64-bit PowerShell
	// can load avicap32.dll but WM_CAP_DRIVER_CONNECT returns 0 (failure) because the
	// WDM video capture driver (VfWWDM32.dll) only works in 32-bit process context.
	// Must use 32-bit PowerShell from SysWOW64.
	powershellPath := os.ExpandEnv("$env:WINDIR\\SysWOW64\\WindowsPowerShell\\v1.0\\powershell.exe")
	if _, err := os.Stat(powershellPath); err != nil {
		// 回退到默认 powershell（32 位系统或路径异常）
		powershellPath = "powershell"
	}

	cmd := exec.Command(powershellPath, "-NoProfile", "-NonInteractive", "-STA", "-Command", psScript)
	hideWindow(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("AviCap capture failed: %v %s", err, stderr.String())
	}
	return os.ReadFile(jpgPath)
}

// cameraPhotoFFmpegAuto 自动检测 dshow 视频设备名并拍照
// 解决硬编码 "Integrated Camera" 在不同机器上不通用的问题
func cameraPhotoFFmpegAuto() ([]byte, error) {
	// 1. 列出所有 dshow 设备
	listCmd := exec.Command("ffmpeg", "-list_devices", "true", "-f", "dshow", "-i", "dummy")
	hideWindow(listCmd)
	out, _ := listCmd.CombinedOutput()
	listStr := string(out)

	// 2. 从输出中提取视频设备名
	// ffmpeg 输出格式: "video devices" 段下的 "Integrated Camera" 等
	// 正则匹配引号内的设备名
	videoDevs := extractDShowVideoDevices(listStr)
	if len(videoDevs) == 0 {
		return nil, fmt.Errorf("no dshow video device found in ffmpeg output")
	}

	// 3. 用第一个视频设备拍照
	tmpPath := filepath.Join(tmpDir(), fmt.Sprintf("_cam_%d.jpg", time.Now().UnixNano()))
	defer os.Remove(tmpPath)

	for _, devName := range videoDevs {
		cmd := exec.Command("ffmpeg", "-y", "-f", "dshow", "-i", "video="+devName,
			"-frames:v", "1", "-q:v", "3", tmpPath)
		hideWindow(cmd)
		if out, err := cmd.CombinedOutput(); err == nil {
			if data, err := os.ReadFile(tmpPath); err == nil && len(data) > 0 {
				return data, nil
			}
		} else {
			_ = out // 继续尝试下一个设备
		}
	}
	return nil, fmt.Errorf("all dshow devices failed: %v", videoDevs)
}

// extractDShowVideoDevices 从 ffmpeg -list_devices 输出中提取视频设备名
// ffmpeg 输出示例:
//   [dshow @ ...] "Integrated Camera" (video)
//   [dshow @ ...] "Microphone" (audio)
func extractDShowVideoDevices(output string) []string {
	var devices []string
	// 匹配 "设备名" (video)  格式
	// ffmpeg 输出中视频设备在 "DirectShow video devices" 段下
	re := regexp.MustCompile(`"([^"]+)"\s*\(video\)`)
	matches := re.FindAllStringSubmatch(output, -1)
	for _, m := range matches {
		if len(m) >= 2 {
			devices = append(devices, m[1])
		}
	}
	return devices
}

// cameraPhotoWIA 用 WIA (Windows Image Acquisition) 拍照
// 错误信息用英文，避免非 UTF-8 控制台乱码
// 增加详细诊断: 列出所有 WIA 设备类型 + PnP 摄像头设备
func cameraPhotoWIA() ([]byte, error) {
	tmpPath := filepath.Join(tmpDir(), fmt.Sprintf("_cam_%d.jpg", time.Now().UnixNano()))
	defer os.Remove(tmpPath)

	// PowerShell 调用 WIA COM 组件拍照
	// 增加诊断: 列出所有设备类型 + 用 Get-PnpDevice 检查摄像头硬件
	psScript := fmt.Sprintf(`$ErrorActionPreference='Stop'
try {
  # Diagnostic: check PnP camera devices
  $pnpCams = @()
  try {
    $pnpCams = Get-PnpDevice -Class Camera,Image -ErrorAction SilentlyContinue | Where-Object { $_.Status -eq 'OK' } | ForEach-Object { $_.FriendlyName }
  } catch {}
  $pnpStr = $pnpCams -join ', '

  $mgr = New-Object -ComObject WIA.DeviceManager
  $cam = $null
  $allDevs = @()
  foreach ($d in $mgr.DeviceInfos) {
    $allDevs += "Type=$($d.Type)"
    if ($d.Type -eq 3) { $cam = $d; break }
  }
  if (-not $cam) {
    $devList = $allDevs -join ','
    $msg = "WIA: no video device (Type=3). WIA devices: [$devList]. PnP cameras: [$pnpStr]"
    if ($pnpCams.Count -eq 0) {
      $msg += ". NO camera hardware detected - machine may not have a camera"
    }
    Write-Error $msg
    exit 1
  }
  $dev = $cam.Connect()
  if ($dev.Items.Count -lt 1) { Write-Error 'WIA: camera has no items'; exit 1 }
  $item = $dev.Items.Item(1)
  $img = $item.Transfer()
  $img.SaveFile('%s')
} catch {
  Write-Error $_.Exception.Message
  exit 1
}`, tmpPath)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
	hideWindow(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("WIA capture failed: %v %s", err, stderr.String())
	}
	return os.ReadFile(tmpPath)
}

// cameraRecordWindows 用 ffmpeg dshow 录像（自动检测设备名）
// 无 ffmpeg 时回退为多帧拍照（cameraRecordFrames 内部调用 cameraPhotoWindows，
// 自动包含 ffmpeg→WIA→AviCap 三级回退）
func cameraRecordWindows(duration int) ([]byte, error) {
	if toolExists("ffmpeg") {
		return cameraRecordFFmpegAuto(duration)
	}
	// 无 ffmpeg 回退: 多帧拍照拼接（每帧自动尝试 ffmpeg/WIA/AviCap）
	return cameraRecordFrames(duration)
}

// cameraRecordFFmpegAuto 自动检测设备名并录像
func cameraRecordFFmpegAuto(duration int) ([]byte, error) {
	// 1. 列出设备
	listCmd := exec.Command("ffmpeg", "-list_devices", "true", "-f", "dshow", "-i", "dummy")
	hideWindow(listCmd)
	out, _ := listCmd.CombinedOutput()
	videoDevs := extractDShowVideoDevices(string(out))
	if len(videoDevs) == 0 {
		return nil, fmt.Errorf("no dshow video device found")
	}

	tmpPath := filepath.Join(tmpDir(), fmt.Sprintf("_cam_%d.avi", time.Now().UnixNano()))
	defer os.Remove(tmpPath)

	for _, devName := range videoDevs {
		cmd := exec.Command("ffmpeg", "-y", "-f", "dshow", "-i", "video="+devName,
			"-t", fmt.Sprintf("%d", duration), "-c:v", "mpeg4", "-vtag", "XVID", "-q:v", "5", tmpPath)
		hideWindow(cmd)
		if err := cmd.Run(); err == nil {
			if data, err := os.ReadFile(tmpPath); err == nil && len(data) > 0 {
				return data, nil
			}
		}
	}
	return nil, fmt.Errorf("all dshow devices failed for recording")
}

// cameraRecordFrames 多帧拍照方式录像（无 ffmpeg 回退方案）
func cameraRecordFrames(duration int) ([]byte, error) {
	fps := 8
	frames := duration * fps
	if frames > 480 {
		frames = 480
	}

	var buf bytes.Buffer
	for i := 0; i < frames; i++ {
		data, err := cameraPhotoWindows()
		if err == nil && len(data) > 0 {
			buf.Write(data)
		}
		if i < frames-1 {
			time.Sleep(time.Second / time.Duration(fps))
		}
	}
	if buf.Len() == 0 {
		return nil, fmt.Errorf("camera record: no frames captured")
	}
	return buf.Bytes(), nil
}
