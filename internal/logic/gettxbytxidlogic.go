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

// getSDKClient 获取 SDK 客户端（从 DB 配置加载）
func (l *GetTxByTxIdLogic) getSDKClient(
	chainName string, fields map[string]interface{},
) (sdk.ChainSdkInterface, error) {
	// 从 context 获取租户 ID（由 gRPC auth interceptor 注入）
	tenantID := middleware.GetTenantID(l.ctx)
	if tenantID == 0 {
		l.Logger.WithFields(util.ConvertToLogFields(fields)...).Error("tenant ID not found in context")
		return nil, fmt.Errorf("tenant identity required")
	}

	client, err := l.svcCtx.TenantSDKManager.GetTenantSDKClient(l.ctx, tenantID, chainName)
	if err != nil {
		fields["err"] = err
		l.Logger.WithFields(util.ConvertToLogFields(fields)...).
			Errorf("failed to get SDK client from DB: tenant=%d, chain=%s, err=%v", tenantID, chainName, err)
		return nil, err
	}

	l.Logger.WithFields(util.ConvertToLogFields(fields)...).
		Infof("got SDK client: tenant=%d, chain=%s", tenantID, chainName)
	return client, nil
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
