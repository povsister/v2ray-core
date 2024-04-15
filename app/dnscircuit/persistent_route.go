package dnscircuit

import (
	"fmt"
	"strings"

	"github.com/v2fly/v2ray-core/v5/app/dnscircuit/ospf"
	"github.com/v2fly/v2ray-core/v5/common/net"
)

func (s *dnsCircuit) initPersistentRoute() {
	var allCIDRs []net.IPNet
	defer func() {
		if len(allCIDRs) > 0 {
			s.ospf.AnnounceASBRRoute(allCIDRs)
		}
	}()
	s.ob.obDestStatRw.Lock()
	defer s.ob.obDestStatRw.Unlock()
	for _, cidrs := range s.persistentRoute {
		geoIPdesc := func() string {
			if len(cidrs.CountryCode) > 0 {
				if cidrs.InverseMatch {
					return "geoip:!" + strings.ToLower(cidrs.CountryCode)
				}
				return "geoip:" + strings.ToLower(cidrs.CountryCode)
			}
			return ""
		}
		if cidrs.InverseMatch {
			newError(fmt.Sprintf("ignored %s persistent route: inverse match is not supported", geoIPdesc())).
				AtWarning().WriteToLog()
			continue
		}
		for _, cidr := range cidrs.Cidr {
			ip := net.IPAddress(cidr.Ip)
			if ip.Family() != net.AddressFamilyIPv4 {
				newError(fmt.Sprintf("ignored non-IPv4 persistent route: %s CIDR %s/%d",
					geoIPdesc(), ip.String(), cidr.Prefix)).
					AtWarning().WriteToLog()
				continue
			}
			mask := net.CIDRMask(int(cidr.Prefix), 32)
			k := kFromIPAndMask(cidr.Ip, mask)
			s.ob.obDestStat[k] = &obDestMeta{
				isPersistent: true,
			}
			ospf.LogImportant("adding persistent route: %s CIDR %s/%d",
				geoIPdesc(), ip.String(), cidr.Prefix)
			allCIDRs = append(allCIDRs, net.IPNet{
				IP:   ip.IP(),
				Mask: mask,
			})
		}
	}
}
