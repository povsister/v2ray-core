package router

import (
	"strings"

	"github.com/v2fly/v2ray-core/v5/features/routing"
)

func (r *Router) GetDynamicRuleIP(ruleName string) routing.DynamicRuleIP {
	ruleName = strings.ToUpper(ruleName)
	for _, m := range globalGeoIPContainer.matchers {
		if m.countryCode == ruleName {
			return m
		}
	}
	return nil
}
