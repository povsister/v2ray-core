package ospf

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"golang.org/x/net/ipv4"

	"github.com/v2fly/v2ray-core/v5/app/dnscircuit/ospf/packet"
)

type InterfaceConfig struct {
	IfName             string
	Address            *net.IPNet
	RouterPriority     uint8
	HelloInterval      uint16
	RouterDeadInterval uint32
}

func NewInterface(ctx context.Context, c *InterfaceConfig) *Interface {
	ifi, err := net.InterfaceByName(c.IfName)
	if err != nil {
		panic(fmt.Errorf("can not find InterfaceByName: %w", err))
	}
	conn, err := ListenOSPFv2Multicast(ctx, ifi, "0.0.0.0", c.Address.IP.String())
	if err != nil {
		panic(fmt.Errorf("can not bind OSPFv2 multicast conn: %w", err))
	}
	ctx, cancel := context.WithCancel(ctx)
	ret := &Interface{
		ctx:                ctx,
		cancel:             cancel,
		c:                  conn,
		pendingProcessPkt:  make(chan recvPkt, 100),
		pendingSendPkt:     make(chan sendPkt, 100),
		Type:               IfTypeBroadcast,
		MTU:                1500,
		Address:            c.Address,
		RouterPriority:     c.RouterPriority,
		HelloInterval:      c.HelloInterval,
		RouterDeadInterval: c.RouterDeadInterval,
		Neighbors:          make(map[uint32]*Neighbor),
		OutputCost:         10,
		RxmtInterval:       5,
		InfTransDelay:      1,
	}
	ret.consumeEvent(IfEvInterfaceUp)
	return ret
}

type recvPkt struct {
	h *ipv4.Header
	p []byte
}

type sendPkt struct {
	dst uint32
	p   packet.SerializableLayerLayerWithType // ospf2 msg
}

type InterfaceState uint8

const (
	// InterfaceDown This is the initial interface state.  In this state, the
	//            lower-level protocols have indicated that the interface is
	//            unusable.  No protocol traffic at all will be sent or
	//            received on such a interface.  In this state, interface
	//            parameters should be set to their initial values.  All
	//            interface timers should be disabled, and there should be no
	//            adjacencies associated with the interface.
	InterfaceDown InterfaceState = iota
	// InterfaceLoopBack In this state, the router's interface to the network is
	//            looped back.  The interface may be looped back in hardware
	//            or software.  The interface will be unavailable for regular
	//            data traffic.  However, it may still be desirable to gain
	//            information on the quality of this interface, either through
	//            sending ICMP pings to the interface or through something
	//            like a bit error test.  For this reason, IP packets may
	//            still be addressed to an interface in Loopback state.  To
	//            facilitate this, such interfaces are advertised in router-
	//            LSAs as single host routes, whose destination is the IP
	//            interface address.[4]
	InterfaceLoopBack
	// InterfaceWaiting In this state, the router is trying to determine the
	//            identity of the (Backup) Designated Router for the network.
	//            To do this, the router monitors the Hello Packets it
	//            receives.  The router is not allowed to elect a Backup
	//            Designated Router nor a Designated Router until it
	//            transitions out of Waiting state.  This prevents unnecessary
	//            changes of (Backup) Designated Router.
	InterfaceWaiting
	// InterfacePointToPoint In this state, the interface is operational, and connects
	//            either to a physical point-to-point network or to a virtual
	//            link.  Upon entering this state, the router attempts to form
	//            an adjacency with the neighboring router.  Hello Packets are
	//            sent to the neighbor every HelloInterval seconds.
	InterfacePointToPoint
	// InterfaceDROther The interface is to a broadcast or NBMA network on which
	//            another router has been selected to be the Designated
	//            Router.  In this state, the router itself has not been
	//            selected Backup Designated Router either.  The router forms
	//            adjacencies to both the Designated Router and the Backup
	//            Designated Router (if they exist).
	InterfaceDROther
	// InterfaceBackup In this state, the router itself is the Backup Designated
	//            Router on the attached network.  It will be promoted to
	//            Designated Router when the present Designated Router fails.
	//            The router establishes adjacencies to all other routers
	//            attached to the network.  The Backup Designated Router
	//            performs slightly different functions during the Flooding
	//            Procedure, as compared to the Designated Router (see Section
	//            13.3).  See Section 7.4 for more details on the functions
	//            performed by the Backup Designated Router.
	InterfaceBackup
	// InterfaceDR In this state, this router itself is the Designated Router
	//            on the attached network.  Adjacencies are established to all
	//            other routers attached to the network.  The router must also
	//            originate a network-LSA for the network node.  The network-
	//            LSA will contain links to all routers (including the
	//            Designated Router itself) attached to the network.  See
	//            Section 7.3 for more details on the functions performed by
	//            the Designated Router.
	InterfaceDR
)

type InterfaceStateChangingEvent int

const (
	_ InterfaceStateChangingEvent = iota
	// IfEvInterfaceUp Lower-level protocols have indicated that the network
	//            interface is operational.  This enables the interface to
	//            transition out of Down state.  On virtual links, the
	//            interface operational indication is actually a result of the
	//            shortest path calculation (see Section 16.7).
	IfEvInterfaceUp
	// IfEvWaitTimer The Wait Timer has fired, indicating the end of the waiting
	//            period that is required before electing a (Backup)
	//            Designated Router.
	IfEvWaitTimer
	// IfEvBackupSeen The router has detected the existence or non-existence of a
	//            Backup Designated Router for the network.  This is done in
	//            one of two ways.  First, an Hello Packet may be received
	//            from a neighbor claiming to be itself the Backup Designated
	//            Router.  Alternatively, an Hello Packet may be received from
	//            a neighbor claiming to be itself the Designated Router, and
	//            indicating that there is no Backup Designated Router.  In
	//            either case there must be bidirectional communication with
	//            the neighbor, i.e., the router must also appear in the
	//            neighbor's Hello Packet.  This event signals an end to the
	//            Waiting state.
	IfEvBackupSeen
	// IfEvNeighborChange There has been a change in the set of bidirectional
	//            neighbors associated with the interface.  The (Backup)
	//            Designated Router needs to be recalculated.  The following
	//            neighbor changes lead to the NeighborChange event.  For an
	//            explanation of neighbor states, see Section 10.1.
	//
	//            o   Bidirectional communication has been established to a
	//                neighbor.  In other words, the state of the neighbor has
	//                transitioned to 2-Way or higher.
	//
	//            o   There is no longer bidirectional communication with a
	//                neighbor.  In other words, the state of the neighbor has
	//                transitioned to Init or lower.
	//
	//            o   One of the bidirectional neighbors is newly declaring
	//                itself as either Designated Router or Backup Designated
	//                Router.  This is detected through examination of that
	//                neighbor's Hello Packets.
	//
	//            o   One of the bidirectional neighbors is no longer
	//                declaring itself as Designated Router, or is no longer
	//                declaring itself as Backup Designated Router.  This is
	//                again detected through examination of that neighbor's
	//                Hello Packets.
	//
	//            o   The advertised Router Priority for a bidirectional
	//                neighbor has changed.  This is again detected through
	//                examination of that neighbor's Hello Packets.
	IfEvNeighborChange
	// IfEvLoopInd An indication has been received that the interface is now
	//            looped back to itself.  This indication can be received
	//            either from network management or from the lower level
	//            protocols.
	IfEvLoopInd
	// IfEvUnLoopInd An indication has been received that the interface is no
	//            longer looped back.  As with the LoopInd event, this
	//            indication can be received either from network management or
	//            from the lower level protocols.
	IfEvUnLoopInd
	// IfEvInterfaceDown Lower-level protocols indicate that this interface is no
	//            longer functional.  No matter what the current interface
	//            state is, the new interface state will be Down.
	IfEvInterfaceDown
)

type InterfaceType uint8

const (
	_ InterfaceType = iota
	IfTypePointToPoint
	IfTypeBroadcast
	IfTypeNBMA
	IfTypePointToMultiPoint
	IfTypeVirtualLink
)

type Interface struct {
	// internal use

	c      *Conn
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	pendingProcessPkt chan recvPkt
	pendingSendPkt    chan sendPkt

	// The OSPF interface type is either point-to-point, broadcast,
	//        NBMA, Point-to-MultiPoint or virtual link.
	// Not actually used yet.
	Type InterfaceType
	MTU  uint16
	// The functional level of an interface.  State determines whether
	//        or not full adjacencies are allowed to form over the interface.
	//        State is also reflected in the router's LSAs.
	State InterfaceState
	// The IP address associated with the interface.  This appears as
	//        the IP source address in all routing protocol packets originated
	//        over this interface.  Interfaces to unnumbered point-to-point
	//        networks do not have an associated IP address.
	// Also referred to as the subnet mask, this indicates the portion
	//        of the IP interface address that identifies the attached
	//        network.  Masking the IP interface address with the IP interface
	//        mask yields the IP network number of the attached network.  On
	//        point-to-point networks and virtual links, the IP interface mask
	//        is not defined. On these networks, the link itself is not
	//        assigned an IP network number, and so the addresses of each side
	//        of the link are assigned independently, if they are assigned at
	//        all.
	Address *net.IPNet
	// The Area ID of the area to which the attached network belongs.
	//        All routing protocol packets originating from the interface are
	//        labelled with this Area ID.
	Area *Area

	// An 8-bit unsigned integer.  When two routers attached to a
	//        network both attempt to become Designated Router, the one with
	//        the highest Router Priority takes precedence.  A router whose
	//        Router Priority is set to 0 is ineligible to become Designated
	//        Router on the attached network.  Advertised in Hello packets
	//        sent out this interface.
	RouterPriority uint8

	// The length of time, in seconds, between the Hello packets that
	//        the router sends on the interface.  Advertised in Hello packets
	//        sent out this interface.
	HelloInterval uint16
	// An interval timer that causes the interface to send a Hello
	//        packet.  This timer fires every HelloInterval seconds.  Note
	//        that on non-broadcast networks a separate Hello packet is sent
	//        to each qualified neighbor.
	HelloTicker *TickerFunc
	// The number of seconds before the router's neighbors will declare
	//        it down, when they stop hearing the router's Hello Packets.
	//        Advertised in Hello packets sent out this interface.
	RouterDeadInterval uint32
	// A single shot timer that causes the interface to exit the
	//        Waiting state, and as a consequence select a Designated Router
	//        on the network.  The length of the timer is RouterDeadInterval
	//        seconds.
	WaitTimer *time.Timer
	// The Designated Router selected for the attached network.  The
	//        Designated Router is selected on all broadcast and NBMA networks
	//        by the Hello Protocol.  Two pieces of identification are kept
	//        for the Designated Router: its Router ID and its IP interface
	//        address on the network.  The Designated Router advertises link
	//        state for the network; this network-LSA is labelled with the
	//        Designated Router's IP address.  The Designated Router is
	//        initialized to 0.0.0.0, which indicates the lack of a Designated
	//        Router.
	DR atomic.Uint32
	// The Backup Designated Router is also selected on all broadcast
	//        and NBMA networks by the Hello Protocol.  All routers on the
	//        attached network become adjacent to both the Designated Router
	//        and the Backup Designated Router.  The Backup Designated Router
	//        becomes Designated Router when the current Designated Router
	//        fails.  The Backup Designated Router is initialized to 0.0.0.0,
	//        indicating the lack of a Backup Designated Router.
	BDR atomic.Uint32

	// guards Neighbors
	nbMu sync.RWMutex
	// The other routers attached to this network.  This list is formed
	//        by the Hello Protocol.  Adjacencies will be formed to some of
	//        these neighbors.  The set of adjacent neighbors can be
	//        determined by an examination of all of the neighbors' states.
	Neighbors map[uint32]*Neighbor

	// The cost of sending a packet on the interface, expressed in
	//            the link state metric.  This is advertised as the link cost
	//            for this interface in the router's router-LSA. The interface
	//            output cost must always be greater than 0.
	OutputCost int

	// The number of seconds between LSA retransmissions, for
	//            adjacencies belonging to this interface.  Also used when
	//            retransmitting Database Description and Link State Request
	//            Packets.  This should be well over the expected round-trip
	//            delay between any two routers on the attached network.  The
	//            setting of this value should be conservative or needless
	//            retransmissions will result.  Sample value for a local area
	//            network: 5 seconds.
	RxmtInterval int

	// The estimated number of seconds it takes to transmit a Link
	//            State Update Packet over this interface.  LSAs contained in
	//            the update packet must have their age incremented by this
	//            amount before transmission.  This value should take into
	//            account the transmission and propagation delays of the
	//            interface.  It must be greater than 0.  Sample value for a
	//            local area network: 1 second.
	InfTransDelay uint16

	// TODO
	AuType string
	// TODO
	Authentication string
}

func (i *Interface) shouldCheckNeighborNetworkMask() bool {
	return i.Type != IfTypePointToPoint && i.Type != IfTypeVirtualLink
}

func (i *Interface) shouldHaveDR() bool {
	return i.Type == IfTypeBroadcast || i.Type == IfTypeNBMA
}

func (i *Interface) changeDRAndBDR(dr, bdr uint32) (changed bool) {
	oldDR := i.DR.Load()
	oldBDR := i.BDR.Load()
	if dr == oldDR && bdr == oldBDR {
		return false
	}
	return i.DR.CompareAndSwap(oldDR, dr) || i.BDR.CompareAndSwap(oldBDR, bdr)
}

func (i *Interface) currState() InterfaceState {
	return i.State
}

func (i *Interface) transState(target InterfaceState) {
	i.State = target
}

func (i *Interface) consumeEvent(e InterfaceStateChangingEvent) {
	switch e {
	case IfEvInterfaceUp:
		if i.currState() == InterfaceDown {
			// Start the interval Hello Timer, enabling the
			// periodic sending of Hello packets out the interface.
			i.runHelloTicker()
			switch i.Type {
			case IfTypePointToPoint, IfTypePointToMultiPoint, IfTypeVirtualLink:
				// If the attached network is a physical point-to-point
				// network, Point-to-MultiPoint network or virtual
				// link, the interface state transitions to Point-to-Point.
				i.transState(InterfacePointToPoint)
			default:
				if i.RouterPriority <= 0 {
					// Else, if the router is not eligible to
					// become Designated Router the interface state
					// transitions to DR Other.
					i.transState(InterfaceDROther)
				} else {
					// Otherwise, the attached network is a broadcast or
					// NBMA network and the router is eligible to become
					// Designated Router.  In this case, in an attempt to
					// discover the attached network's Designated Router
					// the interface state is set to Waiting and the single
					// shot Wait Timer is started.  Additionally, if the
					// network is an NBMA network examine the configured
					// list of neighbors for this interface and generate
					// the neighbor event Start for each neighbor that is
					// also eligible to become Designated Router.
				}
			}
		}
	case IfEvBackupSeen:
		if i.currState() == InterfaceWaiting {
			// Calculate the attached network's Backup Designated
			// Router and Designated Router, as shown in Section
			// 9.4.  As a result of this calculation, the new state
			// of the interface will be either DR Other, Backup or DR.
		}
	case IfEvWaitTimer:
		if i.currState() == InterfaceWaiting {
			// Calculate the attached network's Backup Designated
			// Router and Designated Router, as shown in Section
			// 9.4.  As a result of this calculation, the new state
			// of the interface will be either DR Other, Backup or DR.
		}
	case IfEvNeighborChange:
		switch i.currState() {
		case InterfaceDROther, InterfaceBackup, InterfaceDR:
			// Recalculate the attached network's Backup Designated
			// Router and Designated Router, as shown in Section
			// 9.4.  As a result of this calculation, the new state
			// of the interface will be either DR Other, Backup or DR.
		}
	case IfEvInterfaceDown:
		// All interface variables are reset, and interface
		// timers disabled.  Also, all neighbor connections
		// associated with the interface are destroyed.  This
		// is done by generating the event KillNbr on all
		// associated neighbors (see Section 10.2).
		i.transState(InterfaceDown)
		i.HelloTicker.Terminate()
		i.killAllNeighbor()
	case IfEvLoopInd:
		// Since this interface is no longer connected to the
		// attached network the actions associated with the
		// above InterfaceDown event are executed.
		i.transState(InterfaceLoopBack)
		i.HelloTicker.Terminate()
		i.killAllNeighbor()
	case IfEvUnLoopInd:
		if i.currState() == InterfaceLoopBack {
			// No actions are necessary.  For example, the
			// interface variables have already been reset upon
			// entering the Loopback state.  Note that reception of
			// an InterfaceUp event is necessary before the
			// interface again becomes fully functional.
			i.transState(InterfaceDown)
		}
	}
}

func (i *Interface) start() {
	i.runReadLoop()
	i.runSendLoop()
	i.runReadDispatchLoop()
}

func (i *Interface) close() error {
	i.cancel()
	i.wg.Wait()
	return i.c.Close()
}

func (i *Interface) runReadDispatchLoop() {
	i.wg.Add(1)
	go func() {
		for {
			select {
			case <-i.ctx.Done():
				logDebug("exiting %v runReadDispatchLoop", i.c.ifi.Name)
				i.wg.Done()
				return
			case pkt := <-i.pendingProcessPkt:
				i.doReadDispatch(pkt)
			}
		}
	}()
}

func (i *Interface) runReadLoop() {
	i.wg.Add(1)
	go func() {
		const recvBufLen = 64 << 10
		var (
			buf = make([]byte, recvBufLen)
			n   int
			h   *ipv4.Header
			err error
		)
		for {
			select {
			case <-i.ctx.Done():
				logDebug("exiting %v runReadLoop", i.c.ifi.Name)
				i.wg.Done()
				return
			default:
				n, h, err = i.c.Read(buf)
				if err != nil {
					if !errors.Is(err, os.ErrDeadlineExceeded) {
						logErr(err, "interface %s read err", i.c.ifi.Name)
					}
					continue
				}
				if h.Flags&ipv4.MoreFragments == 1 || h.FragOff != 0 {
					// TODO: deal with ipv4 fragment
					logWarn("interface %s received fragmented IPv4 packet %s->%s and discarded",
						i.c.ifi.Name, h.Src.String(), h.Dst.String())
					continue
				}
				payloadLen := n - ipv4.HeaderLen
				//if h != nil {
				//	logDebug("Received via Interface %s %s->%s payloadSize(%d)", i.c.ifi.Name, h.Src.String(), h.Dst.String(), payloadLen)
				//}
				payload := make([]byte, payloadLen)
				copy(payload, buf[ipv4.HeaderLen:n])
				select {
				case i.pendingProcessPkt <- recvPkt{h: h, p: payload}:
				default:
					logWarn("interface %s pendingProcPkt full. Discarding 1 pkt(%d)", i.c.ifi.Name, payloadLen)
				}
			}
		}
	}()
}

func (i *Interface) runSendLoop() {
	i.wg.Add(1)
	go func() {
		var err error
		for {
			select {
			case <-i.ctx.Done():
				logDebug("exiting %v runSendLoop", i.c.ifi.Name)
				i.wg.Done()
				return
			case pkt := <-i.pendingSendPkt:
				if err = i.doSendPkt(pkt); err != nil {
					// TODO: ?
				}
			}
		}
	}()
}

func (i *Interface) doSendPkt(pkt sendPkt) (err error) {
	dstIP := net.IPv4(byte(pkt.dst>>24), byte(pkt.dst>>16), byte(pkt.dst>>8), byte(pkt.dst))

	p := gopacket.NewSerializeBuffer()
	err = gopacket.SerializeLayers(p, gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}, pkt.p)
	if err != nil {
		logErr(err, "interface %s err marshal pending send %s->%s %v packet", i.c.ifi.Name,
			i.Address.IP.String(), dstIP.String(),
			pkt.p.GetType())
		return
	}

	_, err = i.c.WriteTo(p.Bytes(), &net.IPAddr{
		IP: dstIP,
	})
	if err != nil {
		logErr(err, "interface %s err send %s->%s %v packet", i.c.ifi.Name,
			i.Address.IP.String(), dstIP.String(),
			pkt.p.GetType())
	} else {
		if pkt.p.GetType() != layers.OSPFHello {
			logDebug("sent via interface %s %s->%s %+v", i.c.ifi.Name,
				i.Address.IP.String(), dstIP.String(),
				pkt.p)
		}
	}
	return
}

func (i *Interface) runHelloTicker() {
	i.HelloTicker.Terminate()
	i.HelloTicker = TimeTickerFunc(i.ctx, time.Duration(i.HelloInterval)*time.Second,
		func() {
			// directly writes the pkt and doNot enter queue.
			if err := i.doHello(); err != nil {
				// TODO: more aggressive retry ?
			}
		})
}

func (i *Interface) rangeOverNeighbors(fn func(nb *Neighbor) bool) {
	i.nbMu.RLock()
	defer i.nbMu.RUnlock()
	for _, nb := range i.Neighbors {
		if !fn(nb) {
			break
		}
	}
}

func (i *Interface) getNeighbor(rtId uint32) (nb *Neighbor, ok bool) {
	i.nbMu.RLock()
	defer i.nbMu.RUnlock()
	nb, ok = i.Neighbors[rtId]
	return
}

func (i *Interface) removeNeighbor(nb *Neighbor) {
	i.nbMu.Lock()
	defer i.nbMu.Unlock()
	nb.terminate()
	delete(i.Neighbors, nb.NeighborId)
	// we are not DR or BDR because of zero priority.
	// so remember to reset DR or BDR when neighbor disappear
	nbAddr := ipv4BytesToUint32(nb.NeighborAddress.To4())
	if nbAddr == i.DR.Load() || nbAddr == i.BDR.Load() {
		i.changeDRAndBDR(0, 0)
	}
}

func (i *Interface) killAllNeighbor() {
	i.nbMu.Lock()
	defer i.nbMu.Unlock()
	for _, nb := range i.Neighbors {
		nb.consumeEvent(NbEvKillNbr)
		nb.terminate()
	}
	clear(i.Neighbors)
}

func (i *Interface) addNeighbor(h *ipv4.Header, hello *packet.OSPFv2Packet[packet.HelloPayloadV2]) *Neighbor {
	ctx, cancel := context.WithCancel(i.ctx)
	nb := &Neighbor{
		ctx:              ctx,
		cancel:           cancel,
		i:                i,
		State:            NeighborDown,
		NeighborId:       hello.RouterID,
		NeighborPriority: hello.Content.RtrPriority,
		NeighborAddress:  h.Src,
		NeighborsDR:      hello.Content.DesignatedRouterID,
		NeighborsBDR:     hello.Content.BackupDesignatedRouterID,
		LSRetransmission: make(map[packet.LSAIdentity]struct{}),
	}
	i.nbMu.Lock()
	defer i.nbMu.Unlock()
	i.Neighbors[hello.RouterID] = nb
	return nb
}

func (i *Interface) sendDelayedLSAcks(lsacks []packet.LSAheader, dst uint32) {
	p := &packet.OSPFv2Packet[packet.LSAcknowledgementPayload]{
		OSPFv2: i.Area.ospfPktHeader(func(p *packet.LayerOSPFv2) {
			p.Type = layers.OSPFLinkStateAcknowledgment
		}),
		Content: packet.LSAcknowledgementPayload(lsacks),
	}
	pkt := sendPkt{
		dst: dst,
		p:   p,
	}
	i.queuePktForSend(pkt)
}

func (i *Interface) sendLSUFlood(dst uint32, lids ...packet.LSAIdentity) {
	var (
		allLSAs []packet.LSAdvertisement
	)
	for _, lid := range lids {
		_, lsa, meta, ok := i.Area.lsDbGetLSAByIdentity(lid, true)
		if !ok {
			continue
		}
		lsa.Ager(i.InfTransDelay)
		allLSAs = append(allLSAs, lsa)
		meta.updateLastFloodTime()
	}
	i.Area.splitSendLSAsByMtu(i, allLSAs, dst)
}

func (i *Interface) immediateTickNeighborsRetransmissionList() {
	i.rangeOverNeighbors(func(nb *Neighbor) bool {
		nb.lsRtxmTicker.DoFnNow()
		return true
	})
}
