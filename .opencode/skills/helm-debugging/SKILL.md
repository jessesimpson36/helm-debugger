---
name: Helm debugging
description: Debug or explain a Helm chart end-to-end with the helm-debugger MCP server. Use for any Helm rendering, template, helper, values, or "why isn't this value applied?" question. Renders the chart and traces values.yaml through helpers into the manifest instead of guessing. Use it while reproducing a problem and before editing a template, _helpers.tpl, or values.yaml — not only to validate a fix.
---

# Helm debugging workflow

Use the `helm-debugger` MCP server for every Helm render/troubleshoot task.
Never answer a question about what a chart renders from intuition alone, and do
not grep templates by hand to find a value — ask the tools.

Use it in the phase where it pays off:

- **Reproduce** (`helm_template`): see the actual, possibly wrong output.
- **Locate** (`debug_helm`): find the exact template/helper `file:line` behind a
  value or a rendered line, *before* editing anything.
- **Validate** (`debug_helm` again): confirm the flow changed after the edit.

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

## 2. Locate the source (do this before editing)

Call `debug_helm` with the filter that matches what you know. This is the step
that replaces grepping helpers and templates.

- `values`: `values.yaml` option paths, e.g. `["image.tag"]`. Use this for
  "why isn't my value taking effect?"; the response names the template/helper
  `file:line` that reads the option.
- `rendered`: trace an observed line back to its source. A `file:line` value
  matches the source that wrote that rendered line; **any other string is
  matched as a substring of the rendered output**, so you can paste the wrong
  line (e.g. `["username: \"\""]`) without knowing the source file.
- `helpers`: helper/template names, e.g. `["mychart.fullname"]`.
- `templates`: `file:line`, e.g. `["mychart/templates/deployment.yaml:42"]`.

Example for "why doesn't setting the image tag work?":

```json
{
  "chart_path": "mychart",
  "values": ["image.tag"],
  "extra_args": ["--show-only", "templates/deployment.yaml"]
}
```

`mode` defaults to `locate`, which returns compact source sites (text plus the
structured `sites` and `relevant_values` fields) and omits the rendered write
buffers, so it is cheap to run first. Use `mode: "full"` only when you need the
complete execution flows, and only after you have narrowed the chart with
`extra_args` (for example `--show-only`).

If a query matches nothing, read the `suggestions` field — the tool lists nearby
values, helpers, and templates to retry with. Do not conclude the tool is broken
from an empty result.

## 3. Read the report

In the default `locate` mode each source site is a `file:line`, a source snippet,
and `in <helper>` when the line is inside a helper. `Relevant Values` lists the
`.Values.*` options the matched flows reference.

In `mode: "full"` each execution flow also contains:

- **Execution flow**: the template line and each helper line it called, in
  order. `in <helper>` marks a helper frame.
- **WriteBuffer**: the rendered output. The first buffer is the full buffer at
  the start of the flow; later entries are shown as `+` diffs.

A `debug_helm` result also returns `line_numbers` (the resolved
`text/template/exec.go` breakpoints) and `flow_count`.

## 4. Answer from evidence

Explain the value's path through the helpers and quote the rendered line(s) that
prove it. When the question is "which file do I change", report the `file:line`
from `sites` and edit that line. If the value is overridden, missing, or
shadowed, say where in the flow that happens.
