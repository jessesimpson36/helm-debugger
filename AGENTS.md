# Helm debugger

This repository contains the helm-debugger: a CLI and MCP server that renders a
Helm chart and traces how `values.yaml` flows through `_helpers.tpl` into the
rendered manifests.

## Always verify Helm behavior with the helm-debugger MCP tools

Whenever a task involves a Helm chart — rendering it, reading its templates,
`_helpers.tpl`, `values.yaml`, or dependencies, or questions like "why is this
value not being applied?" / "why does this render like this?" — use the
`helm-debugger` MCP server. Do not speculate about what Helm renders.

Workflow:

1. Call `helm_template` first to see the actual rendered manifests.
2. Call `debug_helm` to trace the execution flow, passing the relevant filters
   (`values`, `helpers`, `templates`, `rendered`). For a question about a
   specific option, pass it as a `values` filter (for example
   `values: ["image.tag"]`).
3. Point at the chart with `chart_path` (for example `chart_path: "test"`), or
   with `chart` + `working_dir`.
4. Answer from the tool output, quoting the relevant flow and rendered lines.
   If the tools disagree with your expectations, trust the tools.

Only use `resolve_breakpoints` when debugging the debugger itself.

The MCP server runs the published Docker image
`jessesimpson/helm-debugger:latest`, so the result does not depend on whatever
Helm/Delve/Go is installed on the host. If a tool reports that the image is
missing, build and push it from this repository with `make docker-push` (or
`make docker-build` for a local-only image), then reconnect the server.

See the `helm-debugging` skill for the full workflow and how to read the output.
