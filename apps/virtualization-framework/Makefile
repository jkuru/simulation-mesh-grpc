MODULE       := github.com/servicemesh/virtualization-framework
BIN_DIR      := bin
IMAGE        ?= virtualization-framework
GOOS_IMG     ?= linux
GOARCH_IMG   ?= $(shell go env GOARCH)

.PHONY: help generate tidy test build build-linux image install uninstall sample coverage golden golden-update istio-check

help:
	@echo "virtualization-framework (v2 operator)"
	@echo "  make tidy test coverage golden build"
	@echo "  make image          # linux binary + docker image $(IMAGE):latest"
	@echo "  make install        # CRD + RBAC + webhooks TLS + NetworkPolicy + operator"
	@echo "  make sample         # apply sample SimulationManifest for reference-app"
	@echo "  make istio-check    # cluster Istio vs support matrix"
	@echo "  make uninstall"
	@echo "  make golden         # generator golden files (UPDATE_GOLDEN=1 to refresh)"

tidy:
	go mod tidy

test:
	go test ./...

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/operator ./cmd/operator

build-linux:
	@mkdir -p $(BIN_DIR)
	GOOS=$(GOOS_IMG) GOARCH=$(GOARCH_IMG) CGO_ENABLED=0 go build -o $(BIN_DIR)/operator ./cmd/operator

image: build-linux
	docker build -f build/Dockerfile -t $(IMAGE):latest .

install:
	@chmod +x ./scripts/install.sh
	@./scripts/install.sh

uninstall:
	@chmod +x ./scripts/uninstall.sh
	@./scripts/uninstall.sh

sample:
	kubectl apply -f config/samples/simulationmanifest_reference_app.yaml
	@echo "watch: kubectl get simm -n poc -w"

# 100% on api/ + internal/ (excludes cmd/operator main).
coverage:
	@chmod +x ./scripts/check-coverage.sh
	@./scripts/check-coverage.sh

# Stable snapshots of generated Istio objects (platform contract).
golden:
	go test ./internal/generator/ -run Golden -count=1

# Regenerate goldens after intentional generator changes.
golden-update:
	UPDATE_GOLDEN=1 go test ./internal/generator/ -run Golden -count=1

istio-check:
	@chmod +x ./scripts/check-istio-version.sh
	@./scripts/check-istio-version.sh

