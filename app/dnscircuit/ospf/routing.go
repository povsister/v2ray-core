package ospf

import (
	"net"

	"github.com/v2fly/v2ray-core/v5/app/dnscircuit/ospf/packet"
)

type RoutingPathType uint8

const (
	_ RoutingPathType = iota
	RoutingPathIntraArea
	RoutingPathInterArea
	RoutingPathExternalType1
	RoutingPathExternalType2
)

type RoutingDestinationType uint8

const (
	_ RoutingDestinationType = iota
	RoutingDestTypeNetwork
	RoutingDestTypeRouter
)

type RoutingTable struct {
	List []*RoutingTableEntry
}

type RoutingTableEntry struct {
	// Destination type is either "network" or "router". Only network
	//        entries are actually used when forwarding IP data traffic.
	//        Router routing table entries are used solely as intermediate
	//        steps in the routing table build process.
	//
	//        A network is a range of IP addresses, to which IP data traffic
	//        may be forwarded.  This includes IP networks (class A, B, or C),
	//        IP subnets, IP supernets and single IP hosts.  The default route
	//        also falls into this category.
	//
	//        Router entries are kept for area border routers and AS boundary
	//        routers.  Routing table entries for area border routers are used
	//        when calculating the inter-area routes (see Section 16.2), and
	//        when maintaining configured virtual links (see Section 15).
	//        Routing table entries for AS boundary routers are used when
	//        calculating the AS external routes (see Section 16.4).
	DestinationType RoutingDestinationType
	// The destination's identifier or name.  This depends on the
	//        Destination Type.  For networks, the identifier is their
	//        associated IP address.  For routers, the identifier is the OSPF
	//        Router ID.[9]
	DestinationId uint32
	// Only defined for networks.  The network's IP address together
	//        with its address mask defines a range of IP addresses.  For IP
	//        subnets, the address mask is referred to as the subnet mask.
	//        For host routes, the mask is "all ones" (0xffffffff).
	AddressMask net.IPMask
	// When the destination is a router this field indicates the
	//        optional OSPF capabilities supported by the destination router.
	//        The only optional capability defined by this specification is
	//        the ability to process AS-external-LSAs.  For a further
	//        discussion of OSPF's optional capabilities, see Section 4.5.
	Options packet.BitOption

	//    The set of paths to use for a destination may vary based on the OSPF
	//    area to which the paths belong.  This means that there may be
	//    multiple routing table entries for the same destination, depending
	//    on the values of the next field.

	// This field indicates the area whose link state information has
	//        led to the routing table entry's collection of paths.  This is
	//        called the entry's associated area.  For sets of AS external
	//        paths, this field is not defined.  For destinations of type
	//        "router", there may be separate sets of paths (and therefore
	//        separate routing table entries) associated with each of several
	//        areas. For example, this will happen when two area border
	//        routers share multiple areas in common.  For destinations of
	//        type "network", only the set of paths associated with the best
	//        area (the one providing the preferred route) is kept.
	Area uint32

	//    The rest of the routing table entry describes the set of paths to
	//    the destination.  The following fields pertain to the set of paths
	//    as a whole.  In other words, each one of the paths contained in a
	//    routing table entry is of the same path-type and cost (see below).

	// There are four possible types of paths used to route traffic to
	//        the destination, listed here in decreasing order of preference:
	//        intra-area, inter-area, type 1 external or type 2 external.
	//        Intra-area paths indicate destinations belonging to one of the
	//        router's attached areas.  Inter-area paths are paths to
	//        destinations in other OSPF areas.  These are discovered through
	//        the examination of received summary-LSAs.  AS external paths are
	//        paths to destinations external to the AS.  These are detected
	//        through the examination of received AS-external-LSAs.
	PathType RoutingPathType

	// The link state cost of the path to the destination.  For all
	//        paths except type 2 external paths this describes the entire
	//        path's cost.  For Type 2 external paths, this field describes
	//        the cost of the portion of the path internal to the AS.  This
	//        cost is calculated as the sum of the costs of the path's
	//        constituent links.
	Cost int
	// Only valid for type 2 external paths.  For these paths, this
	//        field indicates the cost of the path's external portion.  This
	//        cost has been advertised by an AS boundary router, and is the
	//        most significant part of the total path cost.  For example, a
	//        type 2 external path with type 2 cost of 5 is always preferred
	//        over a path with type 2 cost of 10, regardless of the cost of
	//        the two paths' internal components.
	CostType2 int
	// Valid only for intra-area paths, this field indicates the LSA
	//        (router-LSA or network-LSA) that directly references the
	//        destination.  For example, if the destination is a transit
	//        network, this is the transit network's network-LSA.  If the
	//        destination is a stub network, this is the router-LSA for the
	//        attached router.  The LSA is discovered during the shortest-path
	//        tree calculation (see Section 16.1).  Multiple LSAs may
	//        reference the destination, however a tie-breaking scheme always
	//        reduces the choice to a single LSA. The Link State Origin field
	//        is not used by the OSPF protocol, but it is used by the routing
	//        table calculation in OSPF's Multicast routing extensions
	//        (MOSPF).
	LinkStateOrigin []byte

	//    When multiple paths of equal path-type and cost exist to a
	//    destination (called elsewhere "equal-cost" paths), they are stored
	//    in a single routing table entry.  Each one of the "equal-cost" paths
	//    is distinguished by the following fields:

	// The outgoing router interface to use when forwarding traffic to
	//        the destination.  On broadcast, Point-to-MultiPoint and NBMA
	//        networks, the next hop also includes the IP address of the next
	//        router (if any) in the path towards the destination.
	NextHop *Interface

	// Valid only for inter-area and AS external paths.  This field
	//        indicates the Router ID of the router advertising the summary-
	//        LSA or AS-external-LSA that led to this path.
	AdvertisingRouter uint32
}
