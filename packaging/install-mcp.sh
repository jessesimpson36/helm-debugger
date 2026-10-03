#!/usr/bin/env bash
#
# Add or update the helm-debugger MCP server in an AI coding tool's
# configuration, for OpenCode, Claude Code, Cursor, or VS Code.
#
# This is the upgrade path for the MCP server: run it again with a newer tag
# and it rewrites only the `helm-debugger` entry, leaving other servers and
# settings untouched.
set -euo pipefail

DEFAULT_REPO="ghcr.io/jessesimpson36/helm-debugger"
SERVER_NAME="helm-debugger"
HARNESS="opencode"

usage() {
  cat <<'USAGE'
Add or update the helm-debugger MCP server in an AI coding tool's config.

Usage:
  install-mcp.sh [TAG] [options]

Arguments:
  TAG                 Release tag to pin, e.g. v0.1.0, v0, latest.
                      Default: latest. A value containing "/" is treated as a
                      full image reference and used as-is.

Options:
  --harness NAME      Which tool's config format to write. One of:
                        opencode (default), claude, cursor, vscode
  --config PATH       Config file to edit. Default depends on --harness:
                        opencode  ./.opencode/opencode.json  (or the global
                                  ~/.config/opencode/opencode.json with --global)
                        claude    ./.mcp.json                (project-scoped)
                        cursor    ./.cursor/mcp.json
                        vscode    ./.vscode/mcp.json
  --global            Write the user-level config instead of the project one
                      (opencode: ~/.config/opencode/opencode.json; claude:
                      ~/.claude.json; cursor: ~/.cursor/mcp.json; vscode is
                      project-only, so this is an error).
  --registry NAME     Image repository to pull from.
                      Default: ghcr.io/jessesimpson36/helm-debugger.
                      Use "dockerhub" for jessesimpson/helm-debugger.
  --dry-run           Print the merged config instead of writing it.
  -h, --help          Show this help.

The config is merged with jq; a .bak backup is written before changes.
USAGE
}

tag="latest"
config=""
repo="$DEFAULT_REPO"
dry_run=false
global=false

while [ $# -gt 0 ]; do
  case "$1" in
    -h | --help)
      usage
      exit 0
      ;;
    --harness)
      harness="${2:?--harness requires a name}"
      shift 2
      ;;
    --config)
      config="${2:?--config requires a path}"
      shift 2
      ;;
    --registry)
      repo="${2:?--registry requires a name}"
      shift 2
      ;;
    --global)
      global=true
      shift
      ;;
    --dry-run)
      dry_run=true
      shift
      ;;
    -*)
      echo "unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
    *)
      tag="$1"
      shift
      ;;
  esac
done

harness="${harness:-$HARNESS}"

if [ "$repo" = "dockerhub" ]; then
  repo="jessesimpson/helm-debugger"
fi

case "$tag" in
  */*) image="$tag" ;;
  *) image="${repo}:${tag}" ;;
esac

# Default config path per harness. `global` picks the user-level file where one
# exists. Each harness stores its config differently, so the default also
# decides the JSON shape below.
config_default() {
  case "$harness" in
    opencode)
      if $global; then
        echo "${XDG_CONFIG_HOME:-$HOME/.config}/opencode/opencode.json"
      else
        echo "$PWD/.opencode/opencode.json"
      fi
      ;;
    claude)
      if $global; then echo "$HOME/.claude.json"; else echo "$PWD/.mcp.json"; fi
      ;;
    cursor)
      if $global; then echo "$HOME/.cursor/mcp.json"; else echo "$PWD/.cursor/mcp.json"; fi
      ;;
    vscode)
      if $global; then
        echo "error: VS Code MCP config is project-scoped (.vscode/mcp.json); --global is not supported" >&2
        exit 2
      fi
      echo "$PWD/.vscode/mcp.json"
      ;;
    *)
      echo "error: unknown harness: $harness (use opencode, claude, cursor, or vscode)" >&2
      exit 2
      ;;
  esac
}

if [ -z "$config" ]; then
  config=$(config_default)
fi

# The docker invocation is identical for every harness; only the JSON shape
# differs. All four run the server over stdio and need ptrace for delve.
docker_args_json=$(cat <<JSON
[
  "docker",
  "run",
  "--rm",
  "-i",
  "--cap-add=SYS_PTRACE",
  "--security-opt",
  "seccomp=unconfined",
  "-v",
  ".:/workspace",
  "-w",
  "/workspace",
  "$image"
]
JSON
)

# opencode takes the whole command as an array plus a "cwd". The other three
# take a string command and an args array, and rely on the process cwd. Cursor
# and VS Code support ${workspaceFolder}, so they use it for the bind mount.
case "$harness" in
  opencode)
    server_json=$(cat <<JSON
{
  "type": "local",
  "command": $docker_args_json,
  "cwd": "."
}
JSON
)
    base_default='{"$schema":"https://opencode.ai/config.json"}'
    merge_filter='.mcp //= {} | .mcp.servers //= {} | .mcp.servers[$name] = $server'
    ;;
  claude)
    server_json=$(cat <<JSON
{
  "command": "docker",
  "args": $docker_args_json
}
JSON
)
    # claude's `args` is the docker args; strip the leading "docker" element.
    server_json=$(printf '%s' "$server_json" | jq -c '.args = (.args[1:])')
    base_default='{}'
    merge_filter='.mcpServers //= {} | .mcpServers[$name] = $server'
    ;;
  cursor)
    server_json=$(jq -cn --arg image "$image" '{command: "docker", args: [
      "run", "--rm", "-i",
      "--cap-add=SYS_PTRACE",
      "--security-opt", "seccomp=unconfined",
      "-v", "${workspaceFolder}:/workspace",
      "-w", "/workspace",
      $image
    ]}')
    base_default='{}'
    merge_filter='.mcpServers //= {} | .mcpServers[$name] = $server'
    ;;
  vscode)
    server_json=$(jq -cn --arg image "$image" '{type: "stdio", command: "docker", args: [
      "run", "--rm", "-i",
      "--cap-add=SYS_PTRACE",
      "--security-opt", "seccomp=unconfined",
      "-v", "${workspaceFolder}:/workspace",
      "-w", "/workspace",
      $image
    ]}')
    base_default='{}'
    merge_filter='.servers //= {} | .servers[$name] = $server'
    ;;
esac

if ! command -v jq >/dev/null 2>&1; then
  if $dry_run && [ ! -f "$config" ]; then
    if [ "$harness" = "opencode" ]; then
      printf '{\n  "$schema": "https://opencode.ai/config.json",\n  "mcp": { "servers": { "%s": %s } }\n}\n' "$SERVER_NAME" "$server_json"
    elif [ "$harness" = "vscode" ]; then
      printf '{\n  "servers": { "%s": %s }\n}\n' "$SERVER_NAME" "$server_json"
    else
      printf '{\n  "mcpServers": { "%s": %s }\n}\n' "$SERVER_NAME" "$server_json"
    fi
    exit 0
  fi
  echo "jq is required to merge into $config." >&2
  echo "Merge this entry into $config by hand:" >&2
  printf '%s\n' "$server_json" >&2
  exit 1
fi

if [ -f "$config" ]; then
  if ! base=$(jq -e '.' "$config" 2>/dev/null); then
    echo "error: $config is not valid JSON (comments/JSONC are not supported by this script)" >&2
    exit 1
  fi
else
  base="$base_default"
fi

merged=$(printf '%s' "$base" | jq --argjson server "$server_json" --arg name "$SERVER_NAME" \
  "$merge_filter")

if $dry_run; then
  printf '%s\n' "$merged"
  exit 0
fi

mkdir -p "$(dirname "$config")"
if [ -f "$config" ]; then
  cp "$config" "${config}.bak"
fi
printf '%s\n' "$merged" >"$config"

echo "updated $config"
echo "$SERVER_NAME -> $image"

case "$harness" in
  opencode) echo "Reconnect the server in OpenCode (/mcps -> $SERVER_NAME, or 'opencode service restart')." ;;
  claude) echo "Restart Claude Code, then check with '/mcp' (project-scoped servers prompt for approval)." ;;
  cursor) echo "Restart Cursor, then check the MCP panel (Settings -> MCP)." ;;
  vscode) echo "Reload VS Code, then check the MCP servers view or run 'MCP: List Servers'." ;;
esac
