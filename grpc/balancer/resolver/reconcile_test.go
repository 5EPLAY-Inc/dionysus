package resolver

import (
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/gowins/dionysus/grpc/registry"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
)

// fakeRegistry is an in-memory registry.Registry whose GetService result can be
// swapped at will, letting us simulate the authoritative instance list changing
// (e.g. a pod removed during a rolling update) without any watch event firing.
type fakeRegistry struct {
	mu    sync.Mutex
	nodes []*registry.Node
}

func (f *fakeRegistry) setNodes(nodes []*registry.Node) {
	f.mu.Lock()
	f.nodes = nodes
	f.mu.Unlock()
}

func (f *fakeRegistry) GetService(name string) ([]*registry.Service, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return []*registry.Service{{Name: name, Nodes: f.nodes}}, nil
}

func (f *fakeRegistry) Init(...registry.Option) error                 { return nil }
func (f *fakeRegistry) Register(*registry.Service, ...registry.RegisterOption) error { return nil }
func (f *fakeRegistry) Deregister(*registry.Service) error            { return nil }
func (f *fakeRegistry) ListServices() ([]*registry.Service, error)    { return nil, nil }
func (f *fakeRegistry) Watch(...registry.WatchOption) (registry.Watcher, error) {
	return &mockWatcher{}, nil
}
func (f *fakeRegistry) String() string { return "fake" }

func mkNode(id, addr string, port int) *registry.Node {
	return &registry.Node{Id: id, Address: addr, Port: port}
}

func addrsFromState(cc *mockedClientConn) []string {
	out := make([]string, 0, len(cc.state.Addresses))
	for _, a := range cc.state.Addresses {
		out = append(out, a.Addr)
	}
	sort.Strings(out)
	return out
}

// syncClientConn is a thread-safe resolver.ClientConn for tests that exercise
// the resolver's background goroutines concurrently with assertions.
type syncClientConn struct {
	mu    sync.Mutex
	addrs []string
}

func (c *syncClientConn) UpdateState(state resolver.State) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.addrs = c.addrs[:0]
	for _, a := range state.Addresses {
		c.addrs = append(c.addrs, a.Addr)
	}
	return nil
}
func (c *syncClientConn) ReportError(error)                {}
func (c *syncClientConn) NewAddress([]resolver.Address)    {}
func (c *syncClientConn) NewServiceConfig(string)          {}
func (c *syncClientConn) ParseServiceConfig(string) *serviceconfig.ParseResult {
	return nil
}
func (c *syncClientConn) sortedAddrs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := append([]string(nil), c.addrs...)
	sort.Strings(out)
	return out
}

func newTestResolver(cc resolver.ClientConn, reg registry.Registry, endpoint string) *discovResolver {
	return &discovResolver{
		cc:                cc,
		reg:               reg,
		endpoint:          endpoint,
		services:          map[string]resolver.Address{},
		reconcileInterval: defaultReconcileInterval,
		resolveNow:        make(chan struct{}, 1),
		stopCh:            make(chan struct{}),
	}
}

// TestReconcileRemovesStaleEndpoint is the core regression test: a node leaves
// the registry WITHOUT a delete watch event arriving. The periodic/on-demand
// reconcile must rebuild the address set from the authoritative list and drop
// the stale endpoint -- otherwise pick_first could keep routing to a reused IP
// and yield codes.Unimplemented.
func TestReconcileRemovesStaleEndpoint(t *testing.T) {
	reg := &fakeRegistry{}
	reg.setNodes([]*registry.Node{
		mkNode("a", "10.0.0.1", 8080),
		mkNode("b", "10.0.0.2", 8080),
	})

	cc := new(mockedClientConn)
	r := newTestResolver(cc, reg, "svc")

	assert.Nil(t, r.reconcile())
	assert.Equal(t, []string{"10.0.0.1:8080", "10.0.0.2:8080"}, addrsFromState(cc))

	// Rolling update: node "b" is gone from the registry, but no delete event
	// was delivered to the watcher.
	reg.setNodes([]*registry.Node{mkNode("a", "10.0.0.1", 8080)})

	assert.Nil(t, r.reconcile())
	assert.Equal(t, []string{"10.0.0.1:8080"}, addrsFromState(cc),
		"stale endpoint 10.0.0.2 must be pruned by reconcile")
}

// TestApplyDeltaDeleteAndUpsert covers the fast incremental path.
func TestApplyDeltaDeleteAndUpsert(t *testing.T) {
	reg := &fakeRegistry{}
	cc := new(mockedClientConn)
	r := newTestResolver(cc, reg, "svc")

	r.applyDelta(&registry.Result{Action: "create", Service: &registry.Service{
		Name: "svc", Nodes: []*registry.Node{mkNode("a", "10.0.0.1", 8080)},
	}})
	assert.Equal(t, []string{"10.0.0.1:8080"}, addrsFromState(cc))

	r.applyDelta(&registry.Result{Action: "create", Service: &registry.Service{
		Name: "svc", Nodes: []*registry.Node{mkNode("b", "10.0.0.2", 8080)},
	}})
	assert.Equal(t, []string{"10.0.0.1:8080", "10.0.0.2:8080"}, addrsFromState(cc))

	r.applyDelta(&registry.Result{Action: "delete", Service: &registry.Service{
		Name: "svc", Nodes: []*registry.Node{mkNode("a", "10.0.0.1", 8080)},
	}})
	assert.Equal(t, []string{"10.0.0.2:8080"}, addrsFromState(cc))
}

// TestResolveNowTriggersReconcile verifies the run loop reconciles on demand and
// stops cleanly on Close.
func TestResolveNowTriggersReconcile(t *testing.T) {
	reg := &fakeRegistry{}
	reg.setNodes([]*registry.Node{mkNode("a", "10.0.0.1", 8080), mkNode("b", "10.0.0.2", 8080)})

	cc := &syncClientConn{}
	r := newTestResolver(cc, reg, "svc")
	assert.Nil(t, r.reconcile())

	go r.run()
	defer r.Close()

	reg.setNodes([]*registry.Node{mkNode("a", "10.0.0.1", 8080)})
	r.ResolveNow(resolver.ResolveNowOptions{})

	assert.Eventually(t, func() bool {
		return len(cc.sortedAddrs()) == 1
	}, time.Second, 5*time.Millisecond,
		"ResolveNow should trigger a reconcile that prunes the stale endpoint")
}
