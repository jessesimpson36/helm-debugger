package frame

import (
	"fmt"
	"io"

	"github.com/go-delve/delve/service/api"
	"github.com/go-delve/delve/service/rpc2"
	"github.com/jessesimpson36/helm-debugger/internal/templatepath"
)

// A frame represents a breakpoint and a set of variables you want displayed at that frame
type Frame struct {
	Breakpoints []*api.Breakpoint
	ReqVars     []string
	Mapper      Mapper
	WorkingDir  string
	// Resolver maps runtime Go template names to source files on disk. It may be
	// nil, in which case names are resolved relative to WorkingDir.
	Resolver *templatepath.Resolver
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
