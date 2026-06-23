package chainconfig

import (
	"context"
	"strings"

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

func (l *TestChainConnectionLogic) TestChainConnection(req *types.TestChainConnectionFormRequest) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
	}

	role := middleware.GetUserRoleFromContext(l.ctx)
	if role != store.UserRoleAdmin {
		return &types.CommonResponse{Code: 403, Message: "admin role required"}, nil
	}

	// 从数据库获取基础配置（用于权限校验和获取 ID）
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

	// 用前端表单值覆盖数据库配置，以便测试用户实际填写的内容
	testConfig := overlayFormValues(chainConfig, &req.CreateChainConfigRequest)

	// 测试连接仅检测节点连通性，不受链配置启用/禁用状态影响
	// 因此不通过 GetTenantSDKClient（它有 enable 检查），而是直接创建临时客户端测试
	nodes, err := l.svcCtx.Repo.ListChainNodesByConfigID(l.ctx, chainConfig.ID)
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "query chain nodes: " + err.Error()}, nil
	}

	// 如果前端表单提交了节点配置，使用表单中的节点
	testNodes := nodes
	if len(req.Nodes) > 0 {
		testNodes = buildTestNodes(chainConfig.ID, req.Nodes)
	}

	sdkConf := sdk.BuildSDKConf(testConfig, testNodes)
	chainConf := &sdk.ChainConf{
		ChainType: testConfig.ChainType,
		SDKConf:   sdkConf,
	}

	client, testErr := l.svcCtx.ChainClientFactory(l.ctx, testConfig.ChainName, testConfig.ChainType, chainConf, l.svcCtx.Config.Log, l.svcCtx.RedisClient)
	if client != nil {
		_ = client.Stop()
	}

	result := map[string]interface{}{
		"chain_name": testConfig.ChainName,
		"chain_type": testConfig.ChainType,
		"success":    testErr == nil,
	}
	if testErr != nil {
		result["error"] = testErr.Error()
	}

	return &types.CommonResponse{Code: 0, Message: "success", Data: result}, nil
}

// overlayFormValues 用前端表单值覆盖数据库中的链配置
func overlayFormValues(dbConfig *store.TenantChainConfig, form *types.CreateChainConfigRequest) *store.TenantChainConfig {
	// 基于数据库配置做副本
	cfg := *dbConfig

	// 覆盖基础字段
	if form.ChainName != "" {
		cfg.ChainName = form.ChainName
	}
	if form.ChainType != "" {
		cfg.ChainType = strings.ToLower(form.ChainType)
	}
	cfg.Enable = form.Enable

	// 根据链类型覆盖对应字段
	switch strings.ToLower(form.ChainType) {
	case "chainmaker":
		cfg.ChainId = form.ChainId
		cfg.AuthType = form.AuthType
		cfg.OrgId = form.OrgId
		cfg.HashType = form.HashType
		cfg.SignKey = form.SignKey
		cfg.SignCert = form.SignCert
		cfg.UserTlsKey = form.UserTlsKey
		cfg.UserTlsCert = form.UserTlsCert
		cfg.UserEncKey = form.UserEncKey
		cfg.UserEncCert = form.UserEncCert
		cfg.ProxyUrl = form.ProxyUrl
	case "ethereum":
		cfg.EthChainId = form.EthChainId
		cfg.HttpUrl = form.HttpUrl
		cfg.WebsocketUrl = form.WebsocketUrl
		cfg.PrivateKey = form.PrivateKey
	case "solana":
		cfg.SolRpcUrl = form.SolRpcUrl
		cfg.SolPrivateKey = form.SolPrivateKey
		cfg.CommitmentLevel = form.CommitmentLevel
		cfg.SkipPreflight = form.SkipPreflight
		cfg.MaxRetries = form.MaxRetries
	}

	return &cfg
}

// buildTestNodes 从前端表单节点配置构建测试节点列表
func buildTestNodes(chainConfigID uint, nodeReqs []types.NodeConfig) []*store.TenantChainNode {
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
