// copy from go-plugins/registry/etcdv3

package etcdv3

import (
	"context"

	"github.com/gowins/dionysus/grpc/registry"
)

type authKey struct{}

type authCreds struct {
	Username string
	Password string
}

// Auth allows you to specify username/password
func Auth(username, password string) registry.Option {
	return func(o *registry.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}
		o.Context = context.WithValue(o.Context, authKey{}, &authCreds{Username: username, Password: password})
	}
}
