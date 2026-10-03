# Getting started

## Requirements

The recommended way to run this project is the pinned Docker image, which
provides every tool at a known version:

- Docker

To run it directly on your machine instead:

- Delve (the Docker image pins v1.27.2)
- Make
- Helm (compiled with debug symbols, see below)
- Git
- Go (the Docker image pins the Go toolchain version)

The debugger sets breakpoints in the Go standard library's
`text/template/exec.go`. Those line numbers depend on the Go toolchain that
compiled helm. Rather than hard-coding them, the debugger resolves them at
runtime from `GOROOT` (see [`internal/breakpoints/resolve.go`](https://github.com/jessesimpson36/helm-debugger/blob/main/internal/breakpoints/resolve.go)).
When running the Docker image, the Go toolchain that built helm and the Go
toolchain inside the image are the same, so this is automatic.

### Compiling Helm

Helm often discards their debug symbols, which the debugger needs in order to
hit breakpoints within Delve. That is why there are options for specifying a
custom helm binary path (`--helm-path`).

To build against a helm clone with debug symbols:

```bash
make clone_helm
make compile_helm
make
```

The local flow must be compiled with the same Go toolchain that built helm, or
you can point at the right source tree with `-goroot`. The Makefile includes
examples on how to work with the arguments.

## Determinism with Docker

`make docker-build` builds an image from `Dockerfile` whose toolchain is pinned
in [`toolchain.env`](https://github.com/jessesimpson36/helm-debugger/blob/main/toolchain.env) —
the same file CI and the release workflow read, so builds and release metadata
cannot drift:

| Component | Default | Build arg |
| --- | --- | --- |
| Go toolchain | `1.26.7` | `GO_VERSION` |
| Helm | `v4.3.0` (built with debug symbols) | `HELM_VERSION` |
| Delve | `v1.27.2` | `DELVE_VERSION` |

Override them at build time, for example:

```bash
make docker-build GO_VERSION=1.26.7 HELM_VERSION=v4.3.0 DELVE_VERSION=v1.27.2
```

## Running with Docker

Prebuilt images are published to GitHub Container Registry as
`ghcr.io/jessesimpson36/helm-debugger` (primary) and mirrored to Docker Hub as
`jessesimpson/helm-debugger`. Each release publishes the version tags
(`vX.Y.Z`, `X.Y.Z`, `X.Y`, `X`, `latest`) plus an immutable toolchain tag
describing exactly what is inside (for example
`go1.26.7-helm4.3.0-delve1.27.2`); pin a version tag or the digest if you need
strict reproducibility. The image bundles a debug-enabled helm and the pinned
toolchain, so there is nothing else to install:

```bash
# Use the published image. Docker pulls it on first run.
docker run --rm --cap-add=SYS_PTRACE --security-opt seccomp=unconfined \
  -v "$PWD:/workspace" -w /workspace \
  ghcr.io/jessesimpson36/helm-debugger:latest --mode model --helm-path helm --chart test \
  --values image.tag --extra-command-args '--show-only templates/deployment.yaml'
```

Or build and use it locally:

```bash
make docker-build

# Run the bundled model-mode smoke test against the test chart.
make docker-test

# Run the CLI interactively.
make docker-run

# Start the MCP server over stdio.
make docker-mcp
```

To publish a build, push both the moving `latest` tag and the immutable
toolchain tag:

```bash
make docker-push
```

Mount your own chart repository at `/workspace` and pass chart paths relative to
it, e.g.:

```bash
docker run --rm --cap-add=SYS_PTRACE --security-opt seccomp=unconfined \
  -v /path/to/your/repo:/workspace -w /workspace \
  ghcr.io/jessesimpson36/helm-debugger:latest --mode model --helm-path helm --chart mychart \
  --values image.tag --extra-command-args '--show-only templates/deployment.yaml'
```

Delve needs `ptrace`, which is why the examples add `--cap-add=SYS_PTRACE` and
disable the default seccomp profile.

## Running locally

To build the debugger against a helm clone with debug symbols:

```bash
make clone_helm
make compile_helm
make
```

The Makefile targets `test_values_query`, `test_helpers_query`,
`test_template_query`, `test_rendered_query`, and `test_all_queries` show the
arguments in action. See the [CLI reference](cli.md) for what each flag does.

!!! note "Apple Silicon"
    The `linux/arm64` image runs natively on M-series Macs. This matters: the
    debugger drives Delve, which uses `ptrace`, and `ptrace` does not work under
    amd64-on-arm64 emulation (QEMU does not implement it, and Rosetta fails on
    register reads). Docker Desktop picks the arm64 image automatically, so no
    config change is needed on an Apple Silicon Mac.
