package main

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/gowins/dionysus/grpc/registry"
	"github.com/gowins/dionysus/grpc/registry/nacos"
	"github.com/gowins/dionysus/step"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/zeromicro/go-zero/core/netx"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/gowins/dionysus"
	"github.com/gowins/dionysus/cmd"
	"github.com/gowins/dionysus/example/grpc/hw"
	"github.com/gowins/dionysus/grpc/server"
	"github.com/gowins/dionysus/grpc/serverinterceptors"
	"google.golang.org/grpc/metadata"
)

type gserver struct {
	hw.UnimplementedGreeterServer
}

// SayHello implements helloworld.GreeterServer
func (s *gserver) SayHello(ctx context.Context, in *hw.HelloRequest) (*hw.HelloReply, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	fmt.Printf("MetaData: %#v \n", md)
	log.Printf("Received: %v", in.GetName())
	return &hw.HelloReply{Message: "Hello " + in.GetName()}, nil
}

var nacosNamingClient naming_client.INamingClient

func main() {
	dio := dionysus.NewDio()
	cfg := server.DefaultCfg
	cfg.Address = ":8081"
	serviceID := uuid.New().String()
	c := cmd.NewGrpcCmd(cmd.WithCfg(cfg))
	c.EnableDebug()
	// timeout interceptor
	c.AddUnaryServerInterceptors(serverinterceptors.TimeoutUnary(10 * time.Second))
	// recover interceptor
	c.AddUnaryServerInterceptors(serverinterceptors.RecoveryUnary(serverinterceptors.DefaultRecovery()))
	c.AddStreamServerInterceptors(serverinterceptors.RecoveryStream(serverinterceptors.DefaultRecovery()))
	// tacing interceptor
	c.AddUnaryServerInterceptors(serverinterceptors.OpenTracingUnary())
	c.AddStreamServerInterceptors(serverinterceptors.OpenTracingStream())

	// register grpc service
	c.RegisterGrpcService(hw.RegisterGreeterServer, &gserver{})
	preSteps := []step.InstanceStep{
		{
			StepName: "register service to regtriy",
			Func: func() error {

				err := registry.Init("nacos://172.16.8.129:8848?secure=false&timeout=30s", nacos.WithClientConfig(&constant.ClientConfig{
					NamespaceId:         "public", // 如果需要支持多namespace，我们可以创建多个client,它们有不同的NamespaceId。当namespace是public时，此处填空字符串。
					TimeoutMs:           5000,
					NotLoadCacheAtStart: true,
					LogDir:              "/tmp/nacos/log",
					CacheDir:            "/tmp/nacos/cache",
					LogLevel:            "debug",
					AppName:             "dionysus",
				}))

				if err != nil {
					panic(err)
				}

				registryClient := registry.Get(nacos.Name)
				_, port, _ := net.SplitHostPort(cfg.Address)
				p, _ := strconv.Atoi(port)
				err = registryClient.Register(&registry.Service{
					Name:    "helloworld-rpc",
					Version: "1.0.0",
					Metadata: map[string]string{
						"kind":    "grpc",
						"version": "",
					},
					Nodes: []*registry.Node{
						{
							Id: serviceID,
							Metadata: map[string]string{
								"kind":    "grpc",
								"version": "",
							},
							Address: netx.InternalIp(),
							Port:    p,
						},
					},
				})
				if err != nil {
					panic(err)
				}
				return nil
			},
		},
	}

	postSteps := []step.InstanceStep{
		{
			StepName: "deregister service from registry",
			Func: func() error {
				err := registry.Init("nacos://127.0.0.1:8848?secure=false&timeout=30s", nacos.WithClientConfig(&constant.ClientConfig{
					NamespaceId:         "public", // 如果需要支持多namespace，我们可以创建多个client,它们有不同的NamespaceId。当namespace是public时，此处填空字符串。
					TimeoutMs:           5000,
					NotLoadCacheAtStart: true,
					LogDir:              "/tmp/nacos/log",
					CacheDir:            "/tmp/nacos/cache",
					LogLevel:            "debug",
					AppName:             "dionysus",
				}))

				if err != nil {
					panic(err)
				}

				registryClient := registry.Get(nacos.Name)
				_, port, _ := net.SplitHostPort(cfg.Address)
				p, _ := strconv.Atoi(port)
				err = registryClient.Deregister(&registry.Service{
					Name: "helloworld-rpc",
					Nodes: []*registry.Node{
						{
							Id:      serviceID,
							Address: netx.InternalIp(),
							Port:    p,
						},
					},
				})
				if err != nil {
					return err
				}
				return nil
			},
		},
	}
	dio.PreRunStepsAppend(preSteps...)
	dio.PostRunStepsAppend(postSteps...)
	_ = postSteps
	dio.DioStart("grpc", c)
}

func SetupNacosNamingClient() (naming_client.INamingClient, error) {
	clientConfig := constant.ClientConfig{
		NamespaceId:         "e525eafa-f7d7-4029-83d9-008937f9d468", // 如果需要支持多namespace，我们可以创建多个client,它们有不同的NamespaceId。当namespace是public时，此处填空字符串。
		TimeoutMs:           5000,
		NotLoadCacheAtStart: true,
		LogDir:              "/tmp/nacos/log",
		CacheDir:            "/tmp/nacos/cache",
		LogLevel:            "debug",
		AppName:             "dionysus",
	}
	serverConfigs := []constant.ServerConfig{
		*constant.NewServerConfig("mse-af860a50-nacos-ans.mse.aliyuncs.com", 8848),
	}
	return clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  &clientConfig,
			ServerConfigs: serverConfigs,
		},
	)
}
