package includegraph

import (
	"reflect"
	"testing"
)

const sample = `{{- define "test.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 -}}
{{- end }}

{{- define "test.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{- define "test.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "test.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "test.labels" -}}
helm.sh/chart: {{ include "test.chart" . }}
{{ include "test.selectorLabels" . }}
{{- end }}

{{- define "test.dynamic" -}}
{{- include (printf "%s.fullname" .component) . }}
{{- end }}
`

func TestAnchorFindsCallSite(t *testing.T) {
	// A helper invoked from a rendered template's body (outside a define) must
	// be recorded as a call site so the flow can anchor at the invocation line.
	content := "apiVersion: v1\nkind: Pod\nmetadata:\n  name: {{ include \"test.serviceAccountName\" . }}\n"
	g := &Graph{
		directCalls: map[string]map[string]struct{}{},
		unresolved:  map[string]bool{},
		defined:     map[string]bool{},
		callSites:   map[string][]CallSite{},
	}
	g.parseFile("templates/serviceaccount.yaml", content)

	site, ok := g.Anchor("test/templates/serviceaccount.yaml", []string{"test.serviceAccountName"})
	if !ok {
		t.Fatal("expected to find an anchor call site")
	}
	if site.Line != 4 {
		t.Fatalf("anchor line = %d, want 4", site.Line)
	}
	if _, ok := g.Anchor("test/templates/other.yaml", []string{"test.serviceAccountName"}); ok {
		t.Fatal("did not expect an anchor in an unrelated owner")
	}
}

func TestClosureResolvesNestedIncludes(t *testing.T) {
	g := parseString(t, sample)

	names, unbounded := g.Closure([]string{"test.serviceAccountName"})
	if unbounded {
		t.Fatal("serviceAccountName should be bounded")
	}
	want := []string{"test.fullname", "test.serviceAccountName"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("Closure = %v, want %v", names, want)
	}
}

func TestClosureFlagsUnresolved(t *testing.T) {
	g := parseString(t, sample)
	if !g.Unresolved("test.dynamic") {
		t.Fatal("expected test.dynamic to be marked unresolved")
	}
	if g.Unresolved("test.fullname") {
		t.Fatal("test.fullname has no dynamic calls")
	}
	// A caller of an unresolved definition is unbounded.
	g.directCalls["test.caller"] = map[string]struct{}{"test.dynamic": {}}
	g.defined["test.caller"] = true
	_, unbounded := g.Closure([]string{"test.caller"})
	if !unbounded {
		t.Fatal("expected a caller of an unresolved definition to be unbounded")
	}
}

func TestDefined(t *testing.T) {
	g := parseString(t, sample)
	if !g.Defined("test.labels") {
		t.Fatal("expected test.labels to be defined")
	}
	if g.Defined("nope") {
		t.Fatal("did not expect nope to be defined")
	}
}

func TestClosureCond(t *testing.T) {
	cond := ClosureCond([]string{"a", "b"})
	want := `t.name == "a" || t.name == "b"`
	if cond != want {
		t.Fatalf("ClosureCond = %q, want %q", cond, want)
	}
	if ClosureCond(nil) != "" {
		t.Fatal("expected an empty condition for no names")
	}
}

func parseString(t *testing.T, content string) *Graph {
	t.Helper()
	g := &Graph{
		directCalls: map[string]map[string]struct{}{},
		unresolved:  map[string]bool{},
		defined:     map[string]bool{},
		callSites:   map[string][]CallSite{},
	}
	g.parseFile("templates/_helpers.tpl", content)
	return g
}
