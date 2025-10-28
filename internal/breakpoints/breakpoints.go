package breakpoints

import (
	"github.com/jessesimpson36/helm-debugger/internal/frame"
	"github.com/go-delve/delve/service/api"
)


// display -a pipe.tr.ParseName
// display -a pipe.tr.Name
// display -a pipe.Line
// break text/template/exec.go:300
// break text/template/exec.go:307
// break text/template/exec.go:313
//
// # if / else query
// break text/template/exec.go:300
//
// # true
// break text/template/exec.go:307
//
// # false
// break text/template/exec.go:313
//

func GetConditionalFrame() *frame.Frame {
	condStartRequestedBreakpoint := &api.Breakpoint{
		Name: "conditionalstart",
		File: "text/template/exec.go",
		Line: 300,
	}
	condTrueRequestedBreakpoint := &api.Breakpoint{
		Name: "conditionalevaluatedtrue",
		File: "text/template/exec.go",
		Line: 307,
	}
	condFalseRequestedBreakpoint := &api.Breakpoint{
		Name: "conditionalevaluatedfalse",
		File: "text/template/exec.go",
		Line: 313,
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

	frame := &frame.Frame{
		Breakpoints: breakpoints,
		ReqVars:     reqVars,
		Mapper:      mapper,
	}

	return frame
}

func GetLineStartFrame() *frame.Frame {
	lineStartBreakpoint := &api.Breakpoint{
		Name: "linestart",
		File: "text/template/exec.go",
		Line: 263,
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

	frame := &frame.Frame{
		Breakpoints: breakpoints,
		ReqVars:     reqVars,
		Mapper:      mapper,
	}

	return frame
}
