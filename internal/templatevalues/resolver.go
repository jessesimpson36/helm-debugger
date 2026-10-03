package templatevalues

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/go-delve/delve/service/api"
	"github.com/go-delve/delve/service/rpc2"
)

// dotExpr is the local variable that holds the template data at the
// text/template.(*state).walk breakpoint for every node.
const dotExpr = "dot.Interface()"

// valuesKey is the top-level template data field that holds the merged values.
const valuesKey = "Values"

// loadConfig controls how much of the template data Delve materializes when it
// returns dot.Interface(). It follows pointers and struct fields deeply enough
// to reach nested values. MaxArrayValues also caps the number of map entries
// Delve loads, so it must stay high enough to include every top-level key
// (including .Values); MaxStringLen trims long strings but keeps them readable.
var loadConfig = api.LoadConfig{
	FollowPointers:     true,
	MaxVariableRecurse: 30,
	MaxStringLen:       4096,
	MaxArrayValues:     1000,
	MaxStructFields:    -1,
}

// Resolver materializes the template dot at a breakpoint and looks up
// .Values.* paths inside it. It is bound to one stopped delve session and must
// only be used while the debuggee is stopped.
type Resolver struct {
	client      *rpc2.RPCClient
	goroutineID int64
}

// NewResolver returns a Resolver that evaluates dot on the given delve client.
// It also installs a return-value load config so Delve can return the
// materialized data from dot.Interface().
func NewResolver(client *rpc2.RPCClient, goroutineID int64) *Resolver {
	if client != nil {
		client.SetReturnValuesLoadConfig(&loadConfig)
	}
	return &Resolver{client: client, goroutineID: goroutineID}
}

// Resolve materializes the template dot once and returns a display rendering
// for each path (without the leading ".Values.", for example
// "serviceAccount.name") that exists in it. Paths that cannot be resolved are
// omitted. The returned map is non-nil whenever resolution was attempted, which
// lets callers distinguish "not resolved" from "resolution disabled".
func (r *Resolver) Resolve(paths []string) map[string]string {
	resolved := make(map[string]string, len(paths))
	if r == nil || r.client == nil || len(paths) == 0 {
		return resolved
	}
	root, ok := r.dot()
	if !ok {
		return resolved
	}
	// dot is the top-level template data; .Values is a field/key on it.
	values, ok := mapLookup(unwrap(root), valuesKey)
	if !ok {
		return resolved
	}
	values = unwrap(values)
	for _, path := range paths {
		if value, ok := lookupPath(values, path); ok {
			resolved[path] = Format(value)
		}
	}
	return resolved
}

// lookupPath walks a dot-separated .Values path down a map variable.
func lookupPath(node api.Variable, path string) (api.Variable, bool) {
	for _, part := range strings.Split(path, ".") {
		child, ok := mapLookup(unwrap(node), part)
		if !ok {
			return api.Variable{}, false
		}
		node = child
	}
	return unwrap(node), true
}

// dot calls dot.Interface() and returns the materialized template data. Calling
// a method is required because dot is a reflect.Value; EvalVariable cannot
// invoke it.
func (r *Resolver) dot() (api.Variable, bool) {
	state, err := r.client.Call(r.goroutineID, dotExpr, false)
	if err != nil || state == nil || state.CurrentThread == nil {
		return api.Variable{}, false
	}
	values := state.CurrentThread.ReturnValues
	if len(values) == 0 {
		return api.Variable{}, false
	}
	return unwrap(values[0]), true
}

// unwrap follows interface and pointer indirections that Delve inserts, until
// it reaches the concrete value. Delve represents an interface as a variable
// with a single child holding the dynamic value.
func unwrap(v api.Variable) api.Variable {
	for (v.Kind == reflect.Interface || v.Kind == reflect.Pointer) && len(v.Children) == 1 {
		v = v.Children[0]
	}
	return v
}

// mapLookup finds key in a Delve map variable. For maps each entry is stored as
// two children: an even-indexed key followed by its odd-indexed value.
func mapLookup(m api.Variable, key string) (api.Variable, bool) {
	for i := 0; i+1 < len(m.Children); i += 2 {
		k := m.Children[i]
		if k.Value == key || k.Name == key {
			return m.Children[i+1], true
		}
	}
	return api.Variable{}, false
}

// Format renders a resolved value for humans. Strings are quoted so an empty
// value is distinguishable from an absent one; composite values fall back to
// Delve's summary or their type name.
func Format(v api.Variable) string {
	switch v.Kind {
	case reflect.String:
		return strconv.Quote(v.Value)
	case reflect.Invalid:
		return "nil"
	case reflect.Interface, reflect.Map, reflect.Slice, reflect.Array, reflect.Struct:
		if v.Value != "" {
			return v.Value
		}
		if v.Type != "" {
			return "<" + v.Type + ">"
		}
		return "nil"
	}
	if v.Value != "" {
		return v.Value
	}
	if v.Kind == reflect.Bool {
		// Delve reports booleans in Value, but guard against an empty render.
		return "false"
	}
	return "nil"
}
