package middleware

import (
	"net"
	"strings"

	"github.com/bestruirui/octopus/internal/model"
	"github.com/bestruirui/octopus/internal/op"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// isLocalDevOrigin 检查是否为本地开发环境的 origin
func isLocalDevOrigin(origin string) bool {
	if origin == "" {
		return false
	}

	// 提取 host 部分
	host := origin
	if idx := strings.Index(origin, "://"); idx != -1 {
		host = origin[idx+3:]
	}
	host = strings.TrimRight(host, "/")

	// 移除端口
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// 检查是否为 localhost 或 127.0.0.1
	if host == "localhost" || host == "127.0.0.1" {
		return true
	}

	// 检查是否为本地 IP 地址
	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		return true
	}

	return false
}

func Cors() gin.HandlerFunc {
	config := cors.DefaultConfig()
	config.AllowCredentials = true
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"*"}
	config.ExposeHeaders = []string{"Content-Disposition"}
	// CORS 白名单:
	// - 为空: 不允许跨域
	// - "*": 允许所有来源
	// - 逗号分隔的域名列表: 只允许指定的域名 (如 "https://example.com,https://example2.com")
	// - 开发模式: 自动允许 localhost 和 127.0.0.1 的任意端口
	config.AllowOriginFunc = func(origin string) bool {
		// 本地开发 origin 始终允许（localhost/127.0.0.1 是安全的）
		if isLocalDevOrigin(origin) {
			return true
		}

		allowed, err := op.SettingGetString(model.SettingKeyCORSAllowOrigins)
		if err != nil {
			return false
		}
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			return false
		}
		if allowed == "*" {
			return true
		}

		origin = strings.TrimSpace(origin)
		if origin == "" {
			return false
		}

		// 提取 origin 的 host 部分用于匹配
		originHost := origin
		if idx := strings.Index(origin, "://"); idx != -1 {
			originHost = origin[idx+3:]
		}
		originHost = strings.TrimRight(originHost, "/")

		for _, item := range strings.Split(allowed, ",") {
			item = strings.TrimSpace(item)
			item = strings.TrimRight(item, "/")
			if item == "" {
				continue
			}
			// 支持完整 origin (https://example.com) 或仅域名 (example.com)
			if item == origin || item == originHost {
				return true
			}
		}
		return false
	}
	return cors.New(config)
}
