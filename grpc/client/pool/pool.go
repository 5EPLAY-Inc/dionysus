package pool

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

type GrpcPool struct {
	conns                 []*GrpcConn
	poolSize              int
	pickRetry             int
	staleEvictionDuration time.Duration
	dialOptions           []grpc.DialOption
	target                string
	rand                  *rand.Rand
	scaleOption           *ScaleOption
	sync.Locker
	stateUpdate sync.Locker
	isClosed    bool
}

type GrpcConn struct {
	conn     *grpc.ClientConn
	inflight int64
}

type GrpcPoolState struct {
	ConnStates  []GrpcConnState
	ReserveSize int
	Target      string
	ScaleOption ScaleOption
	IsClosed    bool
}

type GrpcConnState struct {
	connState string
	inflight  int64
}

type ScaleOption struct {
	Enable          bool
	ScalePeriod     time.Duration
	MaxConn         int
	DesireMaxStream int
}

var DefaultDialOpts = []grpc.DialOption{
	grpc.WithTransportCredentials(insecure.NewCredentials()),
	grpc.WithBlock(),
}

func InitGrpcPool(target string, opts ...Option) (*GrpcPool, error) {
	if target == "" {
		return nil, fmt.Errorf("grpc pool target should not be nil")
	}
	gp := &GrpcPool{
		poolSize:              defaultPoolSize,
		dialOptions:           DefaultDialOpts,
		pickRetry:             defaultPickRetry,
		staleEvictionDuration: defaultStaleEvictionDuration,
		target:                target,
		Locker:                new(sync.Mutex),
		stateUpdate:           new(sync.Mutex),
		rand:                  rand.New(rand.NewSource(time.Now().Unix())),
		scaleOption:           &ScaleOption{Enable: false, MaxConn: defaultPoolSize},
	}

	for _, opt := range opts {
		opt(gp)
	}

	if gp.scaleOption.MaxConn < gp.poolSize {
		gp.scaleOption.MaxConn = gp.poolSize
	}

	gp.conns = make([]*GrpcConn, gp.scaleOption.MaxConn)

	for i := 0; i < gp.poolSize; i++ {
		conn, err := grpcDialWithTimeout(gp.target, gp.dialOptions...)
		if err != nil {
			return gp, fmt.Errorf("grpc dial target %v error %v", gp.target, err)
		}
		gp.conns[i] = &GrpcConn{
			conn:     conn,
			inflight: 0,
		}
	}

	if gp.scaleOption.Enable {
		go gp.autoScalerRun()
	}

	go gp.evict()
	return gp, nil
}

func GetGrpcPool(target string, opts ...Option) (*GrpcPool, error) {
	if val, ok := grpcPool.Load(target); ok {
		return val.(*GrpcPool), nil
	}

	poolInit.Lock()
	defer poolInit.Unlock()

	// 双检, 避免多次创建
	if val, ok := grpcPool.Load(target); ok {
		return val.(*GrpcPool), nil
	}

	gp, err := InitGrpcPool(target, opts...)
	if err != nil {
		return nil, err
	}

	grpcPool.Store(target, gp)
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
		return fmt.Errorf("invoke, pick least conn error %w,method:%s", err, method)
	}
	atomic.AddInt64(&grpcConn.inflight, 1)
	defer atomic.AddInt64(&grpcConn.inflight, -1)
	return grpcConn.conn.Invoke(ctx, method, args, reply, opts...)
}

func (gp *GrpcPool) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	grpcConn, err := gp.pickLeastConn()
	if err != nil {
		return nil, fmt.Errorf("NewStream, pick least conn error %w,method:%s", err, method)
	}
	atomic.AddInt64(&grpcConn.inflight, 1)
	defer atomic.AddInt64(&grpcConn.inflight, -1)
	return grpcConn.conn.NewStream(ctx, desc, method, opts...)
}

// connUsable reports whether a connection can serve a call. Ready and Idle are
// both fine (Idle simply hasn't had traffic yet). Note this is transport-level
// only -- it cannot tell that a Ready conn points at a wrong/reused endpoint;
// that concern is handled resolver-side, see grpc/balancer/resolver.
func connUsable(c *GrpcConn) bool {
	if c == nil || c.conn == nil {
		return false
	}
	switch c.conn.GetState() {
	case connectivity.Ready, connectivity.Idle:
		return true
	default:
		return false
	}
}

// inflightAt returns the in-flight count of the conn at index i, or MaxInt64 for
// an empty slot so it is never chosen as the least-loaded connection.
func (gp *GrpcPool) inflightAt(i int) int64 {
	if c := gp.conns[i]; c != nil {
		return atomic.LoadInt64(&c.inflight)
	}
	return math.MaxInt64
}

func (gp *GrpcPool) pickLeastConn() (*GrpcConn, error) {
	var retryCounter int
Retry:
	retryCounter++
	gp.Lock()
	// snapshot poolSize so concurrent scale/evict cannot cause an out-of-range
	// or divide-by-zero while we index below.
	size := gp.poolSize
	randIndex1 := gp.rand.Uint32()
	randIndex2 := gp.rand.Uint32()
	randIndex3 := gp.rand.Uint32()
	gp.Unlock()

	if size <= 0 {
		return nil, errors.New("pickLeastConn, grpc pool has no available connection")
	}

	// power of three choices: pick the least loaded of three random conns
	minIndex := int(randIndex1) % size
	if gp.inflightAt(int(randIndex2)%size) < gp.inflightAt(minIndex) {
		minIndex = int(randIndex2) % size
	}
	if gp.inflightAt(int(randIndex3)%size) < gp.inflightAt(minIndex) {
		minIndex = int(randIndex3) % size
	}

	// fast path: the least-loaded conn is usable
	if connUsable(gp.conns[minIndex]) {
		return gp.conns[minIndex], nil
	}

	// the chosen conn is not usable; scan for another usable conn
	if retryCounter <= gp.pickRetry {
		for i := 0; i < size; i++ {
			if candidate := gp.conns[(minIndex+i)%size]; connUsable(candidate) {
				return candidate, nil
			}
		}
		goto Retry
	}

	// no usable conn after retries: don't resurrect a closed pool
	if gp.isClosed {
		return nil, errors.New("pickLeastConn, grpc pool is closed")
	}

	// replace the stale slot with a fresh conn
	c, err := gp.newConnection()
	if err != nil {
		log.Errorf("grpc pool pickLeastConn fallback failed, new connection error %v", err)
		return nil, fmt.Errorf("pickLeastConn, new fallback connection error: %w", err)
	}

	gp.stateUpdate.Lock()
	if minIndex < gp.poolSize {
		// avoid leaking the connection we are replacing
		if stale := gp.conns[minIndex]; stale != nil && stale.conn != nil {
			if err := stale.conn.Close(); err != nil {
				log.Errorf("grpc pool pickLeastConn close the stale connection error %v", err)
			}
		}
		gp.conns[minIndex] = c
	}
	gp.stateUpdate.Unlock()

	// best effort: wait briefly for readiness without busy-spinning on an
	// already-expired context (the old code could spin forever here).
	newCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	for c.conn.GetState() != connectivity.Ready {
		if !c.conn.WaitForStateChange(newCtx, c.conn.GetState()) {
			break
		}
	}
	return c, nil
}

func (gp *GrpcPool) autoScalerRun() {
	log.Infof("grpc pool auto scaler start period %v", gp.scaleOption.ScalePeriod)
	tk := time.NewTicker(gp.scaleOption.ScalePeriod)
	for {
		select {
		case <-tk.C:
			totalUse := gp.GetTotalUse()
			if totalUse > gp.poolSize*gp.scaleOption.DesireMaxStream {
				deltaConn := (totalUse - gp.poolSize*gp.scaleOption.DesireMaxStream) / (gp.scaleOption.DesireMaxStream / 2)
				gp.poolScaler(deltaConn)
			}
		}
	}
}

func (gp *GrpcPool) evict() {
	t := time.NewTicker(gp.staleEvictionDuration)
	defer t.Stop()
	for range t.C {
		gp.reapShutdownConns()
	}
}

// reapShutdownConns closes connections that have entered the Shutdown state and
// refills the pool back up to its original size.
//
// Active connections are kept contiguously in conns[0:poolSize]; the slots in
// [poolSize:cap] are reserved (nil) for scaling. The previous implementation
// used len(gp.conns) (== cap == MaxConn) where it meant poolSize, so it ranged
// over nil reserved slots (nil-pointer panic once any conn was Shutdown), swapped
// nil tail entries into live slots, and closed the wrong connection via a defer
// that ran after the swap. This keeps the [0:poolSize] invariant intact.
func (gp *GrpcPool) reapShutdownConns() {
	gp.stateUpdate.Lock()
	defer gp.stateUpdate.Unlock()

	// don't resurrect connections after the pool has been closed
	if gp.isClosed {
		return
	}

	target := gp.poolSize
	for i := 0; i < gp.poolSize; {
		c := gp.conns[i]
		if c != nil && c.conn != nil && c.conn.GetState() != connectivity.Shutdown {
			i++
			continue
		}
		// drop this shutdown/empty conn: close it, then move the last active
		// conn into this slot and shrink the active region. Re-check slot i.
		if c != nil && c.conn != nil {
			if err := c.conn.Close(); err != nil {
				log.Errorf("grpc pool evict close connection %v error %v", i, err)
			}
		}
		last := gp.poolSize - 1
		gp.conns[i] = gp.conns[last]
		gp.conns[last] = nil
		gp.poolSize--
	}

	for gp.poolSize < target {
		c, err := gp.newConnection()
		if err != nil {
			log.Errorf("grpc pool evict restore dial error %v", err)
			break
		}
		gp.conns[gp.poolSize] = c
		gp.poolSize++
	}
}

func (gp *GrpcPool) newConnection() (*GrpcConn, error) {
	conn, err := grpcDialWithTimeout(gp.target, gp.dialOptions...)
	if err != nil {
		log.Errorf("grpc dial target %v error %v", gp.target, err)
		return nil, err
	}
	c := &GrpcConn{
		conn:     conn,
		inflight: 0,
	}
	return c, nil
}

func (gp *GrpcPool) poolScaler(deltaConn int) {
	gp.stateUpdate.Lock()
	defer gp.stateUpdate.Unlock()
	if gp.isClosed {
		log.Infof("grpc pool is closed, will not scale")
		return
	}
	if deltaConn+gp.poolSize > gp.scaleOption.MaxConn {
		deltaConn = gp.scaleOption.MaxConn - gp.poolSize
	}

	if deltaConn <= 0 {
		log.Warnf("grpc conn reach max conn, be careful")
		return
	}

	for i := 0; i < deltaConn; i++ {
		conn, err := grpcDialWithTimeout(gp.target, gp.dialOptions...)
		if err != nil {
			log.Infof("grpc pool is scaler form %v to %v", gp.poolSize, gp.poolSize+i)
			gp.poolSize = gp.poolSize + i
			log.Errorf("grpc dial target %v error %v", gp.target, err)
			return
		}
		gp.conns[gp.poolSize+i] = &GrpcConn{
			conn:     conn,
			inflight: 0,
		}
	}
	log.Infof("grpc pool is scaler form %v to %v", gp.poolSize, gp.poolSize+deltaConn)
	gp.poolSize = gp.poolSize + deltaConn
}

func (gp *GrpcPool) GetTotalUse() int {
	var totalUse int
	for i := 0; i < gp.poolSize; i++ {
		totalUse = totalUse + int(gp.conns[i].inflight)
	}
	return totalUse
}

func (gp *GrpcPool) GetGrpcPoolState() *GrpcPoolState {
	connStates := make([]GrpcConnState, gp.poolSize)
	for i := 0; i < gp.poolSize; i++ {
		connStates[i] = GrpcConnState{
			connState: gp.conns[i].conn.GetState().String(),
			inflight:  gp.conns[i].inflight,
		}
	}

	return &GrpcPoolState{
		ConnStates:  connStates,
		ReserveSize: gp.poolSize,
		Target:      gp.target,
		IsClosed:    gp.isClosed,
		ScaleOption: ScaleOption{
			Enable:          gp.scaleOption.Enable,
			ScalePeriod:     gp.scaleOption.ScalePeriod,
			MaxConn:         gp.scaleOption.MaxConn,
			DesireMaxStream: gp.scaleOption.DesireMaxStream,
		},
	}
}

func (gp *GrpcPool) Closed() {
	gp.isClosed = true
	gp.stateUpdate.Lock()
	defer gp.stateUpdate.Unlock()
	for i := 0; i < gp.poolSize; i++ {
		if err := gp.conns[i].conn.Close(); err != nil {
			log.Errorf("grpc conn close error %v", err)
		}
	}
}
