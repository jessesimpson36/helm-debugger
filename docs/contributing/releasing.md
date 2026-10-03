# Releasing

A release is produced by
[`.github/workflows/release.yml`](https://github.com/jessesimpson36/helm-debugger/blob/main/.github/workflows/release.yml).
It builds and pushes the Docker image, generates SBOMs and build metadata, and
publishes everything as a GitHub release. Tagging is the normal way to cut one.

## What a release contains

| Asset | Purpose |
| --- | --- |
| Docker image `vX.Y.Z`, `X.Y.Z`, `X.Y`, `X`, `latest` | The MCP server / CLI runtime, multi-arch (`linux/amd64`, `linux/arm64`). |
| Docker image immutable toolchain tag | e.g. `go1.26.7-helm4.3.0-delve1.27.2`; pins exactly what is inside. |
| `helm-debugger-vX.Y.Z-source.spdx.json` / `.cdx.json` | SBOM of the source tree and Go modules (SPDX and CycloneDX). |
| `helm-debugger-vX.Y.Z-image-amd64.spdx.json` / `.cdx.json`, `…-image-arm64.…` | Per-platform SBOM of the container image (SPDX and CycloneDX). |
| `metadata.json` | How the release was built: version, commit, build date, Go/Helm/Delve versions, image digest and platforms, SBOM filenames. |
| `opencode-ghcr.json`, `opencode-dockerhub.json` | Canonical OpenCode MCP config pinned to this version (Docker Hub variant only when Docker Hub is configured). |
| `claude-ghcr.json` / `-dockerhub.json`, `cursor-ghcr.json` / `-dockerhub.json`, `vscode-ghcr.json` / `-dockerhub.json` | The same pinned config for Claude Code (`.mcp.json`), Cursor (`.cursor/mcp.json`), and VS Code (`.vscode/mcp.json`). Generated from `packaging/<tool>.mcp.json`. |
| `install-mcp.sh` | Adds/updates the server in any supported tool's config (`--harness opencode\|claude\|cursor\|vscode`); the upgrade helper. |
| `SHA256SUMS` | Checksums for every asset above. |

In addition, a GitHub **provenance** attestation is generated for the
multi-arch image index and pushed to the registry
(`actions/attest-build-provenance`), so `gh attestation verify` works against
the published image.

## Cutting a release

The version is taken from the git tag; there is no version file to bump. The
value is injected into the binary so `helm-debugger --version` and the MCP
`initialize` response report it.

```sh
git switch main
git pull
git tag -a v0.2.0 -m "v0.2.0"
git push origin v0.2.0
```

A tag containing a hyphen (`v0.2.0-rc.1`) is published as a GitHub prerelease
and does not move `latest`, the major tag, or the minor tag.

## Manual runs

Use **Actions → Release → Run workflow** and provide a version such as
`v0.2.0`. This is meant for re-cutting a release or publishing from a branch; it
creates the tag at the checked-out commit if it does not exist yet.

## Required configuration

- `GITHUB_TOKEN` is automatic. The workflow requests `actions: write` (buildx
  cache), `contents: write`, `packages: write`, `id-token: write`, and
  `attestations: write`.
- Publishing to GitHub Container Registry needs no extra secrets. The first push
  creates the package as **private**; set its visibility to public in the
  package settings (Package settings → Change visibility) so users can pull it.
- Publishing to Docker Hub is optional. Add repository secrets
  `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN`; without them only GHCR is
  published.

## Changing the pinned toolchain

`toolchain.env` is the single source of truth for `GO_VERSION`,
`HELM_VERSION`, and `DELVE_VERSION`. The Makefile includes it, CI sources it,
and the release workflow reads it, so updating that one file changes the CLI
build, the image, and the release metadata together.

When bumping Go, also update the `go` directive in `go.mod`. The `ARG` defaults
in `Dockerfile` and the fallbacks in `docker-compose.yml` mirror
`toolchain.env` for people who build the image without the Makefile; keep them
in step.

## Multi-arch images

Each platform is built on its own **native** runner and pushed by digest, then a
merge job assembles the manifest list:

- `linux/amd64` builds on `ubuntu-latest`;
- `linux/arm64` builds on `ubuntu-24.04-arm` (GitHub's arm64 runner, free for
  public repositories).

Nothing is emulated, so the arm64 image is fast to build and, more importantly,
Delve works in it: `ptrace` is unimplemented under QEMU and broken under
Rosetta, which is why Apple Silicon needs a native arm64 image rather than the
amd64 one.

`PLATFORMS` in the workflow should stay in sync with the image matrix. Adding a
platform means adding a matrix entry with a matching `runner`/`platform`/`arch`.
If the repository is ever made private, the `ubuntu-*-arm` labels stop working
and you would need arm64 larger runners or QEMU.

## Verifying a release

```sh
# Inspect tags and the index digest.
docker buildx imagetools inspect ghcr.io/jessesimpson36/helm-debugger:v0.2.0

# Verify the GitHub provenance attestation.
gh attestation verify oci://ghcr.io/jessesimpson36/helm-debugger:v0.2.0 \
  -R jessesimpson36/helm-debugger

# Check downloaded assets.
sha256sum -c SHA256SUMS
```
