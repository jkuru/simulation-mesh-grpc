// fraud-checker composition root (Service B).
// Business logic + local routing live in internal/fraud; this wires gRPC clients.
//
//	checkout-gateway → [Checker] → RiskResolver → RiskService (external | microcks)
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
	riskv1 "github.com/servicemesh/reference-app/gen/risk/v1"
	"github.com/servicemesh/reference-app/internal/env"
	"github.com/servicemesh/reference-app/internal/fraud"
	"github.com/servicemesh/reference-app/internal/grpcx"
	"github.com/servicemesh/reference-app/internal/sim"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	port := env.Get("GRPC_PORT", "9002")
	mode := env.Get("SIMULATION_MODE", fraud.ModeMesh)
	externalEP := env.Get("EXTERNAL_RISK_ENDPOINT", "localhost:9003")
	microcksEP := env.Get("MICROCKS_ENDPOINT", "localhost:9090")

	// Retry dial: in mesh mode DNS/ServiceEntry and peer pods may not be ready yet.
	extConn, err := grpcx.DialInsecureRetry(externalEP, 5*time.Second, 2*time.Second, 2*time.Minute)
	if err != nil {
		log.Error("dial external-risk failed", "endpoint", externalEP, "err", err)
		os.Exit(1)
	}
	defer extConn.Close()

	resolver := &fraud.LocalRiskResolver{
		Log:            log,
		Mode:           mode,
		External:       fraud.GRPCRiskEvaluator{Client: riskv1.NewRiskServiceClient(extConn)},
		ExternalTarget: externalEP,
	}

	// In local mode, also dial Microcks so header-present traffic can switch.
	if mode == fraud.ModeLocal {
		mcConn, err := grpcx.DialInsecureRetry(microcksEP, 5*time.Second, 2*time.Second, 2*time.Minute)
		if err != nil {
			log.Error("dial microcks failed", "endpoint", microcksEP, "err", err)
			os.Exit(1)
		}
		defer mcConn.Close()
		resolver.Microcks = fraud.GRPCRiskEvaluator{Client: riskv1.NewRiskServiceClient(mcConn)}
		resolver.MicrocksTarget = microcksEP
	}

	checker := fraud.NewChecker(log, resolver)
	srv := &fraud.GRPCServer{Checker: checker}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Error("listen failed", "err", err)
		os.Exit(1)
	}

	gs := grpc.NewServer(grpc.UnaryInterceptor(sim.ServerInterceptor()))
	fraudv1.RegisterFraudCheckerServer(gs, srv)
	reflection.Register(gs)

	errCh := make(chan error, 1)
	go func() {
		log.Info("fraud-checker listening",
			"port", port, "mode", mode, "external", externalEP, "microcks", microcksEP)
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
