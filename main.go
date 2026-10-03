package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jessesimpson36/helm-debugger/internal/alternativemain/model"
	"github.com/jessesimpson36/helm-debugger/internal/mcpserver"
	"github.com/jessesimpson36/helm-debugger/internal/settings"
	"github.com/jessesimpson36/helm-debugger/internal/version"
)

func main() {
	cfg := settings.NewSettings()

	// --version is metadata only: print it and exit before any mode runs.
	if cfg.ShowVersion {
		fmt.Println(version.Get().String())
		return
	}

	var err error
	switch cfg.Mode {
	case "model":
		err = model.Main(cfg)
	case "mcp":
		// The MCP server runs over stdio and must not write anything but
		// protocol messages to stdout.
		err = mcpserver.Run(context.Background())
	default:
		err = fmt.Errorf("no valid mode provided: %q (use model or mcp)", cfg.Mode)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
