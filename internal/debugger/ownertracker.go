package debugger

import (
	"github.com/go-delve/delve/service/api"
	"github.com/go-delve/delve/service/rpc2"
)

// ownerTracker records which top-level rendered template each captured event
// belongs to. Flow assembly groups by this owner, so it does not rely on the
// debugger stopping on every node (which the walk gate deliberately does not).
//
// The current owner is updated when a rendered template starts executing. A
// helper always executes within some rendered template, so the owner persists
// across helper frames until the next rendered template starts.
type ownerTracker struct {
	client   *rpc2.RPCClient
	scope    api.EvalScope
	load     api.LoadConfig
	rendered map[string]struct{}
	current  string
}

func newOwnerTracker(client *rpc2.RPCClient, rendered []string) *ownerTracker {
	set := make(map[string]struct{}, len(rendered))
	for _, name := range rendered {
		set[name] = struct{}{}
	}
	return &ownerTracker{
		client:   client,
		scope:    api.EvalScope{GoroutineID: 1, Frame: 0},
		load:     api.LoadConfig{MaxStringLen: 1024},
		rendered: set,
	}
}

// Current returns the owner to stamp onto an event.
func (o *ownerTracker) Current() string {
	if o == nil {
		return ""
	}
	return o.current
}

// OnExecute updates the owner from t.name when a rendered template starts.
func (o *ownerTracker) OnExecute() {
	if o == nil {
		return
	}
	if name, err := o.read("t.name"); err == nil {
		o.consider(name)
	}
}

// OnWalk updates the owner from s.tmpl.name. Top-level templates and helpers
// both report here; only rendered names change the owner.
func (o *ownerTracker) OnWalk() {
	if o == nil {
		return
	}
	if name, err := o.read("s.tmpl.name"); err == nil {
		o.consider(name)
	}
}

func (o *ownerTracker) consider(name string) {
	if _, ok := o.rendered[name]; ok {
		o.current = name
	}
}

func (o *ownerTracker) read(expr string) (string, error) {
	v, err := o.client.EvalVariable(o.scope, expr, o.load)
	if err != nil || v == nil {
		return "", err
	}
	return v.Value, nil
}
