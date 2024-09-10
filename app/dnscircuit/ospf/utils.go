package ospf

import (
	"context"
	"encoding/binary"
	"net"
	"sync/atomic"
	"time"
	"unsafe"
)

//go:generate go run github.com/v2fly/v2ray-core/v5/common/errors/errorgen

func ipv4BytesToUint32(b []byte) uint32 {
	return binary.BigEndian.Uint32(b[0:4])
}

func uint32ToIPv4(ip uint32) net.IP {
	return net.IPv4(byte(ip>>24), byte(ip>>16), byte(ip>>8), byte(ip))
}

func ipv4MaskToUint32(b []byte) uint32 {
	return binary.BigEndian.Uint32(b[0:4])
}

type TickerFunc struct {
	ctx    context.Context
	cancel context.CancelFunc
	dur    time.Duration
	fn     func()
	t      *time.Ticker
}

func TimeTickerFunc(ctx context.Context, dur time.Duration, fn func(), waitForTicker ...bool) *TickerFunc {
	ctx, cancel := context.WithCancel(ctx)
	ret := &TickerFunc{
		ctx:    ctx,
		cancel: cancel,
		dur:    dur,
		fn:     fn,
		t:      time.NewTicker(dur),
	}
	go func() {
		// immediate call the fn first if do not wait for ticker
		if !(len(waitForTicker) > 0 && waitForTicker[0]) {
			fn()
		}
		// then loop for cancel or tick
		for {
			select {
			case <-ret.ctx.Done():
				ret.t.Stop()
				return
			case <-ret.t.C:
				fn()
			}
		}
	}()
	return ret
}

func (t *TickerFunc) Terminate() {
	if t != nil {
		t.cancel()
	}
}

func (t *TickerFunc) Suspend() {
	if t != nil {
		t.t.Stop()
	}
}

func (t *TickerFunc) Reset() {
	if t != nil {
		t.t.Reset(t.dur)
	}
}

// DoFnNow 暂停ticker并立即执行一次fn，随后恢复tick
func (t *TickerFunc) DoFnNow() {
	if t != nil && t.ctx.Err() == nil {
		t.t.Stop()
		t.fn()
		t.t.Reset(t.dur)
	}
}

// TSS for thread-safe struct
type TSS[T any] struct {
	v unsafe.Pointer
}

func (s *TSS[T]) Set(val T) {
	atomic.StorePointer(&s.v, unsafe.Pointer(&val))
}

func (s *TSS[T]) Get() (ret T) {
	vGet := (*T)(atomic.LoadPointer(&s.v))
	if vGet == nil {
		return
	}
	return *vGet
}
