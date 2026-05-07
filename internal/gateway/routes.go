package gateway

import (
	"net/http"

	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/zeromicro/go-zero/rest"
)

// RegisterRoutes 注册所有 RESTful API 路由
func RegisterRoutes(server *rest.Server, svcCtx *svc.ServiceContext) {
	// API v1 路由组
	server.AddRoutes(
		[]rest.Route{
			// 合约调用
			{
				Method:  http.MethodPost,
				Path:    "/api/v1/contract/call",
				Handler: CallContractHandler(svcCtx),
			},
			// 查询交易
			{
				Method:  http.MethodGet,
				Path:    "/api/v1/tx/:txId",
				Handler: GetTxByTxIdHandler(svcCtx),
			},
			// 获取可用链和合约列表
			{
				Method:  http.MethodGet,
				Path:    "/api/v1/chains",
				Handler: GetAvailableChainsHandler(svcCtx),
			},
		},
		rest.WithPrefix(""),
	)

	// 租户管理 API（需要管理员权限）
	server.AddRoutes(
		[]rest.Route{
			// 租户注册
			{
				Method:  http.MethodPost,
				Path:    "/api/v1/tenants",
				Handler: CreateTenantHandler(svcCtx),
			},
			// 获取租户信息
			{
				Method:  http.MethodGet,
				Path:    "/api/v1/tenants/:id",
				Handler: GetTenantHandler(svcCtx),
			},
			// 租户列表
			{
				Method:  http.MethodGet,
				Path:    "/api/v1/tenants",
				Handler: ListTenantsHandler(svcCtx),
			},
			// 禁用租户
			{
				Method:  http.MethodPost,
				Path:    "/api/v1/tenants/:id/disable",
				Handler: DisableTenantHandler(svcCtx),
			},
			// 启用租户
			{
				Method:  http.MethodPost,
				Path:    "/api/v1/tenants/:id/enable",
				Handler: EnableTenantHandler(svcCtx),
			},
			// API Key 管理
			{
				Method:  http.MethodPost,
				Path:    "/api/v1/api-keys",
				Handler: CreateAPIKeyHandler(svcCtx),
			},
			{
				Method:  http.MethodGet,
				Path:    "/api/v1/api-keys",
				Handler: ListAPIKeysHandler(svcCtx),
			},
			// 链配置管理
			{
				Method:  http.MethodPost,
				Path:    "/api/v1/chain-configs",
				Handler: CreateChainConfigHandler(svcCtx),
			},
			{
				Method:  http.MethodGet,
				Path:    "/api/v1/chain-configs",
				Handler: ListChainConfigsHandler(svcCtx),
			},
			{
				Method:  http.MethodPut,
				Path:    "/api/v1/chain-configs/:id",
				Handler: UpdateChainConfigHandler(svcCtx),
			},
			{
				Method:  http.MethodDelete,
				Path:    "/api/v1/chain-configs/:id",
				Handler: DeleteChainConfigHandler(svcCtx),
			},
			// 用户管理
			{
				Method:  http.MethodGet,
				Path:    "/api/v1/users",
				Handler: ListUsersHandler(svcCtx),
			},
		},
		rest.WithPrefix(""),
	)

	// 管理后台 Dashboard API
	server.AddRoutes(
		[]rest.Route{
			// 仪表盘概览
			{
				Method:  http.MethodGet,
				Path:    "/api/v1/dashboard/overview",
				Handler: DashboardOverviewHandler(svcCtx),
			},
			// 调用日志
			{
				Method:  http.MethodGet,
				Path:    "/api/v1/dashboard/call-logs",
				Handler: ListCallLogsHandler(svcCtx),
			},
			// 用量统计
			{
				Method:  http.MethodGet,
				Path:    "/api/v1/dashboard/usage-stats",
				Handler: GetUsageStatsHandler(svcCtx),
			},
			// 账单列表
			{
				Method:  http.MethodGet,
				Path:    "/api/v1/dashboard/bills",
				Handler: ListBillsHandler(svcCtx),
			},
			// 审计日志
			{
				Method:  http.MethodGet,
				Path:    "/api/v1/dashboard/audit-logs",
				Handler: ListAuditLogsHandler(svcCtx),
			},
		},
		rest.WithPrefix(""),
	)
}
