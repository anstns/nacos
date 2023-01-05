package nacos

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/nacos-group/nacos-sdk-go/util"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/pkg/errors"
	"google.golang.org/grpc/resolver"
)

func init() {
	resolver.Register(&builder{})
}

// schemeName for the urls
// All target URLs like 'nacos://.../...' will be resolved by this resolver
const schemeName = "nacos"

// builder implements resolver.Builder and use for constructing all consul resolvers
type builder struct{}

func (b *builder) Build(url resolver.Target, conn resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	tgt, err := parseURL(url.URL)
	if err != nil {
		return nil, errors.Wrap(err, "Wrong nacos URL")
	}

	host, ports, err := net.SplitHostPort(tgt.Addr)
	if err != nil {
		return nil, fmt.Errorf("failed parsing address error: %v", err)
	}
	port, _ := strconv.ParseUint(ports, 10, 16)

	sc := []constant.ServerConfig{
		*constant.NewServerConfig(host, port, constant.WithContextPath("/nacos")),
	}

	cc := &constant.ClientConfig{
		// 订阅者名称，显示在 Nacos UI 中
		AppName:             tgt.AppName,
		NamespaceId:         tgt.NamespaceID,
		Username:            tgt.User,
		Password:            tgt.Password,
		TimeoutMs:           uint64(tgt.Timeout),
		NotLoadCacheAtStart: true,
	}

	if tgt.CacheDir != "" {
		cc.CacheDir = tgt.CacheDir
	}
	if tgt.LogDir != "" {
		cc.LogDir = tgt.LogDir
	}
	if tgt.LogLevel != "" {
		cc.LogLevel = tgt.LogLevel
	}

	cli, err := clients.NewNamingClient(vo.NacosClientParam{
		ServerConfigs: sc,
		ClientConfig:  cc,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Couldn't connect to the nacos API")
	}

	ctx, cancel := context.WithCancel(context.Background())
	pipe := make(chan []string)

	go func() {
		fmt.Println("Subscribe-ServiceName=", tgt.Service)
		fmt.Println("Subscribe-GroupName=", tgt.GroupName)
		err := cli.Subscribe(&vo.SubscribeParam{
			ServiceName: tgt.Service,
			GroupName:   tgt.GroupName,
			// SubscribeCallback: newWatcher(ctx, cancel, pipe).CallBackHandle, // required
			SubscribeCallback: func(services []model.Instance, err error) {
				fmt.Printf("callback return services:%s \n\n", util.ToJsonString(services))
				// ee := make([]string, 0, len(services))
				for _, s := range services {
					// ee = append(ee, fmt.Sprintf("%s:%d", s.Ip, s.Port))
					fmt.Println("dis:=%s", fmt.Sprintf("%s:%d", s.Ip, s.Port))
				}

				for {
					// select {
					// case cc := <-input:
					// 	connsSet := make(map[string]struct{}, len(cc))
					// 	for _, c := range cc {
					// 		connsSet[c] = struct{}{}
					// 	}
					// 	conns := make([]resolver.Address, 0, len(connsSet))
					// 	for c := range connsSet {
					// 		conns = append(conns, resolver.Address{Addr: c})
					// 	}
					// 	sort.Sort(byAddressString(conns)) // Don't replace the same address list in the balancer
					// 	_ = conn.UpdateState(resolver.State{Addresses: conns})
					// case <-ctx.Done():
					// 	logx.Info("[Nacos resolver] Watch has been finished")
					// 	return
					// }
				}
			},
		})
		if err != nil {
			panic(err)
		}
	}()

	go populateEndpoints(ctx, conn, pipe)

	return &resolvr{cancelFunc: cancel}, nil
}

// Scheme returns the scheme supported by this resolver.
// Scheme is defined at https://github.com/grpc/grpc/blob/master/doc/naming.md.
func (b *builder) Scheme() string {
	return schemeName
}
