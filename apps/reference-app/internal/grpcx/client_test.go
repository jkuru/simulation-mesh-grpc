package grpcx_test

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/servicemesh/reference-app/internal/grpcx"
	"github.com/servicemesh/reference-app/internal/sim"
)

func startBufServer(t *testing.T) (*bufconn.Listener, func()) {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	gs := grpc.NewServer(grpc.UnaryInterceptor(sim.ServerInterceptor()))
	go func() { _ = gs.Serve(lis) }()
	return lis, func() { gs.Stop(); _ = lis.Close() }
}

func TestDialInsecure_Success(t *testing.T) {
	lis, stop := startBufServer(t)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithUnaryInterceptor(sim.ClientInterceptor()),
	)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
}

func TestDialInsecureTimeout_FailsFast(t *testing.T) {
	_, err := grpcx.DialInsecureTimeout("127.0.0.1:1", 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected dial error")
	}
}

func TestDialInsecure_FailsFast(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := grpcx.DialInsecure(ctx, "127.0.0.1:1")
	if err == nil {
		t.Fatal("expected dial error")
	}
}

func TestDialInsecureRetry_EventuallyFails(t *testing.T) {
	_, err := grpcx.DialInsecureRetry("127.0.0.1:1", 30*time.Millisecond, 5*time.Millisecond, 80*time.Millisecond)
	if err == nil {
		t.Fatal("expected dial error")
	}
}

func TestDialInsecureRetry_Succeeds(t *testing.T) {
	// Real TCP listener so DialInsecureTimeout can connect.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	gs := grpc.NewServer()
	go func() { _ = gs.Serve(lis) }()
	defer gs.Stop()

	addr := lis.Addr().String()
	conn, err := grpcx.DialInsecureRetry(addr, time.Second, 10*time.Millisecond, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
}
