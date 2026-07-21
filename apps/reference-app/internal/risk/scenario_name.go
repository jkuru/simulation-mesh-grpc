package risk

import (
	"context"

	"google.golang.org/grpc/metadata"

	"github.com/servicemesh/reference-app/internal/sim"
	"github.com/servicemesh/virtualization-contract"
)

// MicrocksOperationHeader is the header real Microcks uses after Envoy rewrite.
const MicrocksOperationHeader = contract.MicrocksOperationHeader

// ScenarioNameFromContext resolves the active scenario from context / metadata.
//
// Order:
//  1. sim context (ServerInterceptor)
//  2. x-microcks-operation (mesh rewrite target)
//  3. raw simulation header metadata
func ScenarioNameFromContext(ctx context.Context) string {
	if s := sim.ScenarioFromContext(ctx); s != "" {
		return s
	}
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get(MicrocksOperationHeader); len(vals) > 0 && vals[0] != "" {
			return vals[0]
		}
		if vals := md.Get(sim.Header); len(vals) > 0 && vals[0] != "" {
			return vals[0]
		}
	}
	return ""
}
