package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jackz-jones/blockchain-interactive-service/internal"
	"github.com/jackz-jones/blockchain-interactive-service/internal/config"
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
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterChainInteractiveServer(grpcServer, server.NewChainInteractiveServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	// 启动订阅
	sdk.StartSubscribe(c, &ctx.SDKClients, ctx.Logger, ctx.RedisClient)

	// 服务退出前释放所有的 sdk client
	defer func() {
		sdk.StopAllSdkClients(&ctx.SDKClients, ctx.Logger)
	}()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	ctx.Logger.Infof("Starting rpc server at %s", c.ListenOn)
	s.Start()
}
