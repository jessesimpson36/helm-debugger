package breakpointevent

import (
	"testing"

	"github.com/jessesimpson36/helm-debugger/internal/frame"
)

// execUnit builds an execution-unit event. owner is the runtime name of the
// top-level rendered template the event belongs to.
func execUnit(owner, name, file string) *frame.BindResult {
	return &frame.BindResult{ExecutionUnit: &frame.ExecutionUnit{
		FunctionName: name, FileName: file, Owner: owner,
	}}
}

func rendered(owner, content string) *frame.BindResult {
	return &frame.BindResult{RenderedLine: &frame.RenderedLine{Content: content, Owner: owner}}
}

func TestProcessGroupsFlowsAndRenderedBuffers(t *testing.T) {
	const (
		deploy = "chart/templates/deployment.yaml"
		svc    = "chart/templates/service.yaml"
	)
	events := []*frame.BindResult{
		execUnit(deploy, deploy, deploy),
		execUnit(deploy, "chart.fullname", "chart/templates/_helpers.tpl"),
		rendered(deploy, "a"),
		rendered(deploy, "ab"),
		execUnit(svc, svc, svc),
		rendered(svc, "abc"),
		execUnit(svc, "chart.labels", "chart/templates/_helpers.tpl"),
		rendered(svc, "abcd"),
	}

	flows := Process(events)
	if len(flows) != 2 {
		t.Fatalf("expected 2 flows, got %d", len(flows))
	}

	if flows[0].Template.FileName != deploy {
		t.Fatalf("unexpected first template: %+v", flows[0].Template)
	}
	if len(flows[0].Helpers) != 1 || flows[0].Helpers[0].FunctionName != "chart.fullname" {
		t.Fatalf("unexpected first flow helpers: %+v", flows[0].Helpers)
	}
	if len(flows[1].Helpers) != 1 || flows[1].Helpers[0].FunctionName != "chart.labels" {
		t.Fatalf("unexpected second flow helpers: %+v", flows[1].Helpers)
	}

	// Each flow owns exactly the buffer snapshots stamped with its owner.
	wantFirst := []string{"a", "ab"}
	if !sameContents(flows[0].RenderedManifest, wantFirst) {
		t.Fatalf("first rendered manifest = %v, want %v", contents(flows[0].RenderedManifest), wantFirst)
	}
	wantSecond := []string{"abc", "abcd"}
	if !sameContents(flows[1].RenderedManifest, wantSecond) {
		t.Fatalf("second rendered manifest = %v, want %v", contents(flows[1].RenderedManifest), wantSecond)
	}
}

// TestProcessAttributesHelpersWithoutTemplateNodes is the case that motivates
// explicit ownership: the walk breakpoint is gated, so the top-level template's
// own nodes are absent and only a helper is captured. The flow must still be
// anchored to the owning template and include the helper.
func TestProcessAttributesHelpersWithoutTemplateNodes(t *testing.T) {
	const deploy = "chart/templates/deployment.yaml"
	events := []*frame.BindResult{
		// No event from deployment.yaml itself; only its helper was walked.
		execUnit(deploy, "chart.serviceAccountName", "chart/templates/_helpers.tpl"),
		execUnit(deploy, "chart.fullname", "chart/templates/_helpers.tpl"),
	}

	flows := Process(events)
	if len(flows) != 1 {
		t.Fatalf("expected 1 flow, got %d", len(flows))
	}
	// The owner names the anchor even though no template execution unit was
	// captured; the report resolves it from the owner.
	if flows[0].Owner != deploy {
		t.Fatalf("flow owner = %q, want %q", flows[0].Owner, deploy)
	}
	if len(flows[0].Helpers) != 2 {
		t.Fatalf("expected 2 helpers, got %+v", flows[0].Helpers)
	}
}

// TestProcessGroupsInterleavedOwners checks that events are grouped by owner
// even when they interleave, which can happen when a gated sequence skips the
// nodes that would otherwise keep them separated.
func TestProcessGroupsInterleavedOwners(t *testing.T) {
	const (
		a = "chart/templates/a.yaml"
		b = "chart/templates/b.yaml"
	)
	events := []*frame.BindResult{
		execUnit(a, a, a),
		execUnit(b, b, b),
		execUnit(a, "chart.one", "chart/templates/_helpers.tpl"),
		execUnit(b, "chart.two", "chart/templates/_helpers.tpl"),
	}

	flows := Process(events)
	if len(flows) != 2 {
		t.Fatalf("expected 2 flows, got %d", len(flows))
	}
	if flows[0].Owner != a || flows[1].Owner != b {
		t.Fatalf("unexpected owners: %q, %q", flows[0].Owner, flows[1].Owner)
	}
	if len(flows[0].Helpers) != 1 || flows[0].Helpers[0].FunctionName != "chart.one" {
		t.Fatalf("flow a helpers = %+v", flows[0].Helpers)
	}
	if len(flows[1].Helpers) != 1 || flows[1].Helpers[0].FunctionName != "chart.two" {
		t.Fatalf("flow b helpers = %+v", flows[1].Helpers)
	}
}

func TestProcessKeepsOwnerlessHelpersSeparate(t *testing.T) {
	// A helper captured before any owner was known must not attach to an
	// unrelated flow; it forms its own group rather than being dropped.
	events := []*frame.BindResult{
		execUnit("", "chart.fullname", "chart/templates/_helpers.tpl"),
		execUnit("chart/templates/deployment.yaml", "chart/templates/deployment.yaml", "chart/templates/deployment.yaml"),
	}
	flows := Process(events)
	if len(flows) != 2 {
		t.Fatalf("expected 2 flows, got %d", len(flows))
	}
	// The deployment flow is not polluted by the ownerless helper.
	for _, flow := range flows {
		if flow.Owner == "chart/templates/deployment.yaml" && len(flow.Helpers) != 0 {
			t.Fatalf("ownerless helper leaked into the deployment flow: %+v", flow.Helpers)
		}
	}
}

func TestProcessIgnoresNilEvents(t *testing.T) {
	const deploy = "chart/templates/deployment.yaml"
	events := []*frame.BindResult{
		nil,
		execUnit(deploy, deploy, deploy),
		nil,
	}
	if flows := Process(events); len(flows) != 1 {
		t.Fatalf("expected 1 flow, got %d", len(flows))
	}
}

func contents(lines []*frame.RenderedLine) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, line.Content)
	}
	return out
}

func sameContents(lines []*frame.RenderedLine, want []string) bool {
	got := contents(lines)
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
