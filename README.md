# Helm Debugger

[![CI](https://github.com/jessesimpson36/helm-debugger/actions/workflows/ci.yml/badge.svg)](https://github.com/jessesimpson36/helm-debugger/actions/workflows/ci.yml)
[![Docs](https://github.com/jessesimpson36/helm-debugger/actions/workflows/docs.yml/badge.svg)](https://jessesimpson36.github.io/helm-debugger/)

**Debug Helm charts by tracing `values.yaml` through `_helpers.tpl` into the**
**rendered manifests.**

📖 **Documentation: <https://jessesimpson36.github.io/helm-debugger/>**

## Overview

This project lets you debug Helm charts and answer how your `values.yaml`
options change across `_helpers.tpl` functions and eventually make it into the
final rendered manifests.

The debugger makes use of [Delve](https://github.com/go-delve/delve) because
Go's `text/template` library processes entire files at a time, making it
difficult to track down individual operations without introspecting a running
process.

> **State of the project:** this is still pretty experimental / proof of
> concept.

## Quick start

The recommended way to run this project is the pinned Docker image, which
provides every tool at a known version:

```bash
docker run --rm --cap-add=SYS_PTRACE --security-opt seccomp=unconfined \
  -v "$PWD:/workspace" -w /workspace \
  ghcr.io/jessesimpson36/helm-debugger:latest --mode model --helm-path helm \
  --chart test --values image.tag \
  --extra-command-args '--show-only templates/deployment.yaml'
```

For requirements, the deterministic toolchain, and building from source, see
[Getting started](https://jessesimpson36.github.io/helm-debugger/getting-started/).

## Debug charts from your AI coding tool

`--mode mcp` starts a [Model Context Protocol](https://modelcontextprotocol.io/)
server over stdio so AI tools can reproduce what a chart renders and then find
*which template line* produced it:

| Tool | Purpose |
| --- | --- |
| `helm_template` | Reproduce: render a chart with `helm template` to see the actual output. |
| `debug_helm` | Locate: find the template/helper `file:line` behind a value or rendered line. |
| `resolve_breakpoints` | Report the resolved `text/template/exec.go` line numbers. |

Project-local configs are shipped for OpenCode, Claude Code, Cursor, and VS Code.
See the [MCP server docs](https://jessesimpson36.github.io/helm-debugger/mcp-server/)
for setup, then [Releases and upgrading](https://jessesimpson36.github.io/helm-debugger/releases/)
to pin a published version.

## Documentation

The full documentation lives in [`docs/`](docs/) and is published at
<https://jessesimpson36.github.io/helm-debugger/>:

- [Getting started](https://jessesimpson36.github.io/helm-debugger/getting-started/) — requirements, Docker, building locally
- [Concepts](https://jessesimpson36.github.io/helm-debugger/concepts/) — execution flows, breakpoints, modes, query types
- [CLI reference](https://jessesimpson36.github.io/helm-debugger/cli/) — every command line flag
- [MCP server](https://jessesimpson36.github.io/helm-debugger/mcp-server/) — for OpenCode, Claude Code, Cursor, and VS Code
- [Understanding output](https://jessesimpson36.github.io/helm-debugger/output/) — execution flow, relevant values, write buffer
- [Releases and upgrading](https://jessesimpson36.github.io/helm-debugger/releases/) — image tags, SBOMs, `install-mcp.sh`
- [Releasing](https://jessesimpson36.github.io/helm-debugger/contributing/releasing/) — how releases are cut

## Previewing the docs locally

The site is built with [Zensical](https://zensical.org) (the successor to
Material for MkDocs), which reads `mkdocs.yml`:

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
zensical serve
```

Changes under `docs/` are deployed to GitHub Pages by
[`.github/workflows/docs.yml`](.github/workflows/docs.yml) on pushes to `main`.
