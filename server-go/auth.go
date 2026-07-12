package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ============ 认证中间件（与 app.py require_auth 对齐）============

var (
	tokens = map[string]tokenInfo{}
	tokensMu sync.RWMutex
)

type tokenInfo struct {
	UserID   int64
	Username string
	Role     string
	Expire   time.Time
}

// md5Hash MD5 哈希（与 crypto_utils.py md5_hash 对齐）
func md5Hash(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

// generateToken 生成登录 token
func generateToken(username string, userID int64, role string) string {
	token := md5Hash(fmt.Sprintf("%s%d", username, time.Now().UnixNano()))
	tokensMu.Lock()
	tokens[token] = tokenInfo{
		UserID:   userID,
		Username: username,
		Role:     role,
		Expire:   time.Now().Add(24 * time.Hour),
	}
	tokensMu.Unlock()
	return token
}

// validateToken 校验 token，返回 tokenInfo
func validateToken(token string) (tokenInfo, bool) {
	if token == "" {
		return tokenInfo{}, false
	}
	tokensMu.RLock()
	info, ok := tokens[token]
	tokensMu.RUnlock()
	if !ok {
		return tokenInfo{}, false
	}
	if time.Now().After(info.Expire) {
		tokensMu.Lock()
		delete(tokens, token)
		tokensMu.Unlock()
		return tokenInfo{}, false
	}
	return info, true
}

// getTokenFromRequest 从请求头或 query 参数提取 token
// 支持: Authorization: Bearer <token> 或 ?token=<token>（用于 window.open 下载场景）
func getTokenFromRequest(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	auth = strings.TrimPrefix(auth, "Bearer ")
	auth = strings.TrimSpace(auth)
	if auth != "" {
		return auth
	}
	// 回退: query 参数 ?token=xxx（下载场景 window.open 无法带 header）
	return strings.TrimSpace(r.URL.Query().Get("token"))
}

// requireAuth 认证中间件，校验失败返回 401
func requireAuth(handler func(w http.ResponseWriter, r *http.Request, user tokenInfo)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := getTokenFromRequest(r)
		info, ok := validateToken(token)
		if !ok {
			jsonError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		handler(w, r, info)
	}
}

// requireAdmin 要求 admin 角色
func requireAdmin(handler func(w http.ResponseWriter, r *http.Request, user tokenInfo)) http.HandlerFunc {
	return requireAuth(func(w http.ResponseWriter, r *http.Request, user tokenInfo) {
		if user.Role != "admin" {
			jsonError(w, "Forbidden: admin role required", http.StatusForbidden)
			return
		}
		handler(w, r, user)
	})
}

// getRequestIP 获取请求者 IP
func getRequestIP(r *http.Request) string {
	// 优先从 X-Forwarded-For 获取（代理场景）
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// 去掉端口号
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx > 0 {
		addr = addr[:idx]
	}
	return strings.Trim(addr, "[]")
}

// ============ 登录 API（与 app.py /api/login 对齐）============

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &data); err != nil {
		jsonError(w, "Invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	user, err := queryOne("SELECT * FROM users WHERE username=?", data.Username)
	if err != nil || user == nil {
		addLog("login", fmt.Sprintf("用户 %s 登录失败", data.Username), "", 0, getRequestIP(r))
		jsonError(w, "用户名或密码错误", http.StatusUnauthorized)
		return
	}

	storedPwd := getString(user, "password", "")
	if storedPwd != data.Password {
		addLog("login", fmt.Sprintf("用户 %s 登录失败", data.Username), "", 0, getRequestIP(r))
		jsonError(w, "用户名或密码错误", http.StatusUnauthorized)
		return
	}

	userID := getInt(user, "id", 0)
	role := getString(user, "role", "admin")
	token := generateToken(data.Username, userID, role)
	addLog("login", fmt.Sprintf("用户 %s 登录成功", data.Username), "", userID, getRequestIP(r))

	jsonOK(w, map[string]interface{}{
		"token":    token,
		"username": data.Username,
		"role":     role,
	})
}

// ============ 修改密码 API（与 app.py /api/user/password 对齐）============

func handleChangePassword(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	var data struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := decodeJSON(r, &data); err != nil {
		jsonError(w, "Invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	u, _ := queryOne("SELECT * FROM users WHERE id=?", user.UserID)
	if u == nil {
		jsonError(w, "用户不存在", http.StatusNotFound)
		return
	}
	if getString(u, "password", "") != data.OldPassword {
		jsonError(w, "原密码错误", http.StatusBadRequest)
		return
	}
	if len(data.NewPassword) < 4 {
		jsonError(w, "新密码至少 4 个字符", http.StatusBadRequest)
		return
	}

	dbExec("UPDATE users SET password=? WHERE id=?", data.NewPassword, user.UserID)
	addLog("settings", fmt.Sprintf("用户 %s 修改密码", user.Username), "", user.UserID, getRequestIP(r))
	jsonOK(w, map[string]interface{}{"success": true})
}

// ============ 用户管理 API（与 app.py /api/users 对齐）============

func handleListUsers(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	if user.Role != "admin" {
		jsonError(w, "Forbidden: admin role required", http.StatusForbidden)
		return
	}
	users, err := queryAll("SELECT id, username, role, created_at FROM users ORDER BY id")
	if err != nil {
		jsonError(w, "查询失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, users)
}

func handleCreateUser(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	if user.Role != "admin" {
		jsonError(w, "Forbidden: admin role required", http.StatusForbidden)
		return
	}
	var data struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := decodeJSON(r, &data); err != nil {
		jsonError(w, "Invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if data.Username == "" || data.Password == "" {
		jsonError(w, "用户名和密码不能为空", http.StatusBadRequest)
		return
	}
	if data.Role == "" {
		data.Role = "admin"
	}

	// 检查用户名是否已存在
	existing, _ := queryOne("SELECT id FROM users WHERE username=?", data.Username)
	if existing != nil {
		jsonError(w, "用户名已存在", http.StatusBadRequest)
		return
	}

	dbExec("INSERT INTO users (username, password, role) VALUES (?, ?, ?)", data.Username, data.Password, data.Role)
	addLog("settings", fmt.Sprintf("创建用户: %s (%s)", data.Username, data.Role), "", user.UserID, getRequestIP(r))
	jsonOK(w, map[string]interface{}{"success": true})
}

func handleDeleteUser(w http.ResponseWriter, r *http.Request, user tokenInfo) {
	if user.Role != "admin" {
		jsonError(w, "Forbidden: admin role required", http.StatusForbidden)
		return
	}
	uid := r.PathValue("uid")
	if uid == "" {
		jsonError(w, "缺少用户 ID", http.StatusBadRequest)
		return
	}

	// 不允许删除内置 admin 和当前登录用户
	target, _ := queryOne("SELECT username FROM users WHERE id=?", uid)
	if target == nil {
		jsonError(w, "用户不存在", http.StatusNotFound)
		return
	}
	targetName := getString(target, "username", "")
	if targetName == "admin" {
		jsonError(w, "不允许删除内置 admin 账户", http.StatusBadRequest)
		return
	}
	if targetName == user.Username {
		jsonError(w, "不允许删除当前登录用户", http.StatusBadRequest)
		return
	}

	dbExec("DELETE FROM users WHERE id=?", uid)
	addLog("settings", fmt.Sprintf("删除用户: %s", targetName), "", user.UserID, getRequestIP(r))
	jsonOK(w, map[string]interface{}{"success": true})
}
