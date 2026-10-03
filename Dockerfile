# syntax=docker/dockerfile:1
#
# Deterministic build/run environment for the helm debugger.
#
# The project debugger introspects the Go standard library's text/template
# package, so the Go toolchain, delve, and the helm binary under debug all have
# to agree. Instead of assuming versions are installed on the host, this image
# pins them and builds helm from source with debug symbols.
#
# Version pins can be overridden at build time, e.g.:
#   docker build --build-arg GO_VERSION=1.26.7 --build-arg HELM_VERSION=v4.3.0 .

ARG GO_VERSION=1.26.7
ARG HELM_VERSION=v4.3.0
ARG DELVE_VERSION=v1.27.2

# ---------------------------------------------------------------------------
# Build a helm binary with optimization disabled so delve can set breakpoints
# and evaluate variables in text/template.
# ---------------------------------------------------------------------------
FROM golang:${GO_VERSION}-bookworm AS helm-builder
ARG HELM_VERSION
RUN apt-get update \
    && apt-get install -y --no-install-recommends git ca-certificates \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /src
RUN git clone --depth 1 --branch "${HELM_VERSION}" https://github.com/helm/helm .
RUN CGO_ENABLED=0 go build \
    -gcflags="all=-N -l" \
    -trimpath \
    -o /out/helm ./cmd/helm

# ---------------------------------------------------------------------------
# Build the helm-debugger binary (CLI + MCP server).
# ---------------------------------------------------------------------------
FROM golang:${GO_VERSION}-bookworm AS debugger-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/helm-debugger .

# ---------------------------------------------------------------------------
# Runtime image: Go sources (for breakpoint resolution), delve, the debug helm
# binary, and the debugger. The default entrypoint is the MCP server.
# ---------------------------------------------------------------------------
FROM golang:${GO_VERSION}-bookworm AS runtime
ARG DELVE_VERSION
RUN go install github.com/go-delve/delve/cmd/dlv@${DELVE_VERSION}

COPY --from=helm-builder /out/helm /usr/local/bin/helm
COPY --from=debugger-builder /out/helm-debugger /usr/local/bin/helm-debugger

# Users mount their chart repository here. Template paths are resolved relative
# to the working directory.
WORKDIR /workspace

# Sanity check: both tools must be runnable and the standard library source
# used for breakpoints must be present.
RUN dlv version \
    && helm version \
    && test -f "$(go env GOROOT)/src/text/template/exec.go"

ENTRYPOINT ["helm-debugger"]
CMD ["--mode", "mcp"]
