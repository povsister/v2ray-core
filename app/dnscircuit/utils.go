package dnscircuit

import (
	"strings"

	"github.com/v2fly/v2ray-core/v5/common/net"
)

func PrettyPrintIPNet(ipNets ...net.IPNet) string {
	buf := new(strings.Builder)
	for i, ipNet := range ipNets {
		if i != 0 {
			buf.WriteString(" ")
		}
		buf.WriteString(printIPNet(ipNet))
	}
	return buf.String()
}

func printIPNet(n net.IPNet) string {
	return n.String()
}
