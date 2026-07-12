package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rc4"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"

	"golang.org/x/crypto/chacha20"
)

// 通信加密算法（服务端/客户端/WebShell 三方对齐）:
//   none / aes-128-cbc / aes-256-cbc / xor / rc4 / chacha20
// 密钥派生: sha256(password)[:16|32]
// AES-CBC: base64(iv[16] + ciphertext(PKCS7))
// RC4:     base64(ciphertext)              (无前缀)
// ChaCha20:base64(nonce[12] + ciphertext)  (IETF, 12字节 nonce)

// CryptoConfig 保存当前加密配置（生成 payload 时注入，或从环境变量读取）
type CryptoConfig struct {
	Algo     string
	Password string
}

// deriveKey 从密码派生指定字节数密钥（SHA-256 截断），与服务端一致
func deriveKey(pw string, byteLen int) []byte {
	h := sha256.Sum256([]byte(pw))
	return h[:byteLen]
}

// pkcs7Pad / pkcs7Unpad: PKCS#7 填充（块大小 16）
func pkcs7Pad(data []byte, blockSize int) []byte {
	pad := blockSize - len(data)%blockSize
	out := make([]byte, len(data)+pad)
	copy(out, data)
	for i := len(data); i < len(out); i++ {
		out[i] = byte(pad)
	}
	return out
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, errors.New("invalid padding length")
	}
	pad := int(data[len(data)-1])
	if pad < 1 || pad > blockSize || pad > len(data) {
		return nil, errors.New("invalid padding bytes")
	}
	return data[:len(data)-pad], nil
}

func aesEncrypt(plain []byte, pw string, mode string) (string, error) {
	keyLen := 16
	if strings.Contains(mode, "256") {
		keyLen = 32
	}
	key := deriveKey(pw, keyLen)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	iv := make([]byte, 16)
	if _, err := rand.Read(iv); err != nil {
		return "", err
	}
	padded := pkcs7Pad(plain, 16)
	ct := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ct, padded)
	out := append(append([]byte{}, iv...), ct...)
	return base64.StdEncoding.EncodeToString(out), nil
}

func aesDecrypt(b64 string, pw string, mode string) ([]byte, error) {
	keyLen := 16
	if strings.Contains(mode, "256") {
		keyLen = 32
	}
	key := deriveKey(pw, keyLen)
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	if len(raw) < 16 {
		return nil, errors.New("ciphertext too short")
	}
	iv := raw[:16]
	ct := raw[16:]
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	pt := make([]byte, len(ct))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(pt, ct)
	return pkcs7Unpad(pt, 16)
}

func xorEncrypt(plain []byte, pw string) string {
	key := []byte(pw)
	out := make([]byte, len(plain))
	for i, b := range plain {
		out[i] = b ^ key[i%len(key)]
	}
	return base64.StdEncoding.EncodeToString(out)
}

func xorDecrypt(b64 string, pw string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	key := []byte(pw)
	out := make([]byte, len(raw))
	for i, b := range raw {
		out[i] = b ^ key[i%len(key)]
	}
	return out, nil
}

func rc4Encrypt(plain []byte, pw string) string {
	key := deriveKey(pw, 32)
	c, err := rc4.NewCipher(key)
	if err != nil {
		return ""
	}
	out := make([]byte, len(plain))
	c.XORKeyStream(out, plain)
	return base64.StdEncoding.EncodeToString(out)
}

func rc4Decrypt(b64 string, pw string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	key := deriveKey(pw, 32)
	c, err := rc4.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(raw))
	c.XORKeyStream(out, raw)
	return out, nil
}

func chacha20Encrypt(plain []byte, pw string) (string, error) {
	key := deriveKey(pw, 32)
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	c, err := chacha20.NewUnauthenticatedCipher(key, nonce)
	if err != nil {
		return "", err
	}
	ct := make([]byte, len(plain))
	c.XORKeyStream(ct, plain)
	out := append(append([]byte{}, nonce...), ct...)
	return base64.StdEncoding.EncodeToString(out), nil
}

func chacha20Decrypt(b64 string, pw string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	if len(raw) < 12 {
		return nil, errors.New("ciphertext too short")
	}
	key := deriveKey(pw, 32)
	nonce := raw[:12]
	ct := raw[12:]
	c, err := chacha20.NewUnauthenticatedCipher(key, nonce)
	if err != nil {
		return nil, err
	}
	pt := make([]byte, len(ct))
	c.XORKeyStream(pt, ct)
	return pt, nil
}

// encEncrypt 加密任意数据（dict/[]byte/string），返回 (base64密文, 实际算法)
// 与 agent.py enc_encrypt 行为一致: dict 先紧凑 JSON 序列化
func encEncrypt(data interface{}, algo string, pw string) (string, string, error) {
	a := strings.ToLower(strings.TrimSpace(algo))
	var plain []byte
	switch v := data.(type) {
	case nil:
		plain = []byte("null")
	case []byte:
		plain = v
	case string:
		plain = []byte(v)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", "", err
		}
		plain = b
	}
	switch a {
	case "", "none", "plaintext":
		return base64.StdEncoding.EncodeToString(plain), "none", nil
	case "aes-128-cbc", "aes-128":
		s, err := aesEncrypt(plain, pw, "aes-128-cbc")
		return s, "aes-128-cbc", err
	case "aes-256-cbc", "aes-256":
		s, err := aesEncrypt(plain, pw, "aes-256-cbc")
		return s, "aes-256-cbc", err
	case "xor":
		return xorEncrypt(plain, pw), "xor", nil
	case "rc4":
		return rc4Encrypt(plain, pw), "rc4", nil
	case "chacha20":
		s, err := chacha20Encrypt(plain, pw)
		return s, "chacha20", err
	}
	return base64.StdEncoding.EncodeToString(plain), "none", nil
}

// encEncryptJSON 便捷方法: 将 map 序列化为紧凑 JSON 后加密
// (json.Marshal 默认无空格，等价于 Python separators=(',',':'))
func encEncryptJSON(data map[string]interface{}, algo string, pw string) (string, string, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return "", "", err
	}
	return encEncrypt(b, algo, pw)
}

// encDecrypt 解密 base64 密文，返回原始字节
func encDecrypt(b64 string, algo string, pw string) ([]byte, error) {
	a := strings.ToLower(strings.TrimSpace(algo))
	switch a {
	case "", "none", "plaintext":
		return base64.StdEncoding.DecodeString(b64)
	case "aes-128-cbc", "aes-128":
		return aesDecrypt(b64, pw, "aes-128-cbc")
	case "aes-256-cbc", "aes-256":
		return aesDecrypt(b64, pw, "aes-256-cbc")
	case "xor":
		return xorDecrypt(b64, pw)
	case "rc4":
		return rc4Decrypt(b64, pw)
	case "chacha20":
		return chacha20Decrypt(b64, pw)
	}
	return base64.StdEncoding.DecodeString(b64)
}

// encDecryptJSON 解密并解析为 map[string]interface{}
func encDecryptJSON(b64 string, algo string, pw string) (map[string]interface{}, error) {
	raw, err := encDecrypt(b64, algo, pw)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}
