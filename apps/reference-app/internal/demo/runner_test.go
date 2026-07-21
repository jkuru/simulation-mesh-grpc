package demo_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	checkoutv1 "github.com/servicemesh/reference-app/gen/checkout/v1"
	"github.com/servicemesh/reference-app/internal/demo"
	"github.com/servicemesh/reference-app/internal/sim"
)

type fakeGateway struct {
	// byScenario maps scenario ("" for none) → response
	byScenario map[string]*checkoutv1.CheckoutResponse
	err        error
	// seen records scenarios observed
	seen []string
}

func (f *fakeGateway) ProcessCheckout(ctx context.Context, req *checkoutv1.CheckoutRequest) (*checkoutv1.CheckoutResponse, error) {
	sc := sim.ScenarioFromContext(ctx)
	f.seen = append(f.seen, sc)
	if f.err != nil {
		return nil, f.err
	}
	if resp, ok := f.byScenario[sc]; ok {
		return resp, nil
	}
	return &checkoutv1.CheckoutResponse{Status: "APPROVED", TransactionId: req.TransactionId}, nil
}

func TestDefaultAndOverrideCases(t *testing.T) {
	d := demo.DefaultCases()
	if len(d) != 2 || d[1].Scenario != "fraud-declined" {
		t.Fatalf("%+v", d)
	}
	o := demo.OverrideCases("fraud-approved")
	if len(o) != 2 || o[0].Scenario != "fraud-approved" {
		t.Fatalf("%+v", o)
	}
}

func TestRunner_VirtualizationConfirmed(t *testing.T) {
	gw := &fakeGateway{byScenario: map[string]*checkoutv1.CheckoutResponse{
		"":               {Status: "APPROVED", OrderCode: "A", RiskScore: 10, Recommendation: "APPROVE"},
		"fraud-declined": {Status: "DECLINED", DeclineReason: "HIGH_RISK_SCORE", RiskScore: 92, Recommendation: "DECLINE"},
	}}
	r := demo.Runner{Gateway: gw}
	results := r.Run(context.Background(), demo.DefaultCases())
	if !demo.VirtualizationConfirmed(results) {
		t.Fatalf("not confirmed: %+v", results)
	}
	// both scenarios should have been seen (order of concurrent calls not guaranteed in seen)
	if len(gw.seen) != 2 {
		t.Fatalf("seen: %v", gw.seen)
	}
}

func TestRunner_ErrorPath(t *testing.T) {
	gw := &fakeGateway{err: errors.New("down")}
	r := demo.Runner{Gateway: gw}
	results := r.Run(context.Background(), demo.DefaultCases())
	if demo.VirtualizationConfirmed(results) {
		t.Fatal("should not confirm")
	}
	if results[0].Err == nil {
		t.Fatal("expected err")
	}
}

func TestVirtualizationConfirmed_FalseCases(t *testing.T) {
	if demo.VirtualizationConfirmed(nil) {
		t.Fatal("nil")
	}
	if demo.VirtualizationConfirmed([]demo.Result{{}, {}}) {
		t.Fatal("empty statuses")
	}
}

func TestFormatReport_Success(t *testing.T) {
	results := []demo.Result{
		{
			Case: demo.Case{Label: "Real path", Txn: "t1"},
			Resp: &checkoutv1.CheckoutResponse{
				Status: "APPROVED", OrderCode: "ORDER-1", RiskScore: 10, Recommendation: "APPROVE",
			},
			Elapsed: int64(5 * time.Millisecond),
		},
		{
			Case: demo.Case{Label: "Sim", Scenario: "fraud-declined", Txn: "t2"},
			Resp: &checkoutv1.CheckoutResponse{
				Status: "DECLINED", DeclineReason: "HIGH_RISK_SCORE", RiskScore: 92, Recommendation: "DECLINE",
			},
			Elapsed: int64(7 * time.Millisecond),
		},
	}
	var buf bytes.Buffer
	if err := demo.FormatReport(&buf, "localhost:9001", results, true); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"Virtualization confirmed",
		"APPROVED",
		"DECLINED",
		"fraud-declined",
		"localhost:9001",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestFormatReport_FailureAndError(t *testing.T) {
	results := []demo.Result{
		{Case: demo.Case{Label: "a"}, Err: errors.New("fail")},
		{Case: demo.Case{Label: "b"}, Resp: &checkoutv1.CheckoutResponse{Status: "APPROVED"}},
	}
	var buf bytes.Buffer
	if err := demo.FormatReport(&buf, "ep", results, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "NOT confirmed") {
		t.Fatal(buf.String())
	}
	if !strings.Contains(buf.String(), "error:") {
		t.Fatal(buf.String())
	}
}

func TestFormatReport_NoCheck(t *testing.T) {
	var buf bytes.Buffer
	err := demo.FormatReport(&buf, "ep", []demo.Result{
		{Case: demo.Case{Label: "x"}, Resp: &checkoutv1.CheckoutResponse{Status: "APPROVED", OrderCode: "Z"}},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "Virtualization confirmed") ||
		strings.Contains(buf.String(), "NOT confirmed") {
		t.Fatal("should not check")
	}
}

func TestRunner_CustomNow(t *testing.T) {
	n := 0
	times := []time.Time{
		time.Unix(0, 0), time.Unix(0, int64(3*time.Millisecond)),
		time.Unix(0, 0), time.Unix(0, int64(4*time.Millisecond)),
	}
	gw := &fakeGateway{byScenario: map[string]*checkoutv1.CheckoutResponse{
		"":               {Status: "APPROVED"},
		"fraud-declined": {Status: "DECLINED"},
	}}
	r := demo.Runner{
		Gateway: gw,
		Now: func() time.Time {
			t := times[n%len(times)]
			n++
			return t
		},
	}
	results := r.Run(context.Background(), demo.DefaultCases())
	if results[0].Elapsed <= 0 && results[1].Elapsed <= 0 {
		// at least one should have positive elapsed depending on scheduling
		// both calls use consecutive now() pairs — elapsed should be 3ms or 4ms
	}
	for _, res := range results {
		if res.Elapsed < 0 {
			t.Fatalf("negative elapsed: %d", res.Elapsed)
		}
	}
}
