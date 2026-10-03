# Releasing

A release is produced by `.github/workflows/release.yml`. It builds and pushes
the Docker image, generates SBOMs and build metadata, and publishes everything
as a GitHub release. Tagging is the normal way to cut one.

## What a release contains

| Asset | Purpose |
| --- | --- |
| Docker image `vX.Y.Z`, `X.Y.Z`, `X.Y`, `X`, `latest` | The MCP server / CLI runtime. |
| Docker image immutable toolchain tag | e.g. `go1.26.7-helm4.3.0-delve1.27.2`; pins exactly what is inside. |
| `helm-debugger-vX.Y.Z-source.spdx.json` / `.cdx.json` | SBOM of the source tree and Go modules (SPDX and CycloneDX). |
| `helm-debugger-vX.Y.Z-image.spdx.json` / `.cdx.json` | SBOM of the container image (SPDX and CycloneDX). |
| `metadata.json` | How the release was built: version, commit, build date, Go/Helm/Delve versions, image digest and platforms, SBOM filenames. |
| `opencode-ghcr.json`, `opencode-dockerhub.json` | Canonical OpenCode MCP config pinned to this version (Docker Hub variant only when Docker Hub is configured). |
| `install-mcp.sh` | Adds/updates the server in an OpenCode config; the upgrade helper. |
| `SHA256SUMS` | Checksums for every asset above. |

In addition, the image carries registry-native attestations:

- a BuildKit **SBOM** attestation and `mode=max` **provenance** attestation
  (`sbom: true`, `provenance: mode=max`), one per platform;
- a GitHub **provenance** attestation pushed to the registry
  (`actions/attest-build-provenance`).

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

A tag containing a hyphen (`v0.2.0-rc.1`) is published as a GitHub
prerelease and does not move `latest`, the major tag, or the minor tag.

## Manual runs

Use **Actions → Release → Run workflow** and provide a version such as
`v0.2.0`. This is meant for re-cutting a release or publishing from a branch;
it creates the tag at the checked-out commit if it does not exist yet.

## Required configuration

- `GITHUB_TOKEN` is automatic. The workflow requests `contents: write`,
  `packages: write`, `id-token: write`, and `attestations: write`.
- Publishing to GitHub Container Registry needs no extra secrets. The first
  push creates the package as **private**; set its visibility to public in the
  package settings (Package settings → Change visibility) so users can pull it.
- Publishing to Docker Hub is optional. Add repository secrets
  `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN`; without them only GHCR is
  published.

## Changing the pinned toolchain

`toolchain.env` is the single source of truth for `GO_VERSION`,
`HELM_VERSION`, and `DELVE_VERSION`. The Makefile includes it, CI sources it,
and the release workflow reads it, so updating that one file changes the CLI
build, the image, and the release metadata together.

When bumping Go, also update the `go` directive in `go.mod`. The `ARG`
defaults in `Dockerfile` and the fallbacks in `docker-compose.yml` mirror
`toolchain.env` for people who build the image without the Makefile; keep them
in step.

## Multi-arch images

`PLATFORMS` in the release workflow defaults to `linux/amd64`. Adding
`linux/arm64` works, but Helm is compiled from source with
`-gcflags="all=-N -l"` under QEMU emulation, which is slow. Change `PLATFORMS`
only if the build stays within the CI budget.

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
