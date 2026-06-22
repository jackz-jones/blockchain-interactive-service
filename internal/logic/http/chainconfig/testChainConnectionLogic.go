package chainconfig

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/sdk"
	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TestChainConnectionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTestChainConnectionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TestChainConnectionLogic {
	return &TestChainConnectionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TestChainConnectionLogic) TestChainConnection(req *types.ChainConfigIdPathRequest) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	role := middleware.GetUserRoleFromContext(l.ctx)
	if role != store.UserRoleAdmin {
		return &types.CommonResponse{Code: 403, Message: "admin role required"}, nil
	}

	chainConfig, err := l.svcCtx.Repo.GetChainConfigByID(l.ctx, req.Id)
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "get chain config: " + err.Error()}, nil
	}
	if chainConfig == nil {
		return &types.CommonResponse{Code: 404, Message: "chain config not found"}, nil
	}
	if chainConfig.TenantID != tenantID {
		return &types.CommonResponse{Code: 403, Message: "chain config not owned by current tenant"}, nil
	}

	// 测试连接仅检测节点连通性，不受链配置启用/禁用状态影响
	// 因此不通过 GetTenantSDKClient（它有 enable 检查），而是直接创建临时客户端测试
	nodes, err := l.svcCtx.Repo.ListChainNodesByConfigID(l.ctx, chainConfig.ID)
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "query chain nodes: " + err.Error()}, nil
	}

	sdkConf := sdk.BuildSDKConf(chainConfig, nodes)
	chainConf := &sdk.ChainConf{
		ChainType: chainConfig.ChainType,
		SDKConf:   sdkConf,
	}

	client, testErr := l.svcCtx.ChainClientFactory(l.ctx, chainConfig.ChainName, chainConfig.ChainType, chainConf, l.svcCtx.Config.Log, l.svcCtx.RedisClient)
	if client != nil {
		_ = client.Stop()
	}

	result := map[string]interface{}{
		"chain_name": chainConfig.ChainName,
		"chain_type": chainConfig.ChainType,
		"success":    testErr == nil,
	}
	if testErr != nil {
		result["error"] = testErr.Error()
	}

	return &types.CommonResponse{Code: 0, Message: "success", Data: result}, nil
}
