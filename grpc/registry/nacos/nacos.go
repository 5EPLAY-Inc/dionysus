package nacos

import (
	"context"
	"fmt"
	"github.com/gowins/dionysus/grpc/registry"
	logger "github.com/gowins/dionysus/log"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"net"
	"strings"
	"time"
)

const (
	Name           = "nacos"
	DefaultTimeout = time.Second * 5
	DefaultGroup   = "DEFAULT_GROUP"
)

type (
	NamingClientKey            struct{}
	NamingClientGroupNameKey   struct{}
	NamingClientClusterNameKey struct{}
	DefaultClusterName         struct{}
	ClientConfigKey            struct{}
	ServicesConfigKey          struct{}
)

func init() {
	if err := registry.Register(Name, New()); err != nil {
		panic(fmt.Errorf("registry %s error", Name))
	}
}

func WithClientConfig(c *constant.ClientConfig) registry.Option {
	return func(o *registry.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = context.WithValue(o.Context, ClientConfigKey{}, c)
	}
}

type registryImpl struct {
	options registry.Options
	client  naming_client.INamingClient
}

func New() registry.Registry {
	return &registryImpl{}
}

func (r *registryImpl) Init(opts ...registry.Option) error {
	r.options = registry.Options{
		Context: context.Background(),
	}
	for _, o := range opts {
		o(&r.options)
	}

	if r.options.Timeout == 0 {
		r.options.Timeout = DefaultTimeout
	}

	var (
		clientConfig *constant.ClientConfig

		serversConfig []constant.ServerConfig
	)

	clientCfg, clientCfgOk := r.options.Context.Value(ClientConfigKey{}).(*constant.ClientConfig)

	if !clientCfgOk {
		panic("client config not found in context")
	}

	clientConfig = clientCfg

	for _, addr := range r.options.Addrs {
		host, port, _ := net.SplitHostPort(addr)
		serversConfig = append(serversConfig, *constant.NewServerConfig(host, cast.ToUint64(port)))
	}

	c, err := r.newClient(clientConfig, serversConfig)

	if err != nil {
		logger.WithField("client", clientConfig).WithField("serverConfig", serversConfig).Error(err)
		panic(err)
	}

	if c == nil {
		panic("nacos naming client initialize failed")
	}

	if !c.ServerHealthy() {
		panic("nacos naming does not work!")
	}

	r.client = c

	return nil
}

func (r *registryImpl) serviceNameComplex(serviceName string) string {
	return strings.Replace(serviceName, "/", "-", -1)
}

func (r *registryImpl) Register(service *registry.Service, opts ...registry.RegisterOption) error {
	if len(service.Nodes) == 0 {
		return errors.New("Require at least one node")
	}

	options := registry.RegisterOptions{
		Context: context.Background(),
	}
	for _, o := range opts {
		o(&options)
	}

	var (
		groupName = DefaultGroup
	)

	if name, ok := options.Context.Value(NamingClientGroupNameKey{}).(string); ok {
		groupName = name
	}

	for _, node := range service.Nodes {
		_, err := r.client.RegisterInstance(vo.RegisterInstanceParam{
			Ip:          node.Address,                       //required
			Port:        uint64(node.Port),                  //required
			Weight:      100,                                //required,it must be lager than 0
			Enable:      true,                               //required,the instance can be access or not
			Healthy:     true,                               //required,the instance is health or not
			Metadata:    node.Metadata,                      //optional
			ServiceName: r.serviceNameComplex(service.Name), //required
			GroupName:   groupName,                          //optional,default:DEFAULT_GROUP
			Ephemeral:   true,                               //optional
		})
		if err != nil {
			return err
		}
		return nil
	}
	return nil
}

func (r *registryImpl) Deregister(service *registry.Service) error {
	if len(service.Nodes) == 0 {
		return errors.New("Require at least one node")
	}

	_, cancel := context.WithTimeout(context.Background(), r.options.Timeout)
	defer cancel()

	for _, node := range service.Nodes {
		_, err := r.client.DeregisterInstance(vo.DeregisterInstanceParam{
			Ip:          node.Address,                       //required
			Port:        uint64(node.Port),                  //required
			ServiceName: r.serviceNameComplex(service.Name), //required
			Ephemeral:   true,                               //optional
			GroupName:   DefaultGroup,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *registryImpl) GetService(serviceName string) ([]*registry.Service, error) {
	instances, err := r.client.SelectInstances(vo.SelectInstancesParam{
		ServiceName: r.serviceNameComplex(serviceName),
		HealthyOnly: true,
		GroupName:   DefaultGroup,
	})

	logger.WithField("services", instances).
		WithField("serviceName", serviceName).
		Info("GetService success")

	if err != nil {
		return nil, err
	}

	services := make([]*registry.Service, 0, len(instances))

	for _, instance := range instances {
		var nodes []*registry.Node
		nodes = append(nodes, &registry.Node{
			Id:       instance.InstanceId,
			Address:  instance.Ip,
			Port:     int(instance.Port),
			Metadata: instance.Metadata,
		})
		services = append(services, &registry.Service{
			Name:     serviceName,
			Version:  instance.Metadata["version"],
			Nodes:    nodes,
			Metadata: instance.Metadata,
		})
	}
	return services, nil
}

func (r *registryImpl) ListServices() ([]*registry.Service, error) {
	rawInstances, err := r.client.GetAllServicesInfo(vo.GetAllServiceInfoParam{
		NameSpace: "",
		PageSize:  100,
		PageNo:    1,
	})

	if err != nil {
		return nil, err
	}

	services := make([]*registry.Service, 0, len(rawInstances.Doms))

	for _, instance := range rawInstances.Doms {
		services = append(services, &registry.Service{Name: instance})
	}
	return services, nil
}

func (r *registryImpl) Watch(option ...registry.WatchOption) (registry.Watcher, error) {
	return newWatcher(r, option...)
}

func (r *registryImpl) String() string {
	return Name
}
