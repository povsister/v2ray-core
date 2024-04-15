package proxy

import (
	"context"
	"sync"

	"github.com/v2fly/v2ray-core/v5/common/net"
	"github.com/v2fly/v2ray-core/v5/common/signal"
)

type ActivityObservableInbound interface {
	Inbound
	RegisterActivityObserver(obName string, onRequest ActivityNoticeFn, onResponse ActivityNoticeFn)
	UnregisterActivityObserver(obName string)
}

type ActivityNoticeFn func(from, to net.Destination) signal.ActivityUpdater

type actObserver struct {
	onRequest  ActivityNoticeFn
	onResponse ActivityNoticeFn
}

type ActivityObserver struct {
	rw        sync.RWMutex
	observers map[string]*actObserver
}

func NewActivityObserver() *ActivityObserver {
	return &ActivityObserver{
		observers: make(map[string]*actObserver),
	}
}

func (o *ActivityObserver) RegisterActivityObserver(name string, onRequest ActivityNoticeFn, onResponse ActivityNoticeFn) {
	o.rw.Lock()
	defer o.rw.Unlock()
	o.observers[name] = &actObserver{
		onRequest:  onRequest,
		onResponse: onResponse,
	}
}

func (o *ActivityObserver) UnregisterActivityObserver(name string) {
	o.rw.Lock()
	defer o.rw.Unlock()
	delete(o.observers, name)
}

func (o *ActivityObserver) RangeOverAllObservers(f func(name string, onRequest ActivityNoticeFn, onResponse ActivityNoticeFn) bool) {
	o.rw.RLock()
	defer o.rw.RUnlock()
	for k, v := range o.observers {
		if !f(k, v.onRequest, v.onResponse) {
			return
		}
	}
}

type ActivityUpdaters interface {
	Add(updater signal.ActivityUpdater)
	Update()
}

func NewActivityUpdater() ActivityUpdaters {
	return &multiActivityUpdaters{}
}

type multiActivityUpdaters struct {
	allUpdaters []signal.ActivityUpdater
}

func (m *multiActivityUpdaters) Update() {
	for _, updater := range m.allUpdaters {
		updater.Update()
	}
}

func (m *multiActivityUpdaters) Add(updater signal.ActivityUpdater) {
	m.allUpdaters = append(m.allUpdaters, updater)
}

func (m *multiActivityUpdaters) IsEmpty() bool {
	return len(m.allUpdaters) == 0
}

func (o *ActivityObserver) GetAllActivityUpdater(from, to net.Destination) (onRequestUpdater, onResponseUpdater signal.ActivityUpdater) {
	onReqUpd := &multiActivityUpdaters{}
	onRequestUpdater = onReqUpd
	onRespUpd := &multiActivityUpdaters{}
	onResponseUpdater = onRespUpd
	o.RangeOverAllObservers(func(name string, onRequest ActivityNoticeFn, onResponse ActivityNoticeFn) bool {
		if u := onRequest(from, to); u != nil {
			onReqUpd.Add(u)
		}
		if u := onResponse(from, to); u != nil {
			onRespUpd.Add(u)
		}
		return true
	})
	return
}

type ObservableDNSOutBound interface {
	Outbound
	RegisterDNSOutBoundObserver(obName string, notice DNSOutBoundNoticeFn)
	UnregisterDNSOutBoundObserver(obName string)
}

type DNSOutBoundNoticeFn func(ctx context.Context, domain string, results []net.IP, err error)
