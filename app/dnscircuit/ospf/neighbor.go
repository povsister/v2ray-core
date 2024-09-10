package ospf

import (
	"context"
	"math"
	"net"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gopacket/gopacket/layers"
	"golang.org/x/net/ipv4"

	"github.com/v2fly/v2ray-core/v5/app/dnscircuit/ospf/packet"
)

type NeighborState int

const (
	// NeighborDown This is the initial state of a neighbor conversation.  It
	//            indicates that there has been no recent information received
	//            from the neighbor.  On NBMA networks, Hello packets may
	//            still be sent to "Down" neighbors, although at a reduced
	//            frequency (see Section 9.5.1).
	NeighborDown NeighborState = iota
	// NeighborAttempt This state is only valid for neighbors attached to NBMA
	//            networks.  It indicates that no recent information has been
	//            received from the neighbor, but that a more concerted effort
	//            should be made to contact the neighbor.  This is done by
	//            sending the neighbor Hello packets at intervals of
	//            HelloInterval (see Section 9.5.1).
	NeighborAttempt
	// NeighborInit In this state, an Hello packet has recently been seen from
	//            the neighbor.  However, bidirectional communication has not
	//            yet been established with the neighbor (i.e., the router
	//            itself did not appear in the neighbor's Hello packet).  All
	//            neighbors in this state (or higher) are listed in the Hello
	//            packets sent from the associated interface.
	NeighborInit
	// Neighbor2Way In this state, communication between the two routers is
	//            bidirectional.  This has been assured by the operation of
	//            the Hello Protocol.  This is the most advanced state short
	//            of beginning adjacency establishment.  The (Backup)
	//            Designated Router is selected from the set of neighbors in
	//            state 2-Way or greater.
	Neighbor2Way
	// NeighborExStart This is the first step in creating an adjacency between the
	//            two neighboring routers.  The goal of this step is to decide
	//            which router is the master, and to decide upon the initial
	//            DD sequence number.  Neighbor conversations in this state or
	//            greater are called adjacencies.
	NeighborExStart
	// NeighborExchange In this state the router is describing its entire link state
	//            database by sending Database Description packets to the
	//            neighbor.  Each Database Description Packet has a DD
	//            sequence number, and is explicitly acknowledged.  Only one
	//            Database Description Packet is allowed outstanding at any
	//            one time.  In this state, Link State Request Packets may
	//            also be sent asking for the neighbor's more recent LSAs.
	//            All adjacencies in Exchange state or greater are used by the
	//            flooding procedure.  In fact, these adjacencies are fully
	//            capable of transmitting and receiving all types of OSPF
	//            routing protocol packets.
	NeighborExchange
	// NeighborLoading In this state, Link State Request packets are sent to the
	//            neighbor asking for the more recent LSAs that have been
	//            discovered (but not yet received) in the Exchange state.
	NeighborLoading
	// NeighborFull In this state, the neighboring routers are fully adjacent.
	//            These adjacencies will now appear in router-LSAs and
	//            network-LSAs.
	NeighborFull
)

var nbsName = map[NeighborState]string{
	NeighborDown:     "Down",
	NeighborAttempt:  "Attempt",
	NeighborInit:     "Init",
	Neighbor2Way:     "2-Way",
	NeighborExStart:  "ExStart",
	NeighborExchange: "ExChange",
	NeighborLoading:  "Loading",
	NeighborFull:     "Full",
}

func (ns NeighborState) String() string {
	if name, ok := nbsName[ns]; ok {
		return name
	}
	return strconv.FormatInt(int64(ns), 10)
}

type Neighbor struct {
	i *Interface

	ctx    context.Context
	cancel context.CancelFunc
	// The functional level of the neighbor conversation.  This is
	//        described in more detail in Section 10.1.
	State NeighborState
	stMu  sync.RWMutex
	// A single shot timer whose firing indicates that no Hello Packet
	//        has been seen from this neighbor recently.  The length of the
	//        timer is RouterDeadInterval seconds.
	InactivityTimer *time.Timer
	// When the two neighbors are exchanging databases, they form a
	//        master/slave relationship.  The master sends the first Database
	//        Description Packet, and is the only part that is allowed to
	//        retransmit.  The slave can only respond to the master's Database
	//        Description Packets.  The master/slave relationship is
	//        negotiated in state ExStart.
	IsMaster              bool
	negotiationRtxmTicker *TickerFunc
	ddRtxmTicker          *TickerFunc
	// The DD Sequence number of the Database Description packet that
	//        is currently being sent to the neighbor.
	DDSeqNumber atomic.Uint32

	// The initialize(I), more (M) and master(MS) bits, Options field,
	//        and DD sequence number contained in the last Database
	//        Description packet received from the neighbor. Used to determine
	//        whether the next Database Description packet received from the
	//        neighbor is a duplicate.
	LastReceivedDDPacket       TSS[*packet.OSPFv2Packet[packet.DbDescPayload]]
	lastReceivedDDInvalidTimer *time.Timer
	lastSlaveDDSent            TSS[*packet.DbDescPayload] // slave echo with dd summary

	NeighborId uint32
	// The Router Priority of the neighboring router.  Contained in the
	//        neighbor's Hello packets, this item is used when selecting the
	//        Designated Router for the attached network.
	NeighborPriority uint8

	// The IP address of the neighboring router's interface to the
	//        attached network.  Used as the Destination IP address when
	//        protocol packets are sent as unicasts along this adjacency.
	//        Also used in router-LSAs as the Link ID for the attached network
	//        if the neighboring router is selected to be Designated Router
	//        (see Section 12.4.1).  The Neighbor IP address is learned when
	//        Hello packets are received from the neighbor.  For virtual
	//        links, the Neighbor IP address is learned during the routing
	//        table build process (see Section 15).
	NeighborAddress net.IP
	// The optional OSPF capabilities supported by the neighbor.
	//        Learned during the Database Exchange process (see Section 10.6).
	//        The neighbor's optional OSPF capabilities are also listed in its
	//        Hello packets.  This enables received Hello Packets to be
	//        rejected (i.e., neighbor relationships will not even start to
	//        form) if there is a mismatch in certain crucial OSPF
	//        capabilities (see Section 10.5).  The optional OSPF capabilities
	//        are documented in Section 4.5.
	NeighborOptions packet.BitOption
	// The neighbor's idea of the Designated Router address.  If this is the
	//        neighbor itself, this is important in the local calculation of
	//        the Designated Router.  Defined only on broadcast and NBMA
	//        networks.
	NeighborsDR uint32
	// The neighbor's idea of the Backup Designated Router address.  If this is
	//        the neighbor itself, this is important in the local calculation
	//        of the Backup Designated Router.  Defined only on broadcast and
	//        NBMA networks.
	NeighborsBDR uint32

	//    The next set of variables are lists of LSAs.  These lists describe
	//    subsets of the area link-state database.  This memo defines five
	//    distinct types of LSAs, all of which may be present in an area
	//    link-state database: router-LSAs, network-LSAs, and Type 3 and 4
	//    summary-LSAs (all stored in the area data structure), and AS-
	//    external-LSAs (stored in the global data structure).

	// The list of LSAs that have been flooded but not acknowledged on
	//        this adjacency.  These will be retransmitted at intervals until
	//        they are acknowledged, or until the adjacency is destroyed.
	LSRetransmission map[packet.LSAIdentity]struct{}
	lsRtxmRw         sync.RWMutex
	lsRtxmTicker     *TickerFunc

	// The complete list of LSAs that make up the area link-state
	//        database, at the moment the neighbor goes into Database Exchange
	//        state.  This list is sent to the neighbor in Database
	//        Description packets.
	DatabaseSummary []packet.LSAIdentity
	// The list of LSAs that need to be received from this neighbor in
	//        order to synchronize the two neighbors' link-state databases.
	//        This list is created as Database Description packets are
	//        received, and is then sent to the neighbor in Link State Request
	//        packets.  The list is depleted as appropriate Link State Update
	//        packets are received.
	LSRequest           []packet.LSAheader
	lsReqListRw         sync.RWMutex
	lsReqListRtxmTicker *TickerFunc
}

func (n *Neighbor) terminate() {
	n.cancel()
	if n.lastReceivedDDInvalidTimer != nil {
		n.lastReceivedDDInvalidTimer.Stop()
	}
	if n.InactivityTimer != nil {
		n.InactivityTimer.Stop()
	}
}

func (n *Neighbor) currState() NeighborState {
	n.stMu.RLock()
	defer n.stMu.RUnlock()
	return n.State
}

func (n *Neighbor) transState(target NeighborState) {
	n.stMu.Lock()
	defer n.stMu.Unlock()
	var (
		currState    = n.State
		stateChanged = currState != target
	)
	// recycle previous state
	switch currState {
	case NeighborExStart:
		if stateChanged {
			n.negotiationRtxmTicker.Terminate()
		}
	case NeighborExchange:
		if stateChanged {
			n.ddRtxmTicker.Terminate()
		}
	case NeighborLoading:
		if stateChanged {
			n.lsReqListRtxmTicker.Terminate()
		}
	}
	// check dst state
	switch target {
	case NeighborDown:
		if n.InactivityTimer != nil {
			n.InactivityTimer.Stop()
		}
		if n.lastReceivedDDInvalidTimer != nil {
			n.lastReceivedDDInvalidTimer.Stop()
		}
	}
	LogImportant("neighbor %s state change: %v -> %v", uint32ToIPv4(n.NeighborId).String(), currState, target)
	n.State = target
}

func (n *Neighbor) shouldFormAdjacency() bool {
	// for now we form adj only if the neighbor is DR or BDR
	nbAddr := ipv4BytesToUint32(n.NeighborAddress.To4())
	if nbAddr == n.i.DR.Load() || nbAddr == n.i.BDR.Load() {
		return true
	}
	return false
}

type NeighborStateChangingEvent int

const (
	_ NeighborStateChangingEvent = iota
	// NbEvHelloReceived An Hello packet has been received from the neighbor.
	NbEvHelloReceived
	// NbEvStart this is an indication that Hello Packets should now be sent
	//            to the neighbor at intervals of HelloInterval seconds.  This
	//            event is generated only for neighbors associated with NBMA
	//            networks.
	NbEvStart
	// NbEv2WayReceived Bidirectional communication has been realized between the
	//            two neighboring routers.  This is indicated by the router
	//            seeing itself in the neighbor's Hello packet.
	NbEv2WayReceived
	// NbEvNegotiationDone The Master/Slave relationship has been negotiated, and DD
	//            sequence numbers have been exchanged.  This signals the
	//            start of the sending/receiving of Database Description
	//            packets.  For more information on the generation of this
	//            event, consult Section 10.8.
	NbEvNegotiationDone
	// NbEvExchangeDone Both routers have successfully transmitted a full sequence
	//            of Database Description packets.  Each router now knows what
	//            parts of its link state database are out of date.  For more
	//            information on the generation of this event, consult Section
	//            10.8.
	NbEvExchangeDone
	// NbEvBadLSReq A Link State Request has been received for an LSA not
	//            contained in the database.  This indicates an error in the
	//            Database Exchange process.
	NbEvBadLSReq
	// NbEvLoadingDone Link State Updates have been received for all out-of-date
	//            portions of the database.  This is indicated by the Link
	//            state request list becoming empty after the Database
	//            Exchange process has completed.
	NbEvLoadingDone
	// NbEvIsAdjOK A decision must be made as to whether an adjacency should be
	//            established/maintained with the neighbor.  This event will
	//            start some adjacencies forming, and destroy others.
	NbEvIsAdjOK

	//        The following events cause well developed neighbors to revert to
	//        lesser states.  Unlike the above events, these events may occur
	//        when the neighbor conversation is in any of a number of states.

	// NbEvSeqNumberMismatch A Database Description packet has been received that either
	//            a) has an unexpected DD sequence number, b) unexpectedly has
	//            the Init bit set or c) has an Options field differing from
	//            the last Options field received in a Database Description
	//            packet.  Any of these conditions indicate that some error
	//            has occurred during adjacency establishment.
	NbEvSeqNumberMismatch
	// NbEv1Way An Hello packet has been received from the neighbor, in
	//            which the router is not mentioned.  This indicates that
	//            communication with the neighbor is not bidirectional.
	NbEv1Way
	// NbEvKillNbr This  is  an  indication that  all  communication  with  the
	//            neighbor  is now  impossible,  forcing  the  neighbor  to
	//            revert  to  Down  state.
	NbEvKillNbr
	// NbEvInactivityTimer The inactivity Timer has fired.  This means that no Hello
	//            packets have been seen recently from the neighbor.  The
	//            neighbor reverts to Down state.
	NbEvInactivityTimer
	// NbEvLLDown This is an indication from the lower level protocols that
	//            the neighbor is now unreachable.  For example, on an X.25
	//            network this could be indicated by an X.25 clear indication
	//            with appropriate cause and diagnostic fields.  This event
	//            forces the neighbor into Down state.
	NbEvLLDown
)

func (n *Neighbor) consumeEvent(e NeighborStateChangingEvent) {
	switch e {
	case NbEvStart: // NBMA networks only
		if n.currState() == NeighborDown {
			n.transState(NeighborAttempt)
			// Send an Hello Packet to the neighbor (this neighbor
			//                    is always associated with an NBMA network) and start
			//                    the Inactivity Timer for the neighbor.  The timer's
			//                    later firing would indicate that communication with
			//                    the neighbor was not attained.
		}
	case NbEvHelloReceived:
		if n.currState() == NeighborAttempt {
			// NBMA networks only
			n.transState(NeighborInit)
			n.startInactivityTimer()
		} else if n.currState() == NeighborDown {
			n.transState(NeighborInit)
			n.startInactivityTimer()
		} else if n.currState() >= NeighborInit {
			n.startInactivityTimer()
		}
	case NbEv2WayReceived:
		if n.currState() == NeighborInit {
			// Determine whether an adjacency should be established with the neighbor (see Section 10.4).
			// If not, the new neighbor state is 2-Way.
			// Otherwise (an adjacency should be established) the neighbor state transitions to ExStart.
			if n.shouldFormAdjacency() {
				n.transState(NeighborExStart)
				n.startMasterNegotiation()
			} else {
				n.transState(Neighbor2Way)
			}
		}
	case NbEvNegotiationDone:
		if n.currState() == NeighborExStart {
			n.transState(NeighborExchange)
		}
	case NbEvExchangeDone:
		if n.currState() == NeighborExchange {
			// If the neighbor Link state request list is empty,
			//                    the new neighbor state is Full.  No other action is
			//                    required.  This is an adjacency's final state.
			//
			//                    Otherwise, the new neighbor state is Loading.  Start
			//                    (or continue) sending Link State Request packets to
			//                    the neighbor (see Section 10.9).  These are requests
			//                    for the neighbor's more recent LSAs (which were
			//                    discovered but not yet received in the Exchange
			//                    state).  These LSAs are listed in the Link state
			//                    request list associated with the neighbor.
			if n.isLSReqListEmpty() {
				n.transState(NeighborFull)
			} else {
				n.transState(NeighborLoading)
				n.startLSR()
			}
		}
	case NbEvLoadingDone:
		if n.currState() == NeighborLoading {
			n.transState(NeighborFull)
		}
	case NbEvIsAdjOK:
		if n.currState() == Neighbor2Way {
			// Determine whether an adjacency should be formed with
			//                    the neighboring router (see Section 10.4).  If not,
			//                    the neighbor state remains at 2-Way.  Otherwise,
			//                    transition the neighbor state to ExStart and perform
			//                    the actions associated with the above state machine
			//                    entry for state Init and event 2-WayReceived.
			if n.shouldFormAdjacency() {
				n.transState(NeighborExStart)
				n.startMasterNegotiation()
			}
		} else if n.currState() >= NeighborExStart {
			// Determine whether the neighboring router should
			//                    still be adjacent.  If yes, there is no state change
			//                    and no further action is necessary.
			//
			//                    Otherwise, the (possibly partially formed) adjacency
			//                    must be destroyed.  The neighbor state transitions
			//                    to 2-Way.  The Link state retransmission list,
			//                    Database summary list and Link state request list
			//                    are cleared of LSAs.
			if !n.shouldFormAdjacency() {
				n.clearLSRetransmissionList()
				n.clearLSReqList()
				clear(n.DatabaseSummary)
				n.transState(Neighbor2Way)
			}
		}
	case NbEvSeqNumberMismatch:
		if n.currState() >= NeighborExchange {
			n.clearLSRetransmissionList()
			n.clearLSReqList()
			clear(n.DatabaseSummary)
			n.transState(NeighborExStart)
			// The (possibly partially formed) adjacency is torn
			//                    down, and then an attempt is made at
			//                    reestablishment.  The neighbor state first
			//                    transitions to ExStart.  The Link state
			//                    retransmission list, Database summary list and Link
			//                    state request list are cleared of LSAs.  Then the
			//                    router increments the DD sequence number in the
			//                    neighbor data structure, declares itself master
			//                    (sets the master/slave bit to master), and starts
			//                    sending Database Description Packets, with the
			//                    initialize (I), more (M) and master (MS) bits set.
			//                    This Database Description Packet should be otherwise
			//                    empty (see Section 10.8).
			n.startMasterNegotiation()
		}
	case NbEvBadLSReq:
		if n.currState() >= NeighborExchange {
			n.clearLSRetransmissionList()
			n.clearLSReqList()
			clear(n.DatabaseSummary)
			n.transState(NeighborExStart)
			// The action for event BadLSReq is exactly the same as
			//                    for the neighbor event SeqNumberMismatch.  The
			//                    (possibly partially formed) adjacency is torn down,
			//                    and then an attempt is made at reestablishment.  For
			//                    more information, see the neighbor state machine
			//                    entry that is invoked when event SeqNumberMismatch
			//                    is generated in state Exchange or greater.
			n.startMasterNegotiation()
		}
	case NbEvKillNbr:
		n.clearLSRetransmissionList()
		n.clearLSReqList()
		clear(n.DatabaseSummary)
		n.transState(NeighborDown)
		// The Link state retransmission list, Database summary
		//                    list and Link state request list are cleared of
		//                    LSAs.  Also, the Inactivity Timer is disabled.
	case NbEvLLDown:
		n.clearLSRetransmissionList()
		n.clearLSReqList()
		clear(n.DatabaseSummary)
		n.transState(NeighborDown)
		// The Link state retransmission list, Database summary
		//                    list and Link state request list are cleared of
		//                    LSAs.  Also, the Inactivity Timer is disabled.
	case NbEvInactivityTimer:
		n.clearLSRetransmissionList()
		n.clearLSReqList()
		clear(n.DatabaseSummary)
		n.transState(NeighborDown)
		n.i.removeNeighbor(n)
		// The Link state retransmission list, Database summary
		//                    list and Link state request list are cleared of
		//                    LSAs.
	case NbEv1Way:
		if n.currState() >= Neighbor2Way {
			n.clearLSRetransmissionList()
			n.clearLSReqList()
			clear(n.DatabaseSummary)
			n.transState(NeighborInit)
			// The Link state retransmission list, Database summary
			//                    list and Link state request list are cleared of
			//                    LSAs.
		}
	}
}

func (n *Neighbor) startInactivityTimer() {
	inactiveDur := time.Duration(n.i.RouterDeadInterval) * time.Second
	if n.InactivityTimer == nil {
		n.InactivityTimer = time.AfterFunc(inactiveDur,
			func() { n.consumeEvent(NbEvInactivityTimer) })
	} else {
		n.InactivityTimer.Reset(inactiveDur)
	}
}

func (n *Neighbor) startMasterNegotiation() {
	ddSeqNum := n.DDSeqNumber.Load()
	const (
		maxInitialDDSeqNum = 1 << 16
	)
	if ddSeqNum <= 0 {
		ddSeqNum = packet.RandSource.Uint32N(math.MaxUint32) % maxInitialDDSeqNum
		n.DDSeqNumber.Store(ddSeqNum)
	} else {
		ddSeqNum += 1
		n.DDSeqNumber.Store(ddSeqNum)
	}
	dd := &packet.OSPFv2Packet[packet.DbDescPayload]{
		OSPFv2: n.i.Area.ospfPktHeader(func(p *packet.LayerOSPFv2) {
			p.Type = layers.OSPFDatabaseDescription
		}),
		Content: packet.DbDescPayload{
			DbDescPkg: layers.DbDescPkg{
				Options:      uint32(n.i.Area.Options),
				InterfaceMTU: n.i.MTU,
				Flags: uint16(packet.BitOption(0).SetBit(packet.DDFlagMSbit,
					packet.DDFlagIbit, packet.DDFlagMbit)),
				DDSeqNumber: ddSeqNum,
			},
		},
	}

	pkt := sendPkt{
		dst: ipv4BytesToUint32(n.NeighborAddress.To4()),
		p:   dd,
	}

	n.negotiationRtxmTicker.Terminate()
	// retransmitted at intervals of RxmtInterval until the next state is entered
	n.negotiationRtxmTicker = TimeTickerFunc(n.ctx, time.Duration(n.i.RxmtInterval)*time.Second,
		func() { n.i.queuePktForSend(pkt) })
}

func (n *Neighbor) saveLastReceivedDD(dd *packet.OSPFv2Packet[packet.DbDescPayload]) {
	invalidDur := time.Duration(n.i.RouterDeadInterval) * time.Second
	if n.lastReceivedDDInvalidTimer == nil {
		n.lastReceivedDDInvalidTimer = time.AfterFunc(invalidDur,
			func() {
				n.LastReceivedDDPacket.Set(nil)
				n.lastSlaveDDSent.Set(nil)
			})
	} else {
		n.lastReceivedDDInvalidTimer.Reset(invalidDur)
	}
	n.LastReceivedDDPacket.Set(dd)
}

func (n *Neighbor) isDuplicatedDD(dd *packet.OSPFv2Packet[packet.DbDescPayload]) (*packet.OSPFv2Packet[packet.DbDescPayload], bool) {
	lastDD := n.LastReceivedDDPacket.Get()
	if lastDD != nil && lastDD.Content.Flags == dd.Content.Flags &&
		lastDD.Content.Options == dd.Content.Options &&
		lastDD.Content.DDSeqNumber == dd.Content.DDSeqNumber {
		return lastDD, true
	}
	return nil, false
}

func (n *Neighbor) parseDD(dd *packet.OSPFv2Packet[packet.DbDescPayload]) {
	if len(dd.Content.LSAinfo) <= 0 {
		return
	}
	lsReq := n.i.Area.getLSReqListFromDD(dd)
	n.appendLSReqList(lsReq...)
}

func (n *Neighbor) echoDDWithPossibleRetransmission(dd *packet.OSPFv2Packet[packet.DbDescPayload]) {
	echoDD := &packet.OSPFv2Packet[packet.DbDescPayload]{
		OSPFv2: n.i.Area.ospfPktHeader(func(p *packet.LayerOSPFv2) {
			p.Type = layers.OSPFDatabaseDescription
		}),
	}
	if lastDDEcho := n.lastSlaveDDSent.Get(); lastDDEcho != nil {
		echoDD.Content = *lastDDEcho
	} else {
		echoDD.Content = packet.DbDescPayload{
			DbDescPkg: layers.DbDescPkg{
				Options:      uint32(n.i.Area.Options),
				InterfaceMTU: n.i.MTU,
				Flags: func() uint16 {
					retFlag := packet.BitOption(0)
					if len(n.DatabaseSummary) > 0 {
						retFlag = retFlag.SetBit(packet.DDFlagMbit)
					}
					return uint16(retFlag)
				}(),
				DDSeqNumber: dd.Content.DDSeqNumber,
			},
		}
	}
	n.i.queuePktForSend(sendPkt{
		dst: ipv4BytesToUint32(n.NeighborAddress.To4()),
		p:   echoDD,
	})
}

func (n *Neighbor) sendDDExchange() {
	if n.IsMaster {
		// im slave. just return.
		return
	}
	// im master
	// only master can incr the ddSeqNum
	ddSeqNum := n.DDSeqNumber.Load()
	ddSeqNum += 1
	n.DDSeqNumber.Store(ddSeqNum)

	var (
		toSendLSAs []packet.LSAheader
		moreBit    = false
	)
	// simply one LSA per DD to avoid potential MTU issue.
	if len(n.DatabaseSummary) >= 1 {
		toSendLSAs = append(toSendLSAs, n.i.Area.lsDbGetLSAheaderByIdentity(n.DatabaseSummary[0])...)
		n.DatabaseSummary = n.DatabaseSummary[1:]
		if len(n.DatabaseSummary) > 0 {
			moreBit = true
		}
	}
	dd := &packet.OSPFv2Packet[packet.DbDescPayload]{
		OSPFv2: n.i.Area.ospfPktHeader(func(p *packet.LayerOSPFv2) {
			p.Type = layers.OSPFDatabaseDescription
		}),
		Content: packet.DbDescPayload{
			DbDescPkg: layers.DbDescPkg{
				Options:      uint32(n.i.Area.Options),
				InterfaceMTU: n.i.MTU,
				Flags: func() uint16 {
					ret := packet.BitOption(0).SetBit(packet.DDFlagMSbit)
					if moreBit {
						ret = ret.SetBit(packet.DDFlagMbit)
					}
					return uint16(ret)
				}(),
				DDSeqNumber: ddSeqNum,
			},
			LSAinfo: toSendLSAs,
		},
	}

	pkt := sendPkt{
		dst: ipv4BytesToUint32(n.NeighborAddress.To4()),
		p:   dd,
	}
	// Database Description packets are sent when either
	// a) the slave acknowledges the previous Database Description packet
	//    by echoing the DD sequence number or
	// b) RxmtInterval seconds elapse without an acknowledgment, in which case the previous
	//    Database Description packet is retransmitted.
	n.ddRtxmTicker.Terminate()
	n.ddRtxmTicker = TimeTickerFunc(n.ctx, time.Duration(n.i.RxmtInterval)*time.Second,
		func() { n.i.queuePktForSend(pkt) })
}

func (n *Neighbor) fillDatabaseSummary() {
	n.DatabaseSummary = n.i.Area.lsDbGetDatabaseSummary()
}

func (n *Neighbor) masterStartDDExchange(dd *packet.OSPFv2Packet[packet.DbDescPayload]) {
	n.fillDatabaseSummary()
	// for some reason, ROS will send LSA in the slave negotiation result ack packet.
	// so we must parse those DD payload here
	n.parseDD(dd)
	n.sendDDExchange()
}

func (n *Neighbor) masterContinueDDExchange(slaveMoreBitSet bool) (needAck bool) {
	if len(n.DatabaseSummary) > 0 || slaveMoreBitSet {
		// there are still some DD LSA waiting for send.
		// or slave didn't finish DD send, polling for more.
		n.sendDDExchange()
		return true
	}
	n.ddRtxmTicker.Terminate()
	return false
}

func (n *Neighbor) slavePrepareDDExchange() {
	n.fillDatabaseSummary()
	// but do not send dd first.
	// Wait for master for dd sync.
}

func (n *Neighbor) slaveDDEchoAndExchange(dd *packet.OSPFv2Packet[packet.DbDescPayload]) (allDDSent bool) {
	if len(n.DatabaseSummary) >= 1 {
		// simply one LSA per DD to avoid potential MTU issue.
		toSendLSA := n.i.Area.lsDbGetLSAheaderByIdentity(n.DatabaseSummary[0])
		n.DatabaseSummary = n.DatabaseSummary[1:]
		allDDSent = len(n.DatabaseSummary) <= 0
		n.lastSlaveDDSent.Set(&packet.DbDescPayload{
			DbDescPkg: layers.DbDescPkg{
				Options:      uint32(n.i.Area.Options),
				InterfaceMTU: n.i.MTU,
				Flags: func() uint16 {
					retFlag := packet.BitOption(0)
					if !allDDSent {
						retFlag = retFlag.SetBit(packet.DDFlagMbit)
					}
					return uint16(retFlag)
				}(),
				DDSeqNumber: dd.Content.DDSeqNumber,
			},
			LSAinfo: toSendLSA,
		})
	} else {
		allDDSent = true
	}
	n.echoDDWithPossibleRetransmission(dd)
	return
}

func (n *Neighbor) isLSReqListEmpty() bool {
	n.lsReqListRw.RLock()
	defer n.lsReqListRw.RUnlock()
	return len(n.LSRequest) <= 0
}

func (n *Neighbor) isInLSReqList(l packet.LSAIdentity) bool {
	n.lsReqListRw.RLock()
	defer n.lsReqListRw.RUnlock()
	for _, r := range n.LSRequest {
		if r.GetLSAIdentity() == l {
			return true
		}
	}
	return false
}

func (n *Neighbor) getFromLSReqList(l packet.LSAIdentity) (lsaH packet.LSAheader, ok bool) {
	n.lsReqListRw.RLock()
	defer n.lsReqListRw.RUnlock()
	for _, r := range n.LSRequest {
		if r.GetLSAIdentity() == l {
			return r, true
		}
	}
	return
}

func (n *Neighbor) deleteFromLSReqList(l packet.LSAIdentity) {
	defer func() {
		// immediately trans neighbor state by calling ticker
		if n.isLSReqListEmpty() {
			n.lsReqListRtxmTicker.DoFnNow()
		}
	}()
	n.lsReqListRw.Lock()
	defer n.lsReqListRw.Unlock()
	n.LSRequest = slices.DeleteFunc(n.LSRequest, func(r packet.LSAheader) bool {
		return r.GetLSAIdentity() == l
	})
}

func (n *Neighbor) appendLSReqList(lsrs ...packet.LSAheader) {
	n.lsReqListRw.Lock()
	defer n.lsReqListRw.Unlock()
	n.LSRequest = append(n.LSRequest, lsrs...)
}

func (n *Neighbor) clearLSReqList() {
	n.lsReqListRw.Lock()
	defer n.lsReqListRw.Unlock()
	clear(n.LSRequest)
}

func (n *Neighbor) startLSR() {
	n.lsReqListRtxmTicker.Terminate()
	n.lsReqListRtxmTicker = TimeTickerFunc(n.ctx, time.Duration(n.i.RxmtInterval)*time.Second, func() {
		if n.sendOutTopLSR() <= 0 {
			// no LSR has been sent. means that the list is empty.
			n.lsReqListRtxmTicker.Terminate()
			n.consumeEvent(NbEvLoadingDone)
		}
	})
}

func (n *Neighbor) sendOutTopLSR() int {
	n.lsReqListRw.RLock()
	defer n.lsReqListRw.RUnlock()
	if len(n.LSRequest) <= 0 {
		return 0
	}
	// calculate max cnt by MTU
	maxCnt := (int(n.i.MTU) - ipv4.HeaderLen - 24) / packet.LSReq{}.Size()
	singleFlightMax := min(maxCnt, len(n.LSRequest))
	payloads := make([]packet.LSReq, 0, singleFlightMax)
	for i := 0; i < singleFlightMax; i++ {
		payloads = append(payloads, n.LSRequest[i].GetLSReq())
	}
	lsr := &packet.OSPFv2Packet[packet.LSRequestPayload]{
		OSPFv2: n.i.Area.ospfPktHeader(func(p *packet.LayerOSPFv2) {
			p.Type = layers.OSPFLinkStateRequest
		}),
		Content: packet.LSRequestPayload(payloads),
	}
	n.i.queuePktForSend(sendPkt{
		dst: ipv4BytesToUint32(n.NeighborAddress.To4()),
		p:   lsr,
	})
	return singleFlightMax
}

func (n *Neighbor) clearLSRetransmissionList() {
	n.lsRtxmRw.Lock()
	defer n.lsRtxmRw.Unlock()
	n.lsRtxmTicker.Suspend()
	clear(n.LSRetransmission)
}

func (n *Neighbor) tryEmptyLSRetransmissionListByAck(lsAcks *packet.OSPFv2Packet[packet.LSAcknowledgementPayload]) (
	suspiciousAcks []packet.LSAheader) {
	n.lsRtxmRw.Lock()
	defer n.lsRtxmRw.Unlock()
	defer func() {
		if len(n.LSRetransmission) <= 0 {
			n.lsRtxmTicker.Suspend()
		}
	}()

	validAcks := make([]packet.LSAIdentity, 0, len(lsAcks.Content))
	validAckLUT := make(map[packet.LSAIdentity]int, len(lsAcks.Content))
	for idx, ack := range lsAcks.Content {
		id := ack.GetLSAIdentity()
		if _, exist := n.LSRetransmission[id]; exist {
			validAcks = append(validAcks, id)
			validAckLUT[id] = idx
		} else {
			// if LSA acknowledged does not have an instance on the Link state retransmission list.
			// silent discarded.
		}
	}
	// validate it with LSDB
	lsaHdrs := n.i.Area.lsDbGetLSAheaderByIdentity(validAcks...)
	lsaHdrsIdLUT := make(map[packet.LSAIdentity]packet.LSAheader, len(lsaHdrs))
	for _, lsaH := range lsaHdrs {
		lsaHdrsIdLUT[lsaH.GetLSAIdentity()] = lsaH
	}

	for _, vAckId := range validAcks {
		if lsaH, exist := lsaHdrsIdLUT[vAckId]; exist {
			ackReceived := lsAcks.Content[validAckLUT[vAckId]]
			// If the acknowledgment is for the same instance that is
			//            contained on the list, remove the item from the list and
			//            examine the next acknowledgment.
			if lsaH.IsSame(ackReceived) {
				delete(n.LSRetransmission, vAckId)
			} else {
				// Otherwise: Log the questionable acknowledgment, and examine the next
				//            one.
				suspiciousAcks = append(suspiciousAcks, ackReceived)
			}
		} else {
			// LS in LSRtxm List does not exist in LSDB. simply delete id.
			delete(n.LSRetransmission, vAckId)
			logWarn("delete LSA(%+v) from LSRtxmList: no LSA instance found in LSDB", vAckId)
		}
	}
	return
}

func (n *Neighbor) addToLSRetransmissionList(l packet.LSAIdentity) {
	n.lsRtxmRw.Lock()
	defer n.lsRtxmRw.Unlock()
	if n.lsRtxmTicker == nil {
		n.lsRtxmTicker = TimeTickerFunc(n.ctx, time.Duration(n.i.RxmtInterval)*time.Second,
			n.doLSRetransmission, true)
	}
	n.LSRetransmission[l] = struct{}{}
	n.lsRtxmTicker.Reset()
}

func (n *Neighbor) doLSRetransmission() {
	n.lsRtxmRw.RLock()
	defer n.lsRtxmRw.RUnlock()
	lsas := make([]packet.LSAdvertisement, 0, len(n.LSRetransmission))
	for l := range n.LSRetransmission {
		_, lsa, meta, ok := n.i.Area.lsDbGetLSAByIdentity(l, true)
		if !ok {
			continue
		}
		lsa.Ager(n.i.InfTransDelay)
		meta.updateLastFloodTime()
		lsas = append(lsas, lsa)
	}
	n.i.Area.splitSendLSAsByMtu(n.i, lsas, ipv4BytesToUint32(n.NeighborAddress.To4()))
}

func (n *Neighbor) removeFromLSRetransmissionList(lsa packet.LSAIdentity) {
	n.lsRtxmRw.Lock()
	defer n.lsRtxmRw.Unlock()
	delete(n.LSRetransmission, lsa)
}

func (n *Neighbor) isLSRtxmListEmpty() bool {
	n.lsRtxmRw.RLock()
	defer n.lsRtxmRw.RUnlock()
	return len(n.LSRetransmission) <= 0
}

func (n *Neighbor) isInLSRetransmissionList(l packet.LSAIdentity) bool {
	n.lsRtxmRw.RLock()
	defer n.lsRtxmRw.RUnlock()
	_, exist := n.LSRetransmission[l]
	return exist
}

func (n *Neighbor) directSendLSAck(ack packet.LSAheader) {
	p := &packet.OSPFv2Packet[packet.LSAcknowledgementPayload]{
		OSPFv2: n.i.Area.ospfPktHeader(func(p *packet.LayerOSPFv2) {
			p.Type = layers.OSPFLinkStateAcknowledgment
		}),
		Content: packet.LSAcknowledgementPayload{
			ack,
		},
	}
	pkt := sendPkt{
		dst: ipv4BytesToUint32(n.NeighborAddress.To4()),
		p:   p,
	}
	n.i.queuePktForSend(pkt)
}

func (n *Neighbor) directSendLSU(id packet.LSAIdentity) {
	_, lsa, meta, ok := n.i.Area.lsDbGetLSAByIdentity(id, true)
	if !ok {
		return
	}
	defer meta.updateLastFloodTime()
	p := &packet.OSPFv2Packet[packet.LSUpdatePayload]{
		OSPFv2: n.i.Area.ospfPktHeader(func(p *packet.LayerOSPFv2) {
			p.Type = layers.OSPFLinkStateUpdate
		}),
		Content: packet.LSUpdatePayload{
			LSUpdate: layers.LSUpdate{
				NumOfLSAs: 1,
			},
			LSAs: []packet.LSAdvertisement{
				lsa,
			},
		},
	}
	pkt := sendPkt{
		dst: ipv4BytesToUint32(n.NeighborAddress.To4()),
		p:   p,
	}
	n.i.queuePktForSend(pkt)
}

func (n *Neighbor) directSendDelayedLSAcks(acks []packet.LSAheader) {
	p := &packet.OSPFv2Packet[packet.LSAcknowledgementPayload]{
		OSPFv2: n.i.Area.ospfPktHeader(func(p *packet.LayerOSPFv2) {
			p.Type = layers.OSPFLinkStateAcknowledgment
		}),
		Content: packet.LSAcknowledgementPayload(acks),
	}
	pkt := sendPkt{
		dst: ipv4BytesToUint32(n.NeighborAddress.To4()),
		p:   p,
	}
	n.i.queuePktForSend(pkt)
}
