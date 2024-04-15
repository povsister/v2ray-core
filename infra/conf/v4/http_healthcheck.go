package v4

import (
	"github.com/golang/protobuf/proto"

	"github.com/v2fly/v2ray-core/v5/proxy/httphealthcheck"
)

type HttpHealthCheckConfig struct {
	Timeout uint32 `json:"timeout"` // seconds
}

func (c *HttpHealthCheckConfig) Build() (proto.Message, error) {
	if c.Timeout <= 0 {
		c.Timeout = 3
	}
	return &httphealthcheck.HealthServerConfig{
		Timeout: c.Timeout,
	}, nil
}
