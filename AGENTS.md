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

1. Call `helm_template` first to reproduce the published output.
2. Call `debug_helm` to locate the source. Do this while reproducing a problem
   and *before* editing templates, `_helpers.tpl`, or `values.yaml`, not only at
   the end to validate a fix:
   - For a value that is not taking effect, pass it as a `values` filter (for
     example `values: ["image.tag"]`). The compact default (`mode: "locate"`)
     returns the exact template/helper `file:line` that reads it.
   - To trace a wrong rendered line back to its source, pass a snippet of that
     line as a `rendered` filter (for example `rendered: ["username: \"\""]`).
     A non-`file:line` selector is matched as a substring of the rendered output.
   - Re-run after editing to confirm the flow changed. Use `mode: "full"` when
     you need the complete execution flows and rendered write buffers.
   - If a query matches nothing, read the `suggestions` field and retry rather
     than assuming the tool is broken.
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
