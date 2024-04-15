package v4

import (
	"context"
	"strings"

	"github.com/golang/protobuf/proto"

	"github.com/v2fly/v2ray-core/v5/app/dnscircuit"
	"github.com/v2fly/v2ray-core/v5/app/router/routercommon"
	"github.com/v2fly/v2ray-core/v5/common/net"
	"github.com/v2fly/v2ray-core/v5/common/platform"
	"github.com/v2fly/v2ray-core/v5/infra/conf/cfgcommon"
	"github.com/v2fly/v2ray-core/v5/infra/conf/geodata"
	"github.com/v2fly/v2ray-core/v5/infra/conf/rule"
)

type DNSCircuitConfig struct {
	InboundTags    cfgcommon.StringList `json:"inboundTags"`
	OutboundTags   cfgcommon.StringList `json:"outboundTags"`
	DNSOutboundTag string               `json:"dnsOutboundTag"`
	InactiveClean  int64                `json:"inactiveClean"`
	OspfSetting    struct {
		IfName  string `json:"ifName"`
		Address string `json:"address"`
	} `json:"ospfSetting"`
	PersistentRoute cfgcommon.StringList `json:"persistentRoute"`
	cfgCtx          context.Context
}

func (b *DNSCircuitConfig) Build() (proto.Message, error) {
	if b.cfgCtx == nil {
		b.cfgCtx = cfgcommon.NewConfigureLoadingContext(context.Background())

		geoloadername := platform.NewEnvFlag("v2ray.conf.geoloader").GetValue(func() string {
			return "standard"
		})

		if loader, err := geodata.GetGeoDataLoader(geoloadername); err == nil {
			cfgcommon.SetGeoDataLoader(b.cfgCtx, loader)
		} else {
			return nil, newError("unable to create geo data loader ").Base(err)
		}
	}

	// ospf validation
	if len(b.OspfSetting.IfName) <= 0 {
		return nil, newError("OSPF ifName can not be empty")
	}
	_, ipNet, err := net.ParseCIDR(b.OspfSetting.Address)
	if err != nil {
		return nil, newError("invalid OSPF address format").Base(err)
	}
	ipStr := strings.SplitN(b.OspfSetting.Address, "/", 2)[0]
	ip := net.ParseAddress(ipStr)
	if ip == nil {
		return nil, newError("invalid OSPF listen address: ", ipStr)
	}
	if ip.Family() != net.AddressFamilyIPv4 {
		return nil, newError("only IPv4 is supported for OSPF listen address")
	}
	ones, _ := ipNet.Mask.Size()
	if ones < 24 || ones > 32 {
		return nil, newError("invalid OSPF listen address mask: only 24-32 is supported")
	}

	// tags validate
	if len(b.OutboundTags) <= 0 {
		return nil, newError("outbound tags can not be empty")
	}
	if len(b.DNSOutboundTag) <= 0 {
		return nil, newError("dnsOutbound tag can not be empty")
	}
	if b.InactiveClean <= 0 {
		b.InactiveClean = 6 * 60 * 60 // default 6 hours
	}

	persistentIPs, err := rule.ToCidrList(b.cfgCtx, b.PersistentRoute)
	if err != nil {
		return nil, newError("invalid persistent route").Base(err)
	}
	return &dnscircuit.Config{
		InboundTags:     b.InboundTags,
		OutboundTags:    b.OutboundTags,
		DnsOutboundTag:  b.DNSOutboundTag,
		PersistentRoute: persistentIPs,
		InactiveClean:   b.InactiveClean,
		OspfSetting: &dnscircuit.OSPFInstanceConfig{
			IfName: b.OspfSetting.IfName,
			Address: &routercommon.CIDR{
				Ip:     ip.IP(),
				Prefix: uint32(ones),
				IpAddr: b.OspfSetting.Address,
			},
		},
	}, nil
}
