package routing

import (
	"github.com/v2fly/v2ray-core/v5/common/net"
)

type RouterWithDynamicRule interface {
	Router
	// GetDynamicRuleIP returns a dynamic routing rule based on IPNet.
	// currently, only "dynamic-ipset:XXXX" is supported.
	// Be noted that rules with same name are Singleton.
	GetDynamicRuleIP(ruleName string) DynamicRuleIP
}

type DynamicRuleIP interface {
	AddIPNet(ipNets ...net.IPNet)
	RemoveIPNet(ipNets ...net.IPNet)
	AddIPNetConnTrack(src net.IP, dsts ...net.IPNet)
	RemoveIPNetConnTrack(src net.IP, dsts ...net.IPNet)
}
