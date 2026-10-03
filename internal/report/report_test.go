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

func TestSectionsRenderedNeedle(t *testing.T) {
	flows := []*executionflow.ExecutionFlow{sampleFlow()}
	// A bare snippet is matched against the rendered output, without the caller
	// knowing the source file.
	cfg := &settings.Settings{RenderedQueryFiles: []string{"line3"}}
	sections := Sections(flows, cfg)
	if len(sections) != 1 || sections[0].Name != "RENDERED QUERY" {
		t.Fatalf("unexpected sections: %+v", sections)
	}
	if len(sections[0].Flows) != 1 {
		t.Fatalf("expected the rendered needle to match, got %d", len(sections[0].Flows))
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

// resolvedFlow is sampleFlow plus a helper line whose .Values references were
// resolved at render time.
func resolvedFlow() *executionflow.ExecutionFlow {
	flow := sampleFlow()
	helper := &frame.ExecutionUnit{
		FunctionName: "chart.serviceAccountName",
		FileName:     "chart/templates/_helpers.tpl",
		LineNumber:   58,
		LineContent:  `{{- default (include "chart.fullname" .) .Values.serviceAccount.name }}`,
		ResolvedValues: map[string]string{
			"serviceAccount.name": `""`,
		},
	}
	flow.Helpers = []*frame.ExecutionUnit{helper}
	flow.ValuesReference = []*executionflow.ValuesReference{
		{ExecutionUnit: helper, ValuesName: "serviceAccount.name", Values: `""`, Resolved: true, Found: true},
	}
	return flow
}

func TestWriteRendersResolvedValues(t *testing.T) {
	text := Text([]Section{{Name: "EXECUTION FLOWS", Flows: []*executionflow.ExecutionFlow{resolvedFlow()}}})
	for _, want := range []string{
		"Relevant Values",
		`- serviceAccount.name = ""`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("report missing %q:\n%s", want, text)
		}
	}
}

func TestWriteMarksUnresolvedWhenResolutionRan(t *testing.T) {
	flow := sampleFlow()
	flow.ValuesReference = []*executionflow.ValuesReference{
		{ValuesName: "image.tag", Resolved: true, Found: false},
	}
	text := Text([]Section{{Name: "EXECUTION FLOWS", Flows: []*executionflow.ExecutionFlow{flow}}})
	if !strings.Contains(text, "- image.tag = <unset>") {
		t.Fatalf("expected an <unset> marker:\n%s", text)
	}
}

func TestLocateIncludesResolvedValues(t *testing.T) {
	located := Locate([]*executionflow.ExecutionFlow{resolvedFlow()}, &settings.Settings{})
	var helperSite *Site
	for i := range located.Sites {
		if located.Sites[i].Helper == "chart.serviceAccountName" {
			helperSite = &located.Sites[i]
		}
	}
	if helperSite == nil {
		t.Fatalf("expected the helper site, got %+v", located.Sites)
	}
	if got := helperSite.Values["serviceAccount.name"]; got != `""` {
		t.Fatalf("site values = %#v, want serviceAccount.name=%q", helperSite.Values, `""`)
	}

	text := LocateText(located, nil)
	if !strings.Contains(text, `.Values.serviceAccount.name = ""`) {
		t.Fatalf("locate text missing the resolved value:\n%s", text)
	}
}

func TestWarningsText(t *testing.T) {
	if got := WarningsText(nil); got != "" {
		t.Fatalf("WarningsText(nil) = %q, want empty", got)
	}
	text := WarningsText([]string{"a/templates/x.yaml: not found", "b/templates/y.yaml: not found"})
	for _, want := range []string{"WARNINGS", "- a/templates/x.yaml: not found", "- b/templates/y.yaml: not found"} {
		if !strings.Contains(text, want) {
			t.Fatalf("warnings text missing %q:\n%s", want, text)
		}
	}
}

func TestLocateCollectsSitesAndValues(t *testing.T) {
	readUnit := &frame.ExecutionUnit{
		FunctionName: "frontend.effectiveDbUsername",
		FileName:     "chart/templates/_helpers.tpl",
		LineNumber:   57,
		LineContent:  `{{ .Values.frontend.database.auth.username | default .Values.global.database.auth.username }}`,
	}
	unrelatedHelper := &frame.ExecutionUnit{
		FunctionName: "examplePlatform.replicas",
		FileName:     "chart/templates/common/_helpers.tpl",
		LineNumber:   3187,
		LineContent:  `{{- $r := .Values.backend.replicas | default dict -}}`,
	}
	flow := sampleFlow()
	flow.Helpers = []*frame.ExecutionUnit{readUnit, unrelatedHelper}
	flow.ValuesReference = []*executionflow.ValuesReference{
		{ExecutionUnit: readUnit, ValuesName: "frontend.database.auth.username"},
		{ExecutionUnit: readUnit, ValuesName: "global.database.auth.username"},
		{ExecutionUnit: unrelatedHelper, ValuesName: "backend.replicas"},
	}

	located := Locate([]*executionflow.ExecutionFlow{flow}, &settings.Settings{
		ValuesQuery: []string{"global.database.auth.username"},
	})

	// For a values query the read site comes first, then the enclosing template.
	// Reads of other options and the helper chain are not dumped.
	if len(located.Sites) != 2 {
		t.Fatalf("expected read site + template site, got %+v", located.Sites)
	}
	if located.Sites[0].Helper != "frontend.effectiveDbUsername" || located.Sites[0].Line != 57 {
		t.Fatalf("unexpected read site: %+v", located.Sites[0])
	}
	if located.Sites[1].File != "chart/templates/deployment.yaml" || located.Sites[1].Line != 10 {
		t.Fatalf("unexpected template site: %+v", located.Sites[1])
	}
	for _, site := range located.Sites {
		if site.File == "chart/templates/common/_helpers.tpl" {
			t.Fatalf("unrelated helper frame leaked into locate sites: %+v", site)
		}
	}
	wantValues := []string{"global.database.auth.username"}
	if strings.Join(located.RelevantValues, ",") != strings.Join(wantValues, ",") {
		t.Fatalf("unexpected relevant values: %v", located.RelevantValues)
	}
}

func TestLocateRenderedQueryKeepsHelperContext(t *testing.T) {
	flow := sampleFlow()
	flow.Helpers = []*frame.ExecutionUnit{{
		FunctionName: "chart.fullname",
		FileName:     "chart/templates/_helpers.tpl",
		LineNumber:   14,
		LineContent:  `{{- if .Values.fullnameOverride }}`,
	}}

	located := Locate([]*executionflow.ExecutionFlow{flow}, &settings.Settings{
		RenderedQueryFiles: []string{"line3"},
	})

	// Without a values query the enclosing helper chain is still useful context
	// for "what wrote this output".
	found := false
	for _, site := range located.Sites {
		if site.File == "chart/templates/_helpers.tpl" && site.Helper == "chart.fullname" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected helper context for a rendered query, got %+v", located.Sites)
	}
}

func TestLocateTextIsCompact(t *testing.T) {
	located := Locate([]*executionflow.ExecutionFlow{sampleFlow()}, &settings.Settings{})
	text := LocateText(located, nil)
	for _, want := range []string{"LOCATE", "Source sites (1)", "chart/templates/deployment.yaml:10", "- image.tag"} {
		if !strings.Contains(text, want) {
			t.Fatalf("locate text missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "WriteBuffer") {
		t.Fatalf("locate text must not include rendered write buffers:\n%s", text)
	}
}

func TestLocateNoMatchExplainsItself(t *testing.T) {
	located := Locate([]*executionflow.ExecutionFlow{sampleFlow()}, &settings.Settings{
		ValuesQuery: []string{"does.not.exist"},
	})
	if len(located.Sites) != 0 {
		t.Fatalf("expected no sites, got %+v", located.Sites)
	}
	text := LocateText(located, []string{"image.tag"})
	for _, want := range []string{"No source sites matched the query.", "Did you mean:", "- image.tag"} {
		if !strings.Contains(text, want) {
			t.Fatalf("locate text missing %q:\n%s", want, text)
		}
	}
}
