package ospf

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"golang.org/x/net/ipv4"

	"github.com/v2fly/v2ray-core/v5/app/dnscircuit/ospf/iface"
)

const (
	// AllSPFRouters 广播网络上，所有运行 OSPF 的路由器必须准备接收发送到该地址的包. per RFC2328 A.1
	AllSPFRouters = "224.0.0.5"
	allSPFRouters = 224<<24 | 0<<16 | 0<<8 | 5
	// AllDRouters 广播网络上，DR 和 BDR 必须准备接收发送到该地址的包. per RFC2328 A.1
	AllDRouters = "224.0.0.6"
	allDRouters = 224<<24 | 0<<16 | 0<<8 | 6

	// IPProtocolNum OSPF申请的IP协议号. per RFC2328 A.1
	IPProtocolNum = 89
	// IPPacketTos OSPF使用的IP包类别和优先级. per RFC2328 A.1 and RFC791 3.1
	IPPacketTos = 0b11000000 // 0xc0

	MulticastTTL = 1
)

type Conn struct {
	multicastGroups []*net.IPAddr
	ifi             *net.Interface
	listenAddr      string
	srcIP           net.IP
	wMu             *sync.Mutex
	rc              *ipv4.RawConn
}

func (o *Conn) Close() error {
	for _, gp := range o.multicastGroups {
		_ = o.rc.LeaveGroup(o.ifi, gp)
	}
	return o.rc.Close()
}

func ListenOSPFv2Multicast(ctx context.Context, ifi *net.Interface, addr string, srcip string) (ospf *Conn, err error) {
	ospf = &Conn{
		listenAddr: addr,
		srcIP:      net.ParseIP(srcip),
		wMu:        &sync.Mutex{},
		ifi:        ifi,
	}
	rc, err := iface.ListenIPv4ByProtocol(ctx, IPProtocolNum, addr,
		func(rc *ipv4.RawConn) error {
			// 绑定多播的接口
			if err := rc.SetMulticastInterface(ifi); err != nil {
				return fmt.Errorf("err SetMulticastInterface: %w", err)
			}
			return nil
		}, func(rc *ipv4.RawConn) error {
			// 为了确保多播的包不会传送多跳，IP 包的 TTL 必须设定为 1
			if err := rc.SetMulticastTTL(MulticastTTL); err != nil {
				return fmt.Errorf("err SetMulticastTTL: %w", err)
			}
			return nil
		}, func(rc *ipv4.RawConn) error {
			// 所有的 OSPF 路由协议包使用数值为二进制 0000 的普通 TOS 服务,
			// 路由协议包中的 IP 优先级被应该被设定为 Internetwork Control
			if err := rc.SetTOS(IPPacketTos); err != nil {
				return fmt.Errorf("err SetTOS: %w", err)
			}
			return nil
		}, func(rc *ipv4.RawConn) error {
			if err := rc.SetMulticastLoopback(false); err != nil {
				return fmt.Errorf("err SetMulticastLoopback: %w", err)
			}
			return nil
		}, func(rc *ipv4.RawConn) (err error) {
			// 加入指定的multicast group
			groups := []*net.IPAddr{{IP: net.ParseIP(AllSPFRouters)}}
			// 只要不是PTP接口就绑一下DR
			if ifi.Flags&net.FlagPointToPoint == 0 {
				groups = append(groups, &net.IPAddr{IP: net.ParseIP(AllDRouters)})
			}
			for _, gp := range groups {
				if err = rc.JoinGroup(ifi, gp); err != nil {
					return fmt.Errorf("err at join multicast IP %v at ifi %v: %w", gp, ifi, err)
				}
			}
			ospf.multicastGroups = groups
			return
		})
	if err != nil {
		return
	}
	ospf.rc = rc

	return
}

func (o *Conn) Read(buf []byte) (int, *ipv4.Header, error) {
	_ = o.rc.SetReadDeadline(time.Now().Add(1 * time.Second))
	h, payload, _, err := o.rc.ReadFrom(buf)
	return len(payload) + ipv4.HeaderLen, h, err
}

func (o *Conn) fixIPv4HeaderForSend(b []byte) {
	switch runtime.GOOS {
	case "darwin", "ios":
		// Before FreeBSD 11.0 packets received on raw IP sockets had the ip_len and ip_off fields converted to host byte order.
		// Packets written to raw IP sockets were expected to have ip_len and ip_off in host byte order.
		packetLen := binary.BigEndian.Uint16(b[2:4])
		binary.NativeEndian.PutUint16(b[2:4], packetLen)
		flagsAndFragOff := binary.BigEndian.Uint16(b[6:8])
		binary.NativeEndian.PutUint16(b[6:8], flagsAndFragOff)
	}
}

func (o *Conn) WriteTo(ospfMsg []byte, dst *net.IPAddr) (n int, err error) {
	ip := &layers.IPv4{
		Version:  ipv4.Version,
		TTL:      MulticastTTL,
		TOS:      IPPacketTos,
		Protocol: IPProtocolNum,
		SrcIP:    o.srcIP,
		DstIP:    dst.IP,
	}
	p := gopacket.NewSerializeBuffer()
	err = gopacket.SerializeLayers(p, gopacket.SerializeOptions{
		FixLengths: true,
	}, ip, gopacket.Payload(ospfMsg))
	if err != nil {
		return 0, err
	}
	o.fixIPv4HeaderForSend(p.Bytes())

	o.wMu.Lock()
	defer o.wMu.Unlock()
	err = o.rc.SetWriteDeadline(time.Now().Add(1 * time.Second))
	if err != nil {
		return 0, err
	}
	n, err = o.rc.WriteToIP(p.Bytes(), dst)
	return
}

func (o *Conn) WriteMulticastAllSPF(buf []byte) (n int, err error) {
	dst := net.ParseIP(AllSPFRouters)
	dstIPAddr, _ := net.ResolveIPAddr("ip4", AllSPFRouters)
	ip := &layers.IPv4{
		Version:  ipv4.Version,
		TTL:      MulticastTTL,
		TOS:      IPPacketTos,
		Protocol: IPProtocolNum,
		SrcIP:    o.srcIP,
		DstIP:    dst,
	}
	pBuf := gopacket.NewSerializeBuffer()
	err = gopacket.SerializeLayers(pBuf, gopacket.SerializeOptions{
		FixLengths: true,
	}, ip, gopacket.Payload(buf))
	if err != nil {
		return 0, err
	}
	o.fixIPv4HeaderForSend(pBuf.Bytes())

	o.wMu.Lock()
	defer o.wMu.Unlock()
	err = o.rc.SetWriteDeadline(time.Now().Add(1 * time.Second))
	if err != nil {
		return 0, err
	}
	n, err = o.rc.WriteToIP(pBuf.Bytes(), dstIPAddr)
	return n, err
}

func dumpBuf(data []byte) string {
	ret := &strings.Builder{}
	for idx, b := range data {
		if idx > 0 {
			fmt.Fprintf(ret, " ")
		}
		if b <= 0xf {
			fmt.Fprintf(ret, "0%x", b)
		} else {
			fmt.Fprintf(ret, "%x", b)
		}
		if idx >= len(data)-1 {
			fmt.Fprintf(ret, "\n")
		}
	}
	return ret.String()
}
