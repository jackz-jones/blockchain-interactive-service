package middleware

import (
	"context"
	"net/http"

	"github.com/jackz-jones/blockchain-interactive-service/internal/billing"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// QuotaInterceptor 配额检查 gRPC 拦截器
type QuotaInterceptor struct {
	billingService *billing.Service
}

// NewQuotaInterceptor 创建配额检查拦截器
func NewQuotaInterceptor(billingService *billing.Service) *QuotaInterceptor {
	return &QuotaInterceptor{billingService: billingService}
}

// Unary 一元 RPC 配额检查拦截器
func (q *QuotaInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if shouldSkipAuth(info.FullMethod) {
			return handler(ctx, req)
		}

		tenantID := GetTenantID(ctx)
		if tenantID == 0 {
			return handler(ctx, req)
		}

		allowed, warning, err := q.billingService.CheckQuota(ctx, tenantID)
		if err != nil {
			logx.WithContext(ctx).Errorf("check quota error: %v", err)
			// 配额检查出错时放行，避免影响正常业务
			return handler(ctx, req)
		}

		if !allowed {
			return nil, status.Error(codes.ResourceExhausted, "quota exceeded, please upgrade your plan")
		}

		if warning {
			logx.WithContext(ctx).Infof("tenant %d quota warning: approaching limit", tenantID)
		}

		// 调用成功后记录用量
		resp, err := handler(ctx, req)
		if err == nil {
			go func() {
				_ = q.billingService.RecordUsage(context.Background(), tenantID)
			}()
		}

		return resp, err
	}
}

// HTTPQuotaMiddleware HTTP 配额检查中间件
func HTTPQuotaMiddleware(billingService *billing.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := GetTenantIDFromHTTP(r)
			if tenantID == 0 {
				next.ServeHTTP(w, r)
				return
			}

			allowed, warning, err := billingService.CheckQuota(r.Context(), tenantID)
			if err != nil {
				logx.WithContext(r.Context()).Errorf("check quota error: %v", err)
				// 配额检查出错时放行
				next.ServeHTTP(w, r)
				return
			}

			if !allowed {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"code":403,"message":"quota exceeded, please upgrade your plan"}`))
				return
			}

			if warning {
				// 在响应头中添加配额预警信息
				w.Header().Set("X-Quota-Warning", "approaching limit")
			}

			next.ServeHTTP(w, r)

			// 请求完成后记录用量
			go func() {
				_ = billingService.RecordUsage(context.Background(), tenantID)
			}()
		})
	}
}
