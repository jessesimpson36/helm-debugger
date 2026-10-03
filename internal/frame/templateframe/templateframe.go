package templateframe

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-delve/delve/service/api"
	"github.com/go-delve/delve/service/rpc2"
	"github.com/jessesimpson36/helm-debugger/internal/display"
	"github.com/jessesimpson36/helm-debugger/internal/frame"
	"github.com/jessesimpson36/helm-debugger/internal/templatevalues"
)

// A mapper is helps bind a variable name to a common type
// ex. node.pipe.tr.Name is a function name
//
//	in some cases, the variable to introspect is called
//	pipe.tr.Name, but they represent roughly the same thing.
type Mapper map[string]string

var loadConfig = api.LoadConfig{
	FollowPointers:     true,
	MaxVariableRecurse: 10,
	// Templates are read in full to map node byte offsets to line numbers, so
	// this must cover the largest template text, not just a field value.
	MaxStringLen:    1 << 22,
	MaxArrayValues:  10000,
	MaxStructFields: -1,
}

type TemplateFrame frame.Frame

func (f *TemplateFrame) Gather(client *rpc2.RPCClient) (map[string]string, error) {
	reqResponse := make(map[string]string)
	for _, varName := range f.ReqVars {
		variable, err := client.EvalVariable(api.EvalScope{GoroutineID: 1, Frame: 0}, varName, loadConfig)
		if err != nil {
			//println(fmt.Errorf("Failed to eval variable %s: %w", varName, err).Error())
			continue
		}
		if variable != nil {
			// fmt.Printf("%s: %s\n", varName, variable.Value)
		}
		reqResponse[varName] = variable.Value
	}
	return reqResponse, nil
}

func (f *TemplateFrame) Bind(respVars map[string]string) (*frame.BindResult, error) {
	execUnit := &frame.ExecutionUnit{}
	for key, val := range f.Mapper {
		mappedVal, ok := respVars[val]
		if !ok {
			return nil, fmt.Errorf("Failed to find mapped variable %s in response vars", val)
		}
		switch key {
		case "FunctionName":
			execUnit.FunctionName = mappedVal
		case "LineNumber":
			lineNum, err := strconv.Atoi(mappedVal)
			if err != nil {
				return nil, fmt.Errorf("Failed to convert LineNumber to int: %w", err)
			}
			execUnit.LineNumber = lineNum
		case "FileName":
			execUnit.FileName = mappedVal
		case "RootOffset":
			// A template executed at (t *Template).Execute has no node; the root
			// node's byte offset is turned into a line using the template text.
			offset, err := strconv.Atoi(mappedVal)
			if err != nil {
				return nil, fmt.Errorf("Failed to convert RootOffset to int: %w", err)
			}
			execUnit.LineNumber = lineAtOffset(respVars[f.Mapper["TreeText"]], offset)
		case "TreeText":
			// Consumed via RootOffset above; nothing to bind directly.
		default:
			return nil, fmt.Errorf("Unknown key in mapper: %s", key)
		}
		if execUnit.FunctionName != "" && execUnit.FileName != "" && execUnit.LineNumber != 0 {
			lineContent, err := display.ResolveAndReadOneLine(f.ChartPath, execUnit.FileName, execUnit.LineNumber)
			if err != nil {
				// Keep the execution unit so the flow is still reported; only
				// the source line is missing. debugger.Run surfaces SourceError
				// as a warning.
				execUnit.SourceError = err.Error()
			} else {
				execUnit.LineContent = lineContent
			}
		}
	}

	resolveValues(f, execUnit)

	bindResult := &frame.BindResult{
		ExecutionUnit: execUnit,
		RenderedLine:  nil,
	}

	return bindResult, nil
}

// lineAtOffset returns the 1-based line containing byte offset in text. The
// template engine records node positions as byte offsets into the template text.
func lineAtOffset(text string, offset int) int {
	if offset < 0 {
		offset = 0
	}
	if offset > len(text) {
		offset = len(text)
	}
	return 1 + strings.Count(text[:offset], "\n")
}

// resolveValues looks up the .Values.* references on the captured line against
// the live debuggee and records the values Helm rendered with. It is a no-op
// unless value resolution was enabled for the run. A non-nil (possibly empty)
// ResolvedValues map distinguishes "resolution was attempted" from "disabled".
func resolveValues(f *TemplateFrame, execUnit *frame.ExecutionUnit) {
	if !f.ResolveValues || f.ValueResolver == nil {
		return
	}
	if !templatevalues.Contains(execUnit.LineContent) {
		return
	}
	refs := templatevalues.ParseReferences(execUnit.LineContent)
	execUnit.ResolvedValues = f.ValueResolver.Resolve(refs)
}
