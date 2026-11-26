package contractconfig

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// 提取的重复字符串常量（goconst）
const ()

type DeleteContractConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteContractConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteContractConfigLogic {
	return &DeleteContractConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteContractConfigLogic) DeleteContractConfig(
	req *types.DeleteContractConfigRequest,
) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: unauthorizedStr}, nil
	}

	role := middleware.GetUserRoleFromContext(l.ctx)
	if role != store.UserRoleAdmin {
		return &types.CommonResponse{Code: 403, Message: msgAdminRoleRequired}, nil
	}

	existing, err := l.svcCtx.Repo.GetContractConfig(l.ctx, req.Id)
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "get contract config: " + err.Error()}, nil
	}
	if existing == nil {
		return &types.CommonResponse{Code: 404, Message: contractConfigNotFound}, nil
	}
	if existing.TenantID != tenantID {
		return &types.CommonResponse{Code: 403, Message: msgContractNotOwned}, nil
	}

	if err := l.svcCtx.Repo.DeleteContractConfig(l.ctx, req.Id); err != nil {
		return &types.CommonResponse{Code: 500, Message: "delete contract config: " + err.Error()}, nil
	}

	// 停止该合约的订阅协程
	chainConfig, _ := l.svcCtx.Repo.GetChainConfigByID(l.ctx, req.ChainConfigId)
	if chainConfig != nil {
		l.svcCtx.TenantSDKManager.StopContractSubscription(req.ChainConfigId, req.Id, tenantID, chainConfig.ChainName)
	}

	// 使 ConfigResolver 该链下的缓存失效，避免 ResolveByAddr/ByName 返回已删除的合约
	if l.svcCtx.ConfigResolver != nil {
		l.svcCtx.ConfigResolver.InvalidateByChainConfigID(req.ChainConfigId)
	}

	return &types.CommonResponse{Code: 0, Message: successStr}, nil
}
