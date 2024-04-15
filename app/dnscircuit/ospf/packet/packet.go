package packet

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
)

// OSPFv2Packet wrapper implements SerializeTo.
// Make it possible to serialize an OSPF message to wire format
type OSPFv2Packet[T OSPFPayloadV2] struct {
	layers.OSPFv2
	Content T //replace Content interface for marshaling
}

type SerializableLayerLayerWithType interface {
	gopacket.SerializableLayer
	GetType() layers.OSPFType
}

type OSPFPayloadV2 interface {
	HelloPayloadV2 | DbDescPayload |
		LSRequestPayload | LSUpdatePayload | LSAcknowledgementPayload
	marshalable
	String() string
}

var (
	ErrBufferLengthTooShort = errors.New("err buffer length too short")
)

type HelloPayloadV2 layers.HelloPkgV2

func (p HelloPayloadV2) Size() int {
	return 20 + 4*len(p.NeighborID)
}

func (p HelloPayloadV2) SerializeToSizedBuffer(b []byte) error {
	if len(b) < p.Size() {
		return ErrBufferLengthTooShort
	}
	binary.BigEndian.PutUint32(b[0:4], p.NetworkMask)
	binary.BigEndian.PutUint16(b[4:6], p.HelloInterval)
	b[6] = uint8(p.Options)
	b[7] = p.RtrPriority
	binary.BigEndian.PutUint32(b[8:12], p.RouterDeadInterval)
	binary.BigEndian.PutUint32(b[12:16], p.DesignatedRouterID)
	binary.BigEndian.PutUint32(b[16:20], p.BackupDesignatedRouterID)
	for idx, neighbor := range p.NeighborID {
		binary.BigEndian.PutUint32(b[20+idx*4:20+(idx+1)*4], neighbor)
	}
	return nil
}

type HelloPayloadV3 layers.HelloPkg

type DbDescPayload struct {
	layers.DbDescPkg
	LSAinfo []LSAheader
}

func (p DbDescPayload) Size() int {
	return 8 + LSAheader{}.Size()*len(p.LSAinfo)
}

func (p DbDescPayload) SerializeToSizedBuffer(b []byte) (err error) {
	if len(b) < p.Size() {
		return ErrBufferLengthTooShort
	}
	binary.BigEndian.PutUint16(b[0:2], p.InterfaceMTU)
	b[2] = uint8(p.Options)
	b[3] = uint8(p.Flags)
	binary.BigEndian.PutUint32(b[4:8], p.DDSeqNumber)
	for idx := 0; idx < len(p.LSAinfo); idx++ {
		lsaH := p.LSAinfo[idx]
		thisB := b[8+idx*lsaH.Size() : 8+(idx+1)*lsaH.Size()]
		if err = lsaH.SerializeToSizedBuffer(thisB); err != nil {
			return
		}
		// Don't force recalculate chksum since it is mostly directly retrieved from LSDB
		lsaH.recalculateChecksum(thisB, false)
		p.LSAinfo[idx] = lsaH
	}
	return
}

type LSRequestPayload []LSReq

func (p LSRequestPayload) Size() int {
	total := 0
	for _, eachP := range p {
		total += eachP.Size()
	}
	return total
}

func (p LSRequestPayload) SerializeToSizedBuffer(b []byte) (err error) {
	if len(b) < p.Size() {
		return ErrBufferLengthTooShort
	}
	offset := 0
	for _, eachP := range p {
		if err = eachP.SerializeToSizedBuffer(b[offset : offset+eachP.Size()]); err != nil {
			return
		}
		offset += eachP.Size()
	}
	return
}

// LSReq stands for a single link state request entry.
type LSReq layers.LSReq

func (p LSReq) GetLSAIdentity() LSAIdentity {
	return LSAIdentity{
		LSType:      p.LSType,
		LinkStateId: p.LSID,
		AdvRouter:   p.AdvRouter,
	}
}

func (p LSReq) Size() int {
	return 12
}

func (p LSReq) SerializeToSizedBuffer(b []byte) (err error) {
	if len(b) < p.Size() {
		return ErrBufferLengthTooShort
	}
	binary.BigEndian.PutUint32(b[0:4], uint32(p.LSType))
	binary.BigEndian.PutUint32(b[4:8], p.LSID)
	binary.BigEndian.PutUint32(b[8:12], p.AdvRouter)
	return
}

type LSUpdatePayload struct {
	layers.LSUpdate
	LSAs []LSAdvertisement
}

func (p LSUpdatePayload) Size() int {
	totalLen := 4
	for _, l := range p.LSAs {
		totalLen += l.Size()
	}
	return totalLen
}

func (p LSUpdatePayload) SerializeToSizedBuffer(b []byte) (err error) {
	if len(b) < p.Size() {
		return ErrBufferLengthTooShort
	}
	binary.BigEndian.PutUint32(b[0:4], p.NumOfLSAs)
	offset := 0
	for _, l := range p.LSAs {
		if err = l.SerializeToSizedBuffer(b[4+offset : 4+offset+l.Size()]); err != nil {
			return
		}
		offset += l.Size()
	}
	return
}

func (pt *LSUpdatePayload) parse() (err error) {
	pt.LSAs = make([]LSAdvertisement, 0, len(pt.LSUpdate.LSAs))
	for _, l := range pt.LSUpdate.LSAs {
		lsa := LSAdvertisement{LSA: l}
		if err = lsa.parse(); err != nil {
			return fmt.Errorf("err parse LSA: %w", err)
		}
		pt.LSAs = append(pt.LSAs, lsa)
	}
	return
}

type LSAcknowledgementPayload []LSAheader

func (p LSAcknowledgementPayload) Size() int {
	return LSAheader{}.Size() * len(p)
}

func (p LSAcknowledgementPayload) SerializeToSizedBuffer(b []byte) (err error) {
	if len(b) < p.Size() {
		return ErrBufferLengthTooShort
	}
	offset := 0
	for i := 0; i < len(p); i++ {
		h := p[i]
		thisB := b[offset : offset+h.Size()]
		if err = h.SerializeToSizedBuffer(thisB); err != nil {
			return
		}
		// Don't force recalculate chksum since it is mostly directly retrieved from LSUpdate
		h.recalculateChecksum(thisB, false)
		offset += h.Size()
		p[i] = h
	}
	return
}

type LSAdvertisement struct {
	layers.LSA
	LSAheader
	Content LSAContent
}

func (p LSAdvertisement) ValidateLSA() error {
	// (1) Validate the LSA's LS checksum.  If the checksum turns out to be
	//        invalid, discard the LSA and get the next one from the Link
	//        State Update packet.
	// TODO: validate LSA chksum

	// Examine the LSA's LS type.  If the LS type is unknown, discard
	//        the LSA and get the next one from the Link State Update Packet.
	//        This specification defines LS types 1-5 (see Section 4.3).
	switch p.LSType {
	case layers.RouterLSAtypeV2, layers.NetworkLSAtypeV2,
		layers.SummaryLSANetworktypeV2, layers.SummaryLSAASBRtypeV2,
		layers.ASExternalLSAtypeV2:
		return nil
	}
	return fmt.Errorf("unknown LSA type %d", p.LSType)
}

func (pt *LSAdvertisement) parse() error {
	pt.LSAheader = LSAheader(pt.LSA.LSAheader)
	if int(pt.Length) < pt.LSAheader.Size() {
		return fmt.Errorf("LSA too short")
	}
	switch pt.LSType {
	case layers.RouterLSAtypeV2:
		lsa, err := pt.AsV2RouterLSA()
		if err != nil {
			return err
		}
		pt.Content = lsa.Content
	case layers.NetworkLSAtypeV2:
		lsa, err := pt.AsV2NetworkLSA()
		if err != nil {
			return err
		}
		pt.Content = lsa.Content
	case layers.SummaryLSANetworktypeV2:
		lsa, err := pt.AsV2SummaryLSAType3()
		if err != nil {
			return err
		}
		pt.Content = lsa.Content
	case layers.SummaryLSAASBRtypeV2:
		lsa, err := pt.AsV2SummaryLSAType4()
		if err != nil {
			return err
		}
		pt.Content = lsa.Content
	case layers.ASExternalLSAtypeV2:
		lsa, err := pt.AsV2ASExternalLSA()
		if err != nil {
			return err
		}
		pt.Content = lsa.Content
	default:
		return fmt.Errorf("LSA.LSType(%x) not implemented", pt.LSType)
	}
	return nil
}

func (p *LSAdvertisement) FixLengthAndChkSum() error {
	buf := make([]byte, p.Size())
	return p.SerializeToSizedBuffer(buf)
}

func (p LSAdvertisement) Size() int {
	if p.Length > 0 {
		return int(p.Length)
	}
	return p.LSAheader.Size() + p.Content.Size()
}

func (p *LSAdvertisement) SerializeToSizedBuffer(b []byte) (err error) {
	if len(b) < p.Size() || len(b) < p.LSAheader.Size() || len(b) < p.Content.Size() {
		return ErrBufferLengthTooShort
	}

	// always fix length
	p.LSAheader.Length = uint16(p.LSAheader.Size() + p.Content.Size())
	if err = p.LSAheader.SerializeToSizedBuffer(b[0:p.LSAheader.Size()]); err != nil {
		return
	}
	if err = p.Content.SerializeToSizedBuffer(b[p.LSAheader.Size() : p.LSAheader.Size()+p.Content.Size()]); err != nil {
		return
	}
	// always fix chksum
	p.LSAheader.recalculateChecksum(b, true)
	return
}

// Fletcher-16 checksum
// refers https://github.com/vyos-legacy/vyatta-quagga/blob/current/lib/checksum.c#L55
func lsaChecksum(b []byte, offset int32) uint16 {
	var (
		c0, c1 int32 = 0, 0
	)
	const (
		modX = 4102
	)
	left := len(b)
	for left != 0 {
		partial := min(left, modX)
		for i := 0; i < partial; i++ {
			c0 = c0 + int32(b[i])
			c1 += c0
		}
		c0 = c0 % 255
		c1 = c1 % 255
		left -= partial
	}
	x := ((int32(len(b))-offset-1)*c0 - c1) % 255
	if x <= 0 {
		x += 255
	}
	y := 510 - c0 - x
	if y > 255 {
		y -= 255
	}
	return uint16(x<<8) | uint16(y&0xff)
}

type LSAheader layers.LSAheader

func (p LSAheader) GetLSAIdentity() LSAIdentity {
	return LSAIdentity{
		LSType:      p.LSType,
		LinkStateId: p.LinkStateID,
		AdvRouter:   p.AdvRouter,
	}
}

func (p LSAheader) GetLSAck() LSAheader {
	return p
}

func (p LSAheader) IsMoreRecentThan(toCompare LSAheader) bool {
	if p.LSSeqNumber != toCompare.LSSeqNumber {
		// The LSA having the newer LS sequence number is more recent.
		return int32(p.LSSeqNumber) > int32(toCompare.LSSeqNumber)
	}
	// If the two instances have different LS checksums, then the
	// instance having the larger LS checksum (when considered as a
	// 16-bit unsigned integer) is considered more recent.
	if p.LSChecksum != toCompare.LSChecksum {
		return p.LSChecksum > toCompare.LSChecksum
	}
	// if only one of the instances has its LS age field set
	// to MaxAge, the instance of age MaxAge is considered to be more recent.
	if p.LSAge == MaxAge && toCompare.LSAge != MaxAge {
		return true
	}
	// if the LS age fields of the two instances differ by
	// more than MaxAgeDiff, the instance having the smaller (younger)
	// LS age is considered to be more recent.
	if int32(math.Abs(float64(int32(p.LSAge)-int32(toCompare.LSAge)))) > MaxAgeDiff &&
		p.LSAge < toCompare.LSAge {
		return true
	}
	// Else, the two instances are considered to be identical.
	return false
}

func (p LSAheader) IsSame(toCompare LSAheader) bool {
	return !p.IsMoreRecentThan(toCompare) && !toCompare.IsMoreRecentThan(p)
}

func (p LSAheader) GetLSReq() LSReq {
	return LSReq{
		LSType:    p.LSType,
		LSID:      p.LinkStateID,
		AdvRouter: p.AdvRouter,
	}
}

func (p LSAheader) Size() int {
	return 20
}

func (p *LSAheader) Ager(t uint16) uint16 {
	if p.LSAge+t > MaxAge {
		p.LSAge = MaxAge
		return MaxAge // MaxAge will be processed while LSDB aging
	}
	p.LSAge += t
	return p.LSAge
}

func (p *LSAheader) PrepareReOriginating(incrSeq bool) (seqIncred bool) {
	p.LSAge = 0
	if incrSeq {
		if int32(p.LSSeqNumber) < MaxSequenceNumber {
			p.LSSeqNumber = uint32(int32(p.LSSeqNumber) + 1)
			return true
		}
		p.LSSeqNumber = MaxSequenceNumber // MaxSeqNum will be processed while LSDB aging
	}
	return false
}

func (p *LSAheader) recalculateChecksum(b []byte, forceRecalculation bool) {
	// fix length if it is not set
	if p.Length <= 0 {
		p.Length = uint16(p.Size())
		binary.BigEndian.PutUint16(b[18:20], p.Length)
	}
	if p.LSChecksum <= 0 || forceRecalculation {
		// clear chksum bytes
		clear(b[16:18])
		// The Fletcher checksum of the complete contents of the LSA,
		// including the LSA header but excluding the LS age field.
		// So the offset of checksum should be 14 not 16.
		p.LSChecksum = lsaChecksum(b[2:], 14)
		//binary.BigEndian.PutUint16(b[18:20], p.Length)
		binary.BigEndian.PutUint16(b[16:18], p.LSChecksum)
	}
}

func (p LSAheader) SerializeToSizedBuffer(b []byte) error {
	if len(b) < p.Size() {
		return ErrBufferLengthTooShort
	}
	binary.BigEndian.PutUint16(b[0:2], p.LSAge)
	b[2] = p.LSOptions
	b[3] = uint8(p.LSType)
	binary.BigEndian.PutUint32(b[4:8], p.LinkStateID)
	binary.BigEndian.PutUint32(b[8:12], p.AdvRouter)
	binary.BigEndian.PutUint32(b[12:16], p.LSSeqNumber)
	binary.BigEndian.PutUint16(b[16:18], p.LSChecksum)
	binary.BigEndian.PutUint16(b[18:20], p.Length)
	return nil
}

type marshalable interface {
	Size() int
	SerializeToSizedBuffer(b []byte) error
}

type LSAContent interface {
	isLSAContent() // make it private
	marshalable
	String() string
}

type rawLSA []byte

func (p rawLSA) isLSAContent() {}

func (p rawLSA) Size() int {
	return len(p)
}

func (p rawLSA) SerializeToSizedBuffer(b []byte) error {
	if len(b) < len(p) {
		return ErrBufferLengthTooShort
	}
	copy(b, p)
	return nil
}

func (v2 *OSPFv2Packet[T]) packetErr(format string, args ...interface{}) error {
	return fmt.Errorf("malformed ospfv2 %s packet: "+format, append([]interface{}{v2.Type}, args...)...)
}

func ipPacketChecksum(bytes []byte) uint16 {
	// Compute ipPacketChecksum
	var csum uint32
	for i := 0; i < len(bytes); i += 2 {
		csum += uint32(bytes[i]) << 8
		csum += uint32(bytes[i+1])
	}
	for {
		// Break when sum is less or equals to 0xFFFF
		if csum <= 65535 {
			break
		}
		// Add carry to the sum
		csum = (csum >> 16) + uint32(uint16(csum))
	}
	// Flip all the bits
	return ^uint16(csum)
}

func (v2 *OSPFv2Packet[T]) GetType() layers.OSPFType {
	return v2.Type
}

// SerializeTo writes the serialized form of this layer into the
// SerializationBuffer, implementing gopacket.SerializableLayer.
func (v2 *OSPFv2Packet[T]) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) (err error) {
	// proc header at last
	defer func() {
		if err != nil {
			// some err occurred before, simply return it
			return
		}
		var h []byte
		h, err = b.PrependBytes(24) // per RFC2328 A.3.1
		if err != nil {
			return
		}
		// header manipulation
		h[0] = v2.Version
		h[1] = uint8(v2.Type)
		if opts.FixLengths {
			v2.PacketLength = uint16(len(b.Bytes()))
		}
		binary.BigEndian.PutUint16(h[2:4], v2.PacketLength)
		binary.BigEndian.PutUint32(h[4:8], v2.RouterID)
		binary.BigEndian.PutUint32(h[8:12], v2.AreaID)
		binary.BigEndian.PutUint16(h[14:16], v2.AuType)
		if opts.ComputeChecksums {
			// clear ipPacketChecksum bytes
			clear(h[12:14])
			// clear authentication bytes
			clear(h[16:24])
			v2.Checksum = ipPacketChecksum(b.Bytes())
		}
		binary.BigEndian.PutUint16(h[12:14], v2.Checksum)
		// ipPacketChecksum calculation must exclude 64bit authentication bytes.
		// so make it last set.
		binary.BigEndian.PutUint64(h[16:24], v2.Authentication)
	}()

	// proc payload first
	p, err := b.AppendBytes(v2.Content.Size())
	if err != nil {
		return
	}
	return v2.Content.SerializeToSizedBuffer(p)
}
