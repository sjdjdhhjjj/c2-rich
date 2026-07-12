//go:build windows

package main

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"unsafe"

	"golang.org/x/sys/windows"
)

// captureScreenWindows 使用 Windows GDI API 截屏，不依赖 PowerShell。
// 流程: SetProcessDPIAware → GetDC → CreateCompatibleDC → CreateCompatibleBitmap →
//       SelectObject → BitBlt → GetDIBits → BGRA→RGBA 转换 → 等比缩放 → JPEG 编码
func captureScreenWindows(targetH, quality int) ([]byte, error) {
	user32 := windows.NewLazySystemDLL("user32.dll")
	gdi32 := windows.NewLazySystemDLL("gdi32.dll")

	// 启用 DPI 感知，确保 GetSystemMetrics 返回物理分辨率而非缩放后的虚拟分辨率
	// 否则在高 DPI 屏幕（如 150% 缩放）上只截取到左上角的一部分
	setProcessDPIAware := user32.NewProc("SetProcessDPIAware")
	setProcessDPIAware.Call()

	getDC := user32.NewProc("GetDC")
	releaseDC := user32.NewProc("ReleaseDC")
	getSystemMetrics := user32.NewProc("GetSystemMetrics")

	createCompatibleDC := gdi32.NewProc("CreateCompatibleDC")
	createCompatibleBitmap := gdi32.NewProc("CreateCompatibleBitmap")
	selectObject := gdi32.NewProc("SelectObject")
	bitBlt := gdi32.NewProc("BitBlt")
	deleteDC := gdi32.NewProc("DeleteDC")
	deleteObject := gdi32.NewProc("DeleteObject")
	getDIBits := gdi32.NewProc("GetDIBits")

	// 获取主屏幕尺寸
	const (
		SM_CXSCREEN = 0
		SM_CYSCREEN = 1
	)
	cx, _, _ := getSystemMetrics.Call(SM_CXSCREEN)
	cy, _, _ := getSystemMetrics.Call(SM_CYSCREEN)
	width := int32(cx)
	height := int32(cy)
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("获取屏幕尺寸失败")
	}

	// a. GetDC(0) 获取屏幕 DC
	hdc, _, _ := getDC.Call(0)
	if hdc == 0 {
		return nil, fmt.Errorf("GetDC 失败")
	}
	defer releaseDC.Call(0, hdc)

	// b. CreateCompatibleDC 创建兼容 DC
	memDC, _, _ := createCompatibleDC.Call(hdc)
	if memDC == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC 失败")
	}
	defer deleteDC.Call(memDC)

	// c. CreateCompatibleBitmap 创建兼容位图
	hBmp, _, _ := createCompatibleBitmap.Call(hdc, uintptr(width), uintptr(height))
	if hBmp == 0 {
		return nil, fmt.Errorf("CreateCompatibleBitmap 失败")
	}
	defer deleteObject.Call(hBmp)

	// d. SelectObject 选入位图
	oldObj, _, _ := selectObject.Call(memDC, hBmp)
	if oldObj != 0 {
		defer selectObject.Call(memDC, oldObj)
	}

	// e. BitBlt 复制屏幕到位图
	const SRCCOPY = 0x00CC0020
	ret, _, _ := bitBlt.Call(memDC, 0, 0, uintptr(width), uintptr(height), hdc, 0, 0, SRCCOPY)
	if ret == 0 {
		return nil, fmt.Errorf("BitBlt 失败")
	}

	// f. 获取位图数据 (GetDIBits, 32-bit BGRA, top-down)
	type bitmapInfoHeader struct {
		biSize          uint32
		biWidth         int32
		biHeight        int32
		biPlanes        uint16
		biBitCount      uint16
		biCompression   uint32
		biSizeImage     uint32
		biXPelsPerMeter int32
		biYPelsPerMeter int32
		biClrUsed       uint32
		biClrImportant  uint32
	}
	var bi bitmapInfoHeader
	bi.biSize = 40
	bi.biWidth = width
	bi.biHeight = -height // 负值 = top-down (首行为屏幕顶部)
	bi.biPlanes = 1
	bi.biBitCount = 32    // BGRA, 4 字节/像素, 无行填充
	bi.biCompression = 0  // BI_RGB

	rowSize := int(width) * 4
	bufSize := rowSize * int(height)
	pixels := make([]byte, bufSize)

	n, _, _ := getDIBits.Call(memDC, hBmp, 0, uintptr(height),
		uintptr(unsafe.Pointer(&pixels[0])),
		uintptr(unsafe.Pointer(&bi)),
		0) // DIB_RGB_COLORS
	if n == 0 {
		return nil, fmt.Errorf("GetDIBits 失败")
	}

	// BGRA → RGBA
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	for y := 0; y < int(height); y++ {
		srcRow := y * rowSize
		dstRow := y * img.Stride
		for x := 0; x < int(width); x++ {
			s := srcRow + x*4
			d := dstRow + x*4
			img.Pix[d+0] = pixels[s+2] // R <- B
			img.Pix[d+1] = pixels[s+1] // G
			img.Pix[d+2] = pixels[s+0] // B <- R
			img.Pix[d+3] = 255         // A
		}
	}

	// g. 按目标高度等比缩放 (最近邻插值)
	finalImg := img
	if int(height) > targetH && targetH > 0 {
		ratio := float64(targetH) / float64(height)
		newW := int(float64(width) * ratio)
		if newW < 1 {
			newW = 1
		}
		scaled := image.NewRGBA(image.Rect(0, 0, newW, targetH))
		for y := 0; y < targetH; y++ {
			srcY := int(float64(y) / ratio)
			if srcY >= int(height) {
				srcY = int(height) - 1
			}
			srcRow := srcY * img.Stride
			dstRow := y * scaled.Stride
			for x := 0; x < newW; x++ {
				srcX := int(float64(x) / ratio)
				if srcX >= int(width) {
					srcX = int(width) - 1
				}
				s := srcRow + srcX*4
				d := dstRow + x*4
				scaled.Pix[d+0] = img.Pix[s+0]
				scaled.Pix[d+1] = img.Pix[s+1]
				scaled.Pix[d+2] = img.Pix[s+2]
				scaled.Pix[d+3] = img.Pix[s+3]
			}
		}
		finalImg = scaled
	}

	// h. 编码为 JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, finalImg, &jpeg.Options{Quality: quality}); err != nil {
		return nil, fmt.Errorf("JPEG 编码失败: %v", err)
	}
	return buf.Bytes(), nil
}
