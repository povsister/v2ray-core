package iface

import (
	"context"
	"fmt"
	"net"
	"syscall"

	"golang.org/x/net/ipv4"
)

func ListenIPv4ByProtocol(ctx context.Context, protoNum int, addr string, modRc ...func(rc *ipv4.RawConn) error) (rc *ipv4.RawConn, err error) {
	lc := &net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) (err error) {
			return c.Control(func(fd uintptr) {
				//switch runtime.GOOS {
				//case "darwin", "linux":
				//	err = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEPORT, 1)
				//	if err != nil {
				//		return
				//	}
				//}
				//err = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
			})
		},
	}
	nl, err := lc.ListenPacket(ctx, fmt.Sprintf("ip4:%d", protoNum), addr)
	if err != nil {
		return
	}
	rc, err = ipv4.NewRawConn(nl)
	if err != nil {
		return nil, fmt.Errorf("err ipv4.NewRawConn: %w", err)
	}
	// enable all ctrl msg
	if err = rc.SetControlMessage(^ipv4.ControlFlags(0), true); err != nil {
		return nil, fmt.Errorf("err enable all ipv4 ControlMessage: %w", err)
	}
	for idx, modFn := range modRc {
		if err = modFn(rc); err != nil {
			return nil, fmt.Errorf("err at modRc idx(%d): %w", idx, err)
		}
	}
	return
}
