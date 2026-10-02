---
name: Helm debugging
description: Debug or explain a Helm chart end-to-end with the helm-debugger MCP server. Use for any Helm rendering, template, helper, values, or "why isn't this value applied?" question. Renders the chart and traces values.yaml through helpers into the manifest instead of guessing.
---

# Helm debugging workflow

Use the `helm-debugger` MCP server for every Helm render/troubleshoot task.
Never answer a question about what a chart renders from intuition alone.

## Prerequisites

The server runs a pinned Docker image (`jessesimpson/helm-debugger:latest`). If a tool call
reports that the image is missing, build it once from the helm-debugger
repository:

```bash
make docker-build
```

Then reconnect the MCP server in OpenCode (`/mcps` → helm-debugger, or
`opencode service restart`).

## 1. Render first

Call `helm_template` to see the actual output. Prefer `chart_path` for a chart
directory, or `chart` + `working_dir`:

```json
{ "chart_path": "mychart", "extra_args": ["--show-only", "templates/deployment.yaml"] }
```

If rendering fails, fix that before debugging the flow — the error usually
explains the problem.

## 2. Trace the flow

Call `debug_helm` with the filters that match the question. All filters are
lists and are applied as OR-prefix matches against the execution flows:

- `values`: `values.yaml` option paths, e.g. `["image.tag", "image.repository"]`.
  Use this for "why isn't my value taking effect?".
- `helpers`: helper/template names, e.g. `["mychart.fullname"]`.
- `templates`: `file:line`, e.g. `["mychart/templates/deployment.yaml:42"]`.
- `rendered`: `file:line` in the rendered output.

Example for "why doesn't setting the image tag work?":

```json
{
  "chart_path": "mychart",
  "values": ["image.tag"],
  "extra_args": ["--show-only", "templates/deployment.yaml"]
}
```

## 3. Read the report

Each execution flow contains:

- **Execution flow**: the template line and each helper line it called, in
  order. `in <helper>` marks a helper frame.
- **Relevant Values**: the `.Values.*` options referenced by that flow.
- **WriteBuffer**: the rendered output. The first buffer is the full buffer at
  the start of the flow; later entries are shown as `+` diffs.

A `debug_helm` result also returns `line_numbers` (the resolved
`text/template/exec.go` breakpoints) and `flow_count`.

## 4. Answer from evidence

Explain the value's path through the helpers and quote the rendered line(s) that
prove it. If the value is overridden, missing, or shadowed, say where in the
flow that happens.
