#!/usr/bin/env bash
#
# Emit the release metadata JSON on stdout.
#
# CI runs this in the release job (after the image is pushed) and attaches the
# output to the GitHub release. It records how the release was built: the
# source revision, the pinned Go/Helm/Delve toolchain, and the published image.
#
# Required: VERSION
# Optional: COMMIT, BUILD_DATE, GO_VERSION, HELM_VERSION, DELVE_VERSION,
#           IMAGE, IMAGE_DIGEST, PLATFORMS, SBOM_SOURCE, SBOM_IMAGE
set -euo pipefail

json_escape() {
  # Escape backslashes and double quotes for embedding in a JSON string.
  printf '%s' "${1:-}" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g'
}

# json_array turns a comma-separated list into a JSON array of strings.
json_array() {
  local first=1 item
  printf '['
  while IFS= read -r item; do
    if [ -n "$item" ]; then
      if [ "$first" -eq 0 ]; then
        printf ', '
      fi
      printf '"%s"' "$(json_escape "$item")"
      first=0
    fi
  done < <(printf '%s\n' "${1:-}" | tr ',' '\n')
  printf ']'
}

version=${VERSION:?VERSION is required}
commit=${COMMIT:-unknown}
build_date=${BUILD_DATE:-unknown}
go_version=${GO_VERSION:-unknown}
helm_version=${HELM_VERSION:-unknown}
delve_version=${DELVE_VERSION:-unknown}
image=${IMAGE:-}
image_digest=${IMAGE_DIGEST:-}
platforms=${PLATFORMS:-linux/amd64}
sbom_source=${SBOM_SOURCE:-}
sbom_image=${SBOM_IMAGE:-}

# Prefer the full toolchain string when a go binary is on PATH; it records the
# exact patch/patchlevel, not just the pinned major.minor.patch.
go_toolchain="go${go_version}"
if command -v go >/dev/null 2>&1; then
  go_toolchain=$(go version | awk '{print $3}')
fi

cat <<JSON
{
  "name": "helm-debugger",
  "version": "$(json_escape "$version")",
  "commit": "$(json_escape "$commit")",
  "build_date": "$(json_escape "$build_date")",
  "toolchain": {
    "go": "$(json_escape "$go_version")",
    "go_toolchain": "$(json_escape "$go_toolchain")",
    "helm": "$(json_escape "$helm_version")",
    "delve": "$(json_escape "$delve_version")"
  },
  "image": {
    "reference": "$(json_escape "$image")",
    "digest": "$(json_escape "$image_digest")",
    "platforms": "$(json_escape "$platforms")"
  },
  "sbom": {
    "source": $(json_array "$sbom_source"),
    "image": $(json_array "$sbom_image")
  },
  "source": {
    "repository": "https://github.com/jessesimpson36/helm-debugger",
    "commit": "https://github.com/jessesimpson36/helm-debugger/commit/$(json_escape "$commit")"
  }
}
JSON
