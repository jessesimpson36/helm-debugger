// Package version carries build and toolchain metadata for the helm debugger.
//
// Version, Commit, BuildDate, HelmVersion and DelveVersion are zero values in a
// plain `go build` and are overridden at release time with -ldflags -X (see the
// Makefile). The Go version is read from the running toolchain, so it always
// matches the compiler that produced the binary.
package version

import (
	"encoding/json"
	"fmt"
	"runtime"
)

// Build-time variables. Override them with, for example:
//
//	go build -ldflags "\
//	  -X github.com/jessesimpson36/helm-debugger/internal/version.Version=v0.1.0 \
//	  -X github.com/jessesimpson36/helm-debugger/internal/version.Commit=$(git rev-parse HEAD)"
var (
	Version      = "dev"
	Commit       = "unknown"
	BuildDate    = "unknown"
	HelmVersion  = "unknown"
	DelveVersion = "unknown"
)

// Info is the machine-readable build and toolchain metadata for a build.
type Info struct {
	Version      string `json:"version"`
	Commit       string `json:"commit"`
	BuildDate    string `json:"build_date"`
	GoVersion    string `json:"go_version"`
	HelmVersion  string `json:"helm_version"`
	DelveVersion string `json:"delve_version"`
	Platform     string `json:"platform"`
}

// Get returns metadata for the running binary.
func Get() Info {
	return Info{
		Version:      Version,
		Commit:       Commit,
		BuildDate:    BuildDate,
		GoVersion:    runtime.Version(),
		HelmVersion:  HelmVersion,
		DelveVersion: DelveVersion,
		Platform:     runtime.GOOS + "/" + runtime.GOARCH,
	}
}

// JSON returns the metadata encoded as indented JSON.
func (i Info) JSON() (string, error) {
	b, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// String renders the human-readable `helm-debugger --version` output.
func (i Info) String() string {
	return fmt.Sprintf(`helm-debugger %s
  commit:   %s
  built:    %s
  go:       %s
  helm:     %s
  delve:    %s
  platform: %s`,
		i.Version, i.Commit, i.BuildDate, i.GoVersion, i.HelmVersion, i.DelveVersion, i.Platform)
}
