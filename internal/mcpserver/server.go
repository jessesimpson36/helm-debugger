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
)

// Version is reported in the MCP server implementation metadata.
const Version = "0.1.0"

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
		Name:    "helm-debugger",
		Version: Version,
	}, &mcp.ServerOptions{
		Instructions: instructions,
	})
	registerTools(server, logger)
	return server
}

const instructions = `Helm debugger MCP server.

Use this server when you need to verify what a Helm chart actually renders or
why a helper template produced a particular value.

Tools:
- helm_template: render a chart with the helm CLI (fast sanity check).
- debug_helm: run the chart under a delve debugger and report the template and
  helper execution flows that produced each rendered manifest, including the
  values.yaml options involved.
- resolve_breakpoints: report the Go standard library line numbers the debugger
  will use, which is useful when debugging the debugger itself.

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
