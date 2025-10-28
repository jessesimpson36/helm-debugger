package frame

import (
	"fmt"
	"strconv"
	"github.com/jessesimpson36/helm-debugger/internal/display"
	"github.com/go-delve/delve/service/api"
	"github.com/go-delve/delve/service/rpc2"
)

// A frame represents a breakpoint and a set of variables you want displayed at that frame
type Frame struct {
	Breakpoints []*api.Breakpoint
	ReqVars     []string
	Mapper      Mapper
}

// A mapper is helps bind a variable name to a common type
// ex. node.pipe.tr.Name is a function name
//     in some cases, the variable to introspect is called
//     pipe.tr.Name, but they represent roughly the same thing.
type Mapper map[string]string


type ExecutionUnit struct {
	FunctionName string
	LineNumber   int
	FileName     string
	LineContent  string
}


var loadConfig = api.LoadConfig{
	FollowPointers:     true,
	MaxVariableRecurse: 10,
	MaxStringLen:       10000,
	MaxArrayValues:     10000,
	MaxStructFields:    -1,
}

func (f *Frame) Gather(client *rpc2.RPCClient) (map[string]string, error) {
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

func (f *Frame) Bind(respVars map[string]string) (*ExecutionUnit, error) {
	execUnit := &ExecutionUnit{}
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
		default:
			return nil, fmt.Errorf("Unknown key in mapper: %s", key)
		}
		if execUnit.FunctionName != "" && execUnit.FileName != "" && execUnit.LineNumber != 0 {
			lineContent, err := display.ReadOneLine(execUnit.FileName, execUnit.LineNumber)
			if err != nil {
				return nil, fmt.Errorf("Failed to read line content: %w", err)
			}
			execUnit.LineContent = lineContent
		}
	}

	return execUnit, nil
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
