package grpc

import (
	"context"
	"strings"

	"github.com/jackz-jones/blockchain-interactive-service/internal/code"
	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/util"
	pb "github.com/jackz-jones/blockchain-interactive-service/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

// GetAvailableChainAndContractNamesLogic 定义了获取本地可访问的所有链名称，以及旗下的合约名称逻辑执行对象
type GetAvailableChainAndContractNamesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// NewGetAvailableChainAndContractNamesLogic 初始化获取本地可访问的所有链名称，以及旗下的合约名称逻辑执行对象
func NewGetAvailableChainAndContractNamesLogic(ctx context.Context,
	svcCtx *svc.ServiceContext) *GetAvailableChainAndContractNamesLogic {
	return &GetAvailableChainAndContractNamesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// chainTypeMapping 链类型字符串到 pb 枚举的映射
var chainTypeMapping = map[string]pb.ChainType{
	"ethereum":   pb.ChainType_Ethereum,
	"chainmaker": pb.ChainType_Chainmaker,
	"solana":     pb.ChainType_Solana,
}

// GetAvailableChainAndContractNames 获取当前租户可访问的所有链名称，以及旗下的合约名称
func (l *GetAvailableChainAndContractNamesLogic) GetAvailableChainAndContractNames(
	in *pb.GetAvailableChainAndContractNamesRequest) (*pb.GetAvailableChainAndContractNamesResponse, error) {

	// 日志通用信息
	fields := map[string]interface{}{
		"requestId": in.RequestId,
	}
	l.Logger.WithFields(util.ConvertToLogFields(fields)...).Info("receive GetAvailableChainAndContractNames request")

	// 获取租户 ID
	tenantID := middleware.GetTenantID(l.ctx)
	if tenantID == 0 {
		return l.errorResponse(code.ErrGetSDKClient, nil), nil
	}

	// 从 DB 查询租户的链配置
	chainConfigs, err := l.svcCtx.Repo.ListChainConfigsByTenant(l.ctx, tenantID)
	if err != nil {
		l.Logger.WithFields(util.ConvertToLogFields(fields)...).Errorf("list chain configs: %v", err)
		return l.errorResponse(code.ErrGetSDKClient, err), nil
	}

	// 收集链和合约名称
	chainAndContractNames := make([]*pb.ChainAndContractName, 0)
	for _, chainConf := range chainConfigs {
		if !chainConf.Enable {
			continue
		}

		// 解析链类型
		chainType, ok := chainTypeMapping[strings.ToLower(chainConf.ChainType)]
		if !ok {
			fields["chainType"] = chainConf.ChainType
			l.Logger.WithFields(util.ConvertToLogFields(fields)...).Error(code.ErrUnknownChainType.String())
			continue
		}

		// 查询该链下的合约配置
		contractConfigs, contractErr := l.svcCtx.Repo.ListContractConfigsByChain(l.ctx, chainConf.ID)
		if contractErr != nil {
			l.Logger.WithFields(util.ConvertToLogFields(fields)...).
				Errorf("list contract configs for chain %s: %v", chainConf.ChainName, contractErr)
			continue
		}

		contractDescs := make([]*pb.ContractDesc, 0)
		for _, cc := range contractConfigs {
			contractDescs = append(contractDescs, &pb.ContractDesc{
				ContractName:    cc.ContractName,
				ContractAddress: cc.ContractAddr,
				Abi:             cc.AbiJSON,
			})
		}

		chainAndContractNames = append(chainAndContractNames, &pb.ChainAndContractName{
			ChainName:     chainConf.ChainName,
			ChainType:     chainType,
			ContractDescs: contractDescs,
		})
	}

	// 返回成功信息
	l.Logger.WithFields(util.ConvertToLogFields(fields)...).Info("GetAvailableChainAndContractNames success return")
	return l.successResponse(chainAndContractNames), nil
}

// errorResponse returns the error response.
func (l *GetAvailableChainAndContractNamesLogic) errorResponse(code code.RespCode,
	err error) *pb.GetAvailableChainAndContractNamesResponse {
	msg := code.String()
	if err != nil {
		msg = err.Error()
	}

	return &pb.GetAvailableChainAndContractNamesResponse{
		Code: int32(code),
		Msg:  msg,
	}
}

// successResponse returns the success response.
func (l *GetAvailableChainAndContractNamesLogic) successResponse(
	data []*pb.ChainAndContractName) *pb.GetAvailableChainAndContractNamesResponse {
	return &pb.GetAvailableChainAndContractNamesResponse{
		Code: int32(code.Success),
		Msg:  code.Success.String(),
		Data: data,
	}
}
