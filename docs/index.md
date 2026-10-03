# Helm Debugger

[![CI](https://github.com/jessesimpson36/helm-debugger/actions/workflows/ci.yml/badge.svg)](https://github.com/jessesimpson36/helm-debugger/actions/workflows/ci.yml)
[![Docs](https://github.com/jessesimpson36/helm-debugger/actions/workflows/docs.yml/badge.svg)](https://jessesimpson36.github.io/helm-debugger/)

**Debug Helm charts by tracing `values.yaml` through `_helpers.tpl` into the
rendered manifests.**

## Overview

This project lets you debug Helm charts and answer how your `values.yaml`
options change across `_helpers.tpl` functions and eventually make it into the
final rendered manifests.

The debugger makes use of [Delve](https://github.com/go-delve/delve) because
Go's `text/template` library processes entire files at a time, making it
difficult to track down individual operations without introspecting a running
process.

!!! warning "State of the project"
    This is still pretty experimental / proof of concept.

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

Continue with [Getting started](getting-started.md) for requirements, the
deterministic toolchain, and building from source.

## What it produces

For a query, the debugger reports the **execution flow** that produced a line,
the **relevant values** the flow reads, and the **write buffer** (the rendered
output). For example, asking why `serviceAccountName` renders a particular
value returns:

```text
test/templates/deployment.yaml:30
          serviceAccountName: {{ include "test.serviceAccountName" . }}
  test/templates/_helpers.tpl:57
    in test.serviceAccountName
      {{- if .Values.serviceAccount.create }}
  test/templates/_helpers.tpl:58
    in test.serviceAccountName
      {{- default (include "test.fullname" .) .values.serviceAccount.name }}
  ...
```

See [Understanding output](output.md) for a full walk-through.

## Where to go next

<div class="grid cards" markdown>

-   **[Getting started](getting-started.md)**

    Requirements, Docker, and building locally.

-   **[Concepts](concepts.md)**

    Execution flows, breakpoints, modes, and query types.

-   **[CLI reference](cli.md)**

    Every command line flag.

-   **[MCP server](mcp-server.md)**

    Let AI coding tools render and debug charts at runtime.

-   **[Understanding output](output.md)**

    How to read the execution flow, relevant values, and write buffer.

-   **[Releases and upgrading](releases.md)**

    Image tags, SBOMs, and the `install-mcp.sh` helper.

</div>

## Support

If this project is useful to you, consider buying me a coffee:

[Support me on Ko-fi](https://ko-fi.com/jessesimpson36){ .md-button .md-button--primary }

See [Support this project](support.md) for other ways to help.
