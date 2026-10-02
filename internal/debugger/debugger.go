// Package debugger contains the orchestration shared by the CLI modes and the
// MCP server: it starts a headless delve session, installs breakpoints in the
// Go text/template package, and collects the resulting execution flows.
package debugger

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/go-delve/delve/service/api"
	"github.com/jessesimpson36/helm-debugger/internal/breakpointevent"
	"github.com/jessesimpson36/helm-debugger/internal/breakpoints"
	"github.com/jessesimpson36/helm-debugger/internal/dlvcontroller"
	"github.com/jessesimpson36/helm-debugger/internal/executionflow"
	"github.com/jessesimpson36/helm-debugger/internal/frame"
	"github.com/jessesimpson36/helm-debugger/internal/frame/delegate"
	"github.com/jessesimpson36/helm-debugger/internal/settings"
)

// Result is the outcome of a single debug run.
type Result struct {
	// Flows are all execution flows captured during the run, before any query
	// filtering is applied.
	Flows []*executionflow.ExecutionFlow
	// LineNumbers are the resolved text/template breakpoint locations.
	LineNumbers breakpoints.LineNumbers
}

// Run debugs the helm chart described by cfg and returns every captured
// execution flow.
func Run(ctx context.Context, cfg *settings.Settings, log io.Writer) (*Result, error) {
	if cfg == nil {
		return nil, fmt.Errorf("settings are required")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	lines, err := breakpoints.Resolve(cfg.GoRoot)
	if err != nil {
		if log != nil {
			fmt.Fprintf(log, "warning: %v; using built-in line numbers\n", err)
		}
		lines = breakpoints.FallbackLines
	}

	session, err := dlvcontroller.Start(ctx, cfg, log)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := session.Close(); cerr != nil && log != nil {
			fmt.Fprintf(log, "warning: closing delve session: %v\n", cerr)
		}
	}()

	frames := []*delegate.DelegateFrame{
		breakpoints.GetLineStartFrame(lines),
		breakpoints.GetRenderedManifestFrame(lines),
	}
	for _, f := range frames {
		f.WorkingDir = cfg.EffectiveWorkingDir()
	}

	if err := session.Configure(frames); err != nil {
		return nil, err
	}

	if _, err := session.Client.Restart(false); err != nil {
		return nil, fmt.Errorf("restarting debuggee: %w", err)
	}

	state, err := session.Client.GetState()
	if err != nil {
		return nil, fmt.Errorf("reading debugger state: %w", err)
	}

	var events []*frame.BindResult
	for {
		if state == nil || state.Exited {
			break
		}
		if state.Running {
			time.Sleep(200 * time.Millisecond)
			state, err = session.Client.GetState()
			if err != nil {
				return nil, fmt.Errorf("reading debugger state: %w", err)
			}
			continue
		}

		currentFrame := frameForBreakpoint(frames, state)
		if currentFrame == nil {
			state = <-session.Client.Continue()
			continue
		}

		respVars, gatherErr := currentFrame.Gather(session.Client)
		if gatherErr != nil {
			// Expected for nodes that do not expose the requested fields; skip.
		} else if event, bindErr := currentFrame.Bind(respVars); bindErr != nil {
			// Expected for nodes that are not actions/conditionals; skip.
		} else if event != nil {
			events = append(events, event)
		}

		state = <-session.Client.Continue()
	}

	return &Result{
		Flows:       breakpointevent.Process(events),
		LineNumbers: lines,
	}, nil
}

// frameForBreakpoint returns the frame whose breakpoint the debugger is
// currently stopped on, or nil.
func frameForBreakpoint(frames []*delegate.DelegateFrame, state *api.DebuggerState) *delegate.DelegateFrame {
	if state == nil || state.CurrentThread == nil || state.CurrentThread.Breakpoint == nil {
		return nil
	}
	name := state.CurrentThread.Breakpoint.Name
	for _, f := range frames {
		for _, bp := range f.Breakpoints {
			if bp.Name == name {
				return f
			}
		}
	}
	return nil
}
