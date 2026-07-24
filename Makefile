.PHONY: help build test lint clean \
	build-hyperfleet-db build-operator build-api \
	test-hyperfleet-db test-operator test-operator-int test-api \
	test-e2e test-e2e-api test-e2e-cli test-e2e-platform-monitoring test-e2e-zoa test-e2e-authz \
	e2e-authz-infra-up e2e-authz-infra-down e2e-init-db \
	fmt vet verify deps \
	manifests generate setup-envtest \
	image-api image-operator image-e2e image-push-api image-push-operator

# ── Configuration ────────────────────────────────────────────────────────

IMAGE_REPO_API      ?= quay.io/openshift-online/rosa-hyperfleet-api
IMAGE_REPO_OPERATOR ?= quay.io/openshift-online/hyperfleet-operator
IMAGE_TAG           ?= latest
GIT_SHA             := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GOOS                ?= linux
GOARCH              ?= amd64
PLATFORMS           ?= linux/amd64,linux/arm64

TEST_OUTPUT_DIR     ?= $(or $(ARTIFACT_DIR),./test-results)
DYNAMODB_ENDPOINT   ?= http://localhost:8180
CEDAR_AGENT_ENDPOINT?= http://localhost:8181

AWS_PROFILE ?=
AWS_REGION  ?=
FOCUS       ?=
SKIP        ?= Authz

CONTAINER_ENGINE ?= $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)

TOOLS_DIR        := ./hack/tools
TOOLS_BIN_DIR    := $(TOOLS_DIR)/bin
GOLANGCI_LINT    := $(abspath $(TOOLS_BIN_DIR)/golangci-lint)

$(GOLANGCI_LINT): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go build -tags=tools -o $(abspath $(TOOLS_BIN_DIR))/golangci-lint github.com/golangci/golangci-lint/v2/cmd/golangci-lint

# ── Help ─────────────────────────────────────────────────────────────────

help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Build:"
	@echo "  build                Build all components"
	@echo "  build-api            Platform API server"
	@echo "  build-operator       Hyperfleet operator (manager + compactor)"
	@echo "  build-hyperfleet-db  Hyperfleet DB library"
	@echo ""
	@echo "Test:"
	@echo "  test                 All unit tests"
	@echo "  test-api             Platform API"
	@echo "  test-operator        Hyperfleet operator"
	@echo "  test-operator-int    Operator integration (Postgres + DynamoDB)"
	@echo "  test-hyperfleet-db   Hyperfleet DB"
	@echo "  test-e2e-api         E2E API"
	@echo "  test-e2e-cli         E2E CLI"
	@echo "  test-e2e-authz       E2E authz (starts local infra)"
	@echo "  test-e2e-zoa         E2E ZOA"
	@echo "  test-e2e-platform-monitoring  E2E monitoring"
	@echo ""
	@echo "Code Quality:"
	@echo "  lint                 golangci-lint on all modules"
	@echo "  fmt                  Format Go source"
	@echo "  vet                  go vet on all modules"
	@echo "  verify               Verify go.mod tidiness"
	@echo ""
	@echo "Code Generation:"
	@echo "  manifests            Generate CRD manifests"
	@echo "  generate             Generate deepcopy methods"
	@echo "  setup-envtest        Install envtest binaries (etcd, kube-apiserver)"
	@echo "  deps                 Download and tidy all modules"
	@echo ""
	@echo "Images:"
	@echo "  image-api            Platform API image"
	@echo "  image-operator       Hyperfleet operator image"
	@echo "  image-e2e            E2E test image"

# ── Build ────────────────────────────────────────────────────────────────

build: build-hyperfleet-db build-operator build-api

build-hyperfleet-db:
	cd hyperfleet-db && go build ./...

build-operator:
	cd hyperfleet-operator && go build -o ../bin/manager ./cmd/manager
	cd hyperfleet-operator && go build -o ../bin/compactor ./cmd/compactor

build-api:
	cd platform-api && go build -o ../bin/rosa-hyperfleet-api ./cmd

# ── Test ─────────────────────────────────────────────────────────────────

# TODO: add test-operator (needs setup-envtest in CI image) and test-hyperfleet-db (needs podman + postgres)
test: test-api

test-hyperfleet-db:
	cd hyperfleet-db && go test -v -race -count=1 ./...

test-operator:
	cd hyperfleet-operator && KUBEBUILDER_ASSETS="$$(setup-envtest use --print path -p path 2>/dev/null || echo '')" go test -v -race -count=1 ./internal/...

test-operator-int:
	cd hyperfleet-operator && go test -v -race -count=1 ./test/...

test-api:
	cd platform-api && go test -v -race -count=1 $$(go list ./... | grep -v '/test/e2e')

test-e2e: test-e2e-api

test-e2e-api:
	E2E_BASE_URL="$${BASE_URL}" E2E_ACCOUNT_ID="$${E2E_ACCOUNT_ID}" \
	E2E_RHOBS_API_URL="$${RHOBS_API_URL}" \
	ginkgo -vv --skip="Authz" \
		--junit-report=junit-api.xml --output-dir=$(TEST_OUTPUT_DIR) \
		./test/e2e-api

test-e2e-cli:
	E2E_BASE_URL="$${BASE_URL}" E2E_ACCOUNT_ID="$${E2E_ACCOUNT_ID}" \
	E2E_RHOBS_API_URL="$${RHOBS_API_URL}" \
	ROSACTL_BIN="$${ROSACTL_BIN}" AWS_REGION="$${AWS_REGION}" \
	ginkgo -vv --junit-report=junit-cli.xml \
		$(if $(E2E_LABEL_FILTER),--label-filter="$(E2E_LABEL_FILTER)") \
		--output-dir=$(TEST_OUTPUT_DIR) ./test/e2e-cli

test-e2e-platform-monitoring:
	E2E_RHOBS_API_URL="$${RHOBS_API_URL}" \
	ginkgo -vv --junit-report=junit-platform-monitoring.xml \
		--output-dir=$(TEST_OUTPUT_DIR) ./test/e2e-platform-monitoring

test-e2e-zoa:
	E2E_BASE_URL="$${BASE_URL}" E2E_ACCOUNT_ID="$${E2E_ACCOUNT_ID}" \
	ginkgo -vv --junit-report=junit-zoa.xml \
		--output-dir=$(TEST_OUTPUT_DIR) ./test/e2e-zoa

# ── E2E Infrastructure ──────────────────────────────────────────────────

e2e-authz-infra-up:
	podman-compose -f hack/podman-compose.e2e-authz.yaml up -d
	@echo "Waiting for services to be ready..."
	@sleep 5
	@$(MAKE) e2e-init-db

e2e-authz-infra-down:
	podman-compose -f hack/podman-compose.e2e-authz.yaml down -v

e2e-init-db:
	./scripts/e2e-init-dynamodb.sh

test-e2e-authz: e2e-authz-infra-up
	@./scripts/run-e2e-authz.sh

# ── Code Quality ─────────────────────────────────────────────────────────

fmt:
	cd hyperfleet-db && go fmt ./...
	cd hyperfleet-operator && go fmt ./...
	cd platform-api && go fmt ./...

vet:
	cd hyperfleet-db && go vet ./...
	cd hyperfleet-operator && go vet ./...
	cd platform-api && go vet ./...

lint: $(GOLANGCI_LINT)
	cd hyperfleet-db && $(GOLANGCI_LINT) run --config ../.golangci.yml --timeout 5m ./...
	cd hyperfleet-operator && $(GOLANGCI_LINT) run --config ../.golangci.yml --timeout 5m ./...
	cd platform-api && $(GOLANGCI_LINT) run --config ../.golangci.yml --timeout 5m ./...

verify:
	cd hyperfleet-db && go mod tidy
	cd hyperfleet-operator/api && go mod tidy
	cd hyperfleet-operator && go mod tidy
	cd platform-api && go mod tidy
	cd test && go mod tidy
	cd hack/tools && go mod tidy
	git diff --exit-code \
		hyperfleet-db/go.mod hyperfleet-db/go.sum \
		hyperfleet-operator/api/go.mod hyperfleet-operator/api/go.sum \
		hyperfleet-operator/go.mod hyperfleet-operator/go.sum \
		platform-api/go.mod platform-api/go.sum \
		test/go.mod test/go.sum \
		hack/tools/go.mod hack/tools/go.sum

deps:
	cd hyperfleet-db && go mod download && go mod tidy
	cd hyperfleet-operator/api && go mod download && go mod tidy
	cd hyperfleet-operator && go mod download && go mod tidy
	cd platform-api && go mod download && go mod tidy
	cd test && go mod download && go mod tidy

# ── Code Generation ──────────────────────────────────────────────────────

manifests:
	cd hyperfleet-operator && controller-gen crd paths="./api/..." output:crd:dir=config/crd/bases

generate:
	cd hyperfleet-operator && controller-gen object paths="./api/..."

setup-envtest:
	go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
	setup-envtest use

# ── Images ───────────────────────────────────────────────────────────────

image-api:
	$(CONTAINER_ENGINE) build -f platform-api/Containerfile \
		--platform $(GOOS)/$(GOARCH) \
		-t $(IMAGE_REPO_API):$(IMAGE_TAG) .
	$(CONTAINER_ENGINE) tag $(IMAGE_REPO_API):$(IMAGE_TAG) $(IMAGE_REPO_API):$(GIT_SHA)

image-operator:
	$(CONTAINER_ENGINE) build -f hyperfleet-operator/Containerfile \
		--platform $(GOOS)/$(GOARCH) \
		-t $(IMAGE_REPO_OPERATOR):$(IMAGE_TAG) .
	$(CONTAINER_ENGINE) tag $(IMAGE_REPO_OPERATOR):$(IMAGE_TAG) $(IMAGE_REPO_OPERATOR):$(GIT_SHA)

image-e2e:
	$(CONTAINER_ENGINE) build -f platform-api/Containerfile.e2e \
		--platform $(GOOS)/$(GOARCH) \
		-t $(IMAGE_REPO_API)-e2e:$(IMAGE_TAG) .

image-push-api: image-api
	$(CONTAINER_ENGINE) push $(IMAGE_REPO_API):$(IMAGE_TAG)
	$(CONTAINER_ENGINE) push $(IMAGE_REPO_API):$(GIT_SHA)

image-push-operator: image-operator
	$(CONTAINER_ENGINE) push $(IMAGE_REPO_OPERATOR):$(IMAGE_TAG)
	$(CONTAINER_ENGINE) push $(IMAGE_REPO_OPERATOR):$(GIT_SHA)

# ── Clean ────────────────────────────────────────────────────────────────

clean:
	rm -rf bin/
	rm -f coverage.out coverage.html
	rm -rf test-results/

# ── All ──────────────────────────────────────────────────────────────────

all: deps fmt vet lint test build
