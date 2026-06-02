package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jackz-jones/blockchain-interactive-service/internal"
	"github.com/jackz-jones/blockchain-interactive-service/internal/config"
	"github.com/jackz-jones/blockchain-interactive-service/internal/gateway"
	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/sdk"
	"github.com/jackz-jones/blockchain-interactive-service/internal/server"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	pb "github.com/jackz-jones/blockchain-interactive-service/pb"

	commonGrpc "github.com/jackz-jones/common/grpc"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/chaininteractive.yaml", "the config file")

func main() {
	flag.Parse()

	// version 选项打印当前版本信息
	args := flag.Args()
	if len(args) > 0 && args[0] == "version" {
		fmt.Println(internal.VersionInfo())
		os.Exit(0)
	}

	var c config.Config
	conf.MustLoad(*configFile, &c)

	ctx := svc.NewServiceContext(c)

	// 创建认证和权限拦截器
	authInterceptor := middleware.NewAuthInterceptor(ctx.Repo)
	rbacInterceptor := middleware.NewRBACInterceptor()
	quotaInterceptor := middleware.NewQuotaInterceptor(ctx.BillingService)

	// 初始化 grpc 服务注册器
	register := func(grpcServer *grpc.Server) {
		pb.RegisterChainInteractiveServer(grpcServer, server.NewChainInteractiveServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	}

	// 创建 grpc 服务
	s, err := commonGrpc.CreateGRPCServer(c.RpcServerConf, register, c.GrpcConf.CaCertFile, c.GrpcConf.ServerCertFile,
		c.GrpcConf.ServerKeyFile, c.GrpcConf.MaxRecvMsgSize, c.GrpcConf.MaxSendMsgSize)
	if err != nil {
		panic(fmt.Errorf("failed to CreateGRPCServer,error: %v", err))
	}

	defer s.Stop()

	// 注册 gRPC 拦截器
	s.AddUnaryInterceptors(authInterceptor.Unary())
	s.AddUnaryInterceptors(rbacInterceptor.Unary())
	s.AddUnaryInterceptors(quotaInterceptor.Unary())

	// 启动 HTTP API Gateway
	gateway.StartHTTPServer(c, ctx)

	// 启动订阅（仅基于 DB 配置）
	sdk.StartSubscribe(ctx.RootCtx, ctx.Logger, ctx.TenantSDKManager, ctx.Repo)

	// 服务退出前释放所有的 sdk client
	defer func() {
		// 先取消根 ctx，通知订阅 goroutine 等退出
		ctx.Cancel()

		// 停止所有租户级 SDK 客户端
		if ctx.TenantSDKManager != nil {
			ctx.TenantSDKManager.StopAll()
		}
	}()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	ctx.Logger.Infof("Starting rpc server at %s", c.ListenOn)
	s.Start()
}
