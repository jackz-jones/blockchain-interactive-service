package gateway

import (
	"fmt"
	"net/http"

	"github.com/jackz-jones/blockchain-interactive-service/internal/config"
	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

// StartHTTPServer 启动 HTTP API Gateway 服务
func StartHTTPServer(c config.Config, svcCtx *svc.ServiceContext) {
	if !c.GatewayConf.Enable {
		logx.Info("[Gateway] HTTP gateway is disabled")
		return
	}

	server := rest.MustNewServer(rest.RestConf{
		Host: c.GatewayConf.Host,
		Port: c.GatewayConf.Port,
	})

	// 注册全局中间件
	// 1. 认证中间件
	server.Use(rest.ToMiddleware(middleware.HTTPAuthMiddleware(svcCtx.Repo)))
	// 2. 限流中间件
	rateLimiter := middleware.NewRateLimiter(c.GatewayConf.RateLimit)
	server.Use(rest.ToMiddleware(middleware.HTTPRateLimitMiddleware(rateLimiter)))
	// 3. 配额检查中间件
	server.Use(rest.ToMiddleware(middleware.HTTPQuotaMiddleware(svcCtx.BillingService)))
	// 4. 请求日志中间件
	server.Use(rest.ToMiddleware(httpLoggingMiddleware()))

	// 注册路由
	RegisterRoutes(server, svcCtx)

	// 异步启动 HTTP 服务
	go func() {
		addr := fmt.Sprintf("%s:%d", c.GatewayConf.Host, c.GatewayConf.Port)
		logx.Infof("[Gateway] HTTP API Gateway starting at %s", addr)
		fmt.Printf("Starting HTTP API Gateway at %s...\n", addr)
		server.Start()
	}()
}

// httpLoggingMiddleware 请求日志中间件
func httpLoggingMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logx.WithContext(r.Context()).Infof("[Gateway] %s %s from %s",
				r.Method, r.URL.Path, r.RemoteAddr)
			next.ServeHTTP(w, r)
		})
	}
}
