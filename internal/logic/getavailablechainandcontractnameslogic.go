package logic

import (
	"context"
	"strings"

	"github.com/jackz-jones/blockchain-interactive-service/internal/code"
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

// GetAvailableChainAndContractNames 获取本地可访问的所有链名称，以及旗下的合约名称
func (l *GetAvailableChainAndContractNamesLogic) GetAvailableChainAndContractNames(
	in *pb.GetAvailableChainAndContractNamesRequest) (*pb.GetAvailableChainAndContractNamesResponse, error) {

	// 日志通用信息
	fields := map[string]interface{}{
		"requestId": in.RequestId,
	}
	l.Logger.WithFields(util.ConvertToLogFields(fields)...).Info("receive GetAvailableChainAndContractNames request")

	// 收集本地配置可用的链和合约名称
	chainAndContractNames := make([]*pb.ChainAndContractName, 0)
	for chainName, conf := range l.svcCtx.Config.ChainConfs {
		if conf.Enable {

			// 解析链类型
			var chainType pb.ChainType
			switch strings.ToLower(conf.ChainType) {
			case strings.ToLower(pb.ChainType_Ethereum.String()):
				chainType = pb.ChainType_Ethereum
			case strings.ToLower(pb.ChainType_Chainmaker.String()):
				chainType = pb.ChainType_Chainmaker
			default:
				fields["chainType"] = conf.ChainType
				l.Logger.WithFields(util.ConvertToLogFields(fields)...).Error(code.ErrUnknownChainType.String())
				return l.errorResponse(code.ErrUnknownChainType, nil), nil
			}

			contractDescs := make([]*pb.ContractDesc, 0)

			// 收集合约配置名称
			for contractName, contractConf := range conf.ContractConfs {

				// 解析合约类型
				var ct pb.ContractType
				switch strings.ToLower(contractConf.ContractType) {
				case strings.ToLower(pb.ContractType_Notification.String()):
					ct = pb.ContractType_Notification
				case strings.ToLower(pb.ContractType_Nft.String()):
					ct = pb.ContractType_Nft
				default:
					fields["contractType"] = contractConf.ContractType
					l.Logger.WithFields(util.ConvertToLogFields(fields)...).Error(code.ErrUnknownContractType.String())
					return l.errorResponse(code.ErrUnknownContractType, nil), nil
				}

				// 只要以太坊链才需要解析 abi
				var (
					abi string
					err error
				)
				if chainType == pb.ChainType_Ethereum {
					abi, err = util.ReadAbiJsonFile(contractConf.Abi)
					if err != nil {
						fields["abi"] = contractConf.Abi
						fields["error"] = err
						l.Logger.WithFields(util.ConvertToLogFields(fields)...).Error(code.ErrReadAbiJsonFile.String())
						return l.errorResponse(code.ErrReadAbiJsonFile, nil), nil
					}

				}

				// 收集合约配置名称
				contractDescs = append(contractDescs, &pb.ContractDesc{
					ContractName:    contractName,
					ContractType:    ct,
					ContractAddress: contractConf.ContractAddr,
					Abi:             abi,
				})
			}

			// 收集合约配置名称
			chainAndContractNames = append(chainAndContractNames, &pb.ChainAndContractName{
				ChainName:     chainName,
				ChainType:     chainType,
				ContractDescs: contractDescs,
			})
		}
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
