package httphealthcheck

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/v2fly/v2ray-core/v5/common"
	"github.com/v2fly/v2ray-core/v5/common/buf"
	"github.com/v2fly/v2ray-core/v5/common/errors"
	"github.com/v2fly/v2ray-core/v5/common/net"
	"github.com/v2fly/v2ray-core/v5/common/session"
	"github.com/v2fly/v2ray-core/v5/features/routing"
	"github.com/v2fly/v2ray-core/v5/transport/internet"
)

//go:generate go run github.com/v2fly/v2ray-core/v5/common/errors/errorgen
func init() {
	common.Must(common.RegisterConfig((*HealthServerConfig)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		return NewHealthServer(ctx, config.(*HealthServerConfig))
	}))
}

type HealthServer struct {
	timeout time.Duration
	ready   atomic.Bool
}

func NewHealthServer(ctx context.Context, config *HealthServerConfig) (*HealthServer, error) {
	return &HealthServer{
		timeout: time.Duration(config.Timeout) * time.Second,
	}, nil
}

func (s *HealthServer) Network() []net.Network {
	return []net.Network{
		net.Network_TCP,
	}
}

type readerOnly struct {
	io.Reader
}

func isTimeout(err error) bool {
	nerr, ok := errors.Cause(err).(net.Error)
	return ok && nerr.Timeout()
}

func (s *HealthServer) Process(ctx context.Context, net net.Network, conn internet.Connection, dispatcher routing.Dispatcher) error {
	reader := bufio.NewReaderSize(readerOnly{conn}, buf.Size)
	if err := conn.SetReadDeadline(time.Now().Add(s.timeout)); err != nil {
		newError("failed to set read deadline").Base(err).WriteToLog(session.ExportIDToError(ctx))
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	request, err := http.ReadRequest(reader)
	if err != nil {
		trace := newError("failed to read http health check request").Base(err)
		if errors.Cause(err) != io.EOF && !isTimeout(errors.Cause(err)) {
			trace.AtWarning()
		}
		return trace
	}
	err = s.handleHttp(ctx, request, conn, dispatcher)
	if err != nil {
		err = newError("failed to handle http health check request").Base(err).
			AtWarning()
	}
	return err
}

func (s *HealthServer) handleHttp(ctx context.Context, request *http.Request, writer io.Writer, dispatcher routing.Dispatcher) error {
	response := &http.Response{
		Status:        "OK",
		StatusCode:    200,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        http.Header(make(map[string][]string)),
		Body:          nil,
		ContentLength: 0,
		Close:         true,
	}
	response.Header.Set("Connection", "close")
	return response.Write(writer)
}
