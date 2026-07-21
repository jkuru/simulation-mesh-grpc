# Final example: reference-app driven by virtualization-framework.
# No hand-written VirtualService / EnvoyFilter in this package.

.PHONY: help e2e apply-app apply-manifest test-job install-framework assert-no-istio-in-kustomize

help:
	@echo "examples/reference-app-with-framework"
	@echo "  make e2e                 full kind e2e (Istio required on cluster)"
	@echo "  make install-framework   kubectl apply -k ../../apps/virtualization-framework/config"
	@echo "  make apply-app           deploy app only (no VS/EF)"
	@echo "  make apply-manifest      apply SimulationManifest"
	@echo "  make test-job            run proof Job"
	@echo "  make assert-no-istio-in-kustomize"

assert-no-istio-in-kustomize:
	@! grep -rE 'kind:[[:space:]]*(VirtualService|ServiceEntry|EnvoyFilter)' kustomize/ \
		|| (echo "error: kustomize/ must not contain Istio routing resources" && exit 1)
	@echo "ok: no VS/SE/EnvoyFilter in kustomize/"

install-framework:
	$(MAKE) -C ../../apps/virtualization-framework install
	kubectl -n simulation-system rollout status deploy/virtualization-framework --timeout=180s

apply-app: assert-no-istio-in-kustomize
	kubectl apply -k kustomize/

apply-manifest:
	kubectl apply -f simulation-manifest.yaml
	@echo "wait: kubectl get simm -n poc -w"

test-job:
	kubectl -n poc delete job reference-app-with-framework-test --ignore-not-found
	kubectl apply -f test-client-job.yaml
	kubectl -n poc wait --for=condition=complete job/reference-app-with-framework-test --timeout=180s
	kubectl -n poc logs job/reference-app-with-framework-test

e2e:
	@chmod +x ./scripts/e2e.sh
	@./scripts/e2e.sh
