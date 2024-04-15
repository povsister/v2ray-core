package dnscircuit

import (
	"context"

	core "github.com/v2fly/v2ray-core/v5"
	"github.com/v2fly/v2ray-core/v5/common"
	"github.com/v2fly/v2ray-core/v5/features/inbound"
	"github.com/v2fly/v2ray-core/v5/features/outbound"
	"github.com/v2fly/v2ray-core/v5/features/routing"
)

//go:generate go run github.com/v2fly/v2ray-core/v5/common/errors/errorgen
func init() {
	common.Must(common.RegisterConfig((*Config)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		var (
			circuit = new(dnsCircuit)
			err     error
		)
		if err := core.RequireFeatures(ctx, func(router routing.Router,
			ihm inbound.Manager, ohm outbound.Manager) {
			err = circuit.Init(ctx, config.(*Config), router, ihm, ohm)
		}); err != nil {
			return nil, err
		}
		return circuit, err
	}))
}
