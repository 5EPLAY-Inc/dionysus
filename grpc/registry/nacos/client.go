package nacos

import (
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

/*
*

	cC = &constant.ClientConfig{
		NamespaceId:         "public", // 如果需要支持多namespace，我们可以创建多个client,它们有不同的NamespaceId。当namespace是public时，此处填空字符串。
		TimeoutMs:           5000,
		NotLoadCacheAtStart: true,
		LogDir:              "/tmp/nacos/log",
		CacheDir:            "/tmp/nacos/cache",
		LogLevel:            "debug",
		AppName:             "dionysus",
	}
	css = []constant.ServerConfig{
		*constant.NewServerConfig("172.16.8.129", 8848),
	}
*/
func (r *registryImpl) newClient(cC *constant.ClientConfig, css []constant.ServerConfig) (naming_client.INamingClient, error) {
	return clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  cC,
			ServerConfigs: css,
		},
	)
}
