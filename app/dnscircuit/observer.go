package dnscircuit

import (
	"context"
	"errors"
	"fmt"
	stdnet "net"
	"net/netip"
	"sync"
	"time"

	"github.com/v2fly/v2ray-core/v5/app/dnscircuit/ospf"
	router_commands "github.com/v2fly/v2ray-core/v5/app/router/command"
	"github.com/v2fly/v2ray-core/v5/common"
	"github.com/v2fly/v2ray-core/v5/common/net"
	"github.com/v2fly/v2ray-core/v5/common/session"
	"github.com/v2fly/v2ray-core/v5/common/signal"
	dns_feature "github.com/v2fly/v2ray-core/v5/features/dns"
	"github.com/v2fly/v2ray-core/v5/features/routing"
	routing_dns "github.com/v2fly/v2ray-core/v5/features/routing/dns"
	"github.com/v2fly/v2ray-core/v5/proxy"
)

type observer struct {
	c                    *dnsCircuit
	dynDefaultDestRuleIP routing.DynamicRuleIP
	dynSrcRuleIP         map[string]routing.DynamicRuleIP // outboundTag -> dynamic IP rule set
	dynDestRuleIP        map[string]routing.DynamicRuleIP

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	obevQ chan observedEvent

	obDestStatRw sync.RWMutex
	obDestStat   map[obDestMetaKey]*obDestMeta // network -> meta

	obStatGCTicker *ospf.TickerFunc

	obConnTrackStatRw sync.RWMutex
	obConnTrackStat   map[netip.Addr]*obConnTrackMeta // srcAddr -> connTrackMeta
}

func connTrackKey(from net.Destination) netip.Addr {
	addrF, _ := netip.AddrFromSlice(from.Address.IP())
	return addrF
}

type obConnTrackMeta struct {
	rw       sync.RWMutex
	destMeta map[obDestMetaKey]*connTrackDestMeta
}

type connTrackDestMeta struct {
	src          netip.Addr
	g            *obDestMeta
	rw           sync.RWMutex
	lastAccessed time.Time
	domain       string
	outboundTag  string
}

func (ock *connTrackDestMeta) getOutTag() string {
	ock.rw.RLock()
	defer ock.rw.RUnlock()
	return ock.outboundTag
}

func (ock *connTrackDestMeta) setOutTag(tag string) {
	ock.rw.Lock()
	defer ock.rw.Unlock()
	ock.outboundTag = tag
}

func (ock *connTrackDestMeta) updateLastAccessed(d string) {
	ock.rw.Lock()
	defer ock.rw.Unlock()
	ock.lastAccessed = time.Now()
	ock.domain = d
}

func (ock *connTrackDestMeta) durSinceLastAccess() time.Duration {
	ock.rw.RLock()
	defer ock.rw.RUnlock()
	return time.Since(ock.lastAccessed)
}

func (ock *connTrackDestMeta) Update() {
	ock.rw.Lock()
	defer ock.rw.Unlock()
	ock.lastAccessed = time.Now()
}

type obDestMetaKey struct {
	destIP   netip.Addr
	maskOnes int
	maskBits int
}

func kFromIPAndMask(ip net.IP, mask net.IPMask) obDestMetaKey {
	addr, _ := netip.AddrFromSlice(ip)
	ones, bits := mask.Size()
	return obDestMetaKey{
		destIP:   addr,
		maskOnes: ones,
		maskBits: bits,
	}
}

func (obk obDestMetaKey) ipNet() net.IPNet {
	return net.IPNet{
		IP:   obk.destIP.AsSlice(),
		Mask: net.CIDRMask(obk.maskOnes, obk.maskBits),
	}
}

type obDestMeta struct {
	rw            sync.RWMutex
	lastAnnounced time.Time // last announced time to ASBR route
	domain        string

	isPersistent bool
	isOutdated   bool
}

func (m *obDestMeta) durSinceLastAnnounce() time.Duration {
	m.rw.RLock()
	defer m.rw.RUnlock()
	return time.Since(m.lastAnnounced)
}

func (m *obDestMeta) updateLastAnnounced(domain ...string) {
	m.rw.Lock()
	defer m.rw.Unlock()
	m.lastAnnounced = time.Now()
	if len(domain) > 0 && domain[0] != "" {
		m.domain = domain[0]
	}
}

func (m *obDestMeta) Update() {
	m.rw.Lock()
	defer m.rw.Unlock()
	m.lastAnnounced = time.Now()
}

func (s *dnsCircuit) initObserver() error {
	dynDestIPset := s.dynRouter.GetDynamicRuleIP(dns_feature.DynamicIPSetDnsCircuitDestDefault)
	if dynDestIPset == nil {
		return newError(fmt.Sprintf("route default dest rule %s not found", dns_feature.DynamicIPSetDnsCircuitDestDefault))
	}
	srcConnTrackIPset := make(map[string]routing.DynamicRuleIP, len(s.expectTags))
	destConnTrackIPset := make(map[string]routing.DynamicRuleIP, len(s.expectTags))
	// init skip rules
	rtCtx = origRtCtx
	for outTag := range s.expectTags {
		srcIPsetRuleName := dns_feature.DynamicIPSetDNSCircuitConnTrackSrcPrefix + outTag
		if srcTrackRule := s.dynRouter.GetDynamicRuleIP(srcIPsetRuleName); srcTrackRule == nil {
			return newError(fmt.Sprintf("route src-track rule %s not found", srcIPsetRuleName))
		} else {
			srcConnTrackIPset[srcIPsetRuleName] = srcTrackRule
		}
		destIPsetRuleName := dns_feature.DynamicIPSetDNSCircuitConnTrackDestPrefix + outTag
		if destTrackRule := s.dynRouter.GetDynamicRuleIP(destIPsetRuleName); destTrackRule == nil {
			return newError(fmt.Sprintf("route dest-track rule %s not found", destIPsetRuleName))
		} else {
			destConnTrackIPset[destIPsetRuleName] = destTrackRule
		}

		rtCtx = routing_dns.ContextWithSkippingDynamicRule(rtCtx, destIPsetRuleName, "")
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.ob = &observer{
		c:                    s,
		dynDefaultDestRuleIP: dynDestIPset,
		dynSrcRuleIP:         srcConnTrackIPset,
		dynDestRuleIP:        destConnTrackIPset,
		ctx:                  ctx,
		cancel:               cancel,
		obevQ:                make(chan observedEvent, 100),
	}
	s.ob.initObStatGC()
	// start proc
	{
		s.ob.wg.Add(1)
		go s.ob.procLoop()
	}
	return nil
}

func (s *observer) stop() {
	s.cancel()
	s.wg.Wait()
}

type observedEvent struct {
	from net.Destination
	d    string
	res  []net.IP
}

func joinUpdater(us ...signal.ActivityUpdater) signal.ActivityUpdater {
	ret := proxy.NewActivityUpdater()
	for _, u := range us {
		if u != nil {
			ret.Add(u)
		}
	}
	return ret
}

func (s *dnsCircuit) observeInboundOnRequest(from, to net.Destination) signal.ActivityUpdater {
	return joinUpdater(s.ob.updateConnTrackByConnectionActivity(from, to), s.ob.updateDestMetaByConnectionActivity(to))
}

func (s *dnsCircuit) observeInboundOnResponse(from, to net.Destination) signal.ActivityUpdater {
	return joinUpdater(s.ob.updateConnTrackByConnectionActivity(from, to), s.ob.updateDestMetaByConnectionActivity(to))
}

func (s *observer) updateDestMetaByConnectionActivity(to net.Destination) signal.ActivityUpdater {
	maskedIP := to.Address.IP().Mask(defaultMask)
	k := kFromIPAndMask(maskedIP, defaultMask)
	s.obDestStatRw.RLock()
	defer s.obDestStatRw.RUnlock()
	meta, ok := s.obDestStat[k]
	if !ok || meta.isPersistent {
		return nil
	}
	return meta
}

func (s *observer) updateConnTrackByConnectionActivity(from, to net.Destination) signal.ActivityUpdater {
	if from.Address.Family() != net.AddressFamilyIPv4 {
		return nil
	}
	k := connTrackKey(from)
	s.obConnTrackStatRw.RLock()
	defer s.obConnTrackStatRw.RUnlock()
	meta, ok := s.obConnTrackStat[k]
	if !ok {
		return nil
	}
	destK := kFromIPAndMask(to.Address.IP().Mask(defaultMask), defaultMask)
	meta.rw.RLock()
	defer meta.rw.RUnlock()
	dMeta, ok := meta.destMeta[destK]
	if !ok {
		return nil
	}
	return dMeta
}

func (s *dnsCircuit) observeDNSOutBound(ctx context.Context, domain string, result []net.IP, err error) {
	if err != nil {
		return
	}
	in := session.InboundFromContext(ctx)
	if in == nil {
		newError(fmt.Sprintf("unexpected nil inbound from context. domain: %s", domain)).
			AtWarning().WriteToLog()
		return
	}
	s.ob.updateConnTrackByDNS(in, domain, result)
}

func (s *observer) updateConnTrackByDNS(sIn *session.Inbound, domain string, result []net.IP) {
	select {
	case s.obevQ <- observedEvent{
		from: sIn.Source,
		d:    domain,
		res:  result,
	}:
	default:
		newError(fmt.Sprintf("dropped observed DNS event from %s domain: %s", sIn.Source.String(), domain)).
			AtWarning().WriteToLog()
	}
}

func (s *observer) initObStatGC() {
	s.obDestStat = make(map[obDestMetaKey]*obDestMeta)
	s.obConnTrackStat = make(map[netip.Addr]*obConnTrackMeta)
	s.obStatGCTicker = ospf.TimeTickerFunc(s.ctx, time.Minute, s.doObStatGC)
}

func (s *observer) doObStatGC() {
	var (
		revokeIPs   []net.IPNet
		revokedKeys []obDestMetaKey
	)
	defer func() {
		if len(revokeIPs) > 0 {
			s.removeDefaultDestRule(revokeIPs)
			s.c.ospf.RevokeASBRRoute(revokeIPs)
		}
	}()

	s.obDestStatRw.Lock()
	for k, meta := range s.obDestStat {
		if !meta.isPersistent && meta.durSinceLastAnnounce() >= s.c.inactiveClean {
			ipNet := k.ipNet()
			revokeIPs = append(revokeIPs, ipNet)
			revokedKeys = append(revokedKeys, k)
			meta.isOutdated = true
			ospf.LogImportant("revoking domain: %s due to %s inactive. route CIDR: %s",
				meta.domain, s.c.inactiveClean.String(), PrettyPrintIPNet(ipNet))
		}
	}
	for _, k := range revokedKeys {
		delete(s.obDestStat, k)
	}
	s.obDestStatRw.Unlock()

	var (
		revokeDestTracks   = make(map[string][]net.IPNet)
		emptyClients       []netip.Addr
		emptyClientsIPnets []net.IPNet
	)
	s.obConnTrackStatRw.Lock()
	for clientAddr, clientMeta := range s.obConnTrackStat {
		clientMeta.rw.Lock()
		var (
			toDeleteDest []obDestMetaKey
		)
		for destIPNet, destMeta := range clientMeta.destMeta {
			if destMeta.g.isOutdated || destMeta.durSinceLastAccess() >= s.c.inactiveClean {
				tag := destMeta.getOutTag()
				ips := revokeDestTracks[tag]
				ips = append(ips, destIPNet.ipNet())
				revokeDestTracks[tag] = ips
				toDeleteDest = append(toDeleteDest, destIPNet)
			}
		}
		for _, d := range toDeleteDest {
			delete(clientMeta.destMeta, d)
		}
		if len(clientMeta.destMeta) == 0 {
			emptyClients = append(emptyClients, clientAddr)
			emptyClientsIPnets = append(emptyClientsIPnets, net.IPNet{
				IP: clientAddr.AsSlice(), Mask: maskHost,
			})
		}
		clientMeta.rw.Unlock()
		for tag, ips := range revokeDestTracks {
			if len(ips) <= 0 {
				continue
			}
			ospf.LogImportant("revoking conn-track ip rule for outbound:%s due to %s inactive: %s -> %s",
				tag, s.c.inactiveClean.String(), clientAddr.String(), PrettyPrintIPNet(ips...))
			s.dynDestRuleIP[dns_feature.DynamicIPSetDNSCircuitConnTrackDestPrefix+tag].RemoveIPNetConnTrack(clientAddr.AsSlice(), ips...)
		}
		clear(revokeDestTracks)
	}
	// clear clients with empty dest rules
	for _, client := range emptyClients {
		delete(s.obConnTrackStat, client)
	}
	s.obConnTrackStatRw.Unlock()
	// clear clients src rule with empty dest rules
	if len(emptyClientsIPnets) > 0 {
		for _, srcRule := range s.dynSrcRuleIP {
			srcRule.RemoveIPNet(emptyClientsIPnets...)
		}
	}
}

func (s *observer) doObStatUpdate(ipNets []net.IPNet, e observedEvent) (canSendUpdates []net.IPNet, metas []*obDestMeta) {
	s.obDestStatRw.Lock()
	defer s.obDestStatRw.Unlock()
	var (
		isUpdate         = false
		isUpdatedAndSent = false
	)
	const (
		updateInterval = 1 * time.Minute
	)
	for _, ipNet := range ipNets {
		k := kFromIPAndMask(ipNet.IP, ipNet.Mask)
		if meta, ok := s.obDestStat[k]; ok {
			metas = append(metas, meta)
			if meta.isPersistent {
				continue
			}
			isUpdate = true
			// same CIDR are not sent when seen within updateInterval
			if meta.durSinceLastAnnounce() > updateInterval {
				isUpdatedAndSent = true
				canSendUpdates = append(canSendUpdates, ipNet)
				meta.updateLastAnnounced(e.d)
			}
		} else {
			meta := &obDestMeta{
				lastAnnounced: time.Now(),
				domain:        e.d,
			}
			s.obDestStat[k] = meta
			canSendUpdates = append(canSendUpdates, ipNet)
			metas = append(metas, meta)
		}
	}
	if isUpdate {
		if isUpdatedAndSent {
			newError(fmt.Sprintf("updating domain: %s route CIDR: %s", e.d, PrettyPrintIPNet(ipNets...))).
				AtDebug().WriteToLog()
		}
	} else {
		ospf.LogImportant("adding domain: %v route CIDR: %s", e.d, PrettyPrintIPNet(ipNets...))
	}
	return
}

func (s *observer) procLoop() {
	for {
		select {
		case <-s.ctx.Done():
			s.wg.Done()
			return
		case event := <-s.obevQ:
			s.procEvent(event)
		}
	}
}

type routingCtx struct {
	routing.Context
}

func (rc routingCtx) GetSkipDNSResolve() bool {
	// we already have ip. so always skip DNS resolve
	return true
}

var (
	pbCtx = &router_commands.RoutingContext{
		InboundTag: "transparent",
		Network:    net.Network_TCP,
		SourceIPs: [][]byte{{
			127, 0, 0, 1,
		}},
		TargetIPs:    make([][]byte, 0, 10), // filled later
		SourcePort:   12345,
		TargetPort:   80,
		TargetDomain: "",     // filled later
		Protocol:     "http", // assume http
	}
	origRtCtx = routing_dns.ContextWithSkippingDynamicRule(routingCtx{router_commands.AsRoutingContext(pbCtx)},
		dns_feature.DynamicIPSetDnsCircuitDestDefault, "")
	rtCtx = origRtCtx
)

var defaultMask = net.CIDRMask(32, 32)

func (s *observer) procEvent(e observedEvent) {
	var (
		qualifiedIP []net.IP
		finalIP     []net.IP
		finalTag    string
	)
	pbCtx.TargetDomain = e.d
	pbCtx.TargetIPs = pbCtx.TargetIPs[0:0]
	for _, r := range e.res {
		if r.To4() == nil {
			continue
		}
		qualifiedIP = append(qualifiedIP, r)
		pbCtx.TargetIPs = append(pbCtx.TargetIPs, r.To4())
	}
	if len(qualifiedIP) <= 0 {
		return
	}
	pickRt, err := s.c.dynRouter.PickRoute(rtCtx)
	if err != nil {
		if errors.Is(err, common.ErrNoClue) {
			finalTag = s.c.ohm.GetDefaultHandler().Tag()
			newError(fmt.Sprintf("default outbound:%s for domain: %s ip: %v", finalTag, e.d, qualifiedIP)).
				AtWarning().WriteToLog()
		} else {
			newError(fmt.Sprintf("err pick route for domain: %s ip: %v", e.d, qualifiedIP)).
				Base(err).AtWarning().WriteToLog()
			return
		}
	} else {
		finalTag = pickRt.GetOutboundTag()
	}
	if s.c.expectTags[finalTag] {
		for _, r := range qualifiedIP {
			finalIP = append(finalIP, r.Mask(defaultMask))
		}
	}
	if len(finalIP) > 0 {
		ipNets := make([]stdnet.IPNet, 0, len(finalIP))
		for _, ip := range finalIP {
			ipNets = append(ipNets, stdnet.IPNet{
				IP: ip, Mask: defaultMask,
			})
		}
		needUpdateIPNets, metas := s.doObStatUpdate(ipNets, e)
		s.addConnTrack(e, ipNets, metas, finalTag)
		if len(needUpdateIPNets) > 0 {
			s.c.ospf.AnnounceASBRRoute(needUpdateIPNets)
		}
	}
}

var (
	maskHost = net.CIDRMask(32, 32)
)

func (s *observer) addConnTrack(e observedEvent, dests []net.IPNet, metas []*obDestMeta, tag string) {
	newError(fmt.Sprintf("adding default dynamic dest ip rule in %s: %s",
		dns_feature.DynamicIPSetDnsCircuitDestDefault, PrettyPrintIPNet(dests...))).
		AtDebug().WriteToLog()
	// default dest rule
	s.dynDefaultDestRuleIP.AddIPNet(dests...)
	// do conn track
	s.obConnTrackStatRw.Lock()
	defer s.obConnTrackStatRw.Unlock()
	var (
		toAddDest []net.IPNet
	)
	cMeta, ok := s.obConnTrackStat[connTrackKey(e.from)]
	if ok {
		cMeta.rw.Lock()
		defer cMeta.rw.Unlock()
		if cMeta.destMeta == nil {
			cMeta.destMeta = make(map[obDestMetaKey]*connTrackDestMeta)
		}
		for i, dest := range dests {
			if metas[i].isPersistent {
				continue
			}
			destK := kFromIPAndMask(dest.IP, dest.Mask)
			destM, exist := cMeta.destMeta[destK]
			if !exist {
				cMeta.destMeta[destK] = &connTrackDestMeta{
					src:          connTrackKey(e.from),
					g:            metas[i],
					lastAccessed: time.Now(),
					domain:       e.d,
					outboundTag:  tag,
				}
				toAddDest = append(toAddDest, dest)
			} else {
				destM.updateLastAccessed(e.d)
				if prevOutTag := destM.getOutTag(); prevOutTag != tag {
					ospf.LogImportant("updating conn-track src %s ip rule from outbound:%s to outbound:%s domain: %s",
						e.from.Address.String(), prevOutTag, tag, e.d)
					s.dynDestRuleIP[dns_feature.DynamicIPSetDNSCircuitConnTrackDestPrefix+prevOutTag].RemoveIPNetConnTrack(e.from.Address.IP(), destK.ipNet())
					destM.setOutTag(tag)
					toAddDest = append(toAddDest, dest)
				}
			}
		}
	} else {
		cMeta = &obConnTrackMeta{
			destMeta: make(map[obDestMetaKey]*connTrackDestMeta),
		}
		for i, dest := range dests {
			if metas[i].isPersistent {
				continue
			}
			destK := kFromIPAndMask(dest.IP, dest.Mask)
			cMeta.destMeta[destK] = &connTrackDestMeta{
				src:          connTrackKey(e.from),
				g:            metas[i],
				lastAccessed: time.Now(),
				domain:       e.d,
				outboundTag:  tag,
			}
			toAddDest = append(toAddDest, dest)
		}
		s.obConnTrackStat[connTrackKey(e.from)] = cMeta
		newError(fmt.Sprintf("adding dynamic src %s conn-track rules in %s",
			e.from.Address.IP().String(), dns_feature.DynamicIPSetDNSCircuitConnTrackSrcPrefix+tag)).
			AtDebug().WriteToLog()
		s.dynSrcRuleIP[dns_feature.DynamicIPSetDNSCircuitConnTrackSrcPrefix+tag].AddIPNet(net.IPNet{
			IP: e.from.Address.IP(), Mask: maskHost,
		})
	}

	if len(toAddDest) > 0 {
		ospf.LogImportant("adding conn-track ip rule for outbound:%s domain: %s : %s -> %s",
			tag, e.d, e.from.Address.String(), PrettyPrintIPNet(toAddDest...))
		s.dynDestRuleIP[dns_feature.DynamicIPSetDNSCircuitConnTrackDestPrefix+tag].AddIPNetConnTrack(e.from.Address.IP(), toAddDest...)
	}
}

func (s *observer) removeDefaultDestRule(dest []net.IPNet) {
	newError(fmt.Sprintf("revoking default dynamic dest ip rule in %s: %s",
		dns_feature.DynamicIPSetDnsCircuitDestDefault, PrettyPrintIPNet(dest...))).
		AtDebug().WriteToLog()
	// default dest rule
	s.dynDefaultDestRuleIP.RemoveIPNet(dest...)
}
