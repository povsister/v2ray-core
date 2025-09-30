package v4

import (
	"github.com/golang/protobuf/proto"

	"github.com/v2fly/v2ray-core/v5/common/net"
	"github.com/v2fly/v2ray-core/v5/infra/conf/cfgcommon"
	"github.com/v2fly/v2ray-core/v5/proxy/dns"
)

type DNSOutboundConfig struct {
	Network    cfgcommon.Network  `json:"network"`
	Address    *cfgcommon.Address `json:"address"`
	Port       uint16             `json:"port"`
	UserLevel  uint32             `json:"userLevel"`
	NonIPQuery string             `json:"nonIPQuery"`
}

func (c *DNSOutboundConfig) Build() (proto.Message, error) {
	config := &dns.Config{
		Server: &net.Endpoint{
			Network: c.Network.Build(),
			Port:    uint32(c.Port),
		},
		UserLevel:   c.UserLevel,
		Non_IPQuery: c.NonIPQuery,
	}
	if c.Address != nil {
		config.Server.Address = c.Address.Build()
	}
	return config, nil
}
