// Package admission validates SimulationManifest objects (webhook + shared rules).
package admission

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/servicemesh/virtualization-contract"
	simv1 "github.com/servicemesh/virtualization-framework/api/simulation/v1alpha1"
	"github.com/servicemesh/virtualization-framework/internal/config"
)

// DNS-ish host: labels with optional dots; no scheme/path.
var hostRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9.-]*[a-zA-Z0-9])?$`)

// nameRe is a short DNS label for third-party / scenario identifiers.
var nameRe = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// ValidateSpec checks SimulationManifest spec semantics (create/update).
// Returns a field.ErrorList suitable for admission responses.
func ValidateSpec(m *simv1.SimulationManifest) field.ErrorList {
	var all field.ErrorList
	if m == nil {
		return append(all, field.InternalError(field.NewPath(""), fmt.Errorf("manifest is nil")))
	}
	spec := field.NewPath("spec")

	if len(m.Spec.ThirdParties) == 0 {
		all = append(all, field.Required(spec.Child("thirdParties"), "at least one thirdParty is required"))
	}
	if len(m.Spec.Scenarios) == 0 {
		all = append(all, field.Required(spec.Child("scenarios"), "at least one scenario is required"))
	}

	tpNames := map[string]int{}
	tpHosts := map[string]int{}
	for i, tp := range m.Spec.ThirdParties {
		p := spec.Child("thirdParties").Index(i)
		if strings.TrimSpace(tp.Name) == "" {
			all = append(all, field.Required(p.Child("name"), "name is required"))
		} else if !nameRe.MatchString(tp.Name) {
			all = append(all, field.Invalid(p.Child("name"), tp.Name, "must be a lowercase DNS label (a-z0-9-), e.g. external-risk"))
		} else if prev, ok := tpNames[tp.Name]; ok {
			all = append(all, field.Duplicate(p.Child("name"), fmt.Sprintf("duplicate of thirdParties[%d].name", prev)))
		} else {
			tpNames[tp.Name] = i
		}

		host := strings.TrimSpace(tp.Host)
		if host == "" {
			all = append(all, field.Required(p.Child("host"), "host is required"))
		} else if strings.Contains(host, "://") || strings.Contains(host, "/") {
			all = append(all, field.Invalid(p.Child("host"), tp.Host, "must be a bare hostname (no scheme or path)"))
		} else if !hostRe.MatchString(host) {
			all = append(all, field.Invalid(p.Child("host"), tp.Host, "must look like a DNS hostname"))
		} else if prev, ok := tpHosts[host]; ok {
			all = append(all, field.Duplicate(p.Child("host"), fmt.Sprintf("duplicate of thirdParties[%d].host", prev)))
		} else {
			tpHosts[host] = i
		}

		if tp.Port < 1 || tp.Port > 65535 {
			all = append(all, field.Invalid(p.Child("port"), tp.Port, "must be between 1 and 65535"))
		}
		if tp.BackendPort != 0 && (tp.BackendPort < 1 || tp.BackendPort > 65535) {
			all = append(all, field.Invalid(p.Child("backendPort"), tp.BackendPort, "must be between 1 and 65535"))
		}
		if bh := strings.TrimSpace(tp.BackendHost); bh != "" {
			if strings.Contains(bh, "://") || strings.Contains(bh, "/") {
				all = append(all, field.Invalid(p.Child("backendHost"), tp.BackendHost, "must be a bare hostname (no scheme or path)"))
			} else if !hostRe.MatchString(bh) {
				all = append(all, field.Invalid(p.Child("backendHost"), tp.BackendHost, "must look like a DNS hostname"))
			}
		}
	}

	scenarioNames := map[string]int{}
	for i, sc := range m.Spec.Scenarios {
		p := spec.Child("scenarios").Index(i)
		if strings.TrimSpace(sc.Name) == "" {
			all = append(all, field.Required(p.Child("name"), "name is required"))
		} else if !nameRe.MatchString(sc.Name) {
			all = append(all, field.Invalid(p.Child("name"), sc.Name, "must be a lowercase DNS label (a-z0-9-)"))
		} else if prev, ok := scenarioNames[sc.Name]; ok {
			all = append(all, field.Duplicate(p.Child("name"), fmt.Sprintf("duplicate of scenarios[%d].name", prev)))
		} else {
			scenarioNames[sc.Name] = i
		}
		if len(sc.Responses) == 0 {
			all = append(all, field.Required(p.Child("responses"), "at least one thirdParty response map entry is required"))
			continue
		}
		for key, ops := range sc.Responses {
			rp := p.Child("responses").Key(key)
			if len(tpNames) > 0 {
				if _, ok := tpNames[key]; !ok {
					all = append(all, field.NotFound(rp, fmt.Sprintf("response key %q does not match any thirdParties[].name", key)))
				}
			}
			if len(ops) == 0 {
				all = append(all, field.Required(rp, "at least one operation body is required"))
			}
			for j, op := range ops {
				opPath := rp.Index(j)
				if strings.TrimSpace(op.Operation) == "" {
					all = append(all, field.Required(opPath.Child("operation"), "operation is required"))
				}
				if strings.TrimSpace(op.Body) == "" {
					all = append(all, field.Required(opPath.Child("body"), "body is required"))
				}
			}
		}
	}

	if ms := strings.TrimSpace(m.Spec.MicrocksService); ms != "" {
		if err := validateHostPort(ms); err != nil {
			all = append(all, field.Invalid(spec.Child("microcksService"), m.Spec.MicrocksService, err.Error()))
		}
	}

	if vb := strings.TrimSpace(m.Spec.VirtualBackend); vb != "" &&
		vb != contract.BackendTeachingMock && vb != contract.BackendMicrocks {
		all = append(all, field.NotSupported(spec.Child("virtualBackend"), m.Spec.VirtualBackend,
			[]string{contract.BackendTeachingMock, contract.BackendMicrocks}))
	}

	return all
}

// ValidateForAdmission runs ValidateSpec plus environment policy.
func ValidateForAdmission(m *simv1.SimulationManifest, cfg config.Config) error {
	if cfg.IsProd() {
		return fmt.Errorf("SimulationManifest is forbidden when operator ENVIRONMENT is prod/production")
	}
	if m != nil {
		if v, ok := m.Labels[contract.EnvironmentKey]; ok && isProdLabel(v) {
			return fmt.Errorf("SimulationManifest is forbidden when label %s=%s", contract.EnvironmentKey, v)
		}
		if v, ok := m.Annotations[contract.EnvironmentKey]; ok && isProdLabel(v) {
			return fmt.Errorf("SimulationManifest is forbidden when annotation %s=%s", contract.EnvironmentKey, v)
		}
	}
	errs := ValidateSpec(m)
	if len(errs) == 0 {
		return nil
	}
	return errs.ToAggregate()
}

func isProdLabel(v string) bool {
	return strings.EqualFold(v, "prod") || strings.EqualFold(v, "production")
}

func validateHostPort(s string) error {
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		// Allow host-only with default semantics rejected — require host:port for clarity.
		return fmt.Errorf("must be host:port (e.g. microcks-svc.simulation-system.svc.cluster.local:9090)")
	}
	if strings.TrimSpace(host) == "" || !hostRe.MatchString(host) {
		return fmt.Errorf("host part is invalid")
	}
	if strings.TrimSpace(port) == "" {
		return fmt.Errorf("port part is required")
	}
	return nil
}

// ErrorString is a stable multi-line summary for Events / tests.
func ErrorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
