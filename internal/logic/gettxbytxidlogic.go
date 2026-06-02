package logic

import (
	"context"
	"fmt"

	"github.com/jackz-jones/blockchain-interactive-service/internal/code"
	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/sdk"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/util"
	pb "github.com/jackz-jones/blockchain-interactive-service/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

// GetTxByTxIdLogic 定义了查询交易详情逻辑执行对象
type GetTxByTxIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// NewGetTxByTxIdLogic 初始化查询交易详情逻辑执行对象
func NewGetTxByTxIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTxByTxIdLogic {
	return &GetTxByTxIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetTxByTxId 查询交易详情
func (l *GetTxByTxIdLogic) GetTxByTxId(in *pb.GetTxByTxIdRequest) (*pb.TxResponse, error) {

	// 日志通用信息
	fields := map[string]interface{}{
		"requestId": in.RequestId,
		"txId":      in.TxId,
		"chainName": in.ChainName,
	}
	l.Logger.WithFields(util.ConvertToLogFields(fields)...).Info("receive GetTxByTxId request")

	// 获取 SDK 客户端：DB 优先，配置文件回退
	sdkClient, err := l.getSDKClient(in.ChainName, fields)
	if err != nil {
		return l.errorResponse(code.ErrGetSDKClient, err), nil
	}

	// 查询交易详情
	txData, pending, err := sdkClient.GetTxByTxId(in.TxId)
	if err != nil {
		fields["err"] = err
		l.Logger.WithFields(util.ConvertToLogFields(fields)...).Error(code.ErrGetTxByTxId.String())
		return l.errorResponse(code.ErrGetTxByTxId, err), nil
	}

	// 返回成功信息
	l.Logger.WithFields(util.ConvertToLogFields(fields)...).Info("GetTxByTxId success return")
	return l.successResponse(&pb.TxData{
		ChainName: in.ChainName,
		Content:   txData,
		Pending:   pending,
	}), nil
}

// getSDKClient 获取 SDK 客户端，支持 DB 优先查找，配置文件回退
// 1. 如果 context 中有租户身份（tenantID > 0），先尝试从 DB（TenantSDKManager）查找
// 2. DB 查找失败则回退到配置文件路径（sdkClients sync.Map）
// 3. 未提供租户身份时仅从配置文件查找（向后兼容）
func (l *GetTxByTxIdLogic) getSDKClient(
	chainName string, fields map[string]interface{},
) (sdk.ChainSdkInterface, error) {
	// 尝试从 context 获取租户 ID（由 gRPC auth interceptor 注入）
	tenantID := middleware.GetTenantID(l.ctx)

	// 如果有租户身份，优先从 DB 查找（DB 配置优先级高于配置文件）
	if tenantID > 0 {
		client, err := l.svcCtx.TenantSDKManager.GetTenantSDKClient(l.ctx, tenantID, chainName)
		if err == nil {
			l.Logger.WithFields(util.ConvertToLogFields(fields)...).
				Infof("got SDK client from DB: tenant=%d, chain=%s", tenantID, chainName)
			return client, nil
		}
		// DB 查找失败，记录日志后回退到配置文件
		l.Logger.WithFields(util.ConvertToLogFields(fields)...).
			Infof("DB lookup failed (tenant=%d, chain=%s): %v, fallback to config file",
				tenantID, chainName, err)
	}

	// 配置文件路径：检查 chainConf 是否存在
	chainConf, exist := l.svcCtx.Config.ChainConfs[chainName]
	if !exist {
		l.Logger.WithFields(util.ConvertToLogFields(fields)...).Error(code.ErrChainNotExist.String())
		return nil, fmt.Errorf("%s", code.ErrChainNotExist.String())
	}

	// 如果链未启用，直接返回错误
	if !chainConf.Enable {
		l.Logger.WithFields(util.ConvertToLogFields(fields)...).Error(code.ErrChainNotEnable.String())
		return nil, fmt.Errorf("%s", code.ErrChainNotEnable.String())
	}

	// 从配置文件获取 SDK 客户端
	sdkClient, err := sdk.GetSDKClient(l.svcCtx.RootCtx, &l.svcCtx.SDKClients, chainName, l.Logger, chainConf,
		l.svcCtx.Config.Log, l.svcCtx.RedisClient, l.svcCtx.ChainClientFactory)
	if err != nil {
		fields["err"] = err
		l.Logger.WithFields(util.ConvertToLogFields(fields)...).Error(code.ErrGetSDKClient.String())
		return nil, err
	}

	return sdkClient, nil
}

// errorResponse returns the error response.
func (l *GetTxByTxIdLogic) errorResponse(code code.RespCode, err error) *pb.TxResponse {
	msg := code.String()
	if err != nil {
		msg = err.Error()
	}

	return &pb.TxResponse{
		Code: int32(code),
		Msg:  msg,
	}
}

// successResponse returns the success response.
func (l *GetTxByTxIdLogic) successResponse(data *pb.TxData) *pb.TxResponse {
	return &pb.TxResponse{
		Code: int32(code.Success),
		Msg:  code.Success.String(),
		Data: data,
	}
}
