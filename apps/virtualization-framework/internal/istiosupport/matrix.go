// Package istiosupport documents and checks Istio versions the framework claims.
package istiosupport

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Matrix is the claimed support window for Istio control plane images.
// Tested APIs: VirtualService/ServiceEntry/DestinationRule v1beta1, EnvoyFilter v1alpha3.
var Matrix = SupportMatrix{
	// MinInclusive / MaxInclusive are major.minor (patch ignored for claims).
	MinInclusive: Version{Major: 1, Minor: 20},
	MaxInclusive: Version{Major: 1, Minor: 23},
	// Explicitly verified in this monorepo (kind e2e).
	Verified: []Version{
		{Major: 1, Minor: 22},
	},
	Notes: []string{
		"Uses networking.istio.io/v1beta1 (VS, SE, DR) and networking.istio.io/v1alpha3 (EnvoyFilter).",
		"Lua EnvoyFilters require Istio with Envoy Lua filter enabled (default).",
		"DNS capture for ServiceEntry hosts recommended (meshConfig.defaultConfig.proxyMetadata).",
	},
}

// SupportMatrix describes claimed Istio compatibility.
type SupportMatrix struct {
	MinInclusive Version
	MaxInclusive Version
	Verified     []Version
	Notes        []string
}

// Version is an Istio major.minor (patch optional).
type Version struct {
	Major int
	Minor int
	Patch int // 0 if unknown / not parsed
}

func (v Version) String() string {
	if v.Patch > 0 {
		return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	}
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

// Compare returns -1, 0, 1 for major.minor only (patch ignored).
func (v Version) Compare(o Version) int {
	if v.Major != o.Major {
		if v.Major < o.Major {
			return -1
		}
		return 1
	}
	if v.Minor != o.Minor {
		if v.Minor < o.Minor {
			return -1
		}
		return 1
	}
	return 0
}

// Supported reports whether v is inside the claimed matrix.
func (m SupportMatrix) Supported(v Version) bool {
	return v.Compare(m.MinInclusive) >= 0 && v.Compare(m.MaxInclusive) <= 0
}

var imageTagRe = regexp.MustCompile(`(?i)(?:pilot|istiod|proxyv2)[:/]v?(\d+)\.(\d+)(?:\.(\d+))?`)
var bareVerRe = regexp.MustCompile(`^v?(\d+)\.(\d+)(?:\.(\d+))?`)

// ParseVersion parses "1.22.3", "v1.22", or an image ref containing pilot/istiod tags.
func ParseVersion(s string) (Version, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Version{}, fmt.Errorf("empty version string")
	}
	if m := imageTagRe.FindStringSubmatch(s); m != nil {
		return versionFromParts(m[1], m[2], m[3]), nil
	}
	// last path segment tag: registry.example/custom/istiod:1.21.1
	if i := strings.LastIndex(s, ":"); i >= 0 && !strings.Contains(s[i+1:], "/") {
		tag := s[i+1:]
		if m := bareVerRe.FindStringSubmatch(tag); m != nil {
			return versionFromParts(m[1], m[2], m[3]), nil
		}
	}
	if m := bareVerRe.FindStringSubmatch(s); m != nil {
		return versionFromParts(m[1], m[2], m[3]), nil
	}
	return Version{}, fmt.Errorf("cannot parse Istio version from %q", s)
}

func versionFromParts(maj, min, pat string) Version {
	// Callers only pass digits from regex submatches.
	major, _ := strconv.Atoi(maj)
	minor, _ := strconv.Atoi(min)
	v := Version{Major: major, Minor: minor}
	if pat != "" {
		p, _ := strconv.Atoi(pat)
		v.Patch = p
	}
	return v
}

// Check returns nil if the version is supported, else an error with guidance.
func Check(versionString string) error {
	v, err := ParseVersion(versionString)
	if err != nil {
		return err
	}
	if !Matrix.Supported(v) {
		return fmt.Errorf("Istio %s is outside supported range %s–%s (verified: %v)",
			v, Matrix.MinInclusive, Matrix.MaxInclusive, Matrix.Verified)
	}
	return nil
}

// Summary is a human-readable matrix blurb for docs/CLI.
func Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Istio support: %s – %s (inclusive major.minor)\n", Matrix.MinInclusive, Matrix.MaxInclusive)
	b.WriteString("Verified in monorepo e2e:")
	for _, v := range Matrix.Verified {
		fmt.Fprintf(&b, " %s", v)
	}
	b.WriteByte('\n')
	for _, n := range Matrix.Notes {
		fmt.Fprintf(&b, "- %s\n", n)
	}
	return b.String()
}
