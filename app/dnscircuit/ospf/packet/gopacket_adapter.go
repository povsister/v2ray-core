package packet

import (
	"errors"
	"fmt"
	"unsafe"

	"github.com/gopacket/gopacket/layers"
)

var (
	ErrNotImplemented = errors.New("not implemented")
)

type LayerOSPFv2 layers.OSPFv2

func (l *LayerOSPFv2) AsHello() (*OSPFv2Packet[HelloPayloadV2], error) {
	if hello, ok := l.Content.(layers.HelloPkgV2); ok {
		return &OSPFv2Packet[HelloPayloadV2]{
			OSPFv2:  layers.OSPFv2(*l),
			Content: HelloPayloadV2(hello),
		}, nil
	}
	return nil, fmt.Errorf("expecting layers.HelloPkgV2 but got %T", l.Content)
}

func (l *LayerOSPFv2) AsDbDescription() (*OSPFv2Packet[DbDescPayload], error) {
	if dbDesc, ok := l.Content.(layers.DbDescPkg); ok {
		return &OSPFv2Packet[DbDescPayload]{
			OSPFv2: layers.OSPFv2(*l),
			Content: DbDescPayload{
				DbDescPkg: dbDesc,
				LSAinfo:   *(*[]LSAheader)(unsafe.Pointer(&dbDesc.LSAinfo)),
			},
		}, nil
	}
	return nil, fmt.Errorf("expecting layers.DbDescPkg but got %T", l.Content)
}

func (l *LayerOSPFv2) AsLSRequest() (*OSPFv2Packet[LSRequestPayload], error) {
	if lsrs, ok := l.Content.([]layers.LSReq); ok {
		return &OSPFv2Packet[LSRequestPayload]{
			OSPFv2:  layers.OSPFv2(*l),
			Content: *(*LSRequestPayload)(unsafe.Pointer(&lsrs)),
		}, nil
	}
	return nil, fmt.Errorf("expecting []layers.LSReq but got %T", l.Content)
}

func (l *LayerOSPFv2) AsLSUpdate() (ret *OSPFv2Packet[LSUpdatePayload], err error) {
	if lsu, ok := l.Content.(layers.LSUpdate); ok {
		ret = &OSPFv2Packet[LSUpdatePayload]{
			OSPFv2:  layers.OSPFv2(*l),
			Content: LSUpdatePayload{LSUpdate: lsu},
		}
		defer func() {
			if err != nil {
				err = fmt.Errorf("err parse LSUpdate.Content: %w", err)
			}
		}()
		return ret, ret.Content.parse()
	}
	return nil, fmt.Errorf("expecting layers.LSUpdate but got %T", l.Content)
}

func (l *LayerOSPFv2) AsLSAcknowledgment() (*OSPFv2Packet[LSAcknowledgementPayload], error) {
	if lsahs, ok := l.Content.([]layers.LSAheader); ok {
		return &OSPFv2Packet[LSAcknowledgementPayload]{
			OSPFv2:  layers.OSPFv2(*l),
			Content: *(*LSAcknowledgementPayload)(unsafe.Pointer(&lsahs)),
		}, nil
	}
	return nil, fmt.Errorf("expecting []layers.LSAheader but got %T", l.Content)
}

func (p LSAdvertisement) AsV2RouterLSA() (ret LSAdv[V2RouterLSA], err error) {
	if p.LSA.Content == nil {
		if rtLSA, ok := p.Content.(V2RouterLSA); ok {
			return LSAdv[V2RouterLSA]{
				LSAdvertisement: p,
				Content:         rtLSA,
			}, nil
		}
	}
	if lsAdv, ok := p.LSA.Content.(layers.RouterLSAV2); ok {
		return LSAdv[V2RouterLSA]{
			LSAdvertisement: p,
			Content: V2RouterLSA{
				RouterLSAV2: lsAdv,
				Routers: func() (ret []RouterV2) {
					for _, r := range lsAdv.Routers {
						ret = append(ret, RouterV2{
							RouterV2: r,
						})
					}
					return
				}(),
			},
		}, nil
	}
	err = fmt.Errorf("expecting layers.RouterLSAV2 but got %T", p.LSA.Content)
	return
}

func (p LSAdvertisement) AsV2NetworkLSA() (ret LSAdv[V2NetworkLSA], err error) {
	if p.LSA.Content == nil {
		if ntLSA, ok := p.Content.(V2NetworkLSA); ok {
			return LSAdv[V2NetworkLSA]{
				LSAdvertisement: p,
				Content:         ntLSA,
			}, nil
		}
	}
	if lsAdv, ok := p.LSA.Content.(layers.NetworkLSAV2); ok {
		return LSAdv[V2NetworkLSA]{
			LSAdvertisement: p,
			Content:         V2NetworkLSA(lsAdv),
		}, nil
	}
	err = fmt.Errorf("expecting layers.NetworkLSAV2 but got %T", p.LSA.Content)
	return
}

func (p LSAdvertisement) AsV2SummaryLSAType3() (ret LSAdv[V2SummaryLSAType3], err error) {
	if p.LSA.Content == nil {
		if abrSm, ok := p.Content.(V2SummaryLSAImpl); ok {
			return LSAdv[V2SummaryLSAType3]{
				LSAdvertisement: p,
				Content:         V2SummaryLSAType3{abrSm},
			}, nil
		}
	}
	err = fmt.Errorf("err V2SummaryLSAType3 %w", ErrNotImplemented)
	return
}

func (p LSAdvertisement) AsV2SummaryLSAType4() (ret LSAdv[V2SummaryLSAType4], err error) {
	if p.LSA.Content == nil {
		if asbrSm, ok := p.Content.(V2SummaryLSAImpl); ok {
			return LSAdv[V2SummaryLSAType4]{
				LSAdvertisement: p,
				Content:         V2SummaryLSAType4{asbrSm},
			}, nil
		}
	}
	err = fmt.Errorf("err V2SummaryLSAType4 %w", ErrNotImplemented)
	return
}

func (p LSAdvertisement) AsV2ASExternalLSA() (ret LSAdv[V2ASExternalLSA], err error) {
	if p.LSA.Content == nil {
		if extLSA, ok := p.Content.(V2ASExternalLSA); ok {
			return LSAdv[V2ASExternalLSA]{
				LSAdvertisement: p,
				Content:         extLSA,
			}, nil
		}
	}
	if lsAdv, ok := p.LSA.Content.(layers.ASExternalLSAV2); ok {
		return LSAdv[V2ASExternalLSA]{
			LSAdvertisement: p,
			Content:         V2ASExternalLSA(lsAdv),
		}, nil
	}
	err = fmt.Errorf("expecting layers.ASExternalLSAV2 but got %T", p.LSA.Content)
	return
}
