package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/jackz-jones/blockchain-interactive-service/internal/billing"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// shouldSkipQuota 判断 gRPC 方法是否应跳过配额检查
// 业内 BaaS 标准：仅区块链核心操作计配额，查询类操作不计
// 计配额：CallContract（调用合约）、SubscribeContractEvents（订阅合约事件）
// 不计配额：GetTxByTxId（查询交易）、GetAvailableChainAndContractNames（查询链信息）
func shouldSkipQuota(fullMethod string) bool {
	skipMethods := []string{
		"/proto.ChainInteractive/GetTxByTxId",
		"/proto.ChainInteractive/GetAvailableChainAndContractNames",
		"/grpc.health.v1.Health/",
		"/grpc.reflection.v1alpha.ServerReflection/",
	}
	for _, m := range skipMethods {
		if strings.HasPrefix(fullMethod, m) {
			return true
		}
	}
	return false
}

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
	return func(
		ctx context.Context, req interface{},
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
	) (interface{}, error) {
		// 跳过认证的方法（健康检查、反射等）
		if shouldSkipAuth(info.FullMethod) {
			return handler(ctx, req)
		}

		// 跳过配额检查的查询类方法（查询交易、查询链信息等）
		// 业内 BaaS 标准：查询操作不消耗链上资源，不计配额
		if shouldSkipQuota(info.FullMethod) {
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

		// 仅对区块链核心操作（调用合约、订阅事件）记录用量
		resp, err := handler(ctx, req)
		if err == nil {
			go func() {
				_ = q.billingService.RecordUsage(context.Background(), tenantID)
			}()
		}

		return resp, err
	}
}

// shouldSkipHTTPQuota 判断 HTTP 请求是否应跳过配额检查
// 业内 BaaS 标准：仅区块链核心操作计配额，Web 平台管理操作不计
// 核心原则：只要涉及请求链节点的操作都算配额，除此之外都不算
// 计配额：/contract/call（调用合约）、/events/subscribe-by-contract POST（订阅合约事件，请求链节点）
// 不计配额：/tx/:txId（查询交易）、/chains（查询链列表）、/chains/:chainName/status（查询链状态）、
//
//	/dashboard/*（统计查询）、/events/subscriptions（查询订阅）、/events/available-contracts（查询合约）、
//	/events/recent/*（查询事件）、/events/subscribe-by-contract DELETE（取消订阅，仅更新DB+停止本地SDK，不请求链节点）、
//	/chain-configs/*（链配置管理，仅DB操作，不请求链节点）、/tenants/*（租户管理）、
//	/api-keys/*（API Key 管理）、/users/*（用户管理）
func shouldSkipHTTPQuota(path string) bool {
	skipPrefixes := []string{
		"/api/v1/tx/",
		"/api/v1/chains",
		"/api/v1/dashboard/",
		"/api/v1/events/subscriptions",
		"/api/v1/events/available-contracts",
		"/api/v1/events/recent/",
		"/api/v1/events/subscribe-by-contract/", // DELETE 取消订阅（不请求链节点）
		"/api/v1/chain-configs",
		"/api/v1/tenants",
		"/api/v1/api-keys",
		"/api/v1/users",
	}
	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
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

			// 跳过 Web 平台管理操作的配额检查
			// 业内 BaaS 标准：查询/管理类操作不消耗链上资源，不计配额
			if shouldSkipHTTPQuota(r.URL.Path) {
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
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"code":429,"message":"quota exceeded, please upgrade your plan"}`))
				return
			}

			if warning {
				// 在响应头中添加配额预警信息
				w.Header().Set("X-Quota-Warning", "approaching limit")
			}

			next.ServeHTTP(w, r)

			// 仅对区块链核心操作记录用量
			go func() {
				_ = billingService.RecordUsage(context.Background(), tenantID)
			}()
		})
	}
}
