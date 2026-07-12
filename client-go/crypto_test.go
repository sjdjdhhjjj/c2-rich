package main

import (
	"encoding/json"
	"os"
	"testing"
)

// 跨语言加密一致性验证: Go 端
// 1) 读取 Python 生成的 test_vectors.json，用 Go 解密，验证明文一致
// 2) 用 Go 加密同一明文，写入 go_encrypted.json 供 Python 反向验证
//
// 运行顺序:
//   python cryptotest_gen.py        # 生成 test_vectors.json
//   go test -run TestCryptoCompat -v
//   python cryptotest_gen.py        # 反向验证 go_encrypted.json

const testPassword = "C2DemoKey2024!!!"
const testPlaintext = `{"test":"hello","n":42,"中文":"测试"}`

var testAlgos = []string{"none", "aes-128-cbc", "aes-256-cbc", "xor", "rc4", "chacha20"}

func TestCryptoCompat(t *testing.T) {
	// 步骤1: 读取 Python 生成的向量并用 Go 解密
	data, err := os.ReadFile("test_vectors.json")
	if err != nil {
		t.Skipf("test_vectors.json 不存在，先运行 python cryptotest_gen.py: %v", err)
	}
	var tv struct {
		Password  string            `json:"password"`
		Plaintext string            `json:"plaintext"`
		Vectors   map[string]string `json:"vectors"`
	}
	if err := json.Unmarshal(data, &tv); err != nil {
		t.Fatalf("解析 test_vectors.json 失败: %v", err)
	}
	if tv.Plaintext != testPlaintext {
		t.Fatalf("明文不匹配: got %q", tv.Plaintext)
	}

	allOK := true
	for _, algo := range testAlgos {
		ct, ok := tv.Vectors[algo]
		if !ok {
			t.Errorf("[FAIL] %s: 向量缺失", algo)
			allOK = false
			continue
		}
		pt, err := encDecrypt(ct, algo, testPassword)
		if err != nil {
			t.Errorf("[FAIL] %s: Go 解密错误: %v", algo, err)
			allOK = false
			continue
		}
		if string(pt) != testPlaintext {
			t.Errorf("[FAIL] %s: 明文不匹配 got %q want %q", algo, string(pt), testPlaintext)
			allOK = false
			continue
		}
		t.Logf("[OK]   %s: Go 解密 Python 加密 -> 一致", algo)
	}

	// 步骤2: 用 Go 加密，写入 go_encrypted.json 供 Python 反向验证
	goVectors := map[string]string{}
	for _, algo := range testAlgos {
		ct, _, err := encEncrypt([]byte(testPlaintext), algo, testPassword)
		if err != nil {
			t.Fatalf("[FAIL] %s: Go 加密错误: %v", algo, err)
		}
		goVectors[algo] = ct
		// 同时验证 Go 自身能解回来
		pt, err := encDecrypt(ct, algo, testPassword)
		if err != nil || string(pt) != testPlaintext {
			t.Errorf("[FAIL] %s: Go 自身往返失败", algo)
			allOK = false
		}
	}
	out := map[string]interface{}{"password": testPassword, "plaintext": testPlaintext, "vectors": goVectors}
	b, _ := json.Marshal(out)
	if err := os.WriteFile("go_encrypted.json", b, 0644); err != nil {
		t.Fatalf("写入 go_encrypted.json 失败: %v", err)
	}
	t.Logf("[2] 已写入 go_encrypted.json，请重新运行 python cryptotest_gen.py 反向验证")

	if !allOK {
		t.Fatal("加密一致性验证失败")
	}
}
