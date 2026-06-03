package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jackz-jones/blockchain-interactive-service/internal"
	"github.com/jackz-jones/blockchain-interactive-service/internal/config"
	"github.com/jackz-jones/blockchain-interactive-service/internal/handler"
	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/sdk"
	"github.com/jackz-jones/blockchain-interactive-service/internal/server"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	pb "github.com/jackz-jones/blockchain-interactive-service/pb"

	commonGrpc "github.com/jackz-jones/common/grpc"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/proc"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/rest"
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

	// 注册 gRPC 拦截器
	s.AddUnaryInterceptors(authInterceptor.Unary())
	s.AddUnaryInterceptors(rbacInterceptor.Unary())
	s.AddUnaryInterceptors(quotaInterceptor.Unary())

	// 使用 ServiceGroup 统一管理 gRPC 和 HTTP 服务
	group := service.NewServiceGroup()
	// defer 兜底：panic 退出时确保服务被关闭（stopOnce 幂等，信号退出时重复调用无副作用）
	defer group.Stop()

	// 添加 gRPC 服务（先添加，后关闭）
	group.Add(s)

	// 启动订阅（仅基于 DB 配置）
	sdk.StartSubscribe(ctx.RootCtx, ctx.Logger, ctx.TenantSDKManager, ctx.Repo)

	// ========== 注册退出回调 ==========
	// 信号退出（SIGTERM/SIGINT）走 proc 机制：wrapUp → shutdown，有超时强杀保障
	// panic 退出走 defer 机制：按 LIFO 顺序执行兜底释放

	// defer 兜底：panic 时取消根 ctx
	defer ctx.Cancel()
	// defer 兜底：panic 时释放 SDK 客户端
	defer func() {
		if ctx.TenantSDKManager != nil {
			ctx.TenantSDKManager.StopAll()
		}
	}()

	// 信号退出 - wrapUp 阶段：取消根 ctx，通知订阅 goroutine 等退出（在服务关闭之前 1s 执行）
	proc.AddWrapUpListener(func() {
		logx.Info("Wrapping up, cancelling root context")
		ctx.Cancel()
	})

	// 信号退出 - shutdown 阶段：ServiceGroup.Start() 内部已注册 stopOnce（关闭 gRPC 和 HTTP 服务）
	//   以下 listener 用于释放 ServiceGroup 管不到的资源：租户级 SDK 客户端
	proc.AddShutdownListener(func() {
		logx.Info("Shutting down, releasing SDK clients")
		if ctx.TenantSDKManager != nil {
			ctx.TenantSDKManager.StopAll()
		}
	})

	// 启动 HTTP API Gateway
	if c.GatewayConf.Enable {
		httpServer := rest.MustNewServer(rest.RestConf{
			Host: c.GatewayConf.Host,
			Port: c.GatewayConf.Port,
		})
		handler.RegisterHandlers(httpServer, ctx)

		// 添加 HTTP 服务（后添加，先关闭）
		group.Add(httpServer)

		addr := fmt.Sprintf("%s:%d", c.GatewayConf.Host, c.GatewayConf.Port)
		logx.Infof("[Gateway] HTTP API Gateway starting at %s", addr)
		fmt.Printf("Starting HTTP API Gateway at %s...\n", addr)
	} else {
		logx.Info("[Gateway] HTTP gateway is disabled")
	}

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	ctx.Logger.Infof("Starting rpc server at %s", c.ListenOn)

	// Start 阻塞运行，收到退出信号后按逆序 Stop 所有服务
	group.Start()
}
