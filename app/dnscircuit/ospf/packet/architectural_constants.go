package packet

import (
	"math/rand/v2"
	"time"
)

// Several OSPF protocol parameters have fixed architectural values.
//    These parameters have been referred to in the text by names such as
//    LSRefreshTime.  The same naming convention is used for the
//    configurable protocol parameters.  They are defined in Appendix C.
//
//    The name of each architectural constant follows, together with its
//    value and a short description of its function.

const (
	// LSRefreshTime The maximum time between distinct originations of any particular
	//        LSA.  If the LS age field of one of the router's self-originated
	//        LSAs reaches the value LSRefreshTime, a new instance of the LSA
	//        is originated, even though the contents of the LSA (apart from
	//        the LSA header) will be the same.  The value of LSRefreshTime is
	//        set to 30 minutes.
	LSRefreshTime = 30 * 60

	// MinLSInterval The minimum time between distinct originations of any particular
	//        LSA.  The value of MinLSInterval is set to 5 seconds.
	MinLSInterval = 5

	// MinLSArrival For any particular LSA, the minimum time that must elapse
	//        between reception of new LSA instances during flooding. LSA
	//        instances received at higher frequencies are discarded. The
	//        value of MinLSArrival is set to 1 second.
	MinLSArrival = 1

	// MaxAge The maximum age that an LSA can attain. When an LSA's LS age
	//        field reaches MaxAge, it is reflooded in an attempt to flush the
	//        LSA from the routing domain (See Section 14). LSAs of age MaxAge
	//        are not used in the routing table calculation.  The value of
	//        MaxAge is set to 1 hour.
	MaxAge = 60 * 60

	// CheckAge When the age of an LSA in the link state database hits a
	//        multiple of CheckAge, the LSA's checksum is verified.  An
	//        incorrect checksum at this time indicates a serious error.  The
	//        value of CheckAge is set to 5 minutes.
	CheckAge = 5 * 60

	// MaxAgeDiff The maximum time dispersion that can occur, as an LSA is flooded
	//        throughout the AS.  Most of this time is accounted for by the
	//        LSAs sitting on router output queues (and therefore not aging)
	//        during the flooding process.  The value of MaxAgeDiff is set to
	//        15 minutes.
	MaxAgeDiff = 15 * 60

	// LSInfinity The metric value indicating that the destination described by an
	//        LSA is unreachable. Used in summary-LSAs and AS-external-LSAs as
	//        an alternative to premature aging (see Section 14.1). It is
	//        defined to be the 24-bit binary value of all ones: 0xffffff.
	LSInfinity = 0xffffff

	// DefaultDestination The Destination ID that indicates the default route.  This route
	//        is used when no other matching routing table entry can be found.
	//        The default destination can only be advertised in AS-external-
	//        LSAs and in stub areas' type 3 summary-LSAs.  Its value is the
	//        IP address 0.0.0.0. Its associated Network Mask is also always
	//        0.0.0.0.
	DefaultDestination = 0

	// The sequence number field is a signed 32-bit integer.  It is
	//            used to detect old and duplicate LSAs.  The space of
	//            sequence numbers is linearly ordered.  The larger the
	//            sequence number (when compared as signed 32-bit integers)
	//            the more recent the LSA.  To describe to sequence number
	//            space more precisely, let N refer in the discussion below to
	//            the constant 2**31.
	//
	//            The sequence number -N (0x80000000) is reserved (and
	//            unused).  This leaves -N + 1 (0x80000001) as the smallest
	//            (and therefore oldest) sequence number; this sequence number
	//            is referred to as the constant InitialSequenceNumber. A
	//            router uses InitialSequenceNumber the first time it
	//            originates any LSA.  Afterwards, the LSA's sequence number
	//            is incremented each time the router originates a new
	//            instance of the LSA.  When an attempt is made to increment
	//            the sequence number past the maximum value of N - 1
	//            (0x7fffffff; also referred to as MaxSequenceNumber), the
	//            current instance of the LSA must first be flushed from the
	//            routing domain.  This is done by prematurely aging the LSA
	//            (see Section 14.1) and reflooding it.  As soon as this flood
	//            has been acknowledged by all adjacent neighbors, a new
	//            instance can be originated with sequence number of
	//            InitialSequenceNumber.

	// InitialSequenceNumber The value used for LS Sequence Number when originating the first
	//        instance of any LSA. Its value is the signed 32-bit integer
	//        0x80000001.
	InitialSequenceNumber = 0x80000001

	// MaxSequenceNumber The maximum value that LS Sequence Number can attain.  Its value
	//        is the signed 32-bit integer 0x7fffffff.
	MaxSequenceNumber = 0x7fffffff
)

var (
	RandSource = rand.New(rand.NewPCG(uint64(time.Now().Unix()), uint64(time.Now().Unix()/2)))
)
