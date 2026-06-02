package chainconfig

import (
	"context"
	"strings"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"
	"github.com/jackz-jones/blockchain-interactive-service/internal/validator"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateChainConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateChainConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateChainConfigLogic {
	return &CreateChainConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateChainConfigLogic) CreateChainConfig(req *types.CreateChainConfigRequest) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	role := middleware.GetUserRoleFromContext(l.ctx)
	if role != store.UserRoleAdmin {
		return &types.CommonResponse{Code: 403, Message: "admin role required for write operations"}, nil
	}

	if req.ChainName == "" || req.ChainType == "" {
		return &types.CommonResponse{Code: 400, Message: "chain_name and chain_type are required"}, nil
	}

	if err := validator.ValidateChainType(req.ChainType); err != nil {
		return &types.CommonResponse{Code: 400, Message: err.Error()}, nil
	}

	// 构建旧格式请求体用于校验
	oldReq := toOldChainConfigRequest(req)
	if err := validator.ValidateChainConfig(oldReq); err != nil {
		return &types.CommonResponse{Code: 400, Message: err.Error()}, nil
	}

	// 校验链名称唯一性
	unique, err := l.svcCtx.Repo.CheckChainNameUnique(l.ctx, tenantID, req.ChainName, 0)
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "check chain name: " + err.Error()}, nil
	}
	if !unique {
		return &types.CommonResponse{Code: 409, Message: "chain_name already exists for this tenant"}, nil
	}

	// 构建链配置模型
	config := buildChainConfigModel(tenantID, req)

	if err := l.svcCtx.Repo.CreateChainConfig(l.ctx, config); err != nil {
		return &types.CommonResponse{Code: 500, Message: "create chain config: " + err.Error()}, nil
	}

	// 创建节点配置
	if len(req.Nodes) > 0 {
		nodes := buildChainNodesModel(config.ID, req.Nodes)
		if err := l.svcCtx.Repo.CreateChainNodes(l.ctx, nodes); err != nil {
			return &types.CommonResponse{Code: 500, Message: "create chain nodes: " + err.Error()}, nil
		}
	}

	return &types.CommonResponse{Code: 0, Message: "success", Data: config}, nil
}

// toOldChainConfigRequest 转换为旧格式请求体（用于复用 validator）
func toOldChainConfigRequest(req *types.CreateChainConfigRequest) *validator.CreateChainConfigRequestBody {
	oldReq := &validator.CreateChainConfigRequestBody{
		ChainName:       req.ChainName,
		ChainType:       req.ChainType,
		Enable:          req.Enable,
		ChainId:         req.ChainId,
		AuthType:        req.AuthType,
		OrgId:           req.OrgId,
		HashType:        req.HashType,
		SignKey:         req.SignKey,
		SignCert:        req.SignCert,
		UserTlsKey:      req.UserTlsKey,
		UserTlsCert:     req.UserTlsCert,
		UserEncKey:      req.UserEncKey,
		UserEncCert:     req.UserEncCert,
		ProxyUrl:        req.ProxyUrl,
		EthChainId:      req.EthChainId,
		HttpUrl:         req.HttpUrl,
		WebsocketUrl:    req.WebsocketUrl,
		PrivateKey:      req.PrivateKey,
		GasLimit:        req.GasLimit,
		SolRpcUrl:       req.SolRpcUrl,
		SolPrivateKey:   req.SolPrivateKey,
		CommitmentLevel: req.CommitmentLevel,
		SkipPreflight:   req.SkipPreflight,
		MaxRetries:      req.MaxRetries,
	}
	for _, n := range req.Nodes {
		oldReq.Nodes = append(oldReq.Nodes, validator.NodeRequest{
			NodeAddr:    n.NodeAddr,
			ConnCnt:     n.ConnCnt,
			EnableTls:   n.EnableTls,
			TlsHostName: n.TlsHostName,
			CaCert:      n.CaCert,
		})
	}
	return oldReq
}

// buildChainConfigModel 从请求体构建链配置模型
func buildChainConfigModel(tenantID uint, req *types.CreateChainConfigRequest) *store.TenantChainConfig {
	config := &store.TenantChainConfig{
		TenantID:  tenantID,
		ChainName: req.ChainName,
		ChainType: strings.ToLower(req.ChainType),
		Enable:    req.Enable,
	}

	switch strings.ToLower(req.ChainType) {
	case "chainmaker":
		config.ChainId = req.ChainId
		config.AuthType = req.AuthType
		config.OrgId = req.OrgId
		config.HashType = req.HashType
		config.SignKey = req.SignKey
		config.SignCert = req.SignCert
		config.UserTlsKey = req.UserTlsKey
		config.UserTlsCert = req.UserTlsCert
		config.UserEncKey = req.UserEncKey
		config.UserEncCert = req.UserEncCert
		config.ProxyUrl = req.ProxyUrl
	case "ethereum":
		config.EthChainId = req.EthChainId
		config.HttpUrl = req.HttpUrl
		config.WebsocketUrl = req.WebsocketUrl
		config.PrivateKey = req.PrivateKey
		config.GasLimit = req.GasLimit
	case "solana":
		config.SolRpcUrl = req.SolRpcUrl
		config.SolPrivateKey = req.SolPrivateKey
		config.CommitmentLevel = req.CommitmentLevel
		config.SkipPreflight = req.SkipPreflight
		config.MaxRetries = req.MaxRetries
	}

	return config
}

// buildChainNodesModel 从请求体构建节点配置列表
func buildChainNodesModel(chainConfigID uint, nodeReqs []types.NodeConfig) []*store.TenantChainNode {
	nodes := make([]*store.TenantChainNode, 0, len(nodeReqs))
	for _, n := range nodeReqs {
		connCnt := n.ConnCnt
		if connCnt <= 0 {
			connCnt = 10
		}
		nodes = append(nodes, &store.TenantChainNode{
			ChainConfigID: chainConfigID,
			NodeAddr:      n.NodeAddr,
			ConnCnt:       connCnt,
			EnableTls:     n.EnableTls,
			TlsHostName:   n.TlsHostName,
			CaCert:        n.CaCert,
		})
	}
	return nodes
}
