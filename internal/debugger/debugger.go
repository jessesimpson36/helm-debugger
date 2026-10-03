// Package debugger contains the orchestration shared by the CLI modes and the
// MCP server: it starts a headless delve session, installs breakpoints in the
// Go text/template package, and collects the resulting execution flows.
package debugger

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/go-delve/delve/service/api"
	"github.com/jessesimpson36/helm-debugger/internal/breakpointevent"
	"github.com/jessesimpson36/helm-debugger/internal/breakpoints"
	"github.com/jessesimpson36/helm-debugger/internal/dlvcontroller"
	"github.com/jessesimpson36/helm-debugger/internal/executionflow"
	"github.com/jessesimpson36/helm-debugger/internal/frame"
	"github.com/jessesimpson36/helm-debugger/internal/frame/delegate"
	"github.com/jessesimpson36/helm-debugger/internal/includegraph"
	"github.com/jessesimpson36/helm-debugger/internal/prepass"
	"github.com/jessesimpson36/helm-debugger/internal/settings"
)

// maxSourceWarnings caps how many distinct source-resolution warnings are
// reported, so a chart with many vendored or packaged templates does not flood
// the report.
const maxSourceWarnings = 10

// Result is the outcome of a single debug run.
type Result struct {
	// Flows are all execution flows captured during the run, before any query
	// filtering is applied.
	Flows []*executionflow.ExecutionFlow
	// LineNumbers are the resolved text/template breakpoint locations.
	LineNumbers breakpoints.LineNumbers
	// Warnings describe template sources that could not be resolved to files.
	// They explain a partial or empty flow set.
	Warnings []string
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

	runPlan := planFrames(ctx, cfg, lines, log)

	if err := session.Configure(runPlan.frames); err != nil {
		return nil, err
	}

	// Gate the per-node walk breakpoint to the queried helpers' subtree. It
	// starts disabled and is toggled at runtime, so nodes outside the subtree do
	// not stop the debugger.
	var gate *walkGate
	if len(runPlan.walkNames) > 0 {
		gate = newWalkGate(session.Client, runPlan.walkBP, runPlan.walkNames)
		if err := gate.Begin(); err != nil {
			if log != nil {
				fmt.Fprintf(log, "warning: walk gating disabled: %v\n", err)
			}
			gate = nil
		}
	}

	// Value resolution on this branch captures the value at the point the
	// template engine computed it (evalField's map return) instead of
	// materializing the whole template data at every walk node.
	var capturer *fieldCapturer
	if cfg.ResolveValues {
		bp := &api.Breakpoint{
			Name: evalFieldBreakpointName,
			File: "text/template/exec.go",
			Line: lines.EvalFieldReturn,
			Cond: evalFieldCond,
		}
		if _, err := session.Client.CreateBreakpoint(bp); err != nil {
			if log != nil {
				fmt.Fprintf(log, "warning: value capture disabled: %v\n", err)
			}
		} else {
			capturer = newFieldCapturer(session.Client)
		}
	}

	if _, err := session.Client.Restart(false); err != nil {
		return nil, fmt.Errorf("restarting debuggee: %w", err)
	}

	state, err := session.Client.GetState()
	if err != nil {
		return nil, fmt.Errorf("reading debugger state: %w", err)
	}

	var events []*frame.BindResult
	var captures []*fieldCapture
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

		if capturer != nil && breakpointName(state) == evalFieldBreakpointName {
			capture, captureErr := capturer.Capture(session.Client)
			if captureErr != nil {
				// A failed capture is not fatal; keep the rest of the run.
			} else if capture != nil {
				captures = append(captures, capture)
			}
			state = <-session.Client.Continue()
			continue
		}

		currentFrame := frameForBreakpoint(runPlan.frames, state)
		if currentFrame == nil {
			state = <-session.Client.Continue()
			continue
		}

		// A relevant template/helper is starting; enable the walk breakpoint so
		// its nodes are captured. The walk breakpoint cannot enable itself: when
		// it is disabled there are no walk stops, so the Execute trigger is the
		// only place the gate can turn it on.
		if gate != nil && breakpointName(state) == "templateexecute" {
			gate.OnEnter(session.Client)
		}
		if gate != nil && breakpointName(state) == "linestart" {
			gate.AtWalk()
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

	if len(captures) > 0 {
		attachFieldCaptures(events, captures)
	}

	return &Result{
		Flows:       breakpointevent.Process(events),
		LineNumbers: lines,
		Warnings:    collectSourceWarnings(events),
	}, nil
}

// plan is the breakpoint configuration for a run.
type plan struct {
	frames []*delegate.DelegateFrame
	// walkBP is the per-node walk breakpoint, when installed. The gate toggles
	// it at runtime.
	walkBP *api.Breakpoint
	// walkNames is the set of template names whose subtree should be walked. It
	// is empty when the walk breakpoint should stay unconditionally enabled.
	walkNames []string
}

// planFrames builds the breakpoint frames for a run. It always includes the walk
// and rendered-manifest frames. When the caller names helpers/templates it adds
// a conditional Execute breakpoint and returns the include-graph closure of the
// named helpers, so the caller can gate the walk breakpoint to just that
// subtree: full per-line detail where it matters, invocation-only elsewhere.
func planFrames(ctx context.Context, cfg *settings.Settings, lines breakpoints.LineNumbers, log io.Writer) plan {
	walk := breakpoints.GetLineStartFrame(lines)
	rendered := breakpoints.GetRenderedManifestFrame(lines)
	for _, f := range []*delegate.DelegateFrame{walk, rendered} {
		f.ChartPath = cfg.ChartDirectory()
	}
	base := plan{frames: []*delegate.DelegateFrame{walk, rendered}, walkBP: walk.Breakpoints[0]}

	scopedNames := cfg.ScopedTemplateNames()
	if len(scopedNames) == 0 {
		return base
	}

	// Anchors: the chart's rendered templates, discovered by a cheap pre-pass.
	// They are needed so flows are tied to what actually rendered.
	renderedNames, prepassErr := prepass.RenderedTemplates(ctx, cfg)
	if prepassErr != nil {
		if log != nil {
			fmt.Fprintf(log, "warning: pre-pass failed, using per-node capture: %v\n", prepassErr)
		}
		return base
	}

	// The subtree to walk: the queried names plus everything they call. When a
	// queried helper reaches a dynamic include the closure is unbounded, so the
	// gate falls back to walking unconditionally rather than dropping lines.
	walkNames, unbounded := includegraph.Names(cfg.ChartDirectory(), scopedNames)
	if unbounded {
		if log != nil {
			fmt.Fprintf(log, "warning: a queried helper has an unresolved include; walking all nodes\n")
		}
		walkNames = nil
	}

	triggerNames := append(append([]string(nil), renderedNames...), walkNames...)
	triggerNames = append(triggerNames, scopedNames...)
	scoped := breakpoints.GetTemplateExecuteFrame(lines, breakpoints.TemplateExecuteCond(dedupe(triggerNames)))
	scoped.ChartPath = cfg.ChartDirectory()

	return plan{
		frames:    []*delegate.DelegateFrame{walk, rendered, scoped},
		walkBP:    walk.Breakpoints[0],
		walkNames: walkNames,
	}
}

// dedupe removes empty and repeated names while preserving order.
func dedupe(names []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(names))
	for _, name := range names {
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

// breakpointName returns the name of the breakpoint the debugger is stopped at,
// or "" when it is not stopped at a named breakpoint.
func breakpointName(state *api.DebuggerState) string {
	if state == nil || state.CurrentThread == nil || state.CurrentThread.Breakpoint == nil {
		return ""
	}
	return state.CurrentThread.Breakpoint.Name
}

// collectSourceWarnings reports, once per template name, the sources that could
// not be resolved to a file during the run. The execution flows are still
// returned; the warnings explain any missing line content or empty flow set.
func collectSourceWarnings(events []*frame.BindResult) []string {
	reasons := map[string]string{}
	for _, event := range events {
		if event == nil || event.ExecutionUnit == nil || event.ExecutionUnit.SourceError == "" {
			continue
		}
		unit := event.ExecutionUnit
		if _, seen := reasons[unit.FileName]; !seen {
			reasons[unit.FileName] = unit.SourceError
		}
	}
	if len(reasons) == 0 {
		return nil
	}

	files := make([]string, 0, len(reasons))
	for file := range reasons {
		files = append(files, file)
	}
	sort.Strings(files)

	var warnings []string
	for i, file := range files {
		if i == maxSourceWarnings {
			warnings = append(warnings, fmt.Sprintf(
				"... and %d more template sources could not be resolved", len(files)-maxSourceWarnings))
			break
		}
		warnings = append(warnings, fmt.Sprintf("%s: %s", file, reasons[file]))
	}
	return warnings
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
