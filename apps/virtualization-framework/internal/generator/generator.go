// Package generator builds Istio unstructured objects from a SimulationManifest.
package generator

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/servicemesh/virtualization-contract"
	simv1 "github.com/servicemesh/virtualization-framework/api/simulation/v1alpha1"
	"github.com/servicemesh/virtualization-framework/internal/config"
)

// Result holds generated objects and stable names for status.
type Result struct {
	Objects []unstructured.Unstructured
	Names   []string
}

// Generate produces ServiceEntry, VirtualService, DestinationRule, and EnvoyFilters
// for the manifest namespace. Microcks DestinationRule is included once.
func Generate(m *simv1.SimulationManifest, cfg config.Config) (Result, error) {
	if m == nil {
		return Result{}, fmt.Errorf("manifest is nil")
	}
	if len(m.Spec.ThirdParties) == 0 {
		return Result{}, fmt.Errorf("spec.thirdParties is required")
	}
	if len(m.Spec.Scenarios) == 0 {
		return Result{}, fmt.Errorf("spec.scenarios is required")
	}

	ns := m.Namespace
	owner := m.Name
	microcksHost, microcksPort := splitHostPort(
		firstNonEmpty(strings.TrimSpace(m.Spec.MicrocksService), strings.TrimSpace(cfg.DefaultMicrocksHostPort)),
		9090,
	)

	var out Result
	add := func(u *unstructured.Unstructured) {
		out.Objects = append(out.Objects, *u)
		out.Names = append(out.Names, fmt.Sprintf("%s/%s", u.GetKind(), u.GetName()))
	}

	// Namespace-scoped EnvoyFilters for header propagation (once per reconcile set).
	add(envoyFilterInbound(ns, owner, cfg))
	add(envoyFilterOutbound(ns, owner, cfg))

	// Real Microcks (or compatible) selects examples via x-microcks-operation.
	// EnvoyFilter must live in the backend's namespace (system), not the app ns.
	if isMicrocksBackend(m.Spec.VirtualBackend) {
		sysNS := firstNonEmpty(cfg.SystemNamespace, contract.SystemNamespace)
		add(envoyFilterMicrocksRewrite(sysNS, owner, cfg))
	}

	// DestinationRule for Microcks (from this namespace's VS routes).
	add(destinationRule(ns, owner, "microcks-grpc", microcksHost))

	for _, tp := range m.Spec.ThirdParties {
		if tp.Name == "" || tp.Host == "" || tp.Port == 0 {
			return Result{}, fmt.Errorf("thirdParty name, host, and port are required")
		}
		backendHost := firstNonEmpty(tp.BackendHost, tp.Host)
		backendPort := tp.BackendPort
		if backendPort == 0 {
			backendPort = tp.Port
		}
		add(serviceEntry(ns, owner, tp, backendHost, backendPort))
		add(virtualService(ns, owner, tp, microcksHost, microcksPort, cfg.SimulationHeader))
		add(destinationRule(ns, owner, sanitizeName(tp.Name)+"-third-party", tp.Host))
	}

	return out, nil
}

func serviceEntry(ns, owner string, tp simv1.ThirdParty, backendHost string, backendPort int32) *unstructured.Unstructured {
	name := sanitizeName(tp.Name) + "-serviceentry"
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("networking.istio.io/v1beta1")
	u.SetKind("ServiceEntry")
	u.SetName(name)
	u.SetNamespace(ns)
	setManagedLabels(u, owner)

	_ = unstructured.SetNestedStringSlice(u.Object, []string{tp.Host}, "spec", "hosts")
	_ = unstructured.SetNestedField(u.Object, "MESH_INTERNAL", "spec", "location")
	_ = unstructured.SetNestedField(u.Object, "DNS", "spec", "resolution")
	_ = unstructured.SetNestedStringSlice(u.Object, []string{"."}, "spec", "exportTo")
	ports := []interface{}{
		map[string]interface{}{
			"number":   int64(tp.Port),
			"name":     "grpc",
			"protocol": "GRPC",
		},
	}
	_ = unstructured.SetNestedSlice(u.Object, ports, "spec", "ports")
	endpoints := []interface{}{
		map[string]interface{}{
			"address": backendHost,
			"ports": map[string]interface{}{
				"grpc": int64(backendPort),
			},
		},
	}
	_ = unstructured.SetNestedSlice(u.Object, endpoints, "spec", "endpoints")
	return u
}

func virtualService(ns, owner string, tp simv1.ThirdParty, microcksHost string, microcksPort int32, header string) *unstructured.Unstructured {
	name := sanitizeName(tp.Name) + "-simulation"
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("networking.istio.io/v1beta1")
	u.SetKind("VirtualService")
	u.SetName(name)
	u.SetNamespace(ns)
	setManagedLabels(u, owner)

	_ = unstructured.SetNestedStringSlice(u.Object, []string{tp.Host}, "spec", "hosts")
	http := []interface{}{
		map[string]interface{}{
			"name": "simulate",
			"match": []interface{}{
				map[string]interface{}{
					"headers": map[string]interface{}{
						header: map[string]interface{}{
							"regex": ".+",
						},
					},
				},
			},
			"route": []interface{}{
				map[string]interface{}{
					"destination": map[string]interface{}{
						"host": microcksHost,
						"port": map[string]interface{}{"number": int64(microcksPort)},
					},
				},
			},
		},
		map[string]interface{}{
			"name": "real",
			"route": []interface{}{
				map[string]interface{}{
					"destination": map[string]interface{}{
						"host": tp.Host,
						"port": map[string]interface{}{"number": int64(tp.Port)},
					},
				},
			},
		},
	}
	_ = unstructured.SetNestedSlice(u.Object, http, "spec", "http")
	return u
}

func destinationRule(ns, owner, name, host string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("networking.istio.io/v1beta1")
	u.SetKind("DestinationRule")
	u.SetName(name)
	u.SetNamespace(ns)
	setManagedLabels(u, owner)
	_ = unstructured.SetNestedField(u.Object, host, "spec", "host")
	// Teaching / in-mesh plaintext gRPC (matches reference-app mesh demos).
	_ = unstructured.SetNestedField(u.Object, "DISABLE", "spec", "trafficPolicy", "tls", "mode")
	_ = unstructured.SetNestedField(u.Object, "UPGRADE", "spec", "trafficPolicy", "connectionPool", "http", "h2UpgradePolicy")
	return u
}

func envoyFilterInbound(ns, owner string, cfg config.Config) *unstructured.Unstructured {
	return envoyFilterLua(ns, owner, "vf-inbound-capture", "SIDECAR_INBOUND", cfg, `
local HEADER    = "`+cfg.SimulationHeader+`"
local STATE_KEY = "test.simulation.action.name"
function envoy_on_request(request_handle)
  local scenario = request_handle:headers():get(HEADER)
  if scenario ~= nil and scenario ~= "" then
    request_handle:streamInfo():dynamicMetadata():set("envoy.filters.http.lua", STATE_KEY, scenario)
  end
end
`)
}

func envoyFilterOutbound(ns, owner string, cfg config.Config) *unstructured.Unstructured {
	return envoyFilterLua(ns, owner, "vf-outbound-inject", "SIDECAR_OUTBOUND", cfg, `
local HEADER    = "`+cfg.SimulationHeader+`"
local STATE_KEY = "test.simulation.action.name"
function envoy_on_request(request_handle)
  local meta = request_handle:streamInfo():dynamicMetadata():get("envoy.filters.http.lua")
  if meta ~= nil and meta[STATE_KEY] ~= nil then
    request_handle:headers():replace(HEADER, meta[STATE_KEY])
  end
end
`)
}

func envoyFilterLua(ns, owner, name, context string, cfg config.Config, inline string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("networking.istio.io/v1alpha3")
	u.SetKind("EnvoyFilter")
	u.SetName(name)
	u.SetNamespace(ns)
	setManagedLabels(u, owner)
	_ = unstructured.SetNestedStringMap(u.Object, map[string]string{
		cfg.PropagationLabelKey: cfg.PropagationLabelValue,
	}, "spec", "workloadSelector", "labels")

	patchValue := map[string]interface{}{
		"name": "envoy.filters.http.lua",
		"typed_config": map[string]interface{}{
			"@type":      "type.googleapis.com/envoy.extensions.filters.http.lua.v3.Lua",
			"inlineCode": strings.TrimSpace(inline) + "\n",
		},
	}
	configPatches := []interface{}{
		map[string]interface{}{
			"applyTo": "HTTP_FILTER",
			"match": map[string]interface{}{
				"context": context,
				"listener": map[string]interface{}{
					"filterChain": map[string]interface{}{
						"filter": map[string]interface{}{
							"name": "envoy.filters.network.http_connection_manager",
						},
					},
				},
			},
			"patch": map[string]interface{}{
				"operation": "INSERT_BEFORE",
				"value":     patchValue,
			},
		},
	}
	_ = unstructured.SetNestedSlice(u.Object, configPatches, "spec", "configPatches")
	return u
}

func isMicrocksBackend(v string) bool {
	return strings.EqualFold(strings.TrimSpace(v), contract.BackendMicrocks)
}

// envoyFilterMicrocksRewrite maps the simulation header → x-microcks-operation
// on pods labeled app=microcks (real Microcks or teaching mock that understands both).
func envoyFilterMicrocksRewrite(ns, owner string, cfg config.Config) *unstructured.Unstructured {
	simH := cfg.SimulationHeader
	if simH == "" {
		simH = contract.SimulationHeader
	}
	opH := cfg.MicrocksOperationHeader
	if opH == "" {
		opH = contract.MicrocksOperationHeader
	}
	return envoyFilterLuaWorkload(ns, owner, "vf-microcks-scenario-rewrite", "SIDECAR_INBOUND",
		map[string]string{"app": "microcks"}, `
local SRC = "`+simH+`"
local DST = "`+opH+`"
function envoy_on_request(request_handle)
  local scenario = request_handle:headers():get(SRC)
  if scenario ~= nil and scenario ~= "" then
    request_handle:headers():replace(DST, scenario)
  end
end
`)
}

func envoyFilterLuaWorkload(ns, owner, name, context string, workloadLabels map[string]string, inline string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("networking.istio.io/v1alpha3")
	u.SetKind("EnvoyFilter")
	u.SetName(name)
	u.SetNamespace(ns)
	setManagedLabels(u, owner)
	_ = unstructured.SetNestedStringMap(u.Object, workloadLabels, "spec", "workloadSelector", "labels")
	patchValue := map[string]interface{}{
		"name": "envoy.lua",
		"typed_config": map[string]interface{}{
			"@type":      "type.googleapis.com/envoy.extensions.filters.http.lua.v3.Lua",
			"inlineCode": inline,
		},
	}
	configPatches := []interface{}{
		map[string]interface{}{
			"applyTo": "HTTP_FILTER",
			"match": map[string]interface{}{
				"context": context,
				"listener": map[string]interface{}{
					"filterChain": map[string]interface{}{
						"filter": map[string]interface{}{
							"name": "envoy.filters.network.http_connection_manager",
							"subFilter": map[string]interface{}{
								"name": "envoy.filters.http.router",
							},
						},
					},
				},
			},
			"patch": map[string]interface{}{
				"operation": "INSERT_BEFORE",
				"value":     patchValue,
			},
		},
	}
	_ = unstructured.SetNestedSlice(u.Object, configPatches, "spec", "configPatches")
	return u
}

func setManagedLabels(u *unstructured.Unstructured, owner string) {
	labels := u.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[contract.ManagedByLabelKey] = contract.ManagedByLabelValue
	labels[contract.ManifestLabelKey] = owner
	u.SetLabels(labels)
}

func sanitizeName(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, ".", "-")
	s = strings.ReplaceAll(s, "/", "-")
	return s
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func splitHostPort(hp string, defaultPort int32) (string, int32) {
	hp = strings.TrimSpace(hp)
	if hp == "" {
		return "microcks-svc.simulation-system.svc.cluster.local", defaultPort
	}
	if i := strings.LastIndex(hp, ":"); i > 0 && !strings.Contains(hp[i:], "]") {
		host := hp[:i]
		if p, err := strconv.Atoi(hp[i+1:]); err == nil {
			return host, int32(p)
		}
	}
	return hp, defaultPort
}
