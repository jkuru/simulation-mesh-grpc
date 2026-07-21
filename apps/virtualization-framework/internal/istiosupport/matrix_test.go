package istiosupport_test

import (
	"strings"
	"testing"

	"github.com/servicemesh/virtualization-framework/internal/istiosupport"
)

func TestParseAndSupport(t *testing.T) {
	cases := []struct {
		in      string
		ok      bool
		wantMaj int
		wantMin int
	}{
		{"1.22.3", true, 1, 22},
		{"v1.21", true, 1, 21},
		{"docker.io/istio/pilot:1.22.3", true, 1, 22},
		{"istio/proxyv2:1.20.0", true, 1, 20},
		{"1.19.0", false, 1, 19},
		{"1.24.0", false, 1, 24},
		{"", false, 0, 0},
		{"not-a-version", false, 0, 0},
	}
	for _, tc := range cases {
		v, err := istiosupport.ParseVersion(tc.in)
		if tc.in == "" || tc.in == "not-a-version" {
			if err == nil {
				t.Fatalf("%q: expected parse error", tc.in)
			}
			if err := istiosupport.Check(tc.in); err == nil {
				t.Fatalf("%q: Check should fail", tc.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if v.Major != tc.wantMaj || v.Minor != tc.wantMin {
			t.Fatalf("%q => %v", tc.in, v)
		}
		err = istiosupport.Check(tc.in)
		if tc.ok && err != nil {
			t.Fatalf("%q supported but Check=%v", tc.in, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("%q should be unsupported", tc.in)
		}
	}
}

func TestCompareAndSummary(t *testing.T) {
	a := istiosupport.Version{Major: 1, Minor: 20}
	b := istiosupport.Version{Major: 1, Minor: 22, Patch: 3}
	if a.Compare(b) >= 0 {
		t.Fatal("compare")
	}
	if b.Compare(a) <= 0 {
		t.Fatal("compare reverse")
	}
	if a.Compare(a) != 0 {
		t.Fatal("equal")
	}
	if !strings.Contains(istiosupport.Summary(), "Istio support") {
		t.Fatal(istiosupport.Summary())
	}
	if b.String() != "1.22.3" {
		t.Fatal(b.String())
	}
	if a.String() != "1.20" {
		t.Fatal(a.String())
	}
	// major differ both directions
	c := istiosupport.Version{Major: 2, Minor: 0}
	if c.Compare(a) <= 0 {
		t.Fatal("major greater")
	}
	if a.Compare(c) >= 0 {
		t.Fatal("major less")
	}
	// colon tag without pilot/istiod keyword
	v, err := istiosupport.ParseVersion("ghcr.io/example/control-plane:1.21.4")
	if err != nil || v.Minor != 21 || v.Patch != 4 {
		t.Fatalf("colon tag: %v %v", v, err)
	}
}
