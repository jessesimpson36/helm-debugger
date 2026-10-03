package executionflow

import (
	"testing"

	"github.com/jessesimpson36/helm-debugger/internal/frame"
)

func TestIsTemplate(t *testing.T) {
	if IsTemplate(nil) {
		t.Fatal("nil execution unit should not be a template")
	}
	tmpl := &frame.ExecutionUnit{FunctionName: "mychart/templates/deployment.yaml", FileName: "mychart/templates/deployment.yaml"}
	if !IsTemplate(tmpl) {
		t.Fatal("expected execution unit with matching function/file to be a template")
	}
	helper := &frame.ExecutionUnit{FunctionName: "mychart.fullname", FileName: "mychart/templates/_helpers.tpl"}
	if IsTemplate(helper) {
		t.Fatal("helper should not be classified as a template")
	}
}

func TestGetValuesReferences(t *testing.T) {
	unit := &frame.ExecutionUnit{
		LineContent: `  image: "{{ .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}"`,
	}
	got := GetValuesReferences(unit)
	want := []string{"image.repository", "image.tag"}
	if len(got) != len(want) {
		t.Fatalf("GetValuesReferences = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("GetValuesReferences = %#v, want %#v", got, want)
		}
	}
}

func TestFillValuesReferences(t *testing.T) {
	flow := &ExecutionFlow{}
	unit := &frame.ExecutionUnit{LineContent: `name: {{ .Values.serviceAccount.name }},`}
	FillValuesReferences(flow, unit)
	if len(flow.ValuesReference) != 1 {
		t.Fatalf("expected 1 values reference, got %d", len(flow.ValuesReference))
	}
	if flow.ValuesReference[0].ValuesName != "serviceAccount.name" {
		t.Fatalf("unexpected values name %q", flow.ValuesReference[0].ValuesName)
	}
}

func TestFillValuesReferencesRoot(t *testing.T) {
	flow := &ExecutionFlow{}
	unit := &frame.ExecutionUnit{
		LineContent: `{{ range .Values.items }}{{ $.Values.serviceAccount.name }}{{ end }}`,
		ResolvedValues: map[string]string{
			".Values.items":                "<[]interface {}>",
			"$.Values.serviceAccount.name": `""`,
		},
	}
	FillValuesReferences(flow, unit)
	if len(flow.ValuesReference) != 2 {
		t.Fatalf("expected 2 values references, got %d: %+v", len(flow.ValuesReference), flow.ValuesReference)
	}
	if flow.ValuesReference[0].ValuesName != "items" || flow.ValuesReference[0].Root {
		t.Fatalf("unexpected first reference: %+v", flow.ValuesReference[0])
	}
	root := flow.ValuesReference[1]
	if root.ValuesName != "serviceAccount.name" || !root.Root {
		t.Fatalf("unexpected root reference: %+v", root)
	}
	if !root.Resolved || !root.Found || root.Values != `""` {
		t.Fatalf("root reference was not resolved: %+v", root)
	}
}

func TestContainsValuesReference(t *testing.T) {
	if ContainsValuesReference(&frame.ExecutionUnit{LineContent: "no values here"}) {
		t.Fatal("did not expect a values reference")
	}
	if !ContainsValuesReference(&frame.ExecutionUnit{LineContent: "{{ .Values.foo }}"}) {
		t.Fatal("expected a values reference")
	}
}
