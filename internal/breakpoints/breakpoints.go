package breakpoints

import (
	"github.com/go-delve/delve/service/api"
)

func GetConditionalBreakpoints() []*api.Breakpoint {
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
	return breakpoints
}

func GetLineStartBreakpoints() []*api.Breakpoint {
	lineStartBreakpoint := &api.Breakpoint{
		Name: "linestart",
		File: "text/template/exec.go",
		Line: 263, // TODO: figure out what line number
	}
	breakpoints := []*api.Breakpoint{
		lineStartBreakpoint,
	}
	return breakpoints

}
