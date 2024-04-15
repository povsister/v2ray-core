package packet

import (
	"encoding/binary"

	"github.com/gopacket/gopacket/layers"
)

// LSAIdentity The LS type, Link State ID, and Advertising Router fields
// can uniquely identify an LSA.
// If two LSAs have the same LS type, Link State ID, and Advertising Router fields,
// the two LSAs are considered to be the same, with one being old and the other being new.
type LSAIdentity struct {
	LSType      uint16
	LinkStateId uint32
	AdvRouter   uint32
}

var InvalidLSAIdentity = LSAIdentity{}

type LSAdvPayload interface {
	V2RouterLSA | V2NetworkLSA |
		V2SummaryLSAType3 | V2SummaryLSAType4 |
		V2ASExternalLSA
	marshalable
}

type LSAdv[T LSAdvPayload] struct {
	LSAdvertisement
	Content T
}

type V2RouterLSA struct {
	layers.RouterLSAV2
	Routers []RouterV2
}

type RouterV2 struct {
	layers.RouterV2
	TOSNum uint8
	TOSs   []LegacyTOSInfo
}

// LegacyTOSInfo for backward compatibility with previous versions
// of the OSPF specification. aka RFC 1583
type LegacyTOSInfo struct {
	// IP Type of Service that this metric refers to.  The encoding of
	// TOS in OSPF LSAs is described in RFC 2328 Section 12.3.
	TOS uint8
	// TOS-specific metric information.
	// Only the lower 3 bytes are used.
	TOSMetric uint32
}

type V2NetworkLSA layers.NetworkLSAV2

// V2SummaryLSAImpl implements summary LSA marshaling.
// The format of Type 3 and 4 summary-LSAs is identical.
type V2SummaryLSAImpl struct {
	// For Type 3 summary-LSAs, this indicates the destination
	// network's IP address mask.  For example, when advertising the
	// location of a class A network the value 0xff000000 would be
	// used.  This field is not meaningful and must be zero for Type 4
	// summary-LSAs.
	NetworkMask uint32
	// The cost of this route.  Expressed in the same units as the
	// interface costs in the router-LSAs.
	// Only the lower 3 bytes are used.
	Metric uint32
}

// V2SummaryLSAType3 is alson known as ABR-SummaryLSA or SummaryLSA-IPNetwork.
// The format of Type 3 and 4 summary-LSAs is identical.
type V2SummaryLSAType3 struct {
	V2SummaryLSAImpl
}

// V2SummaryLSAType4 is also known as ASBR-Summary LSA.
// The format of Type 3 and 4 summary-LSAs is identical.
type V2SummaryLSAType4 struct {
	V2SummaryLSAImpl
}

type V2ASExternalLSA layers.ASExternalLSAV2

func (p V2RouterLSA) isLSAContent() {}

func (p V2RouterLSA) Size() int {
	totalLen := 4
	for _, r := range p.Routers {
		totalLen += r.Size()
	}
	return totalLen
}

func (p V2RouterLSA) SerializeToSizedBuffer(b []byte) (err error) {
	if len(b) < p.Size() {
		return ErrBufferLengthTooShort
	}
	b[0] = p.Flags
	b[1] = 0
	binary.BigEndian.PutUint16(b[2:4], p.Links)
	offset := 0
	for _, r := range p.Routers {
		if err = r.SerializeToSizedBuffer(b[4+offset : 4+offset+r.Size()]); err != nil {
			return
		}
		offset += r.Size()
	}
	return
}

func (p RouterV2) Size() int {
	return 12 + int(p.TOSNum)*4
}

func (p RouterV2) SerializeToSizedBuffer(b []byte) (err error) {
	if len(b) < p.Size() {
		return ErrBufferLengthTooShort
	}
	binary.BigEndian.PutUint32(b[0:4], p.LinkID)
	binary.BigEndian.PutUint32(b[4:8], p.LinkData)
	b[8] = p.Type
	b[9] = p.TOSNum
	binary.BigEndian.PutUint16(b[10:12], p.Metric)
	for i := 0; i < int(p.TOSNum); i++ {
		tosB := b[12+i*4 : 12+(i+1)*4]
		// only the lower 2 bytes are used
		binary.BigEndian.PutUint32(tosB, p.TOSs[i].TOSMetric&0x0000FFFF)
		tosB[0] = p.TOSs[i].TOS
	}
	return
}

func (p V2NetworkLSA) isLSAContent() {}

func (p V2NetworkLSA) Size() int {
	return 4 + 4*len(p.AttachedRouter)
}

func (p V2NetworkLSA) SerializeToSizedBuffer(b []byte) (err error) {
	if len(b) < p.Size() {
		return ErrBufferLengthTooShort
	}
	binary.BigEndian.PutUint32(b[0:4], p.NetworkMask)
	for i := 0; i < len(p.AttachedRouter); i++ {
		binary.BigEndian.PutUint32(b[4+i*4:4+(i+1)*4], p.AttachedRouter[i])
	}
	return
}

func (p V2SummaryLSAImpl) isLSAContent() {}

func (p V2SummaryLSAImpl) Size() int {
	return 8
}

func (p V2SummaryLSAImpl) SerializeToSizedBuffer(b []byte) (err error) {
	if len(b) < p.Size() {
		return ErrBufferLengthTooShort
	}
	binary.BigEndian.PutUint32(b[0:4], p.NetworkMask)
	binary.BigEndian.PutUint32(b[4:8], p.Metric&0x00FFFFFF)
	return
}

func (p V2ASExternalLSA) isLSAContent() {}

func (p V2ASExternalLSA) Size() int {
	return 16
}

func (p V2ASExternalLSA) SerializeToSizedBuffer(b []byte) (err error) {
	if len(b) < p.Size() {
		return ErrBufferLengthTooShort
	}
	binary.BigEndian.PutUint32(b[0:4], p.NetworkMask)
	binary.BigEndian.PutUint32(b[4:8], p.Metric&0x00FFFFFF)
	b[4] = p.ExternalBit
	binary.BigEndian.PutUint32(b[8:12], p.ForwardingAddress)
	binary.BigEndian.PutUint32(b[12:16], p.ExternalRouteTag)
	return
}
