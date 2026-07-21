package sim

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestServerInterceptorCapturesHeader(t *testing.T) {
	md := metadata.Pairs(Header, "fraud-declined")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	interceptor := ServerInterceptor()
	var got string
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, func(ctx context.Context, req any) (any, error) {
		got = ScenarioFromContext(ctx)
		return "ok", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "fraud-declined" {
		t.Fatalf("scenario = %q, want fraud-declined", got)
	}
}

func TestServerInterceptor_EmptyOrMissing(t *testing.T) {
	interceptor := ServerInterceptor()

	// no metadata
	var got string
	_, _ = interceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, req any) (any, error) {
		got = ScenarioFromContext(ctx)
		return nil, nil
	})
	if got != "" {
		t.Fatalf("got %q", got)
	}

	// empty value
	md := metadata.Pairs(Header, "")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	_, _ = interceptor(ctx, nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, req any) (any, error) {
		got = ScenarioFromContext(ctx)
		return nil, nil
	})
	if got != "" {
		t.Fatalf("empty header should not set scenario, got %q", got)
	}
}

func TestClientInterceptorInjectsHeader(t *testing.T) {
	ctx := WithScenario(context.Background(), "fraud-approved")
	interceptor := ClientInterceptor()

	err := interceptor(ctx, "/risk.v1.RiskService/EvaluateRisk", nil, nil, nil,
		func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			md, ok := metadata.FromOutgoingContext(ctx)
			if !ok {
				t.Fatal("missing outgoing metadata")
			}
			vals := md.Get(Header)
			if len(vals) != 1 || vals[0] != "fraud-approved" {
				t.Fatalf("header = %v, want [fraud-approved]", vals)
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
}

func TestClientInterceptor_NoScenario(t *testing.T) {
	interceptor := ClientInterceptor()
	err := interceptor(context.Background(), "/m", nil, nil, nil,
		func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			if _, ok := metadata.FromOutgoingContext(ctx); ok {
				// may be empty md; ensure header absent
				md, _ := metadata.FromOutgoingContext(ctx)
				if vals := md.Get(Header); len(vals) > 0 {
					t.Fatalf("unexpected header %v", vals)
				}
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
}

func TestClientInterceptor_PropagatesInvokerError(t *testing.T) {
	interceptor := ClientInterceptor()
	want := errors.New("rpc err")
	err := interceptor(context.Background(), "/m", nil, nil, nil,
		func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			return want
		})
	if !errors.Is(err, want) {
		t.Fatalf("got %v", err)
	}
}

func TestScenarioFromContextEmpty(t *testing.T) {
	if ScenarioFromContext(context.Background()) != "" {
		t.Fatal("expected empty scenario")
	}
}

func TestWithScenario_EmptyNoop(t *testing.T) {
	ctx := context.Background()
	if WithScenario(ctx, "") != ctx {
		// same pointer not required; value must stay empty
	}
	if ScenarioFromContext(WithScenario(ctx, "")) != "" {
		t.Fatal("empty scenario should not set")
	}
}
