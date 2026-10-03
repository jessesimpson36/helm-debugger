package breakpointevent

import (
	"testing"

	"github.com/jessesimpson36/helm-debugger/internal/frame"
)

func execUnit(name, file string) *frame.BindResult {
	return &frame.BindResult{ExecutionUnit: &frame.ExecutionUnit{FunctionName: name, FileName: file}}
}

func rendered(content string) *frame.BindResult {
	return &frame.BindResult{RenderedLine: &frame.RenderedLine{Content: content}}
}

func TestProcessGroupsFlowsAndRenderedBuffers(t *testing.T) {
	events := []*frame.BindResult{
		execUnit("chart/templates/deployment.yaml", "chart/templates/deployment.yaml"),
		execUnit("chart.fullname", "chart/templates/_helpers.tpl"),
		rendered("a"),
		rendered("ab"),
		execUnit("chart/templates/service.yaml", "chart/templates/service.yaml"),
		rendered("abc"),
		execUnit("chart.labels", "chart/templates/_helpers.tpl"),
		rendered("abcd"),
	}

	flows := Process(events)
	if len(flows) != 2 {
		t.Fatalf("expected 2 flows, got %d", len(flows))
	}

	if flows[0].Template.FileName != "chart/templates/deployment.yaml" {
		t.Fatalf("unexpected first template: %+v", flows[0].Template)
	}
	if len(flows[0].Helpers) != 1 || flows[0].Helpers[0].FunctionName != "chart.fullname" {
		t.Fatalf("unexpected first flow helpers: %+v", flows[0].Helpers)
	}
	if len(flows[1].Helpers) != 1 || flows[1].Helpers[0].FunctionName != "chart.labels" {
		t.Fatalf("unexpected second flow helpers: %+v", flows[1].Helpers)
	}

	// The new flow's first buffer snapshot is the previous flow's after-state,
	// so it is recorded in both flows.
	wantFirst := []string{"a", "ab", "abc"}
	if !sameContents(flows[0].RenderedManifest, wantFirst) {
		t.Fatalf("first rendered manifest = %v, want %v", contents(flows[0].RenderedManifest), wantFirst)
	}
	wantSecond := []string{"abc", "abcd"}
	if !sameContents(flows[1].RenderedManifest, wantSecond) {
		t.Fatalf("second rendered manifest = %v, want %v", contents(flows[1].RenderedManifest), wantSecond)
	}
}

// TestProcessDoesNotDuplicateFirstRenderedLine guards against the historical
// bug where the first flow's initial buffer snapshot was appended twice,
// producing a spurious blank diff line.
func TestProcessDoesNotDuplicateFirstRenderedLine(t *testing.T) {
	events := []*frame.BindResult{
		execUnit("chart/templates/deployment.yaml", "chart/templates/deployment.yaml"),
		rendered("a"),
		rendered("ab"),
	}
	flows := Process(events)
	if len(flows) != 1 {
		t.Fatalf("expected 1 flow, got %d", len(flows))
	}
	got := contents(flows[0].RenderedManifest)
	want := []string{"a", "ab"}
	if !sameContents(flows[0].RenderedManifest, want) {
		t.Fatalf("rendered manifest = %v, want %v", got, want)
	}
}

func TestProcessSkipsLeadingHelpers(t *testing.T) {
	events := []*frame.BindResult{
		execUnit("chart.fullname", "chart/templates/_helpers.tpl"),
		execUnit("chart/templates/deployment.yaml", "chart/templates/deployment.yaml"),
	}
	flows := Process(events)
	if len(flows) != 1 {
		t.Fatalf("expected 1 flow, got %d", len(flows))
	}
	if len(flows[0].Helpers) != 0 {
		t.Fatalf("leading helper should be skipped, got %+v", flows[0].Helpers)
	}
}

func TestProcessIgnoresNilEvents(t *testing.T) {
	events := []*frame.BindResult{
		nil,
		execUnit("chart/templates/deployment.yaml", "chart/templates/deployment.yaml"),
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
