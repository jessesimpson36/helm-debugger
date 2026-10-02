.PHONY: all build test test-unit test-race test-integration vet fmt clean \
	clone_helm compile_helm run \
	test_values_query test_helpers_query test_template_query test_rendered_query test_all_queries \
	docker-build docker-test docker-run docker-mcp docker-shell

# Pinned toolchain versions for the deterministic Docker environment. Override
# on the command line, e.g. `make docker-build GO_VERSION=1.26.7`.
GO_VERSION    ?= 1.26.7
HELM_VERSION  ?= v4.3.0
DELVE_VERSION ?= v1.27.2
IMAGE         ?= helm-debugger:dev

# Delve needs ptrace; the default seccomp profile blocks it on many kernels.
DOCKER_RUN_FLAGS := --rm --cap-add=SYS_PTRACE --security-opt seccomp=unconfined

all: build

build:
	go build -o helm-debugger .

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
		-t $(IMAGE) .

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

# Start the MCP server over stdio (used by opencode via .opencode/opencode.json).
docker-mcp: docker-build
	docker run $(DOCKER_RUN_FLAGS) -i -v "$(CURDIR):/workspace" -w /workspace $(IMAGE) --mode mcp

docker-shell: docker-build
	docker run $(DOCKER_RUN_FLAGS) -it --entrypoint bash -v "$(CURDIR):/workspace" -w /workspace $(IMAGE)
