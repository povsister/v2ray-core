package dnscircuit

import (
	"context"
	"fmt"
	"time"

	"github.com/v2fly/v2ray-core/v5/app/dnscircuit/ospf"
	"github.com/v2fly/v2ray-core/v5/app/router/routercommon"
	"github.com/v2fly/v2ray-core/v5/common/net"
	"github.com/v2fly/v2ray-core/v5/features/inbound"
	"github.com/v2fly/v2ray-core/v5/features/outbound"
	"github.com/v2fly/v2ray-core/v5/features/routing"
	"github.com/v2fly/v2ray-core/v5/proxy"
)

type dnsCircuit struct {
	dynRouter  routing.RouterWithDynamicRule
	expectTags map[string]bool

	inboundTags []string
	ihm         inbound.Manager
	obIns       []proxy.ActivityObservableInbound
	dnsOutTag   string
	ohm         outbound.Manager
	obDNSOut    proxy.ObservableDNSOutBound

	ospfIfName    string
	ospfIfAddr    net.IPNet
	ospfRtId      string
	ospf          *ospf.Router
	inactiveClean time.Duration

	ob              *observer
	persistentRoute []*routercommon.GeoIP
}

const (
	inBoundObserver     = "dnscircuit"
	outboundDNSObserver = "dnscircuit"
)

func (s *dnsCircuit) Init(ctx context.Context, c *Config, router routing.Router, ihm inbound.Manager, ohm outbound.Manager) (err error) {
	// outbound matching tags
	s.expectTags = make(map[string]bool, len(c.OutboundTags))
	for _, tag := range c.OutboundTags {
		s.expectTags[tag] = true
	}
	// confirm router capable
	dynRouter, ok := router.(routing.RouterWithDynamicRule)
	if !ok {
		return newError("router is not capable for dynamic rule")
	}
	s.dynRouter = dynRouter

	// ospf init
	leadingOnes := c.GetOspfSetting().GetAddress().GetPrefix()
	ifIP := c.GetOspfSetting().GetAddress().GetIp()
	ifMask := net.CIDRMask(int(leadingOnes), 32)
	s.ospfIfAddr = net.IPNet{
		IP:   ifIP,
		Mask: ifMask,
	}
	rt, err := ospf.NewRouter(c.OspfSetting.GetIfName(), &s.ospfIfAddr, net.IP(ifIP).String())
	if err != nil {
		return newError("err init ospf router instance").Base(err)
	}
	s.ospf = rt
	s.inactiveClean = time.Duration(c.InactiveClean) * time.Second

	// all other fields
	s.persistentRoute = c.PersistentRoute
	s.ohm = ohm
	s.ihm = ihm
	s.inboundTags = c.InboundTags
	s.dnsOutTag = c.DnsOutboundTag

	return nil
}

func (s *dnsCircuit) Type() interface{} {
	return (*dnsCircuit)(nil)
}

// Start implements common.Runnable.
func (s *dnsCircuit) Start() error {
	if err := s.initObservableInbounds(); err != nil {
		return err
	}
	if err := s.initObservableDNSOutbounds(); err != nil {
		return err
	}
	if err := s.initObserver(); err != nil {
		return err
	}
	s.ospf.Start()
	s.initPersistentRoute()
	return nil
}

func (s *dnsCircuit) initObservableInbounds() error {
	// confirm given inbound is observable
	for _, tag := range s.inboundTags {
		h, err := s.ihm.GetHandler(context.TODO(), tag)
		if err != nil {
			return newError(fmt.Sprintf("failed to get inbound %q handler", tag)).Base(err)
		}
		gh, ok := h.(proxy.GetInbound)
		if !ok {
			return newError(fmt.Sprintf("inbound handler %q is not a proxy.GetInbound", tag))
		}
		obIn, ok := gh.GetInbound().(proxy.ActivityObservableInbound)
		if !ok {
			return newError(fmt.Sprintf("inbound handler %q does not have a proxy.ActivityObservableInbound", tag))
		}
		obIn.RegisterActivityObserver(inBoundObserver, s.observeInboundOnRequest, s.observeInboundOnResponse)
		s.obIns = append(s.obIns, obIn)
	}
	return nil
}

func (s *dnsCircuit) initObservableDNSOutbounds() error {
	h := s.ohm.GetHandler(s.dnsOutTag)
	if h == nil {
		return newError(fmt.Sprintf("can not get outbound handler %q", s.dnsOutTag))
	}
	gh, ok := h.(proxy.GetOutbound)
	if !ok {
		return newError(fmt.Sprintf("outbound handler %q is not a proxy.GetOutbound", s.dnsOutTag))
	}
	obDNSOut, ok := gh.GetOutbound().(proxy.ObservableDNSOutBound)
	if !ok {
		return newError(fmt.Sprintf("outbound handler %q is not a proxy.ObservableDNSOutBound", s.dnsOutTag))
	}
	obDNSOut.RegisterDNSOutBoundObserver(outboundDNSObserver, s.observeDNSOutBound)
	s.obDNSOut = obDNSOut
	return nil
}

// Close implements common.Closable.
func (s *dnsCircuit) Close() error {
	for _, ob := range s.obIns {
		ob.UnregisterActivityObserver(inBoundObserver)
	}
	if s.obDNSOut != nil {
		s.obDNSOut.UnregisterDNSOutBoundObserver(outboundDNSObserver)
	}
	s.ob.stop()
	return s.ospf.Close()
}
