package debugger

import (
	"fmt"
	"os"

	"github.com/go-delve/delve/service/api"
	"github.com/go-delve/delve/service/rpc2"
)

// debug enables gate tracing when HELM_DEBUGGER_DEBUG is set.
var debug = os.Getenv("HELM_DEBUGGER_DEBUG") != ""

// walkGate controls the per-node walk breakpoint at runtime. The walk breakpoint
// is the only source of per-line detail, but it fires on every template node,
// which is expensive. When a caller asks about specific helpers, the walk
// breakpoint starts disabled and is enabled only while execution is inside one
// of those helpers (or a helper it transitively calls), then disabled again when
// execution leaves.
//
// The gate is exact for the common case and conservative for the rest: a helper
// whose subtree could not be resolved statically (dynamic include) makes the
// gate stay enabled for the remainder of the run's current position rather than
// risk dropping lines.
type walkGate struct {
	client  *rpc2.RPCClient
	bp      *api.Breakpoint // the walk breakpoint; BP.ID is valid after Configure
	scope   api.EvalScope
	load    api.LoadConfig
	inSet   map[string]struct{}
	enabled bool
	// alwaysOn disables gating entirely (the walk breakpoint stays enabled).
	alwaysOn bool
}

// newWalkGate returns a gate for the given walk breakpoint. names is the set of
// template names whose subtree should be walked; when empty the gate is a no-op
// (walk always on).
func newWalkGate(client *rpc2.RPCClient, walkBP *api.Breakpoint, names []string) *walkGate {
	g := &walkGate{
		client: client,
		bp:     walkBP,
		scope:  api.EvalScope{GoroutineID: 1, Frame: 0},
		load:   api.LoadConfig{MaxStringLen: 1024},
		inSet:  map[string]struct{}{},
	}
	for _, n := range names {
		g.inSet[n] = struct{}{}
	}
	if len(names) == 0 {
		g.alwaysOn = true
		g.enabled = true
	}
	return g
}

// Begin disables the walk breakpoint before the run starts, when gating is in
// effect. It is a no-op when the gate always runs.
func (g *walkGate) Begin() error {
	if g == nil || g.alwaysOn {
		return nil
	}
	return g.set(false)
}

// OnEnter is called when a template/helper starts (the Execute trigger). It
// enables the walk breakpoint when the entering template is in the scoped set,
// so its nodes (and any helper it calls, which will re-enter through Execute)
// are captured. It never disables; AtWalk handles leaving the subtree.
func (g *walkGate) OnEnter(client *rpc2.RPCClient) {
	if g == nil || g.alwaysOn {
		return
	}
	name, err := client.EvalVariable(g.scope, "t.name", g.load)
	if err != nil || name == nil {
		return
	}
	if _, ok := g.inSet[name.Value]; ok && !g.enabled {
		_ = g.set(true)
	}
}

// AtWalk is called on every walk stop. It disables the breakpoint once the
// current template is no longer in the scoped set, so nodes outside the subtree
// stop halting the debugger. Enabling is left to OnEnter, because a disabled
// breakpoint produces no walk stops to enable from.
func (g *walkGate) AtWalk() {
	if g == nil || g.alwaysOn {
		return
	}
	current, err := g.currentTemplate()
	if err != nil {
		// If the template name cannot be read, keep the current state rather
		// than guess; a wrong disable would silently lose lines.
		return
	}
	if _, ok := g.inSet[current]; ok {
		return
	}
	if g.enabled {
		_ = g.set(false)
	}
}

// currentTemplate reads s.tmpl.name (the field, not the Name() method, which
// returns a function descriptor).
func (g *walkGate) currentTemplate() (string, error) {
	v, err := g.client.EvalVariable(g.scope, "s.tmpl.name", g.load)
	if err != nil {
		return "", err
	}
	if v == nil {
		return "", fmt.Errorf("no value for s.tmpl.name")
	}
	return v.Value, nil
}

// set enables or disables the walk breakpoint, tracking state so it is only
// amended on a transition.
func (g *walkGate) set(enabled bool) error {
	if g.bp == nil {
		return fmt.Errorf("no walk breakpoint")
	}
	if g.enabled == enabled {
		return nil
	}
	amended := *g.bp
	amended.Disabled = !enabled
	if err := g.client.AmendBreakpoint(&amended); err != nil {
		return err
	}
	g.enabled = enabled
	return nil
}
