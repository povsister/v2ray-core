package packet

import (
	"fmt"
	"net"
	"strings"
)

func uint32ToIPv4(ip uint32) net.IP {
	return net.IPv4(byte(ip>>24), byte(ip>>16), byte(ip>>8), byte(ip))
}

const (
	padding  = "  "
	padding2 = "    "
)

func (v2 *OSPFv2Packet[T]) String() string {
	buf := new(strings.Builder)
	buf.WriteString(fmt.Sprintf("OSPFv%d | %s | PacketLength: %d\n", v2.Version, v2.Type.String(), v2.PacketLength))
	buf.WriteString(fmt.Sprintf("AreaId: %d | RouterId: %s\n", v2.AreaID, uint32ToIPv4(v2.RouterID).String()))
	buf.WriteString(v2.Content.String())
	return buf.String()
}

func (p HelloPayloadV2) String() string {
	buf := new(strings.Builder)
	buf.WriteString(fmt.Sprintf("Hello Content:\n"+
		padding+"HelloInterval: %d, Options: %d, Priority: %d\n"+
		padding+"RouterDeadInterval: %d\n"+
		padding+"DR: %s, BDR: %s\n"+
		padding+"NetworkMask: %s",
		p.HelloInterval, p.Options, p.RtrPriority,
		p.RouterDeadInterval,
		uint32ToIPv4(p.DesignatedRouterID).String(), uint32ToIPv4(p.BackupDesignatedRouterID).String(),
		uint32ToIPv4(p.NetworkMask)))
	for i := 0; i < len(p.NeighborID); i += 3 {
		buf.WriteString("\n")
		if i == 0 {
			buf.WriteString(padding + "NeighborID: ")
		}
		for j := i; j < i+3; j++ {
			if j < len(p.NeighborID) {
				buf.WriteString(padding2 + uint32ToIPv4(p.NeighborID[j]).String() + " ")
			}
		}
	}
	return buf.String()
}

func (p DbDescPayload) String() string {
	buf := new(strings.Builder)
	buf.WriteString(fmt.Sprintf("Database Description Content:\n"))
	buf.WriteString(fmt.Sprintf(padding+"InterfaceMTU: %d, Options: %d, Flags: %d\n",
		p.InterfaceMTU, p.Options, p.Flags))
	buf.WriteString(fmt.Sprintf(padding+"DDSequenceNumber: %d", p.DDSeqNumber))
	for i, l := range p.LSAinfo {
		buf.WriteString("\n")
		if i == 0 {
			buf.WriteString(padding + "LSAInfo:\n")
		}
		buf.WriteString(fmt.Sprintf(padding2+"%+v", l))
	}
	return buf.String()
}

func (p LSRequestPayload) String() string {
	buf := new(strings.Builder)
	buf.WriteString(fmt.Sprintf("LSRequest Content:"))
	for _, l := range p {
		buf.WriteString("\n")
		buf.WriteString(fmt.Sprintf(padding+"%+v", l))
	}
	return buf.String()
}

func (p LSUpdatePayload) String() string {
	buf := new(strings.Builder)
	buf.WriteString(fmt.Sprintf("LSUpdate Content:\n"))
	buf.WriteString(padding + fmt.Sprintf("NumOfLSAs: %d", p.NumOfLSAs))
	for _, l := range p.LSAs {
		buf.WriteString("\n")
		buf.WriteString(padding + l.String())
	}
	return buf.String()
}

func (p LSAcknowledgementPayload) String() string {
	buf := new(strings.Builder)
	buf.WriteString(fmt.Sprintf("LSAcknowledgement Content:"))
	for _, l := range p {
		buf.WriteString("\n")
		buf.WriteString(fmt.Sprintf(padding+"%+v", l))
	}
	return buf.String()
}

func (p LSAdvertisement) String() string {
	buf := new(strings.Builder)
	buf.WriteString(fmt.Sprintf("LSAheader: %+v\n", p.LSAheader))
	buf.WriteString(fmt.Sprintf("LSAContent: %+v", p.Content.String()))
	return buf.String()
}

func (p V2RouterLSA) String() string {
	return fmt.Sprintf("{Flags:%d Links:%d Routers:%+v}",
		p.Flags, p.Links, p.Routers)
}

func (p RouterV2) String() string {
	return fmt.Sprintf("{Type:%d LinkId:%d LinkData: %d Metric:%d TOSNum:%d TOSs:%+v}",
		p.Type, p.LinkID, p.LinkData, p.Metric, p.TOSNum, p.TOSs)
}

func (p V2NetworkLSA) String() string {
	return fmt.Sprintf("{NetworkMask:%d AttachedRouter:%+v}",
		p.NetworkMask, p.AttachedRouter)
}

func (p V2SummaryLSAType3) String() string {
	return p.V2SummaryLSAImpl.String()
}

func (p V2SummaryLSAType4) String() string {
	return p.V2SummaryLSAImpl.String()
}

func (p V2SummaryLSAImpl) String() string {
	return fmt.Sprintf("{NetworkMask:%d Metric:%d}",
		p.NetworkMask, p.Metric)
}

func (p V2ASExternalLSA) String() string {
	return fmt.Sprintf("{NetworkMask:%d  ExternalBit:%d Metric:%d ForwardingAddress:%d ExternalRouteTag: %d}",
		p.NetworkMask, p.ExternalBit, p.Metric, p.ForwardingAddress, p.ExternalRouteTag)
}
