package debugger

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/go-delve/delve/service/api"
	"github.com/go-delve/delve/service/rpc2"
	"github.com/jessesimpson36/helm-debugger/internal/frame"
	"github.com/jessesimpson36/helm-debugger/internal/templatevalues"
)

// evalFieldBreakpointName is the breakpoint on (*state).evalField's map-return.
// Unlike the walk-based capture, a hit here has the just-computed field value in
// the `result` local, so no whole-template-data materialization is needed.
const evalFieldBreakpointName = "evalfield"

// evalFieldCond keeps the breakpoint to the final field of a .Values / $.Values
// chain. Delve evaluates the condition in the debugger process, so every other
// map lookup is rejected without an RPC round trip.
const evalFieldCond = `fieldName == s.node.Ident[len(s.node.Ident)-1] && (s.node.Ident[0] == "Values" || (s.node.Ident[0] == "$" && s.node.Ident[1] == "Values"))`

var (
	// metaLoad reads the small parse-node metadata (Ident, Pos, tree name).
	metaLoad = api.LoadConfig{FollowPointers: true, MaxVariableRecurse: 1, MaxStringLen: 64, MaxArrayValues: 16}
	// textLoad reads a template's raw text so a node's byte offset can be
	// turned into a line number.
	textLoad = api.LoadConfig{MaxStringLen: 1 << 22}
	// valueLoad reads the single captured field value.
	valueLoad = api.LoadConfig{FollowPointers: true, MaxVariableRecurse: 2, MaxStringLen: 4096, MaxArrayValues: 8, MaxStructFields: -1}
)

// fieldCapture is one resolved .Values value captured at evalField exit.
type fieldCapture struct {
	parseName string
	line      int
	expr      string
	value     string
}

// fieldCapturer captures values at the evalField map-return breakpoint. It
// caches each template's raw text, since the offset-to-line mapping for every
// field in a template uses the same text.
type fieldCapturer struct {
	scope api.EvalScope
	text  map[string]string // tree pointer -> template text
}

func newFieldCapturer(client *rpc2.RPCClient) *fieldCapturer {
	client.SetReturnValuesLoadConfig(&valueLoad)
	return &fieldCapturer{
		scope: api.EvalScope{GoroutineID: 1, Frame: 0},
		text:  map[string]string{},
	}
}

// Capture reads the field path, source position, and value at the current
// evalField stop. It returns nil when the node has no usable field chain.
func (c *fieldCapturer) Capture(client *rpc2.RPCClient) (*fieldCapture, error) {
	idents, err := c.identifiers(client)
	if err != nil || len(idents) == 0 {
		return nil, err
	}
	pos, err := c.intExpr(client, "s.node.Pos")
	if err != nil {
		return nil, err
	}
	parseName, err := c.stringExpr(client, "s.node.tr.ParseName", metaLoad)
	if err != nil {
		return nil, err
	}
	text, err := c.templateText(client)
	if err != nil {
		return nil, err
	}
	value, err := c.fieldValue(client)
	if err != nil {
		return nil, err
	}

	return &fieldCapture{
		parseName: parseName,
		line:      lineAt(text, pos),
		expr:      exprFor(idents),
		value:     value,
	}, nil
}

// identifiers returns s.node.Ident, the full field path of the current chain.
func (c *fieldCapturer) identifiers(client *rpc2.RPCClient) ([]string, error) {
	v, err := client.EvalVariable(c.scope, "s.node.Ident", metaLoad)
	if err != nil || v == nil {
		return nil, err
	}
	idents := make([]string, 0, len(v.Children))
	for i := range v.Children {
		idents = append(idents, v.Children[i].Value)
	}
	return idents, nil
}

func (c *fieldCapturer) intExpr(client *rpc2.RPCClient, expr string) (int, error) {
	v, err := client.EvalVariable(c.scope, expr, metaLoad)
	if err != nil || v == nil {
		return 0, err
	}
	return strconv.Atoi(v.Value)
}

func (c *fieldCapturer) stringExpr(client *rpc2.RPCClient, expr string, load api.LoadConfig) (string, error) {
	v, err := client.EvalVariable(c.scope, expr, load)
	if err != nil || v == nil {
		return "", err
	}
	return v.Value, nil
}

// templateText returns the current template's raw text, cached by tree pointer.
func (c *fieldCapturer) templateText(client *rpc2.RPCClient) (string, error) {
	tree, err := client.EvalVariable(c.scope, "s.node.tr", metaLoad)
	if err != nil || tree == nil {
		return "", err
	}
	if cached, ok := c.text[tree.Value]; ok {
		return cached, nil
	}
	text, err := c.stringExpr(client, "s.node.tr.text", textLoad)
	if err != nil {
		return "", err
	}
	if tree.Value != "" {
		c.text[tree.Value] = text
	}
	return text, nil
}

// fieldValue reads the just-computed `result` local. Reading it is cheap: it is
// one value, not the whole template data.
func (c *fieldCapturer) fieldValue(client *rpc2.RPCClient) (string, error) {
	state, err := client.Call(c.scope.GoroutineID, "result.Interface()", false)
	if err != nil || state == nil || state.CurrentThread == nil || len(state.CurrentThread.ReturnValues) == 0 {
		return "", err
	}
	return renderValue(state.CurrentThread.ReturnValues[0]), nil
}

// renderValue unwraps Delve's interface indirection and formats the value the
// same way the materializing resolver does.
func renderValue(v api.Variable) string {
	for (v.Kind == reflect.Interface || v.Kind == reflect.Pointer) && len(v.Children) == 1 {
		v = v.Children[0]
	}
	return templatevalues.Format(v)
}

// exprFor turns a parse-node Ident list into the template expression used as
// the resolved-values key: "$.Values.x" or ".Values.x".
func exprFor(idents []string) string {
	if len(idents) == 0 {
		return ""
	}
	if idents[0] == "$" {
		return strings.Join(idents, ".")
	}
	return "." + strings.Join(idents, ".")
}

// lineAt returns the 1-based line containing byte offset pos.
func lineAt(text string, pos int) int {
	if pos < 0 {
		pos = 0
	}
	if pos > len(text) {
		pos = len(text)
	}
	return 1 + strings.Count(text[:pos], "\n")
}

// attachFieldCaptures copies captured values onto the matching execution units,
// keyed by their runtime template name and line. Resolution-by-pointer is not
// needed here because the engine already resolved the value.
func attachFieldCaptures(events []*frame.BindResult, captures []*fieldCapture) {
	type key struct {
		file string
		line int
	}
	units := map[key][]*frame.ExecutionUnit{}
	for _, event := range events {
		if event == nil || event.ExecutionUnit == nil {
			continue
		}
		unit := event.ExecutionUnit
		k := key{unit.FileName, unit.LineNumber}
		units[k] = append(units[k], unit)
	}
	for _, capture := range captures {
		for _, unit := range units[key{capture.parseName, capture.line}] {
			if unit.ResolvedValues == nil {
				unit.ResolvedValues = map[string]string{}
			}
			unit.ResolvedValues[capture.expr] = capture.value
		}
	}
}
