package chain

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetChainStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetChainStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetChainStatusLogic {
	return &GetChainStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetChainStatusLogic) GetChainStatus(req *types.GetChainStatusRequest) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	if req.ChainName == "" {
		return &types.CommonResponse{Code: 400, Message: "chainName is required"}, nil
	}

	// 获取客户端状态
	status := l.svcCtx.TenantSDKManager.GetClientStatus(tenantID, req.ChainName)

	// 获取链配置信息
	chainConfig, err := l.svcCtx.Repo.GetChainConfig(l.ctx, tenantID, req.ChainName)
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "get chain config: " + err.Error()}, nil
	}
	if chainConfig == nil {
		return &types.CommonResponse{Code: 404, Message: "chain not found for this tenant"}, nil
	}

	// 获取合约配置列表
	contractConfigs, _ := l.svcCtx.Repo.ListContractConfigsByChain(l.ctx, chainConfig.ID)
	var contractStatuses []map[string]interface{}
	for _, cc := range contractConfigs {
		cs := map[string]interface{}{
			"contract_name": cc.ContractName,
			"contract_addr": cc.ContractAddr,
		}
		contractStatuses = append(contractStatuses, cs)
	}

	status["chain_type"] = chainConfig.ChainType
	status["enable"] = chainConfig.Enable
	status["source"] = "database"
	status["contracts"] = contractStatuses

	return &types.CommonResponse{
		Code:    0,
		Message: "success",
		Data:    status,
	}, nil
}
