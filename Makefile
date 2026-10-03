.PHONY: all build test test-unit test-race test-integration vet fmt clean \
	clone_helm compile_helm clone_dlv compile_dlv run \
	test_values_query test_helpers_query test_template_query test_rendered_query test_all_queries \
	docker-build docker-test docker-run docker-mcp docker-shell docker-push \
	version print-toolchain release-metadata

# Pinned toolchain versions for the CLI and the deterministic Docker
# environment. They live in one file that CI also sources, so the two cannot
# drift. Override them on the command line, e.g.
#   make docker-build GO_VERSION=1.26.7
include toolchain.env

# Release identity. VERSION defaults to the nearest git tag (or a short SHA,
# with -dirty when the tree is modified) so local builds are traceable too.
VERSION       ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT        ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
BUILD_DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

MODULE        := github.com/jessesimpson36/helm-debugger
VERSION_PKG   := $(MODULE)/internal/version
# Injected into internal/version so `helm-debugger --version` and the MCP
# initialize response report the same metadata written into the release.
LDFLAGS       := -X $(VERSION_PKG).Version=$(VERSION) \
                 -X $(VERSION_PKG).Commit=$(COMMIT) \
                 -X $(VERSION_PKG).BuildDate=$(BUILD_DATE) \
                 -X $(VERSION_PKG).HelmVersion=$(HELM_VERSION) \
                 -X $(VERSION_PKG).DelveVersion=$(DELVE_VERSION)

IMAGE         ?= jessesimpson/helm-debugger:latest
REGISTRY_REPO ?= jessesimpson/helm-debugger
# Immutable tag that records exactly what the image contains.
PUSH_TAG      ?= go$(GO_VERSION)-helm$(patsubst v%,%,$(HELM_VERSION))-delve$(patsubst v%,%,$(DELVE_VERSION))

# Delve needs ptrace; the default seccomp profile blocks it on many kernels.
DOCKER_RUN_FLAGS := --rm --cap-add=SYS_PTRACE --security-opt seccomp=unconfined

all: build

build:
	go build -trimpath -ldflags '$(LDFLAGS)' -o helm-debugger .

# Print the build/toolchain metadata the binary would report.
version:
	@go run -ldflags '$(LDFLAGS)' . --version

print-toolchain:
	@echo "go=$(GO_VERSION) helm=$(HELM_VERSION) delve=$(DELVE_VERSION)"

# Write the same release metadata JSON CI attaches to a release. Set
# IMAGE_DIGEST (and friends) to record the published image.
release-metadata:
	@mkdir -p dist
	VERSION=$(VERSION) COMMIT=$(COMMIT) BUILD_DATE=$(BUILD_DATE) \
	GO_VERSION=$(GO_VERSION) HELM_VERSION=$(HELM_VERSION) DELVE_VERSION=$(DELVE_VERSION) \
	IMAGE_DIGEST='$(IMAGE_DIGEST)' IMAGE='$(IMAGE)' \
	./scripts/release-metadata.sh > dist/metadata.json
	@echo "wrote dist/metadata.json"

test: test-unit

test-unit:
	go test ./...

test-race:
	go test -race ./...

# Runs the delve-backed integration test. Requires a debug-enabled helm binary
# (set HELM_DEBUGGER_HELM) and dlv on PATH.
test-integration:
	go test ./internal/debugger/ -run TestRunAgainstLocalHelm -count=1 -timeout 300s

vet:
	go vet ./...

fmt:
	gofmt -w .

clean:
	rm -f helm-debugger
	go clean

# ---------------------------------------------------------------------------
# Legacy local build (uses a helm clone compiled with debug symbols)
# ---------------------------------------------------------------------------

clone_helm:
	git clone https://github.com/helm/helm

compile_helm:
	sed -i 's/LDFLAGS\s*:=.*/LDFLAGS := /' helm/Makefile
	sed -i 's/GOFLAGS\s*:=.*/GOFLAGS := -gcflags="all=-N -l"/' helm/Makefile
	cd helm && make

clone_dlv:
	git clone https://github.com/go-delve/delve

compile_dlv:
	cd delve && make build

run: test_all_queries

test_values_query:
	go run . --mode model --helm-path ./helm/bin/helm --values image.tag --chart test --extra-command-args '--show-only templates/deployment.yaml'

test_helpers_query:
	go run . --mode model --helm-path ./helm/bin/helm --helper-file test.serviceAccountName --chart test --extra-command-args '--show-only templates/deployment.yaml'

test_template_query:
	go run . --mode model --helm-path ./helm/bin/helm --template-file test/templates/deployment.yaml:42 --chart test --extra-command-args '--show-only templates/deployment.yaml'

test_rendered_query:
	go run . --mode model --helm-path ./helm/bin/helm --rendered-file test/templates/deployment.yaml:32 --chart test --extra-command-args '--show-only templates/deployment.yaml'

test_all_queries:
	go run . --mode model \
		--helm-path ./helm/bin/helm \
		--rendered-file test/templates/deployment.yaml:32 \
		--template-file test/templates/deployment.yaml:42 \
		--helper-file test.serviceAccountName \
		--values image.tag \
		--chart test \
		--extra-command-args '--show-only templates/deployment.yaml'

# ---------------------------------------------------------------------------
# Deterministic Docker environment
# ---------------------------------------------------------------------------

docker-build:
	docker build \
		--build-arg GO_VERSION=$(GO_VERSION) \
		--build-arg HELM_VERSION=$(HELM_VERSION) \
		--build-arg DELVE_VERSION=$(DELVE_VERSION) \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(IMAGE) .

# Publish both the moving `latest` tag and an immutable tag describing the
# exact toolchain versions baked into the image.
docker-push: docker-build
	docker push $(IMAGE)
	docker tag $(IMAGE) $(REGISTRY_REPO):$(PUSH_TAG)
	docker push $(REGISTRY_REPO):$(PUSH_TAG)

# Run the bundled model-mode smoke test against the test chart inside the image.
docker-test: docker-build
	docker run $(DOCKER_RUN_FLAGS) -v "$(CURDIR):/workspace" -w /workspace $(IMAGE) \
		--mode model --helm-path helm --chart test \
		--values image.tag \
		--extra-command-args '--show-only templates/deployment.yaml'

docker-run: docker-build
	docker run $(DOCKER_RUN_FLAGS) -it -v "$(CURDIR):/workspace" -w /workspace $(IMAGE) \
		--mode model --helm-path helm --chart test \
		--extra-command-args '--show-only templates/deployment.yaml'

# Start the MCP server over stdio (used by the project-local MCP configs under
# .opencode/, .mcp.json, .cursor/mcp.json and .vscode/mcp.json).
docker-mcp: docker-build
	docker run $(DOCKER_RUN_FLAGS) -i -v "$(CURDIR):/workspace" -w /workspace $(IMAGE) --mode mcp

docker-shell: docker-build
	docker run $(DOCKER_RUN_FLAGS) -it --entrypoint bash -v "$(CURDIR):/workspace" -w /workspace $(IMAGE)
