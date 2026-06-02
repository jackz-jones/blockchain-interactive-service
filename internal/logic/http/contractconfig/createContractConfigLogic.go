package contractconfig

import (
	"context"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"
	"github.com/jackz-jones/blockchain-interactive-service/internal/validator"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateContractConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateContractConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateContractConfigLogic {
	return &CreateContractConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateContractConfigLogic) CreateContractConfig(
	req *types.CreateContractConfigRequest,
) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	role := middleware.GetUserRoleFromContext(l.ctx)
	if role != store.UserRoleAdmin {
		return &types.CommonResponse{Code: 403, Message: "admin role required for write operations"}, nil
	}

	// 校验链配置归属
	chainConfig, errResp := l.getAndValidateChainConfig(tenantID, req.ChainConfigId)
	if errResp != nil {
		return errResp.(*types.CommonResponse), nil
	}

	if req.ContractName == "" {
		return &types.CommonResponse{Code: 400, Message: "contract_name is required"}, nil
	}

	// 字段校验
	contractReq := &validator.ContractConfigRequest{
		ContractName:    req.ContractName,
		ContractAddr:    req.ContractAddr,
		AbiJSON:         req.AbiJson,
		EnableSubscribe: req.EnableSubscribe,
		ExtraConf:       req.ExtraConf,
	}
	if err := validator.ValidateContractConfig(chainConfig.ChainType, contractReq); err != nil {
		return &types.CommonResponse{Code: 400, Message: err.Error()}, nil
	}

	// 检查合约名称唯一性
	existingConfigs, err := l.svcCtx.Repo.ListContractConfigsByChain(l.ctx, req.ChainConfigId)
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "check contract name: " + err.Error()}, nil
	}
	for _, ec := range existingConfigs {
		if ec.ContractName == req.ContractName {
			return &types.CommonResponse{Code: 409, Message: "contract name already exists under this chain config"}, nil
		}
	}

	config := &store.TenantContractConfig{
		TenantID:        tenantID,
		ChainConfigID:   req.ChainConfigId,
		ContractName:    req.ContractName,
		ContractAddr:    req.ContractAddr,
		AbiJSON:         req.AbiJson,
		EnableSubscribe: req.EnableSubscribe,
		ExtraConf:       req.ExtraConf,
	}

	if err := l.svcCtx.Repo.CreateContractConfig(l.ctx, config); err != nil {
		return &types.CommonResponse{Code: 500, Message: "create contract config: " + err.Error()}, nil
	}

	l.svcCtx.TenantSDKManager.InvalidateTenantCache(tenantID, chainConfig.ChainName)

	return &types.CommonResponse{Code: 0, Message: "success", Data: config}, nil
}

// getAndValidateChainConfig 获取并校验链配置归属
func (l *CreateContractConfigLogic) getAndValidateChainConfig(
	tenantID, chainConfigID uint,
) (*store.TenantChainConfig, interface{}) {
	chainConfigs, err := l.svcCtx.Repo.ListChainConfigsByTenant(l.ctx, tenantID)
	if err != nil {
		return nil, &types.CommonResponse{Code: 500, Message: "list chain configs: " + err.Error()}
	}
	for _, cc := range chainConfigs {
		if cc.ID == chainConfigID {
			if !cc.Enable {
				return nil, &types.CommonResponse{Code: 400, Message: "关联的链配置未启用，请先启用链配置"}
			}
			return cc, nil
		}
	}
	return nil, &types.CommonResponse{Code: 403, Message: "chain config not found or not owned by current tenant"}
}
