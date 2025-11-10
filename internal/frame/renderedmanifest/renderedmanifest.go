package renderedmanifest

import (
	"fmt"

	"github.com/go-delve/delve/service/api"
	"github.com/go-delve/delve/service/rpc2"
	"github.com/jessesimpson36/helm-debugger/internal/frame"
)

// A mapper is helps bind a variable name to a common type
// ex. node.pipe.tr.Name is a function name
//
//	in some cases, the variable to introspect is called
//	pipe.tr.Name, but they represent roughly the same thing.
type Mapper map[string]string

type RenderedManifestFrame frame.Frame

var loadConfig = api.LoadConfig{
	FollowPointers:     true,
	MaxVariableRecurse: 10,
	MaxStringLen:       10000,
	MaxArrayValues:     10000,
	MaxStructFields:    -1,
}

func (f *RenderedManifestFrame) Gather(client *rpc2.RPCClient) (map[string]string, error) {
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

func (f *RenderedManifestFrame) Bind(respVars map[string]string) (*frame.BindResult, error) {
	renderedLine := &frame.RenderedLine{}
	for key, val := range f.Mapper {
		mappedVal, ok := respVars[val]
		if !ok {
			return nil, fmt.Errorf("Failed to find mapped variable %s in response vars", val)
		}
		switch key {
		case "Content":
			renderedLine.Content = string(mappedVal)
		//case "CharPosition":
		//	charPos, err := strconv.Atoi(mappedVal)
		//	if err != nil {
		//		return nil, fmt.Errorf("Failed to convert LineNumber to int: %w", err)
		//	}
		//	renderedLine.CharPosition = charPos
		//case "FileName":
		//	renderedLine.FileName = mappedVal
		default:
			return nil, fmt.Errorf("Unknown key in mapper: %s", key)
		}
	}
	bindResult := &frame.BindResult{
		ExecutionUnit: nil,
		RenderedLine:  renderedLine,
	}

	return bindResult, nil
}
