package breakpoints

import (
	"github.com/go-delve/delve/service/api"
	"github.com/jessesimpson36/helm-debugger/internal/frame"
	"github.com/jessesimpson36/helm-debugger/internal/frame/delegate"
)

// The line numbers below are resolved at runtime from the Go standard library
// source that the helm binary under debug was compiled with. See resolve.go.
//
// display -a pipe.tr.ParseName
// display -a pipe.tr.Name
// display -a pipe.Line
//
// # if / else query
// break text/template/exec.go:<ConditionalStart>
//
// # true
// break text/template/exec.go:<ConditionalTrue>
//
// # false
// break text/template/exec.go:<ConditionalFalse>

func GetConditionalFrame(lines LineNumbers) *delegate.DelegateFrame {
	condStartRequestedBreakpoint := &api.Breakpoint{
		Name: "conditionalstart",
		File: "text/template/exec.go",
		Line: lines.ConditionalStart,
	}
	condTrueRequestedBreakpoint := &api.Breakpoint{
		Name: "conditionalevaluatedtrue",
		File: "text/template/exec.go",
		Line: lines.ConditionalTrue,
	}
	condFalseRequestedBreakpoint := &api.Breakpoint{
		Name: "conditionalevaluatedfalse",
		File: "text/template/exec.go",
		Line: lines.ConditionalFalse,
	}

	breakpoints := []*api.Breakpoint{
		condStartRequestedBreakpoint,
		condTrueRequestedBreakpoint,
		condFalseRequestedBreakpoint,
	}

	reqVars := []string{
		"pipe.tr.ParseName",
		"pipe.tr.Name",
		"pipe.Line",
	}

	mapper := frame.Mapper{
		"FunctionName": "pipe.tr.Name",
		"LineNumber":   "pipe.Line",
		"FileName":     "pipe.tr.ParseName",
	}

	frame := &delegate.DelegateFrame{
		Breakpoints: breakpoints,
		ReqVars:     reqVars,
		Mapper:      mapper,
	}

	return frame
}

func GetLineStartFrame(lines LineNumbers) *delegate.DelegateFrame {
	lineStartBreakpoint := &api.Breakpoint{
		Name: "linestart",
		File: "text/template/exec.go",
		Line: lines.LineStart,
	}
	breakpoints := []*api.Breakpoint{
		lineStartBreakpoint,
	}

	reqVars := []string{
		"node.Pipe.tr.ParseName",
		"node.Pipe.tr.Name",
		"node.Pipe.Line",
	}

	mapper := frame.Mapper{
		"FunctionName": "node.Pipe.tr.Name",
		"LineNumber":   "node.Pipe.Line",
		"FileName":     "node.Pipe.tr.ParseName",
	}

	frame := &delegate.DelegateFrame{
		Breakpoints: breakpoints,
		ReqVars:     reqVars,
		Mapper:      mapper,
	}

	return frame
}

func GetRenderedManifestFrame(lines LineNumbers) *delegate.DelegateFrame {
	renderedManifestBreakpoint := &api.Breakpoint{
		Name: "renderedmanifest",
		File: "text/template/exec.go",
		Line: lines.RenderedManifest,
	}
	breakpoints := []*api.Breakpoint{
		renderedManifestBreakpoint,
	}

	reqVars := []string{
		"string(s.wr.buf)",
		//"node.Pipe.Line",
		//"node.Pipe.tr.ParseName",
	}

	mapper := frame.Mapper{
		"Content": "string(s.wr.buf)",
		//"CharPosition": "node.Pos",
		//"FileName":     "node.tr.ParseName",
	}

	frame := &delegate.DelegateFrame{
		Breakpoints: breakpoints,
		ReqVars:     reqVars,
		Mapper:      mapper,
	}

	return frame
}
