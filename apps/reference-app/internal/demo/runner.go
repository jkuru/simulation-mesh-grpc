package demo

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"google.golang.org/grpc/metadata"

	checkoutv1 "github.com/servicemesh/reference-app/gen/checkout/v1"
	"github.com/servicemesh/reference-app/internal/sim"
)

// Default demo NFT / amount (same for both paths).
const (
	DemoNftToken   = "nft_low_risk_4242"
	DemoAmountCents = int64(5000)
	DemoCurrency    = "USD"
)

// DefaultCases is the two-path virtualization proof.
func DefaultCases() []Case {
	return []Case{
		{Label: "Real path (no simulation header)", Scenario: "", Txn: "txn-001"},
		{Label: "Simulated path (fraud-declined scenario)", Scenario: "fraud-declined", Txn: "txn-002"},
	}
}

// OverrideCases runs the same scenario twice (manual SCENARIO= experiments).
func OverrideCases(scenario string) []Case {
	return []Case{
		{Label: "Override scenario: " + scenario, Scenario: scenario, Txn: "txn-override-1"},
		{Label: "Override scenario (repeat): " + scenario, Scenario: scenario, Txn: "txn-override-2"},
	}
}

// Runner executes demo cases concurrently against a CheckoutGateway.
type Runner struct {
	Gateway CheckoutGateway
	// Clock is optional; nil uses time.Now (inject for tests if needed).
	Now func() time.Time
}

// Run executes cases in parallel and returns results in input order.
func (r *Runner) Run(parent context.Context, cases []Case) []Result {
	now := r.Now
	if now == nil {
		now = time.Now
	}
	results := make([]Result, len(cases))
	var wg sync.WaitGroup
	for i, c := range cases {
		wg.Add(1)
		go func(i int, c Case) {
			defer wg.Done()
			results[i] = r.call(parent, c, now)
		}(i, c)
	}
	wg.Wait()
	return results
}

func (r *Runner) call(parent context.Context, c Case, now func() time.Time) Result {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()

	if c.Scenario != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, sim.Header, c.Scenario)
		ctx = sim.WithScenario(ctx, c.Scenario)
	}

	start := now()
	resp, err := r.Gateway.ProcessCheckout(ctx, &checkoutv1.CheckoutRequest{
		TransactionId: c.Txn,
		NftToken:     DemoNftToken,
		AmountCents:   DemoAmountCents,
		Currency:      DemoCurrency,
	})
	elapsed := now().Sub(start)
	return Result{Case: c, Resp: resp, Err: err, Elapsed: elapsed.Nanoseconds()}
}

// VirtualizationConfirmed is true when default two-path demo succeeds:
// first APPROVED, second DECLINED, no errors.
func VirtualizationConfirmed(results []Result) bool {
	if len(results) != 2 {
		return false
	}
	r1, r2 := results[0], results[1]
	return r1.Err == nil && r2.Err == nil &&
		r1.Resp != nil && r2.Resp != nil &&
		r1.Resp.GetStatus() == "APPROVED" &&
		r2.Resp.GetStatus() == "DECLINED"
}

// FormatReport writes the teaching log to w.
// Write errors from w are ignored for the teaching client (stdout never fails
// in practice); tests use bytes.Buffer. Signature keeps error for API stability.
func FormatReport(w io.Writer, endpoint string, results []Result, checkVirtualization bool) error {
	_, _ = fmt.Fprintln(w, "NFT MARKETPLACE DEMO — Virtualization proof")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintf(w, "checkout-gateway: %s\n", endpoint)
	_, _ = fmt.Fprintln(w)

	for i, r := range results {
		printResult(w, i+1, r)
	}

	if !checkVirtualization {
		return nil
	}
	if VirtualizationConfirmed(results) {
		_, _ = fmt.Fprint(w,
			"✓ Virtualization confirmed\n",
			"  Same NFT. Same price. Different outcome.\n",
			"  Scenario controlled entirely by header.\n",
		)
		return nil
	}
	_, _ = fmt.Fprintln(w, "✗ Virtualization NOT confirmed — check service logs")
	return nil
}

func printResult(w io.Writer, n int, r Result) {
	_, _ = fmt.Fprintf(w, "[%d] %s\n", n, r.Case.Label)
	_, _ = fmt.Fprintln(w, "    nft:    nft_low_risk_4242")
	_, _ = fmt.Fprintln(w, "    amount: $50.00")
	if r.Case.Scenario != "" {
		_, _ = fmt.Fprintf(w, "    header: %s: %s\n", sim.Header, r.Case.Scenario)
	}
	_, _ = fmt.Fprintln(w, "    ────────────────────────────────")
	if r.Err != nil {
		_, _ = fmt.Fprintf(w, "    error:  %v\n\n", r.Err)
		return
	}
	_, _ = fmt.Fprintf(w, "    result: %s\n", r.Resp.GetStatus())
	if r.Resp.GetOrderCode() != "" {
		_, _ = fmt.Fprintf(w, "    order:  %s\n", r.Resp.GetOrderCode())
	}
	if r.Resp.GetDeclineReason() != "" {
		_, _ = fmt.Fprintf(w, "    reason: %s\n", r.Resp.GetDeclineReason())
	}
	_, _ = fmt.Fprintf(w, "    fraud:  risk_score=%d, recommendation=%s\n",
		r.Resp.GetRiskScore(), r.Resp.GetRecommendation())
	took := time.Duration(r.Elapsed).Round(time.Millisecond)
	_, _ = fmt.Fprintf(w, "    took:   %s\n\n", took)
}
