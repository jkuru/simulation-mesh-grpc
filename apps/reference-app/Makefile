# apps/reference-app — reference app (v3 no mesh + v1 mesh manifests).
# Monorepo: docs/MONOREPO.md

MODULE  := github.com/servicemesh/reference-app
BIN_DIR := bin
PROTO_DIR := proto
GEN_DIR := gen
IMAGE_PREFIX ?= reference-app
SERVICES := payment-gateway fraud-checker external-risk microcks-mock test-client
# Container images must be Linux (kind/docker), not host OS binaries.
GOOS_IMG ?= linux
GOARCH_IMG ?= $(shell go env GOARCH)

.PHONY: all generate deps build build-local build-linux build-images unit coverage test up down clean \
	run-local demo help deploy-dev deploy-prod mesh-e2e mesh-test

help:
	@echo "apps/reference-app — reference application"
	@echo "  v3 no mesh:  make demo | coverage | up/test/down"
	@echo "  v1 on mesh:  make mesh-e2e   (kind + Istio full proof)"
	@echo "               make deploy-dev (apply kube overlay)"
	@echo "               make mesh-test  (Job test-client on current cluster)"
	@echo ""
	@echo "  make generate       protoc → gen/"
	@echo "  make build-local    binaries → bin/"
	@echo "  make unit           unit tests"
	@echo "  make coverage       100% on internal/"
	@echo "  make demo           local multi-process proof (no mesh)"
	@echo "  make up/test/down   Docker Compose"
	@echo "  make build-images   docker images for kube"
	@echo "  make deploy-dev     teaching mesh overlay (v1)"
	@echo "  make deploy-prod    prod strip-header only"
	@echo "  make mesh-e2e       kind + Istio + deploy + proof (see docs/MESH.md)"
	@echo "  make mesh-test      re-run proof Job on current cluster"
	@echo "  make clean"

deps:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

generate: deps
	@mkdir -p $(GEN_DIR)
	PATH="$$PATH:$$(go env GOPATH)/bin" protoc \
		-I $(PROTO_DIR) \
		--go_out=$(GEN_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR) --go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/payment/v1/payment.proto \
		$(PROTO_DIR)/fraud/v1/fraud.proto \
		$(PROTO_DIR)/risk/v1/risk.proto
	@echo "generated into $(GEN_DIR)/"

# Host-native binaries (for make demo on macOS/Linux).
build-local: generate
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/payment-gateway ./cmd/payment-gateway
	go build -o $(BIN_DIR)/fraud-checker   ./cmd/fraud-checker
	go build -o $(BIN_DIR)/external-risk   ./cmd/external-risk
	go build -o $(BIN_DIR)/microcks-mock   ./cmd/microcks-mock
	go build -o $(BIN_DIR)/test-client     ./cmd/test-client
	@echo "binaries in $(BIN_DIR)/"

# Linux binaries for container images (GOOS=linux).
build-linux: generate
	@mkdir -p $(BIN_DIR)
	GOOS=$(GOOS_IMG) GOARCH=$(GOARCH_IMG) CGO_ENABLED=0 go build -o $(BIN_DIR)/payment-gateway ./cmd/payment-gateway
	GOOS=$(GOOS_IMG) GOARCH=$(GOARCH_IMG) CGO_ENABLED=0 go build -o $(BIN_DIR)/fraud-checker   ./cmd/fraud-checker
	GOOS=$(GOOS_IMG) GOARCH=$(GOARCH_IMG) CGO_ENABLED=0 go build -o $(BIN_DIR)/external-risk   ./cmd/external-risk
	GOOS=$(GOOS_IMG) GOARCH=$(GOARCH_IMG) CGO_ENABLED=0 go build -o $(BIN_DIR)/microcks-mock   ./cmd/microcks-mock
	GOOS=$(GOOS_IMG) GOARCH=$(GOARCH_IMG) CGO_ENABLED=0 go build -o $(BIN_DIR)/test-client     ./cmd/test-client
	@echo "linux binaries in $(BIN_DIR)/ ($(GOOS_IMG)/$(GOARCH_IMG))"

unit: generate
	go test ./...

# Interface-first packages under internal/ must stay ≥98% covered.
coverage: generate
	@./scripts/check-coverage.sh

# Local multi-process demo (goroutines inside each process; processes independent).
demo: build-local
	@./scripts/run-local-demo.sh

up:
	docker compose up --build -d
	@echo "stack up — run: make test"

test:
	docker compose run --rm test-client

down:
	docker compose down

# Images expected by kube/kustomize (imagePullPolicy: IfNotPresent).
# Builds host binaries then packages them (distroless) — no in-image Go compile.
# Load into kind: kind load docker-image $(IMAGE_PREFIX)/payment-gateway:latest …
build-images: build-linux
	@set -e; for s in $(SERVICES); do \
		echo "==> packaging $(IMAGE_PREFIX)/$$s:latest"; \
		docker build -f build/Dockerfile --build-arg SERVICE=$$s -t $(IMAGE_PREFIX)/$$s:latest . ; \
	done

deploy-dev:
	kubectl apply -k kube/kustomize/overlays/dev
	@echo "applied dev overlay — kubectl get deploy -n poc --show-labels"
	@echo "proof job: make mesh-test   (docs: docs/MESH.md)"

deploy-prod:
	kubectl apply -k kube/kustomize/overlays/prod
	@echo "applied prod strip-header EnvoyFilter (namespace istio-system)"

# Full v1 path: kind cluster + Istio + images + deploy + virtualization proof.
mesh-e2e:
	@chmod +x ./scripts/mesh-e2e.sh
	@./scripts/mesh-e2e.sh

# Re-run test-client Job against an already-deployed mesh stack.
mesh-test:
	kubectl -n poc delete job reference-app-mesh-test --ignore-not-found
	kubectl apply -f kube/kustomize/overlays/dev/test-client-job.yaml
	kubectl -n poc wait --for=condition=complete job/reference-app-mesh-test --timeout=180s
	kubectl -n poc logs job/reference-app-mesh-test

clean:
	rm -rf $(GEN_DIR) $(BIN_DIR)
