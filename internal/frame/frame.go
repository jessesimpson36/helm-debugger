package frame

import (
	"fmt"
	"io"

	"github.com/go-delve/delve/service/api"
	"github.com/go-delve/delve/service/rpc2"
	"github.com/jessesimpson36/helm-debugger/internal/templatevalues"
)

// A frame represents a breakpoint and a set of variables you want displayed at that frame
type Frame struct {
	Breakpoints []*api.Breakpoint
	ReqVars     []string
	Mapper      Mapper
	// ChartPath is the path to the helm chart directory being debugged. It is
	// used to resolve runtime template names (which helm prefixes with the
	// Chart.yaml name) back to files on disk.
	ChartPath string
	// ResolveValues enables resolving the .Values.* references on each captured
	// line to the values Helm actually rendered with.
	ResolveValues bool
	// ValueResolver performs that resolution against the stopped debuggee. It is
	// shared across the run so it can be reused for every breakpoint.
	ValueResolver *templatevalues.Resolver
}

type RenderedLine struct {
	//CharPosition int
	//FileName     string
	Content string
}

type ExecutionUnit struct {
	FunctionName string
	LineNumber   int
	FileName     string
	LineContent  string
	// SourceError is set when the source line could not be read from disk. The
	// execution unit is still reported; only LineContent is missing.
	SourceError string
	// ResolvedValues maps a .Values reference expression (".Values.name" or
	// "$.Values.name") to the value Helm saw at render time. It is nil unless
	// value resolution was enabled for the run.
	ResolvedValues map[string]string
}

// A mapper is helps bind a variable name to a common type
// ex. node.pipe.tr.Name is a function name
//
//	in some cases, the variable to introspect is called
//	pipe.tr.Name, but they represent roughly the same thing.
type Mapper map[string]string

type BindResult struct {
	ExecutionUnit *ExecutionUnit
	RenderedLine  *RenderedLine
}

type FrameBinder interface {
	Gather(client *rpc2.RPCClient) (map[string]string, error)
	Bind(respVars map[string]string) (*BindResult, error)
}

func (ex *BindResult) Display(w io.Writer, isHelper bool) error {
	if ex.ExecutionUnit != nil {
		return ex.ExecutionUnit.Display(w, isHelper)
	}
	return nil
}

func (ex *ExecutionUnit) Display(w io.Writer, isHelper bool) error {
	indent := ""
	if isHelper {
		indent = "  "
	}
	fmt.Fprintf(w, "%s%s:%d\n", indent, ex.FileName, ex.LineNumber)
	if ex.FunctionName != ex.FileName {
		fmt.Fprintf(w, "%s  in %s\n", indent, ex.FunctionName)
	}
	fmt.Fprintf(w, "%s    ", indent)
	fmt.Fprint(w, ex.LineContent+"\n")
	return nil
}
