package main

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// ============ Web 控制台安全中间件（Origin 验证 + CSP 安全头）============
// 遵循 config.json 的 host/port/protocol 构建允许的 Origin 白名单
// 防止 CSRF 攻击（跨站请求伪造）和 XSS 注入

var (
	allowedOriginCache map[string]bool
	originCacheMu      sync.RWMutex
)

// getAllowedOrigins 基于 config.json 构建允许的 Origin 集合
// 白名单来源: config.json 的 web.host + web.port + web.protocol
func getAllowedOrigins() map[string]bool {
	originCacheMu.RLock()
	if allowedOriginCache != nil {
		origins := allowedOriginCache
		originCacheMu.RUnlock()
		return origins
	}
	originCacheMu.RUnlock()

	originCacheMu.Lock()
	defer originCacheMu.Unlock()

	// 双重检查
	if allowedOriginCache != nil {
		return allowedOriginCache
	}

	origins := map[string]bool{}
	cfg, _ := loadFileConfig()
	host := cfg.Web.Host
	port := strconv.Itoa(cfg.Web.Port)
	proto := strings.ToLower(cfg.Web.Protocol)
	if proto == "" {
		proto = "http"
	}

	if host == "0.0.0.0" || host == "" {
		// 监听所有接口，允许常见的本机访问地址
		for _, h := range []string{"localhost", "127.0.0.1"} {
			origins[proto+"://"+h+":"+port] = true
		}
		// 加上本机实际 IP（局域网访问）
		if ip := getLocalIP(); ip != "" && ip != "127.0.0.1" {
			origins[proto+"://"+ip+":"+port] = true
		}
	} else {
		// 绑定到具体地址，只允许该地址
		origins[proto+"://"+host+":"+port] = true
		// 同时允许通过 127.0.0.1 访问（本机调试场景）
		if host != "127.0.0.1" && host != "localhost" {
			origins[proto+"://127.0.0.1:"+port] = true
			origins[proto+"://localhost:"+port] = true
		}
	}

	allowedOriginCache = origins
	return origins
}

// refreshAllowedOrigins 刷新 Origin 缓存（config.json 重新加载后调用）
func refreshAllowedOrigins() {
	originCacheMu.Lock()
	allowedOriginCache = nil
	originCacheMu.Unlock()
}

// isOriginAllowed 检查 Origin 是否在白名单中
func isOriginAllowed(origin string) bool {
	if origin == "" {
		// 无 Origin 头的请求（curl/API 客户端/浏览器导航），放行靠 token 鉴权
		return true
	}
	allowed := getAllowedOrigins()
	return allowed[origin]
}

// securityMiddleware Web 控制台安全中间件
// 1. 设置 CSP 等安全响应头
// 2. 对 /api/* 和 /ws 路径验证 Origin（防 CSRF）
func securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 设置安全响应头
		setSecurityHeaders(w)

		// Origin 验证（仅对 /api/* 和 /ws 路径，静态文件不验证）
		path := r.URL.Path
		if strings.HasPrefix(path, "/api/") || path == "/ws" {
			origin := r.Header.Get("Origin")
			if !isOriginAllowed(origin) {
				jsonError(w, "Origin not allowed", http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// setSecurityHeaders 设置 CSP 和其他安全响应头
func setSecurityHeaders(w http.ResponseWriter) {
	// CSP：限制资源加载来源，防止 XSS 注入
	// 所有第三方库（Tailwind/Chart.js/jQuery/Font Awesome）已下载到本地 static/ 目录，不再依赖 CDN
	// - script-src: 仅同源 + unsafe-inline（前端有 inline script）
	// - style-src: 仅同源 + unsafe-inline（动态样式）
	// - img-src: 同源 + data URI + blob（截图预览）
	// - connect-src: 仅同源（API + WebSocket）
	// - font-src: 同源 + data URI（Font Awesome 本地字体）
	// - object-src 'none': 禁止 Flash/Java 插件
	// - base-uri 'self': 禁止 base 标签篡改
	// - form-action 'self': 禁止表单提交到外部
	csp := "default-src 'self'; " +
		"script-src 'self' 'unsafe-inline'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: blob:; " +
		"connect-src 'self'; " +
		"font-src 'self' data:; " +
		"object-src 'none'; " +
		"base-uri 'self'; " +
		"form-action 'self'; " +
		"frame-ancestors 'none'"

	w.Header().Set("Content-Security-Policy", csp)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.Header().Set("X-DNS-Prefetch-Control", "off")
}
