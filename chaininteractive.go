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

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
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

	// 验证配置是否合法
	if err := c.Validate(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	ctx := svc.NewServiceContext(c)

	// 创建认证和权限拦截器
	authInterceptor := middleware.NewAuthInterceptor(ctx.Repo)
	rbacInterceptor := middleware.NewRBACInterceptor()
	quotaInterceptor := middleware.NewQuotaInterceptor(ctx.BillingService)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterChainInteractiveServer(grpcServer, server.NewChainInteractiveServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	// 注册 gRPC 拦截器
	s.AddUnaryInterceptors(authInterceptor.Unary())
	s.AddUnaryInterceptors(rbacInterceptor.Unary())
	s.AddUnaryInterceptors(quotaInterceptor.Unary())

	// 启动 HTTP API Gateway
	gateway.StartHTTPServer(c, ctx)

	// 启动订阅（传入服务级根 ctx，便于统一优雅退出）
	sdk.StartSubscribe(ctx.RootCtx, c, &ctx.SDKClients, ctx.Logger, ctx.RedisClient)

	// 服务退出前释放所有的 sdk client
	defer func() {
		// 先取消根 ctx，通知订阅 goroutine 等退出；然后并发调用各 SDK 的 Stop
		ctx.Cancel()
		sdk.StopAllSdkClients(&ctx.SDKClients, ctx.Logger)
	}()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	ctx.Logger.Infof("Starting rpc server at %s", c.ListenOn)
	s.Start()
}
