package query

import (
	"testing"

	"github.com/jessesimpson36/helm-debugger/internal/executionflow"
	"github.com/jessesimpson36/helm-debugger/internal/frame"
)

func makeFlow(templateFile string, templateLine int, helpers []string, values []string, rendered ...string) *executionflow.ExecutionFlow {
	flow := &executionflow.ExecutionFlow{
		Template: &frame.ExecutionUnit{
			FunctionName: templateFile,
			FileName:     templateFile,
			LineNumber:   templateLine,
		},
	}
	for _, name := range helpers {
		flow.Helpers = append(flow.Helpers, &frame.ExecutionUnit{
			FunctionName: name,
			FileName:     "chart/templates/_helpers.tpl",
		})
	}
	for _, value := range values {
		flow.ValuesReference = append(flow.ValuesReference, &executionflow.ValuesReference{ValuesName: value})
	}
	for _, content := range rendered {
		flow.RenderedManifest = append(flow.RenderedManifest, &frame.RenderedLine{Content: content})
	}
	return flow
}

func TestQueryValuesReference(t *testing.T) {
	flows := []*executionflow.ExecutionFlow{
		makeFlow("chart/templates/deployment.yaml", 1, nil, []string{"image.tag"}),
		makeFlow("chart/templates/service.yaml", 1, nil, []string{"service.port"}),
	}
	got := QueryValuesReference(flows, []string{"image"})
	if len(got) != 1 || got[0].Template.FileName != "chart/templates/deployment.yaml" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestQueryHelpers(t *testing.T) {
	flows := []*executionflow.ExecutionFlow{
		makeFlow("chart/templates/deployment.yaml", 1, []string{"chart.fullname"}, nil),
		makeFlow("chart/templates/service.yaml", 1, []string{"chart.labels"}, nil),
	}
	got := QueryHelpers(flows, []string{"chart.full"})
	if len(got) != 1 || got[0].Template.FileName != "chart/templates/deployment.yaml" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestQueryTemplateMatchesTemplateAndHelperLines(t *testing.T) {
	templateFlow := makeFlow("chart/templates/deployment.yaml", 42, nil, nil)
	helperFlow := makeFlow("chart/templates/service.yaml", 1, []string{"chart.fullname"}, nil)
	helperFlow.Helpers[0].FileName = "chart/templates/_helpers.tpl"
	helperFlow.Helpers[0].LineNumber = 57

	flows := []*executionflow.ExecutionFlow{templateFlow, helperFlow}

	got := QueryTemplate(flows, []string{"chart/templates/deployment.yaml:42"})
	if len(got) != 1 || got[0] != templateFlow {
		t.Fatalf("template match failed: %+v", got)
	}

	got = QueryTemplate(flows, []string{"chart/templates/_helpers.tpl:57"})
	if len(got) != 1 || got[0] != helperFlow {
		t.Fatalf("helper match failed: %+v", got)
	}

	if got := QueryTemplate(flows, []string{"chart/templates/deployment.yaml:999"}); len(got) != 0 {
		t.Fatalf("expected no match, got %+v", got)
	}
}

func TestQueryRenderedTemplate(t *testing.T) {
	// The flow started with a 3-line buffer and ended with a 5-line buffer, so
	// lines 4 and 5 are the flow's output. The query treats the bounds as
	// exclusive, matching strict interior lines.
	flow := makeFlow("chart/templates/deployment.yaml", 1, nil, nil,
		"a\nb\nc", "a\nb\nc\nd\ne")
	flows := []*executionflow.ExecutionFlow{flow}

	if got := QueryRenderedTemplate(flows, []string{"chart/templates/deployment.yaml:4"}); len(got) != 1 {
		t.Fatalf("expected match for line 4, got %+v", got)
	}
	if got := QueryRenderedTemplate(flows, []string{"chart/templates/deployment.yaml:100"}); len(got) != 0 {
		t.Fatalf("expected no match for line 100, got %+v", got)
	}
}

func TestIsFileLine(t *testing.T) {
	tests := map[string]bool{
		"chart/templates/deployment.yaml:42": true,
		"deployment.yaml:1":                  true,
		"deployment.yaml":                    false,
		"username: \"\"":                     false,
		"chart/templates/deployment.yaml:x":  false,
		":42":                                false,
		"":                                   false,
	}
	for selector, want := range tests {
		if got := IsFileLine(selector); got != want {
			t.Errorf("IsFileLine(%q) = %v, want %v", selector, got, want)
		}
	}
}

func TestQueryRenderedNeedle(t *testing.T) {
	target := makeFlow("chart/templates/configmap.yaml", 1, nil, nil, "security:\n  username: \"\"\n  ssl:")
	other := makeFlow("chart/templates/service.yaml", 1, nil, nil, "spec:\n  port: 80")

	// The needle is a snippet of rendered output; it must find the flow that
	// wrote it without the caller knowing the source file.
	got := QueryRenderedNeedle([]*executionflow.ExecutionFlow{other, target}, []string{`username: ""`})
	if len(got) != 1 || got[0] != target {
		t.Fatalf("unexpected needle match: %+v", got)
	}

	if got := QueryRenderedNeedle([]*executionflow.ExecutionFlow{other, target}, []string{"nothing here"}); len(got) != 0 {
		t.Fatalf("expected no needle match, got %+v", got)
	}
}

func TestQueryRenderedNeedleIgnoresPreexistingOutput(t *testing.T) {
	// A flow whose starting buffer already contains the needle but that only
	// adds other content did not write the needle, so it must not match.
	didNotWrite := makeFlow("chart/templates/b.yaml", 1, nil, nil,
		`username: ""`, "username: \"\"\nother: value")
	got := QueryRenderedNeedle([]*executionflow.ExecutionFlow{didNotWrite}, []string{`username: ""`})
	if len(got) != 0 {
		t.Fatalf("expected no match for pre-existing output, got %+v", got)
	}

	// The flow that actually appends the needle does match.
	didWrite := makeFlow("chart/templates/a.yaml", 1, nil, nil,
		"es:", "es:\n  username: \"\"")
	got = QueryRenderedNeedle([]*executionflow.ExecutionFlow{didWrite}, []string{`username: ""`})
	if len(got) != 1 || got[0] != didWrite {
		t.Fatalf("expected the writer flow to match, got %+v", got)
	}
}

func TestQueryRenderedMixedSelectors(t *testing.T) {
	byLine := makeFlow("chart/templates/deployment.yaml", 1, nil, nil, "a\nb\nc", "a\nb\nc\nd\ne")
	byNeedle := makeFlow("chart/templates/configmap.yaml", 1, nil, nil, "leave: alone")
	flows := []*executionflow.ExecutionFlow{byLine, byNeedle}

	got := QueryRendered(flows, []string{"chart/templates/deployment.yaml:4", "leave: alone"})
	if len(got) != 2 {
		t.Fatalf("expected both selectors to match, got %+v", got)
	}

	got = QueryRendered(flows, []string{"leave: alone"})
	if len(got) != 1 || got[0] != byNeedle {
		t.Fatalf("expected only the needle match, got %+v", got)
	}
}

func TestKnownNames(t *testing.T) {
	flows := []*executionflow.ExecutionFlow{
		makeFlow("chart/templates/b.yaml", 1, []string{"chart.fullname"}, []string{"image.tag"}),
		makeFlow("chart/templates/a.yaml", 1, []string{"chart.labels"}, []string{"image.tag", "service.port"}),
	}

	if got := KnownValueNames(flows); len(got) != 2 || got[0] != "image.tag" || got[1] != "service.port" {
		t.Fatalf("KnownValueNames = %v", got)
	}
	if got := KnownHelperNames(flows); len(got) != 2 || got[0] != "chart.fullname" || got[1] != "chart.labels" {
		t.Fatalf("KnownHelperNames = %v", got)
	}
	if got := KnownTemplateFiles(flows); len(got) != 2 || got[0] != "chart/templates/a.yaml" || got[1] != "chart/templates/b.yaml" {
		t.Fatalf("KnownTemplateFiles = %v", got)
	}
}
