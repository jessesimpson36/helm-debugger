package report

import (
	"strings"
	"testing"

	"github.com/jessesimpson36/helm-debugger/internal/executionflow"
	"github.com/jessesimpson36/helm-debugger/internal/frame"
	"github.com/jessesimpson36/helm-debugger/internal/settings"
)

func sampleFlow() *executionflow.ExecutionFlow {
	return &executionflow.ExecutionFlow{
		Template: &frame.ExecutionUnit{
			FunctionName: "chart/templates/deployment.yaml",
			FileName:     "chart/templates/deployment.yaml",
			LineNumber:   10,
			LineContent:  `image: "{{ .Values.image.tag }}"`,
		},
		ValuesReference: []*executionflow.ValuesReference{{ValuesName: "image.tag"}},
		RenderedManifest: []*frame.RenderedLine{
			{Content: "line1\nline2"},
			{Content: "line1\nline2\nline3"},
		},
	}
}

func TestSectionsDefault(t *testing.T) {
	flows := []*executionflow.ExecutionFlow{sampleFlow()}
	sections := Sections(flows, &settings.Settings{})
	if len(sections) != 1 || sections[0].Name != "EXECUTION FLOWS" {
		t.Fatalf("unexpected sections: %+v", sections)
	}
}

func TestSectionsWithQueries(t *testing.T) {
	flows := []*executionflow.ExecutionFlow{sampleFlow()}
	cfg := &settings.Settings{ValuesQuery: []string{"image"}}
	sections := Sections(flows, cfg)
	if len(sections) != 1 || sections[0].Name != "VALUES QUERY" {
		t.Fatalf("unexpected sections: %+v", sections)
	}
	if len(sections[0].Flows) != 1 {
		t.Fatalf("expected the flow to match, got %d", len(sections[0].Flows))
	}
}

func TestWriteRendersDiff(t *testing.T) {
	text := Text([]Section{{Name: "EXECUTION FLOWS", Flows: []*executionflow.ExecutionFlow{sampleFlow()}}})
	for _, want := range []string{
		"EXECUTION FLOWS",
		"chart/templates/deployment.yaml:10",
		"Relevant Values",
		"- image.tag",
		"WriteBuffer",
		"+     line3",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("report missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "+     \n") {
		t.Fatalf("report contains a spurious blank diff line:\n%s", text)
	}
}
