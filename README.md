
## Helm debugger

## Overview

This project allows you to debug helm charts and hopefully answer how your `values.yaml` options change across `_helper.tpl` functions and eventually make it into the final rendered manifests.

The debugger makes use of dlv because golangs text/template library will process entire files at a time, making it difficult to track down individual operations without introspecting a running process.


## State of the project

This is still pretty experimental / proof of concept.


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
runtime from `GOROOT` (see `internal/breakpoints/resolve.go`). When running the
Docker image, the Go toolchain that built helm and the Go toolchain inside the
image are the same, so this is automatic.

### Determinism with Docker

`make docker-build` builds an image from `Dockerfile` that pins:

| Component | Default | Build arg |
| --- | --- | --- |
| Go toolchain | `1.26.7` | `GO_VERSION` |
| Helm | `v4.3.0` (built with debug symbols) | `HELM_VERSION` |
| Delve | `v1.27.2` | `DELVE_VERSION` |

Override them at build time, for example:

```bash
make docker-build GO_VERSION=1.26.7 HELM_VERSION=v4.3.0 DELVE_VERSION=v1.27.2
```

## Concepts / Terminology

This program starts a headless golang debugger in the background and translates places in memory of that debugger to helm chart logic. In the context of this program, I often refer to this as **Breakpoints**, even though there is not yet an interactive debugging mode of this program.

The execution **modes** of this program are simply alternative main functions I've been trying out to see which is most useful. I might remove the not-so-useful ones.

**Execution flow** refers to the linear path the program takes executing instructions after if/else conditions are evaluated. This may look a bit like a stacktrace, but it's not.

### Compiling Helm

Helm often discards their debug symbols, which I think I need to be able to hit breakpoints within delve. That's why there are options for specifying a custom helm binary path.

### Helm chart names vs paths

Helm names templates after the chart's `Chart.yaml` name, not after the
directory the chart lives in. For a chart in a versioned directory (for example
`example-platform-8.9/` for chart name `example-platform`) the names reported at
runtime are `example-platform/templates/...`. The debugger resolves those names
back to the chart directory it was pointed at, so the directory and the chart
name are allowed to differ.

The command still has to point at a local chart folder. Pointing `--chart` at a
`.tgz` archive or a registry reference is not supported, and helper files from
dependencies vendored as `.tgz` archives cannot be read from disk. When a
template source cannot be resolved the debugger keeps the execution flow and
emits a warning instead of dropping it. Warnings appear in a `WARNINGS` report
section and in the `warnings` field of the MCP response, so an empty or partial
flow set explains itself.

### Modes
- **model**: This mode builds a complete data structure representing all execution flows within the chart templates and helpers. Then allows you to query which execution flows you want to follow.
- **branch**: This is the first mode I built and it only captures if/else conditions and whether they evaluate to true or false. It's not very useful. 
- **line**: After writing the branch flow, I wanted to print out every line as it's processed. This mode is pretty overwhelming without being filtered.
- **mcp**: Runs a Model Context Protocol server over stdio so AI tools can render and debug charts at runtime. See [MCP server](#mcp-server).


### Query types

Each time a breakpoint is hit, the program captures the execution path affecting that line.

- **template file queries**: these queries specify which Golang template files to set breakpoints on.
- **helper file queries**: these queries specify which helper files to set breakpoints on.
- **rendered file queries**: these queries specify which rendered manifest files to set breakpoints on.
- **values queries**: these queries specify which values to capture at each breakpoint or after rendering.

## Command line options

```
  -chart string
    	The name of the Helm chart to debug.
  -debug-port int
    	Port for the headless delve server. 0 picks a free port.
  -extra-command-args string
    	Additional command line arguments to pass to 'helm template' command.
  -goroot string
    	GOROOT used to resolve text/template breakpoints. Defaults to the debugger's own GOROOT.
  -helm-path string
    	Path to the compiled Helm binary. (default "helm")
  -helper-file string
    	Comma-delimited list of query files for helpers.
  -mode string
    	Mode of operation: model, branch, line, mcp (default "model")
  -rendered-file string
    	Comma-delimited list of query files for rendered manifest.
  -template-file string
    	Comma-delimited list of query files for templates and helpers.
  -values string
    	Comma-delimited list of values queries to capture.
  -working-dir string
    	Directory the helm chart paths are relative to. Defaults to the current directory.
```

## Running with Docker

A prebuilt image is published to Docker Hub as
`jessesimpson/helm-debugger:latest`. An immutable tag describing the exact
toolchain is pushed alongside it (for example
`jessesimpson/helm-debugger:go1.26.7-helm4.3.0-delve1.27.2`); pin that tag or the
digest if you need strict reproducibility. The image bundles a debug-enabled
helm and the pinned toolchain, so there is nothing else to install:

```bash
# Use the published image. Docker pulls it on first run.
docker run --rm --cap-add=SYS_PTRACE --security-opt seccomp=unconfined \
  -v "$PWD:/workspace" -w /workspace \
  jessesimpson/helm-debugger:latest --mode model --helm-path helm --chart test \
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
  jessesimpson/helm-debugger:latest --mode model --helm-path helm --chart mychart \
  --values image.tag --extra-command-args '--show-only templates/deployment.yaml'
```

## Running locally

To build against a helm clone with debug symbols:

```bash
make clone_helm
make compile_helm
make
```

The local flow must be compiled with the same Go toolchain that built helm, or
you can point at the right source tree with `-goroot`. The Makefile includes
examples on how to work with the arguments.

## MCP server

`--mode mcp` starts a [Model Context Protocol](https://modelcontextprotocol.io/)
server over stdio. It lets AI coding tools reproduce what a chart renders and
then find *which template line* produced it, instead of grepping templates by
hand or guessing. It exposes three tools:

| Tool | Purpose |
| --- | --- |
| `helm_template` | Reproduce: render a chart with `helm template` to see the actual (possibly wrong) output. |
| `debug_helm` | Locate: find the template/helper `file:line` behind a value or rendered line, before editing. |
| `resolve_breakpoints` | Report the resolved `text/template/exec.go` line numbers, useful when debugging the debugger. |

Use `debug_helm` while reproducing a problem and *before* editing a template,
`_helpers.tpl`, or `values.yaml` — not only at the end to validate a fix. The
MCP server advertises the same workflow in its `instructions`, so clients that
surface server instructions get the guidance without any per-repository config.

### opencode

This repository ships an `opencode.json` that registers the server using the
Docker image. Build the image first, then it is available to opencode in this
project:

```bash
make docker-build
opencode mcp list
```

`debug_helm` accepts filters analogous to the CLI flags. `chart` is a name or
path relative to `working_dir`, or pass `chart_path` to point directly at a
chart directory:

```json
{
  "chart_path": "test",
  "extra_args": ["--show-only", "templates/deployment.yaml"],
  "values": ["image.tag"],
  "helpers": ["test.serviceAccountName"],
  "templates": ["test/templates/deployment.yaml:42"],
  "rendered": ["test/templates/deployment.yaml:32"],
  "mode": "locate"
}
```

- `values` answers "why isn't this option taking effect?" with the exact
  template/helper `file:line` that reads it. The read sites are returned first
  and unrelated helper frames are omitted, so a broad flow does not bury the
  line you need to change.
- `rendered` is the "I see this wrong output — where does it come from?"
  selector. A `file:line` value matches the source that wrote that rendered
  line; any other string is treated as a **substring of the rendered output**, so
  you can paste the bad line (for example `username: ""`) without knowing the
  source file.
- `mode` defaults to `locate`, which returns compact source sites and referenced
  values as both text and structured fields (`sites`, `relevant_values`). Set
  `mode: "full"` for the complete execution flows with rendered write buffers.
- When a query matches nothing, the response says so and suggests nearby known
  values, helpers, and templates instead of returning a silent empty report.

Chart paths are relative to `/workspace` (the mounted repository). If you run
the server outside Docker, use the local binary instead:

```json
{
  "mcp": {
    "servers": {
      "helm-debugger": {
        "type": "local",
        "command": ["helm-debugger", "--mode", "mcp"],
        "cwd": "."
      }
    }
  }
}
```

## Example output and what it means

Full output:
```
================= HELPERS QUERY =================
test/templates/serviceaccount.yaml:5
      name: {{ include "test.serviceAccountName" . }}
  test/templates/_helpers.tpl:57
    in test.serviceAccountName
      {{- if .Values.serviceAccount.create }}
  test/templates/_helpers.tpl:58
    in test.serviceAccountName
      {{- default (include "test.fullname" .) .Values.serviceAccount.name }}
  test/templates/_helpers.tpl:14
    in test.fullname
      {{- if .Values.fullnameOverride }}
  test/templates/_helpers.tpl:17
    in test.fullname
      {{- $name := default .Chart.Name .Values.nameOverride }}
  test/templates/_helpers.tpl:18
    in test.fullname
      {{- if contains $name .Release.Name }}
  test/templates/_helpers.tpl:21
    in test.fullname
      {{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}

Relevant Values
- serviceAccount.create
- serviceAccount.name
- fullnameOverride
- nameOverride

WriteBuffer
   0  apiVersion: v1
   1  kind: ServiceAccount
   2  metadata:
   3    name: release-name-test
```

### Breakdown

#### Execution flow
```
test/templates/deployment.yaml:30
          serviceAccountName: {{ include "test.serviceAccountName" . }}
  test/templates/_helpers.tpl:57
    in test.serviceAccountName
      {{- if .Values.serviceAccount.create }}
  test/templates/_helpers.tpl:58
    in test.serviceAccountName
      {{- default (include "test.fullname" .) .Values.serviceAccount.name }}
  test/templates/_helpers.tpl:14
    in test.fullname
      {{- if .Values.fullnameOverride }}
  test/templates/_helpers.tpl:17
    in test.fullname
      {{- $name := default .Chart.Name .Values.nameOverride }}
  test/templates/_helpers.tpl:18
    in test.fullname
      {{- if contains $name .Release.Name }}
  test/templates/_helpers.tpl:21
    in test.fullname
      {{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
```

This part is the **Execution flow**. It shows each line that got executed on it's way to being rendered.

#### Relevant Values

```
Relevant Values
- serviceAccount.create
- serviceAccount.name
- fullnameOverride
- nameOverride
```

Any time the execution flow references a `values.yaml` option via the `.Values` keyword, it gets captured here. One day I'd like for the actual values to be shown here too, but for now, it tells the user that you should focus on these `values.yaml` options when debugging the function.


#### WriteBuffer

```
WriteBuffer
   0  apiVersion: v1
   1  kind: ServiceAccount
   2  metadata:
   3    name: release-name-test
```

The write buffer is the rendered output. In golangs text/template library, the write buffer is a string builder that I captured the contents of. In most cases, I try to display a diff of the before/after the execution flow happens, but in this case, it was the first function, so the entire file was added to the buffer at once.

A different WriteBuffer that shows the changes might look like the following:

```diff
WriteBuffer
   0  apiVersion: apps/v1
   1  kind: Deployment
   2  metadata:
   3    name: release-name-test
   4    labels:
   5      helm.sh/chart: test-0.1.0
   6      app.kubernetes.io/name: test
   7      app.kubernetes.io/instance: release-name
   8      app.kubernetes.io/version: "1.16.0"
   9      app.kubernetes.io/managed-by: Helm
  10  spec:
  11    replicas: 1
  12    selector:
  13      matchLabels:
  14        app.kubernetes.io/name: test
  15        app.kubernetes.io/instance: release-name
  16    template:
  17      metadata:
  18        labels:
  19          helm.sh/chart: test-0.1.0
  20          app.kubernetes.io/name: test
  21          app.kubernetes.io/instance: release-name
  22          app.kubernetes.io/version: "1.16.0"
  23          app.kubernetes.io/managed-by: Helm
  24      spec:
  25        serviceAccountName: release-name-test
  26        containers:
  27          - name: test
  28            image: "nginx:1.16.0"
  29            imagePullPolicy: IfNotPresent
+     
+               ports:
+                 - name: http
+                   containerPort: 80
```

The write buffer display might also glitch out a little if there are multiple functions being called on the same line. such as:

```
  24      spec:
  25        serviceAccountName: release-name-test
  26        containers:
  27          - name: test
  28            image: "nginx:1.16.0
+     "
+               imagePullPolicy: IfNotPresent
```

The quote does get rendered correctly, but the print of the write buffer doesn't know that.
