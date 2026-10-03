# Concepts

This page explains the vocabulary the debugger and its output use.

## Breakpoints

This program starts a headless Go debugger in the background and translates
places in memory of that debugger to helm chart logic. In the context of this
program, those places are referred to as **breakpoints**, even though there is
not yet an interactive debugging mode.

The debugger resolves the actual `text/template/exec.go` line numbers at runtime
from `GOROOT` instead of hard-coding them, so it keeps working across Go
toolchain versions.

## Modes

The execution **modes** of this program are alternative main functions used to
explore which output is most useful. The less useful ones may be removed.

- **model**: Builds a complete data structure representing all execution flows
  within the chart templates and helpers, then lets you query which execution
  flows to follow.
- **branch**: The first mode built; it only captures `if`/`else` conditions and
  whether they evaluate to true or false. It is not very useful.
- **line**: Prints out every line as it is processed. This mode is overwhelming
  without being filtered.
- **mcp**: Runs a Model Context Protocol server over stdio so AI tools can
  render and debug charts at runtime. See [MCP server](mcp-server.md).

## Query types

Each time a breakpoint is hit, the program captures the execution path affecting
that line.

- **template file queries** — specify which Go template files to set breakpoints
  on.
- **helper file queries** — specify which helper files to set breakpoints on.
- **rendered file queries** — specify which rendered manifest files to set
  breakpoints on.
- **values queries** — specify which values to capture at each breakpoint or
  after rendering.

## Execution flow

**Execution flow** refers to the linear path the program takes executing
instructions after `if`/`else` conditions are evaluated. It may look a bit like
a stacktrace, but it is not one. See [Understanding output](output.md) for how a
flow is presented.

## Helm chart names vs paths

Helm names templates after the chart's `Chart.yaml` name, not after the
directory the chart lives in. For a chart in a versioned directory (for example
`example-platform-8.9/` for chart name `example-platform`) the names reported at
runtime are `example-platform/templates/...`. The debugger resolves those names
back to the chart directory it was pointed at, so the directory and the chart
name are allowed to differ.

The command still has to point at a local chart folder. Pointing `--chart` at a
`.tgz` archive or a registry reference is not supported. Dependencies vendored
as `.tgz` archives *are* supported: template names inside them are resolved and
read directly from the archive (as are decompressed subcharts under `charts/`).
When a template source cannot be resolved, the debugger keeps the execution flow
and emits a warning instead of dropping it. Warnings appear in a `WARNINGS`
report section and in the `warnings` field of the MCP response, so an empty or
partial flow set explains itself.
