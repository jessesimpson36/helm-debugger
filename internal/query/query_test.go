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
