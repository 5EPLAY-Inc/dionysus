package main

import (
	"context"
	"fmt"
	_ "github.com/gowins/dionysus/grpc/balancer/resolver"
	"github.com/gowins/dionysus/grpc/registry"
	"github.com/gowins/dionysus/grpc/registry/nacos"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"log"
	"sync"
	"time"

	"github.com/gowins/dionysus/example/grpc/hw"
	"github.com/gowins/dionysus/grpc/client/pool"
	xlog "github.com/gowins/dionysus/log"
	"google.golang.org/grpc/metadata"
)

var DefaultMaxRecvMsgSize = 1024 * 1024 * 4

// DefaultMaxSendMsgSize maximum message that client can send
// (4 MB).
var DefaultMaxSendMsgSize = 1024 * 1024 * 4

var clientParameters = keepalive.ClientParameters{
	Time:                10 * time.Second, // send pings every 10 seconds if there is no activity
	Timeout:             2 * time.Second,  // wait 2 second for ping ack before considering the connection dead
	PermitWithoutStream: true,             // send pings even without active streams
}

var defaultDialOpts = []grpc.DialOption{
	grpc.WithTransportCredentials(insecure.NewCredentials()),
	grpc.WithBlock(),
	grpc.WithKeepaliveParams(clientParameters),
	grpc.WithDefaultCallOptions(
		grpc.MaxCallRecvMsgSize(DefaultMaxRecvMsgSize),
		grpc.MaxCallSendMsgSize(DefaultMaxSendMsgSize)),
}

func main() {
	xlog.Setup(xlog.SetProjectName("grpc-client"))
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

	target := "discov://nacos" + "/helloworld-rpc"
	//discov://nacos/helloworld-rpc
	//discov://nacos/helloworld-rpc
	//
	gPool, err := pool.GetGrpcPool(target)
	if err != nil {
		fmt.Printf("grpc pool init dial error %v\n", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 1; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := hw.NewGreeterClient(gPool)
			// Contact the server and print out its response.
			mdCtx := metadata.NewOutgoingContext(context.Background(), metadata.New(map[string]string{"k": "v"}))
			r, err := c.SayHello(mdCtx, &hw.HelloRequest{Name: "nameing"})
			if err != nil {
				log.Printf("could not greet: %v", err)
				return
			}
			log.Printf("Greeting: %s", r.GetMessage())
		}()
	}
	wg.Wait()
	//sigs := make(chan os.Signal, 1)
	//signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	select {}
}
