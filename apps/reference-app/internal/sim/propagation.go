// Package sim provides header-driven simulation propagation for gRPC.
//
// # System context
//
// The Simulation Framework activates virtualization with a single metadata
// header on the test request. That header must survive every hop of the
// internal call chain so that, when a mesh service finally dials a third
// party, the data plane (or local router) still knows which scenario to use.
//
//	test-client  --header-->  checkout-gateway  --header-->  fraud-checker
//	                                                          |
//	                     EvaluateRisk(+header) ---------------+
//	                                                          v
//	                          Microcks / microcks-mock   (if simulating)
//	                          or real external-risk      (if not)
//
// This package is the only shared simulation-aware application code in the
// POC. It is used in local / Docker Compose mode. In Kubernetes with Istio,
// EnvoyFilter resources under kube/kustomize/overlays/dev perform the same
// capture/inject job at the sidecar; these interceptors remain useful for
// observability (ScenarioFromContext) and for environments without a mesh.
//
// Business logic must not branch on scenario *names*. Logging is fine;
// product decisions belong to real services and virtual backends, not here.
//
// Header name is fixed by the v1 design — do not rename without updating
// EnvoyFilters, VirtualServices, and docs.
package sim

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/servicemesh/virtualization-contract"
)

// Header is the single request metadata key that activates virtualization.
// Value is a scenario name (e.g. "fraud-declined", "fraud-approved").
// Canonical definition: packages/virtualization-contract.
const Header = contract.SimulationHeader

// unexported context key avoids collisions with other packages' context values.
type ctxKey struct{}

// WithScenario stores a scenario name in context (useful for test clients
// that build context without going through ServerInterceptor first).
func WithScenario(ctx context.Context, scenario string) context.Context {
	if scenario == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, scenario)
}

// ScenarioFromContext returns the simulation scenario name, if any.
// Empty string means "no simulation — use the real third-party path".
func ScenarioFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKey{}).(string)
	return v
}

// ServerInterceptor captures the simulation header from inbound metadata
// and stores it on the request context for handlers and outbound clients.
//
// Register on every mesh service gRPC server:
//
//	grpc.NewServer(grpc.UnaryInterceptor(sim.ServerInterceptor()))
func ServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if vals := md.Get(Header); len(vals) > 0 && vals[0] != "" {
				ctx = WithScenario(ctx, vals[0])
			}
		}
		return handler(ctx, req)
	}
}

// ClientInterceptor injects the simulation header from context into every
// outbound gRPC call so the scenario propagates A → B → third party.
//
// Register on every outbound dial that participates in the chain:
//
//	grpc.DialContext(ctx, target,
//	    grpc.WithUnaryInterceptor(sim.ClientInterceptor()),
//	    ...)
func ClientInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		if scenario := ScenarioFromContext(ctx); scenario != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, Header, scenario)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
