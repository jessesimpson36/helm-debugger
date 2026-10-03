#!/usr/bin/env bash
#
# Add or update the helm-debugger MCP server in an OpenCode configuration.
#
# This is the upgrade path for the MCP server: run it again with a newer tag
# and it rewrites only the `helm-debugger` entry, leaving other servers and
# settings untouched.
set -euo pipefail

DEFAULT_REPO="ghcr.io/jessesimpson36/helm-debugger"
SERVER_NAME="helm-debugger"

usage() {
  cat <<'USAGE'
Add or update the helm-debugger MCP server in an OpenCode configuration.

Usage:
  install-mcp.sh [TAG] [options]

Arguments:
  TAG                 Release tag to pin, e.g. v0.1.0, v0, latest.
                      Default: latest. A value containing "/" is treated as a
                      full image reference and used as-is.

Options:
  --config PATH       OpenCode config file to edit.
                      Default: ./.opencode/opencode.json (project-local).
  --global            Edit the global config (~/.config/opencode/opencode.json).
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

while [ $# -gt 0 ]; do
  case "$1" in
    -h | --help)
      usage
      exit 0
      ;;
    --config)
      config="${2:?--config requires a path}"
      shift 2
      ;;
    --global)
      config="${XDG_CONFIG_HOME:-$HOME/.config}/opencode/opencode.json"
      shift
      ;;
    --registry)
      repo="${2:?--registry requires a name}"
      shift 2
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

if [ "$repo" = "dockerhub" ]; then
  repo="jessesimpson/helm-debugger"
fi

case "$tag" in
  */*) image="$tag" ;;
  *) image="${repo}:${tag}" ;;
esac

if [ -z "$config" ]; then
  config="$PWD/.opencode/opencode.json"
fi

server_json=$(cat <<JSON
{
  "type": "local",
  "command": [
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
  ],
  "cwd": "."
}
JSON
)

if ! command -v jq >/dev/null 2>&1; then
  if $dry_run && [ ! -f "$config" ]; then
    cat <<JSON
{
  "\$schema": "https://opencode.ai/config.json",
  "mcp": { "servers": { "$SERVER_NAME": $server_json } }
}
JSON
    exit 0
  fi
  echo "jq is required to merge into $config." >&2
  echo "Merge this entry under mcp.servers.$SERVER_NAME by hand:" >&2
  printf '%s\n' "$server_json" >&2
  exit 1
fi

if [ -f "$config" ]; then
  if ! base=$(jq -e '.' "$config" 2>/dev/null); then
    echo "error: $config is not valid JSON (comments/JSONC are not supported by this script)" >&2
    exit 1
  fi
else
  base='{"$schema":"https://opencode.ai/config.json"}'
fi

merged=$(printf '%s' "$base" | jq --argjson server "$server_json" --arg name "$SERVER_NAME" \
  '.mcp //= {} | .mcp.servers //= {} | .mcp.servers[$name] = $server')

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
echo "Reconnect the server in OpenCode (/mcps -> $SERVER_NAME, or 'opencode service restart')."
