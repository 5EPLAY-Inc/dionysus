package pool

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

var (
	// ErrClosed is the error when the client pool is closed
	ErrClosed = errors.New("grpc pool: client pool is closed")
	// ErrTimeout is the error when the client pool timed out
	ErrTimeout = errors.New("grpc pool: client pool timed out")
	// ErrAlreadyClosed is the error when the client conn was already closed
	ErrAlreadyClosed = errors.New("grpc pool: the connection was already closed")
	// ErrFullPool is the error when the pool is already full
	ErrFullPool = errors.New("grpc pool: closing a ClientConn into a full pool")
)

type GrpcPool struct {
	target               string
	maxIdle              int64
	maxActive            int64
	maxConcurrentStreams int64
	dialOptions          []grpc.DialOption
	conns                chan *GrpcConn
	factory              func(string, ...grpc.DialOption) (*grpc.ClientConn, error)
	Locker               sync.RWMutex
	isClosed             bool
}

type GrpcConn struct {
	conn     *grpc.ClientConn
	gp       *GrpcPool
	inflight int64
}

// Close todo with locker
func (gc *GrpcConn) Close() error {
	if gc == nil {
		return nil
	}

	gc.gp.Locker.Lock()
	defer gc.gp.Locker.Unlock()

	if gc.conn == nil {
		return ErrAlreadyClosed
	}
	if gc.gp.IsClosed() {
		return ErrClosed
	}

	select {
	case gc.gp.conns <- gc:
		return nil
	default:

		return ErrFullPool
	}
}

func New(target string, opts ...Options) (*GrpcPool, error) {
	if target == "" {
		return nil, fmt.Errorf("grpc pool target should not be nil")
	}
	gp := &GrpcPool{
		target:               target,
		maxIdle:              DefaultMaxIdle,
		maxActive:            DefaultMaxActive,
		maxConcurrentStreams: DefaultMaxConcurrent,
		dialOptions:          DefaultDialOpts,
		factory:              grpcDialWithTimeout,
	}

	for _, opt := range opts {
		opt(gp)
	}
	if gp.maxIdle > gp.maxActive {
		return nil, fmt.Errorf("grpc pool gp.maxIdle > gp.maxActive")
	}
	if gp.maxIdle == 0 || gp.maxActive == 0 || gp.maxConcurrentStreams == 0 {
		return nil, fmt.Errorf("grpc pool gp.maxIdle == 0 || gp.maxActive == 0  || gp.maxConcurrentStreams == 0")
	}

	gp.conns = make(chan *GrpcConn, gp.maxActive)

	for i := 0; i < int(gp.maxIdle); i++ {
		conn, err := gp.factory(target, gp.dialOptions...)
		if err != nil {
			return gp, fmt.Errorf("grpc dial target %v error %v", gp.target, err)
		}
		gp.conns <- &GrpcConn{
			conn:     conn,
			gp:       gp,
			inflight: 0,
		}
	}

	for i := 0; i < int(gp.maxActive-gp.maxIdle); i++ {
		gp.conns <- &GrpcConn{
			gp:       gp,
			inflight: 0,
		}
	}

	return gp, nil
}

func grpcDialWithTimeout(target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultDialTimeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, target, opts...)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (gp *GrpcPool) Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	grpcConn, err := gp.pickLeastConn()
	if err != nil {
		return err
	}
	atomic.AddInt64(&grpcConn.inflight, 1)
	defer func() {
		atomic.AddInt64(&grpcConn.inflight, -1)
		_ = grpcConn.Close()
	}()
	return grpcConn.conn.Invoke(ctx, method, args, reply, opts...)
}

func (gp *GrpcPool) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	grpcConn, err := gp.pickLeastConn()
	if err != nil {
		return nil, err
	}

	atomic.AddInt64(&grpcConn.inflight, 1)
	defer func() {
		atomic.AddInt64(&grpcConn.inflight, -1)
		grpcConn.Close()
	}()
	return grpcConn.conn.NewStream(ctx, desc, method, opts...)
}

func (gp *GrpcPool) pickLeastConn() (*GrpcConn, error) {
	var err error
	for {
		select {
		case c, ok := <-gp.conns:
			if !ok || c == nil {
				// 处理通道被关闭的情况，或者接收到零值的情况
				return nil, errors.New("无法从连接池中获取连接")
			}
			if c.conn == nil || c.conn.GetState() != connectivity.Idle ||
				c.conn.GetState() != connectivity.Ready {
				c.conn, err = gp.factory(gp.target, gp.dialOptions...)
				if err != nil {
					c.conn = nil
					_ = c.Close()
					return nil, err
				}
			}

			if atomic.CompareAndSwapInt64(&c.inflight, gp.maxConcurrentStreams, c.inflight) {
				return nil, fmt.Errorf("conn's flight already equal maxConcurrentStreams %d", gp.maxConcurrentStreams)
			}
			return c, nil
		case <-time.After(100 * time.Microsecond):
			return nil, errors.New("without useful conn")
		}
	}

}

func (gp *GrpcPool) Closed() {
	//todo locker with get conns
	gp.Locker.Lock()
	defer gp.Locker.Unlock()
	gp.isClosed = true
	close(gp.conns)
	for i := 0; i < int(gp.maxActive); i++ {
		c := <-gp.conns
		_ = c.conn.Close()
	}
}

func (gp *GrpcPool) IsClosed() bool {
	gp.Locker.Lock()
	defer gp.Locker.Unlock()
	return gp.isClosed || gp == nil
}
