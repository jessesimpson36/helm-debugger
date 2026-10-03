# Releases and upgrading

Every release is built and published by
[`.github/workflows/release.yml`](https://github.com/jessesimpson36/helm-debugger/blob/main/.github/workflows/release.yml);
the process is documented in [Releasing](contributing/releasing.md). A release
contains:

- the container image (the MCP server / CLI runtime) on GHCR and Docker Hub,
  built for `linux/amd64` and `linux/arm64` on native runners;
- an **SBOM** for the source tree and for each platform image, in both SPDX and
  CycloneDX;
- **build metadata** (`metadata.json`) recording the source commit, build date,
  and the pinned Go, Helm, and Delve versions, plus the image digest;
- a version-pinned **MCP config for each supported AI tool** (OpenCode, Claude
  Code, Cursor, VS Code) and the `install-mcp.sh` helper.

The image also carries the same information as OCI labels and as
`/usr/local/share/helm-debugger/version.txt`, and a GitHub provenance
attestation is pushed to the registry for `gh attestation verify`.

## Apple Silicon

The `linux/arm64` image runs natively on M-series Macs. This matters: the
debugger drives Delve, which uses `ptrace`, and `ptrace` does not work under
amd64-on-arm64 emulation (QEMU does not implement it, and Rosetta fails on
register reads). Docker Desktop picks the arm64 image automatically, so no
config change is needed on an Apple Silicon Mac.

## Upgrading

For Docker use, pick a tag and change one line:

```bash
docker pull ghcr.io/jessesimpson36/helm-debugger:v0.2.0
```

| Tag | Behavior |
| --- | --- |
| `vX.Y.Z` | Immutable. Reproducible; upgrade by changing the tag. |
| `X.Y` / `X` | Moving. Receive patch/minor updates within a line. |
| `latest` | Moving. Newest non-prerelease. |
| `goX.Y.Z-helm...-delve...` | Immutable toolchain tag; changes only when the toolchain does. |

An AI coding tool does not update MCP server configuration on its own, so the
config is shipped as a release asset. Each release attaches a version-pinned
config per supported tool plus `install-mcp.sh`, which merges only the
`helm-debugger` server entry into an existing config (leaving other servers and
settings alone):

```bash
bash install-mcp.sh v0.2.0                          # opencode, project-local
bash install-mcp.sh latest --global                 # or track latest globally
bash install-mcp.sh v0.2.0 --harness claude         # Claude Code (.mcp.json)
bash install-mcp.sh v0.2.0 --harness cursor         # Cursor (.cursor/mcp.json)
bash install-mcp.sh v0.2.0 --harness vscode         # VS Code (.vscode/mcp.json)
```

(The download may not keep the executable bit, hence `bash`.) The script needs
`jq` and only rewrites the `helm-debugger` entry. Alternatively, copy the pinned
`opencode-ghcr.json`, `claude-ghcr.json`, `cursor-ghcr.json`, or
`vscode-ghcr.json` from the release assets (Docker Hub variants are attached
when Docker Hub publishing is configured).

After changing the config, reconnect the server (`/mcps` → helm-debugger, or
`opencode service restart`), then confirm the reported version:

```bash
docker run --rm ghcr.io/jessesimpson36/helm-debugger:v0.2.0 --version
```

See [MCP server](mcp-server.md) for how each tool consumes the config.
