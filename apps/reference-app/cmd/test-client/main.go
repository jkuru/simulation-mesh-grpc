// test-client composition root — teaching proof of header-driven virtualization.
// Case selection / report formatting live in internal/demo.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	checkoutv1 "github.com/servicemesh/reference-app/gen/checkout/v1"
	"github.com/servicemesh/reference-app/internal/demo"
	"github.com/servicemesh/reference-app/internal/env"
)

func main() {
	endpoint := env.Get("CHECKOUT_GATEWAY_ENDPOINT", "localhost:9001")
	override := env.Get("SCENARIO", "")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial checkout-gateway failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	// Adapter: generated client → demo.CheckoutGateway port.
	runner := demo.Runner{Gateway: demo.GRPCCheckoutGateway{Client: checkoutv1.NewCheckoutGatewayClient(conn)}}

	cases := demo.DefaultCases()
	checkVirtualization := override == ""
	if override != "" {
		cases = demo.OverrideCases(override)
	}

	results := runner.Run(context.Background(), cases)
	if err := demo.FormatReport(os.Stdout, endpoint, results, checkVirtualization); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if checkVirtualization && !demo.VirtualizationConfirmed(results) {
		os.Exit(1)
	}
}
