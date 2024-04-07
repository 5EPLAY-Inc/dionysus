package pool

import (
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

type Options func(gp *GrpcPool)

func WithMaxIdle(maxIdle int64) Options {
	return func(pool *GrpcPool) {
		pool.maxIdle = maxIdle
	}
}

func WithMaxActive(maxActive int64) Options {
	return func(pool *GrpcPool) {
		pool.maxActive = maxActive
	}
}
func WithMaxConcurrentStreams(maxConcurrentStreams int64) Options {
	return func(pool *GrpcPool) {
		pool.maxConcurrentStreams = maxConcurrentStreams
	}
}

func WithDialOptions(dialOptions []grpc.DialOption) Options {
	return func(pool *GrpcPool) {
		pool.dialOptions = dialOptions
	}
}

const (
	// KeepAliveTime is the duration of time after which if the client doesn't see
	// any activity it pings the server to see if the transport is still alive.
	KeepAliveTime = time.Duration(10) * time.Second

	// KeepAliveTimeout is the duration of time for which the client waits after having
	// pinged for keepalive check and if no activity is seen even after that the connection
	// is closed.
	KeepAliveTimeout = time.Duration(3) * time.Second

	// InitialWindowSize we set it 256M is to provide system's throughput.
	InitialWindowSize = 1 << 28

	// InitialConnWindowSize we set it 256M is to provide system's throughput.
	InitialConnWindowSize = 1 << 28

	// MaxSendMsgSize set max gRPC request message poolSize sent to server.
	// If any request message poolSize is larger than current value, an error will be reported from gRPC.
	MaxSendMsgSize = 1 << 30

	// MaxRecvMsgSize set max gRPC receive message poolSize received from server.
	// If any message poolSize is larger than current value, an error will be reported from gRPC.
	MaxRecvMsgSize = 1 << 30

	DefaultDialTimeout = 1 * time.Second

	DefaultMaxIdle = 6

	DefaultMaxActive = 10

	DefaultMaxConcurrent = 16
)

var DefaultDialOpts = []grpc.DialOption{
	grpc.WithTransportCredentials(insecure.NewCredentials()),
	grpc.WithBlock(),
	grpc.WithInitialWindowSize(InitialWindowSize),
	grpc.WithInitialConnWindowSize(InitialConnWindowSize),
	grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(MaxSendMsgSize)),
	grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxRecvMsgSize)),
	grpc.WithKeepaliveParams(keepalive.ClientParameters{
		Time:                KeepAliveTime,
		Timeout:             KeepAliveTimeout,
		PermitWithoutStream: true,
	}),
}
