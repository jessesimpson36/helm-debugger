// Package mcpserver exposes the helm debugger as a Model Context Protocol
// server so that AI tooling can verify helm chart rendering and debug it at
// runtime with delve.
package mcpserver

import (
	"context"
	"io"
	"log"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jessesimpson36/helm-debugger/internal/version"
)

// Run starts the MCP server on stdio. It blocks until the client disconnects or
// ctx is cancelled.
//
// All diagnostic logging goes to stderr because stdout carries the MCP
// protocol framing.
func Run(ctx context.Context) error {
	logger := log.New(os.Stderr, "helm-debugger-mcp: ", log.LstdFlags)
	server := NewServer(logger)
	return server.Run(ctx, &mcp.StdioTransport{})
}

// NewServer builds the helm debugger MCP server. It is exported so tests can
// connect to it over an in-memory transport instead of stdio.
func NewServer(logger *log.Logger) *mcp.Server {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	server := mcp.NewServer(&mcp.Implementation{
		Name: "helm-debugger",
		// The release version is injected at build time; clients see it in the
		// MCP initialize response, which is how an upgrade is confirmed.
		Version: version.Version,
	}, &mcp.ServerOptions{
		Instructions: instructions,
	})
	registerTools(server, logger)
	return server
}

const instructions = `Helm debugger MCP server.

Use this server whenever a task involves a Helm chart's rendered output or how a
values.yaml option flows through helpers into the manifests. Do not speculate
about what Helm renders, and do not grep templates by hand to find a value: ask
the tools.

When to use which tool:
- helm_template: reproduce. Call it first to see exactly what the chart renders,
  including wrong or surprising output.
- debug_helm: locate. Call it while reproducing a problem and BEFORE editing
  templates, _helpers.tpl, or values.yaml. Pass the option that is not taking
  effect as a "values" filter to get the exact template/helper file:line that
  reads it. Pass a snippet of the wrong rendered output as a "rendered" filter
  to find the template that wrote it. Re-run after editing to confirm the flow
  changed. It returns compact source sites by default; use mode="full" for the
  complete execution flows and rendered write buffers.
- resolve_breakpoints: only when debugging the debugger itself.

Chart paths are relative to the server's working directory (or the working_dir
argument). The helm binary must be compiled with debug symbols
(-gcflags="all=-N -l") for debug_helm to work; the Docker image in this repo
provides one.`

// newTimeoutContext derives a context with a timeout, defaulting to fallback
// when seconds is not positive.
func newTimeoutContext(ctx context.Context, seconds int, fallback time.Duration) (context.Context, context.CancelFunc) {
	if seconds <= 0 {
		return context.WithTimeout(ctx, fallback)
	}
	return context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
}
