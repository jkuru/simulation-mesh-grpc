// Package grpcx holds small gRPC dial helpers shared by composition roots.
package grpcx

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/servicemesh/reference-app/internal/sim"
)

// DialInsecure blocks until connected (or timeout) with sim client interceptor.
func DialInsecure(ctx context.Context, target string) (*grpc.ClientConn, error) {
	return grpc.DialContext(ctx, target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithUnaryInterceptor(sim.ClientInterceptor()),
	)
}

// DialInsecureTimeout is DialInsecure with a timeout on background context.
func DialInsecureTimeout(target string, timeout time.Duration) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return DialInsecure(ctx, target)
}

// DialInsecureRetry retries dial until maxWait elapses (mesh startup races).
// Each attempt uses attemptTimeout; waits interval between failures.
func DialInsecureRetry(target string, attemptTimeout, interval, maxWait time.Duration) (*grpc.ClientConn, error) {
	deadline := time.Now().Add(maxWait)
	var last error = fmt.Errorf("dial %s: no attempt succeeded", target)
	for {
		conn, err := DialInsecureTimeout(target, attemptTimeout)
		if err == nil {
			return conn, nil
		}
		last = err
		if !time.Now().Before(deadline) {
			break
		}
		time.Sleep(interval)
	}
	return nil, fmt.Errorf("dial %s after %s: %w", target, maxWait, last)
}
