package router

import (
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"

	"go4.org/netipx"

	"github.com/v2fly/v2ray-core/v5/common/net"
)

type dynamicIPMatcher struct {
	m *GeoIPMatcher

	builderMu  sync.RWMutex
	ip4Builder *netipx.IPSetBuilder
	ip6Builder *netipx.IPSetBuilder

	ip4 atomic.Pointer[netipx.IPSet]
	ip6 atomic.Pointer[netipx.IPSet]

	connTrackMu sync.RWMutex
	connTrack   map[netip.Addr]*connTrackFromSrc
}

type connTrackFromSrc struct {
	dm  *dynamicIPMatcher
	src netip.Addr

	builderMu  sync.RWMutex
	ip4Builder *netipx.IPSetBuilder
	ip6Builder *netipx.IPSetBuilder

	ip4 atomic.Pointer[netipx.IPSet]
	ip6 atomic.Pointer[netipx.IPSet]
}

func newConnTrackFromSrc(dm *dynamicIPMatcher, src netip.Addr) *connTrackFromSrc {
	return &connTrackFromSrc{
		dm:         dm,
		src:        src,
		ip4Builder: new(netipx.IPSetBuilder),
		ip6Builder: new(netipx.IPSetBuilder),
	}
}

func (m *GeoIPMatcher) InitDynamicMatcher() (err error) {
	dm := &dynamicIPMatcher{
		m:          m,
		ip4Builder: new(netipx.IPSetBuilder),
		ip6Builder: new(netipx.IPSetBuilder),
		connTrack:  make(map[netip.Addr]*connTrackFromSrc),
	}
	m.dynamicIPMatcher = dm
	ipSet4, err := dm.ip4Builder.IPSet()
	if err != nil {
		return
	}
	dm.ip4.Store(ipSet4)
	ipSet6, err := dm.ip6Builder.IPSet()
	if err != nil {
		return
	}
	dm.ip6.Store(ipSet6)
	return
}

func (cm *connTrackFromSrc) match(dst net.IP) bool {
	nip, ok := netipx.FromStdIP(dst)
	if !ok {
		return false
	}
	switch len(dst) {
	case net.IPv4len:
		return cm.ip4.Load().Contains(nip)
	case net.IPv6len:
		return cm.ip6.Load().Contains(nip)
	}
	return false
}

func (cm *connTrackFromSrc) addIPNet(ipNets ...net.IPNet) {
	var ip4Modified, ip6Modified bool
	cm.builderMu.RLock()
	defer cm.builderMu.RUnlock()
	defer func() {
		cm.update(ip4Modified, ip6Modified)
	}()

	for _, ipNet := range ipNets {
		addr, ok := netip.AddrFromSlice(ipNet.IP)
		if !ok {
			continue
		}
		addr = addr.Unmap()
		ones, _ := ipNet.Mask.Size()
		prefix := netip.PrefixFrom(addr, ones)
		if prefix.Bits() == -1 {
			continue
		}
		switch {
		case addr.Is4():
			cm.ip4Builder.AddPrefix(prefix)
			ip4Modified = true
		case addr.Is6():
			cm.ip6Builder.AddPrefix(prefix)
			ip6Modified = true
		}
	}
}

func (cm *connTrackFromSrc) removeIPNet(ipNets ...net.IPNet) {
	var ip4Modified, ip6Modified bool
	cm.builderMu.RLock()
	defer cm.builderMu.RUnlock()
	defer func() {
		cm.update(ip4Modified, ip6Modified)
	}()

	for _, ipNet := range ipNets {
		addr, ok := netip.AddrFromSlice(ipNet.IP)
		if !ok {
			continue
		}
		addr = addr.Unmap()
		ones, _ := ipNet.Mask.Size()
		prefix := netip.PrefixFrom(addr, ones)
		if prefix.Bits() == -1 {
			continue
		}
		switch {
		case addr.Is4():
			cm.ip4Builder.RemovePrefix(prefix)
			ip4Modified = true
		case addr.Is6():
			cm.ip6Builder.RemovePrefix(prefix)
			ip6Modified = true
		}
	}
}

func (cm *connTrackFromSrc) update(ip4Updated, ip6Updated bool) {
	if ip4Updated {
		ipSet4, err := cm.ip4Builder.IPSet()
		if err != nil {
			newError(fmt.Sprintf("%s err update conn-track %s ip4 set",
				strings.ToLower(cm.dm.m.countryCode), cm.src.String())).
				Base(err).AtWarning().WriteToLog()
		} else {
			cm.ip4.Store(ipSet4)
		}
	}
	if ip6Updated {
		ipSet6, err := cm.ip6Builder.IPSet()
		if err != nil {
			newError(fmt.Sprintf("%s err update conn-track %s ip6 set",
				strings.ToLower(cm.dm.m.countryCode), cm.src.String())).
				Base(err).AtWarning().WriteToLog()
		} else {
			cm.ip6.Store(ipSet6)
		}
	}
}

func (m *dynamicIPMatcher) ConnTrackMatch(src, dst net.IP) bool {
	addr, ok := netip.AddrFromSlice(src)
	if !ok {
		return false
	}
	m.connTrackMu.RLock()
	ct, ok := m.connTrack[addr]
	m.connTrackMu.RUnlock()
	if !ok {
		return false
	}
	return ct.match(dst)
}

func (m *dynamicIPMatcher) AddIPNetConnTrack(src net.IP, dsts ...net.IPNet) {
	addr, ok := netip.AddrFromSlice(src)
	if !ok {
		return
	}

	m.connTrackMu.Lock()
	ct, ok := m.connTrack[addr]
	if !ok {
		ct = newConnTrackFromSrc(m, addr)
		m.connTrack[addr] = ct
	}
	m.connTrackMu.Unlock()

	ct.addIPNet(dsts...)
}

func (m *dynamicIPMatcher) RemoveIPNetConnTrack(src net.IP, dsts ...net.IPNet) {
	addr, ok := netip.AddrFromSlice(src)
	if !ok {
		return
	}

	m.connTrackMu.Lock()
	ct, ok := m.connTrack[addr]
	if !ok {
		ct = newConnTrackFromSrc(m, addr)
		m.connTrack[addr] = ct
	}
	m.connTrackMu.Unlock()

	ct.removeIPNet(dsts...)
}

func (m *dynamicIPMatcher) Match(ip net.IP) bool {
	nip, ok := netipx.FromStdIP(ip)
	if !ok {
		return false
	}
	switch len(ip) {
	case net.IPv4len:
		return m.ip4.Load().Contains(nip)
	case net.IPv6len:
		return m.ip6.Load().Contains(nip)
	}
	return false
}

func (m *dynamicIPMatcher) updateIPSet(is4M, is6M bool) {
	if is4M {
		ipSet4, err := m.ip4Builder.IPSet()
		if err != nil {
			newError(fmt.Sprintf("%s err update dynamic ip4 set", strings.ToLower(m.m.countryCode))).
				Base(err).AtWarning().WriteToLog()
		} else {
			m.ip4.Store(ipSet4)
		}
	}
	if is6M {
		ipSet6, err := m.ip6Builder.IPSet()
		if err != nil {
			newError(fmt.Sprintf("%s err update dynamic ip6 set", strings.ToLower(m.m.countryCode))).
				Base(err).AtWarning().WriteToLog()
		} else {
			m.ip6.Store(ipSet6)
		}
	}
}

func (m *dynamicIPMatcher) AddIPNet(ipNets ...net.IPNet) {
	var is4Modified, is6Modified bool

	m.builderMu.Lock()
	defer m.builderMu.Unlock()
	defer func() {
		m.updateIPSet(is4Modified, is6Modified)
	}()

	for _, ipNet := range ipNets {
		addr, ok := netip.AddrFromSlice(ipNet.IP)
		if !ok {
			continue
		}
		addr = addr.Unmap()
		ones, _ := ipNet.Mask.Size()
		prefix := netip.PrefixFrom(addr, ones)
		if prefix.Bits() == -1 {
			continue
		}
		switch {
		case addr.Is4():
			m.ip4Builder.AddPrefix(prefix)
			is4Modified = true
		case addr.Is6():
			m.ip6Builder.AddPrefix(prefix)
			is6Modified = true
		}
	}
}

func (m *dynamicIPMatcher) RemoveIPNet(ipNets ...net.IPNet) {
	var is4Modified, is6Modified bool

	m.builderMu.Lock()
	defer m.builderMu.Unlock()
	defer func() {
		m.updateIPSet(is4Modified, is6Modified)
	}()

	for _, ipNet := range ipNets {
		addr, ok := netip.AddrFromSlice(ipNet.IP)
		if !ok {
			continue
		}
		addr = addr.Unmap()
		ones, _ := ipNet.Mask.Size()
		prefix := netip.PrefixFrom(addr, ones)
		if prefix.Bits() == -1 {
			continue
		}
		switch {
		case addr.Is4():
			m.ip4Builder.RemovePrefix(prefix)
			is4Modified = true
		case addr.Is6():
			m.ip6Builder.RemovePrefix(prefix)
			is6Modified = true
		}
	}
}
