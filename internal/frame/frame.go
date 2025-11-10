package frame

import (
	"fmt"

	"github.com/go-delve/delve/service/api"
	"github.com/go-delve/delve/service/rpc2"
)

// A frame represents a breakpoint and a set of variables you want displayed at that frame
type Frame struct {
	Breakpoints []*api.Breakpoint
	ReqVars     []string
	Mapper      Mapper
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

func (ex *BindResult) Display(isHelper bool) error {
	if ex.ExecutionUnit != nil {
		return ex.ExecutionUnit.Display(isHelper)
	}
	return nil
}

func (ex *ExecutionUnit) Display(isHelper bool) error {
	indent := ""
	if isHelper {
		indent = "  "
	}
	fmt.Printf("%s%s:%d\n", indent, ex.FileName, ex.LineNumber)
	if ex.FunctionName != ex.FileName {
		fmt.Printf("%s  in %s\n", indent, ex.FunctionName)
	}
	fmt.Printf("%s    ", indent)
	fmt.Print(ex.LineContent + "\n")
	return nil
}
