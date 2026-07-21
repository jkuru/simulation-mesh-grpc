// microcks-mock composition root — virtual RiskService (scenario backend).
// Scenario catalog lives in internal/risk.DefaultScenarios().
package main

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	riskv1 "github.com/servicemesh/reference-app/gen/risk/v1"
	"github.com/servicemesh/reference-app/internal/env"
	"github.com/servicemesh/reference-app/internal/risk"
	"github.com/servicemesh/reference-app/internal/sim"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	port := env.Get("GRPC_PORT", "9090")

	svc := risk.NewVirtualService(log, risk.DefaultScenarios())
	srv := &risk.GRPCServer{Handler: svc}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Error("listen failed", "err", err)
		os.Exit(1)
	}

	gs := grpc.NewServer(grpc.UnaryInterceptor(sim.ServerInterceptor()))
	riskv1.RegisterRiskServiceServer(gs, srv)
	reflection.Register(gs)

	errCh := make(chan error, 1)
	go func() {
		log.Info("microcks-mock listening", "port", port)
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
