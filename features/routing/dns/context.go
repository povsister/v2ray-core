package dns

//go:generate go run github.com/v2fly/v2ray-core/v5/common/errors/errorgen

import (
	"strings"

	"github.com/v2fly/v2ray-core/v5/common/net"
	"github.com/v2fly/v2ray-core/v5/features/dns"
	"github.com/v2fly/v2ray-core/v5/features/routing"
)

// ResolvableContext is an implementation of routing.Context, with domain resolving capability.
type ResolvableContext struct {
	routing.Context
	dnsClient   dns.Client
	resolvedIPs []net.IP
}

// GetTargetIPs overrides original routing.Context's implementation.
func (ctx *ResolvableContext) GetTargetIPs() []net.IP {
	if ips := ctx.Context.GetTargetIPs(); len(ips) != 0 {
		return ips
	}

	if len(ctx.resolvedIPs) > 0 {
		return ctx.resolvedIPs
	}

	if domain := ctx.GetTargetDomain(); len(domain) != 0 {
		ips, err := ctx.dnsClient.LookupIP(domain)
		if err == nil {
			ctx.resolvedIPs = ips
			return ips
		}
		newError("resolve ip for ", domain).Base(err).WriteToLog()
	}

	return nil
}

// ContextWithDNSClient creates a new routing context with domain resolving capability.
// Resolved domain IPs can be retrieved by GetTargetIPs().
func ContextWithDNSClient(ctx routing.Context, client dns.Client) routing.Context {
	return &ResolvableContext{Context: ctx, dnsClient: client}
}

// RoutingContextWithSkipDynamicRule is an optional feature for routing context, to
// control the behavior of whether skip certain dynamic rule while rule matching.
// By default, all rules are checked.
type RoutingContextWithSkipDynamicRule interface {
	routing.Context
	GetSkipDynamicRuleIP(ruleName string) bool
	GetSkipDynamicRuleDomain(ruleName string) bool
}

type SkipDynamicRuleContext struct {
	routing.Context
	skipRuleNameIP     string
	skipRuleNameDomain string // TODO: not used yet
}

func (ctx *SkipDynamicRuleContext) GetSkipDynamicRuleIP(ruleName string) bool {
	ruleName = strings.ToUpper(ruleName)
	if ctx.skipRuleNameIP == ruleName {
		return true
	}
	if sCtx, ok := ctx.Context.(RoutingContextWithSkipDynamicRule); ok {
		return sCtx.GetSkipDynamicRuleIP(ruleName)
	}
	return false
}

func (ctx *SkipDynamicRuleContext) GetSkipDynamicRuleDomain(ruleName string) bool {
	ruleName = strings.ToUpper(ruleName)
	if ctx.skipRuleNameDomain == ruleName {
		return true
	}
	if sCtx, ok := ctx.Context.(RoutingContextWithSkipDynamicRule); ok {
		return sCtx.GetSkipDynamicRuleDomain(ruleName)
	}
	return false
}

func ContextWithSkippingDynamicRule(ctx routing.Context, ruleNameIP, ruleNameDomain string) routing.Context {
	return &SkipDynamicRuleContext{
		Context:            ctx,
		skipRuleNameIP:     strings.ToUpper(ruleNameIP),
		skipRuleNameDomain: ruleNameDomain,
	}
}
