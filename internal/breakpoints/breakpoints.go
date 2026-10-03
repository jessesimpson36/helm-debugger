package breakpoints

import (
	"github.com/go-delve/delve/service/api"
	"github.com/jessesimpson36/helm-debugger/internal/frame"
	"github.com/jessesimpson36/helm-debugger/internal/frame/delegate"
)

// The line numbers below are resolved at runtime from the Go standard library
// source that the helm binary under debug was compiled with. See resolve.go.

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
