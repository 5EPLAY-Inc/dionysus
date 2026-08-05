package resolver

import (
	"fmt"
	"sync"
	"time"

	logger "github.com/gowins/dionysus/log"

	"github.com/gowins/dionysus/grpc/registry"
	"github.com/pkg/errors"
	"google.golang.org/grpc/resolver"
)

// defaultReconcileInterval is how often the discov resolver performs a full,
// authoritative re-resolve against the registry.
//
// The watch stream only carries incremental deltas (create/update/delete). If a
// "delete" event is ever missed -- a transient watch error, a watcher
// reconnect, or an event dropped during a rolling update -- the stale endpoint
// would otherwise linger in the address set forever. k8s can later reuse that
// terminated pod's IP for a completely different service; pick_first then keeps
// the connection and switches onto the reused IP, and RPCs come back as
// codes.Unimplemented. Periodically rebuilding the address set from the
// registry's full instance list self-heals any such missed delete.
var defaultReconcileInterval = 30 * time.Second

type discovBuilder struct{}

func (d *discovBuilder) Scheme() string { return DiscovScheme }

// Build discovBuilder discov://wpt.etcd/service_name
func (d *discovBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	// target.URL.Host 得到注册中心的地址;
	// 当然也可以直接通过全局变量 [registry.Default] 获取注册中心, 然后进行判断
	reg := registry.Get(target.URL.Host)
	if reg == nil {
		return nil, fmt.Errorf("registry %s not exists\n", target.URL.Host)
	}

	r := &discovResolver{
		cc:                cc,
		reg:               reg,
		endpoint:          target.Endpoint(),
		services:          map[string]resolver.Address{},
		reconcileInterval: defaultReconcileInterval,
		resolveNow:        make(chan struct{}, 1),
		stopCh:            make(chan struct{}),
	}

	// 启动时先做一次权威解析, 保持与旧实现一致的启动语义:
	// 拿不到服务或者没有可用节点时, Build 直接失败.
	if err := r.reconcile(); err != nil {
		return nil, err
	}
	if r.addrLen() == 0 {
		return nil, fmt.Errorf("service none available")
	}

	w, err := reg.Watch(registry.WatchService(r.endpoint))
	if err != nil {
		return nil, errors.Wrapf(err, "target.Endpoint:%s\n", r.endpoint)
	}
	r.watcher = w

	logger.
		WithField("target", r.endpoint).
		WithField("addrs", r.snapshot()).
		Info("【Build】Initialize Resource successfully！")

	go r.watchLoop()
	go r.run()

	return r, nil
}

// discovResolver holds the resolver state for a single service (one Build call).
// State is intentionally per-resolver -- not shared on the builder -- so that
// different services never cross-contaminate each other's address sets.
type discovResolver struct {
	cc       resolver.ClientConn
	reg      registry.Registry
	watcher  registry.Watcher
	endpoint string

	mu       sync.Mutex
	services map[string]resolver.Address // getServiceUniqueId -> address

	reconcileInterval time.Duration
	resolveNow        chan struct{}
	stopCh            chan struct{}
	stopOnce          sync.Once
}

// reconcile rebuilds the whole address set from the registry's authoritative
// instance list, dropping any endpoint that is no longer registered.
func (r *discovResolver) reconcile() error {
	services, err := r.reg.GetService(r.endpoint)
	if err != nil {
		return errors.Wrap(err, "registry GetService error")
	}

	m := buildServiceMap(services)

	r.mu.Lock()
	r.services = m
	r.mu.Unlock()

	r.pushState()
	return nil
}

// applyDelta applies a single incremental watch event for fast reaction. The
// periodic reconcile is the safety net that corrects anything this path misses.
func (r *discovResolver) applyDelta(res *registry.Result) {
	if res == nil || res.Service == nil {
		return
	}

	r.mu.Lock()
	for _, n := range res.Service.Nodes {
		for j := 0; j < Replica; j++ {
			id := getServiceUniqueId(n.Id, j)
			if res.Action == "delete" {
				delete(r.services, id)
				continue
			}
			r.services[id] = newAddr(nodeAddr(n), res.Service.Name)
		}
	}
	r.mu.Unlock()

	r.pushState()
}

// pushState publishes the current address set to the grpc ClientConn.
func (r *discovResolver) pushState() {
	addrs := r.snapshot()
	if err := r.cc.UpdateState(resolver.State{Addresses: addrs}); err != nil {
		log.Errorf("[grpc resolver] update state for %s error: %v", r.endpoint, err)
	}
}

func (r *discovResolver) snapshot() []resolver.Address {
	r.mu.Lock()
	addrs := make([]resolver.Address, 0, len(r.services))
	for _, a := range r.services {
		addrs = append(addrs, a)
	}
	r.mu.Unlock()
	return reshuffle(addrs)
}

func (r *discovResolver) addrLen() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.services)
}

// watchLoop consumes incremental registry events. On a transient watch error it
// falls back to a full reconcile so a dropped event cannot leave stale state.
func (r *discovResolver) watchLoop() {
	for {
		res, err := r.watcher.Next()
		if errors.Is(err, registry.ErrWatcherStopped) {
			return
		}
		if err != nil {
			log.Errorf("[grpc resolver] watch %s error: %v", r.endpoint, err)
			r.triggerResolve()
			continue
		}
		r.applyDelta(res)
	}
}

// run drives periodic and on-demand full reconciles until the resolver closes.
func (r *discovResolver) run() {
	t := time.NewTicker(r.reconcileInterval)
	defer t.Stop()
	for {
		select {
		case <-r.stopCh:
			return
		case <-t.C:
			if err := r.reconcile(); err != nil {
				log.Errorf("[grpc resolver] periodic reconcile %s error: %v", r.endpoint, err)
			}
		case <-r.resolveNow:
			if err := r.reconcile(); err != nil {
				log.Errorf("[grpc resolver] on-demand reconcile %s error: %v", r.endpoint, err)
			}
		}
	}
}

// ResolveNow is called by grpc when it wants fresh addresses, e.g. after a
// subconn fails. We use it to re-resolve immediately, so a broken connection to
// a terminated pod is replaced from the authoritative list rather than letting
// pick_first fall onto a possibly stale endpoint.
func (r *discovResolver) ResolveNow(_ resolver.ResolveNowOptions) {
	r.triggerResolve()
}

func (r *discovResolver) triggerResolve() {
	select {
	case r.resolveNow <- struct{}{}:
	default: // a reconcile is already pending; coalesce
	}
}

func (r *discovResolver) Close() {
	r.stopOnce.Do(func() { close(r.stopCh) })
	if r.watcher != nil {
		r.watcher.Stop()
	}
}

// buildServiceMap turns a registry instance list into the resolver address set,
// keyed so it can be rebuilt wholesale on every reconcile.
func buildServiceMap(services []*registry.Service) map[string]resolver.Address {
	m := make(map[string]resolver.Address)
	for _, s := range services {
		for _, n := range s.Nodes {
			for j := 0; j < Replica; j++ {
				m[getServiceUniqueId(n.Id, j)] = newAddr(nodeAddr(n), s.Name)
			}
		}
	}
	return m
}

func nodeAddr(n *registry.Node) string {
	// 如果 port 不存在, 那么 address 中已经包含 port
	if n.Port > 0 {
		return fmt.Sprintf("%s:%d", n.Address, n.Port)
	}
	return n.Address
}
