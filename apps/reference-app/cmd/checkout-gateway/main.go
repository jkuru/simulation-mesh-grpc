// checkout-gateway composition root (Service A).
// Business logic lives in internal/checkout — this file only wires dependencies.
//
//	Client → [Gateway] → FraudChecker (gRPC) → …
package main

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	fraudv1 "github.com/servicemesh/reference-app/gen/fraud/v1"
	checkoutv1 "github.com/servicemesh/reference-app/gen/checkout/v1"
	"github.com/servicemesh/reference-app/internal/env"
	"github.com/servicemesh/reference-app/internal/grpcx"
	"github.com/servicemesh/reference-app/internal/checkout"
	"github.com/servicemesh/reference-app/internal/sim"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	port := env.Get("GRPC_PORT", "9001")
	fraudEndpoint := env.Get("FRAUD_CHECKER_ENDPOINT", "localhost:9002")

	// Retry: fraud-checker + sidecars may start after us in mesh/kind.
	conn, err := grpcx.DialInsecureRetry(fraudEndpoint, 5*time.Second, 2*time.Second, 2*time.Minute)
	if err != nil {
		log.Error("dial fraud-checker failed", "endpoint", fraudEndpoint, "err", err)
		os.Exit(1)
	}
	defer conn.Close()

	// Adapter: generated client → checkout.FraudChecker port.
	gw := checkout.NewGateway(log, checkout.GRPCFraudChecker{Client: fraudv1.NewFraudCheckerClient(conn)}, checkout.RandomOrderCode{})
	srv := &checkout.GRPCServer{Gateway: gw}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Error("listen failed", "err", err)
		os.Exit(1)
	}

	gs := grpc.NewServer(grpc.UnaryInterceptor(sim.ServerInterceptor()))
	checkoutv1.RegisterCheckoutGatewayServer(gs, srv)
	reflection.Register(gs)

	errCh := make(chan error, 1)
	go func() {
		log.Info("checkout-gateway listening", "port", port, "fraud", fraudEndpoint)
		errCh <- gs.Serve(lis)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-sigCh:
		log.Info("shutting down", "signal", sig.String())
		gs.GracefulStop()
	case err := <-errCh:
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
