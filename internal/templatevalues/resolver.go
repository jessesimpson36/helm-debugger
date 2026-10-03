package templatevalues

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/go-delve/delve/service/api"
	"github.com/go-delve/delve/service/rpc2"
)

// dotExpr is the local variable that holds the current template data at the
// text/template.(*state).walk breakpoint. The root data (template "$") is the
// first entry on the state's variable stack.
const (
	dotExpr  = "dot.Interface()"
	rootExpr = "s.vars[0].value.Interface()"
)

// valuesKey is the top-level template data field that holds the merged values.
const valuesKey = "Values"

// ptrExpr returns the expression that reads the data pointer of a base. Reading
// a pointer field is a cheap EvalVariable, unlike materializing the data, so it
// can be used as a cache key before deciding whether a call is needed.
func ptrExpr(root bool) string {
	if root {
		return "s.vars[0].value.ptr"
	}
	return "dot.ptr"
}

// ptrLoadConfig is a deliberately tiny config for reading a pointer field.
var ptrLoadConfig = api.LoadConfig{MaxStringLen: 64}

// cacheKey identifies a materialized base by its data pointer. The same map
// reused across a chart's templates (Helm shares one vals map per chart) maps to
// a single entry.
type cacheKey struct {
	root bool
	ptr  string
}

// cachedValues is a materialized ".Values" subtree, or a marker that the base
// had no values.
type cachedValues struct {
	values api.Variable
	found  bool
}

// loadConfig controls how much of the template data Delve materializes when it
// returns a materialized Interface() call. It follows pointers and struct
// fields deeply enough to reach nested values. MaxArrayValues also caps the
// number of map entries Delve loads, so it must stay high enough to include
// every top-level key (including .Values); MaxStringLen trims long strings but
// keeps them readable.
var loadConfig = api.LoadConfig{
	FollowPointers:     true,
	MaxVariableRecurse: 30,
	MaxStringLen:       4096,
	MaxArrayValues:     1000,
	MaxStructFields:    -1,
}

// Resolver materializes the template data at a breakpoint and looks up
// .Values.* paths inside it, against either the current dot or the root. It
// caches the materialized values per data pointer, so a chart that shares one
// values map across many templates pays for the materialization once.
type Resolver struct {
	client      *rpc2.RPCClient
	goroutineID int64
	cache       map[cacheKey]cachedValues
}

// NewResolver returns a Resolver that evaluates the template data on the given
// delve client. It also installs a return-value load config so Delve can return
// the materialized data from the Interface() calls.
func NewResolver(client *rpc2.RPCClient, goroutineID int64) *Resolver {
	if client != nil {
		client.SetReturnValuesLoadConfig(&loadConfig)
	}
	return &Resolver{
		client:      client,
		goroutineID: goroutineID,
		cache:       map[cacheKey]cachedValues{},
	}
}

// Resolve materializes the template data once per base and returns a display
// rendering for each reference (keyed by Reference.Expr) whose path exists in
// it. References that cannot be resolved are omitted. The returned map is
// non-nil whenever resolution was attempted, which lets callers distinguish
// "not resolved" from "resolution disabled".
func (r *Resolver) Resolve(refs []Reference) map[string]string {
	resolved := make(map[string]string, len(refs))
	if r == nil || r.client == nil || len(refs) == 0 {
		return resolved
	}

	need := map[bool]bool{}
	for _, ref := range refs {
		need[ref.Root] = true
	}

	values := map[bool]api.Variable{}
	found := map[bool]bool{}
	for root := range need {
		v, ok := r.valuesFor(root)
		values[root], found[root] = v, ok
	}

	for _, ref := range refs {
		if !found[ref.Root] {
			continue
		}
		if value, ok := lookupPath(values[ref.Root], ref.Path); ok {
			resolved[ref.Expr()] = Format(value)
		}
	}
	return resolved
}

// valuesFor returns the ".Values" entry of a base, materializing and caching it
// on first use. The cache is keyed by the data pointer, which is read with a
// cheap EvalVariable before any call, so repeat lines against the same map are
// free.
func (r *Resolver) valuesFor(root bool) (api.Variable, bool) {
	key, keyed := r.cacheKey(root)
	if keyed {
		if cached, hit := r.cache[key]; hit {
			return cached.values, cached.found
		}
	}

	values, found := r.materializeValues(baseExpr(root))
	if keyed {
		r.cache[key] = cachedValues{values: values, found: found}
	}
	return values, found
}

// cacheKey reads the base's data pointer without materializing the data. The
// second result is false when the pointer could not be read, in which case the
// caller simply skips the cache.
func (r *Resolver) cacheKey(root bool) (cacheKey, bool) {
	v, err := r.client.EvalVariable(
		api.EvalScope{GoroutineID: r.goroutineID, Frame: 0}, ptrExpr(root), ptrLoadConfig)
	if err != nil || v == nil {
		return cacheKey{}, false
	}
	ptr, ok := parsePointer(v.Value)
	if !ok {
		return cacheKey{}, false
	}
	return cacheKey{root: root, ptr: ptr}, true
}

// parsePointer extracts a non-nil address from a Delve pointer value. Delve
// renders unsafe pointers as decimal ("824645894816") or, when formatted
// explicitly, hexadecimal ("unsafe.Pointer(0xc000acc870)"); either spelling is
// an acceptable cache key as long as it is stable.
func parsePointer(value string) (string, bool) {
	v := strings.TrimSpace(value)
	v = strings.TrimPrefix(v, "unsafe.Pointer(")
	v = strings.TrimSuffix(v, ")")
	v = strings.TrimSpace(v)
	switch v {
	case "", "0", "0x0", "nil", "<nil>":
		return "", false
	}
	return v, true
}

// baseExpr returns the Delve expression that yields the template data for a base.
func baseExpr(root bool) string {
	if root {
		return rootExpr
	}
	return dotExpr
}

// materializeValues evaluates expr (which must yield the template data) and
// returns its ".Values" entry. It returns false when the call fails or the
// base has no values.
func (r *Resolver) materializeValues(expr string) (api.Variable, bool) {
	root, ok := r.materialize(expr)
	if !ok {
		return api.Variable{}, false
	}
	values, ok := mapLookup(unwrap(root), valuesKey)
	if !ok {
		return api.Variable{}, false
	}
	return unwrap(values), true
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

// materialize calls expr via Delve and returns the resulting value. Calling a
// method is required because the template data is held in a reflect.Value;
// EvalVariable cannot invoke methods.
func (r *Resolver) materialize(expr string) (api.Variable, bool) {
	state, err := r.client.Call(r.goroutineID, expr, false)
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
