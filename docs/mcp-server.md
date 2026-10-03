# MCP server

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

## `debug_helm` filters

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
the server outside Docker, use the local binary instead.

## AI coding tools

The server is a standard stdio MCP server, so any MCP client can launch it. The
repository ships project-local configs for the common ones, all pointing at the
published Docker image (`jessesimpson/helm-debugger:latest`):

| Tool | Project config | Shape |
| --- | --- | --- |
| [OpenCode](#opencode) | `opencode.json` | `mcp.servers`, array command + `cwd` |
| [Claude Code](#claude-code) | `.mcp.json` | `mcpServers`, string command + args |
| [Cursor](#cursor) | `.cursor/mcp.json` | `mcpServers`, string command + args |
| [VS Code](#vs-code) | `.vscode/mcp.json` | `servers`, `type: stdio` |

The `AGENTS.md` file and the server's own `instructions` (advertised in the MCP
`initialize` response) carry the render → locate → validate workflow for every
client that reads them.

### OpenCode

This repository ships an `opencode.json` that registers the server using the
Docker image from the Docker Hub mirror (the GHCR image works identically).
Build the image first, then it is available to OpenCode in this project:

```bash
make docker-build
opencode mcp list
```

To consume a published version instead of building locally, use the
`install-mcp.sh` helper attached to each release (see
[Releases and upgrading](../releases.md#upgrading)), or copy the pinned
`opencode-ghcr.json` / `opencode-dockerhub.json` from the release assets. The
server config changes only when the release changes how the server is invoked;
the release assets are generated from `packaging/opencode.mcp.json` so that
contract is explicit.

An OpenCode config entry looks like this:

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

### Claude Code

The repository ships a project-scoped `.mcp.json`. Claude Code prompts for
approval before using project-scoped servers, so confirm it once:

```bash
claude mcp list      # should show helm-debugger
```

To pin a released version, run `install-mcp.sh --harness claude` (see
[Releases and upgrading](../releases.md#upgrading)) or copy `claude-ghcr.json` to
`.mcp.json`. The release assets are generated from `packaging/claude.mcp.json`.

### Cursor

The repository ships `.cursor/mcp.json`, using `${workspaceFolder}` for the bind
mount so the chart being debugged is the open project. To pin a released
version, run `install-mcp.sh --harness cursor` or copy `cursor-ghcr.json` over
`.cursor/mcp.json`. The release assets are generated from
`packaging/cursor.mcp.json`.

### VS Code

The repository ships `.vscode/mcp.json` for GitHub Copilot's agent mode. VS Code
calls the entry a *server* and requires `"type": "stdio"`. To pin a released
version, run `install-mcp.sh --harness vscode` or copy `vscode-ghcr.json` over
`.vscode/mcp.json`. The release assets are generated from
`packaging/vscode.mcp.json`.
