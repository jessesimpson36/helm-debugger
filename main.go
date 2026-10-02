package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jessesimpson36/helm-debugger/internal/alternativemain/branch"
	"github.com/jessesimpson36/helm-debugger/internal/alternativemain/line"
	"github.com/jessesimpson36/helm-debugger/internal/alternativemain/model"
	"github.com/jessesimpson36/helm-debugger/internal/mcpserver"
	"github.com/jessesimpson36/helm-debugger/internal/settings"
)

func main() {
	cfg := settings.NewSettings()

	var err error
	switch cfg.Mode {
	case "branch":
		err = branch.Main(cfg)
	case "line":
		err = line.Main(cfg)
	case "model":
		err = model.Main(cfg)
	case "mcp":
		// The MCP server runs over stdio and must not write anything but
		// protocol messages to stdout.
		err = mcpserver.Run(context.Background())
	default:
		err = fmt.Errorf("no valid mode provided: %q (use model, branch, line, or mcp)", cfg.Mode)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
