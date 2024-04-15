package ospf

import (
	"net"
	"sync"

	"golang.org/x/net/context"
)

type Router struct {
	startOnce sync.Once
	ctx       context.Context
	cancel    context.CancelFunc

	closeErr  error
	closeOnce sync.Once

	// global parameters
	// per RFC2328 C.1
	routerId uint32
	// Controls the preference rules used in Section 16.4 when
	//            choosing among multiple AS-external-LSAs advertising the
	//            same destination. When set to "enabled", the preference
	//            rules remain those specified by RFC 1583 ([Ref9]). When set
	//            to "disabled", the preference rules are those stated in
	//            Section 16.4.1, which prevent routing loops when AS-
	//            external-LSAs for the same destination have been originated
	//            from different areas. Set to "enabled" by default.
	//
	//            In order to minimize the chance of routing loops, all OSPF
	//            routers in an OSPF routing domain should have
	//            RFC1583Compatibility set identically. When there are routers
	//            present that have not been updated with the functionality
	//            specified in Section 16.4.1 of this memo, all routers should
	//            have RFC1583Compatibility set to "enabled". Otherwise, all
	//            routers should have RFC1583Compatibility set to "disabled",
	//            preventing all routing loops.
	rfc1583Compatibility bool
	// ospf2 instance
	ins *Instance
}

func NewRouter(ifName string, addr *net.IPNet, rtid string) (*Router, error) {
	ctx, cancel := context.WithCancel(context.Background())
	r := &Router{
		ctx:    ctx,
		cancel: cancel,
		ins: NewInstance(ctx, &InstanceConfig{
			RouterId:           ipv4BytesToUint32(net.ParseIP(rtid).To4()[0:4]),
			HelloInterval:      10,
			RouterDeadInterval: 40,
			Network:            addr,
			IfName:             ifName,
			ASBR:               true,
		}),
	}
	r.routerId = r.ins.RouterId
	return r, nil
}

func (r *Router) Start() {
	r.startOnce.Do(func() {
		r.ins.start()
	})
}

func (r *Router) Close() (err error) {
	r.closeOnce.Do(func() {
		if r.cancel != nil {
			r.cancel()
		}
		r.ins.shutdown()
	})
	return r.closeErr
}

func (r *Router) AnnounceASBRRoute(ips []net.IPNet) {
	r.ins.addASBRLSA(ips...)
}

func (r *Router) RevokeASBRRoute(ips []net.IPNet) {
	r.ins.delASBRLSA(ips...)
}
