package breakpoints

import (
	"strings"
	"testing"
)

func TestTemplateExecuteCond(t *testing.T) {
	tests := []struct {
		name  string
		names []string
		want  string
	}{
		{"single", []string{"test.serviceAccountName"}, `t.name == "test.serviceAccountName"`},
		{
			"multiple",
			[]string{"test.fullname", "test.serviceAccountName"},
			`t.name == "test.fullname" || t.name == "test.serviceAccountName"`,
		},
		{"trims and drops empties", []string{" a ", "", "  "}, `t.name == "a"`},
		{"dedupes", []string{"a", "a"}, `t.name == "a"`},
		{"empty", nil, ""},
		{"escapes quotes", []string{`a"b`}, `t.name == "a\"b"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TemplateExecuteCond(tt.names); got != tt.want {
				t.Fatalf("TemplateExecuteCond(%v) = %q, want %q", tt.names, got, tt.want)
			}
		})
	}
}

func TestTemplateExecuteCondIsSupersetOfSpellings(t *testing.T) {
	// Both a bare helper name and a full file path are allowed, because the
	// condition only needs to be a safe superset of what the query may match.
	cond := TemplateExecuteCond([]string{"test.serviceAccountName", "test/templates/deployment.yaml"})
	for _, want := range []string{"test.serviceAccountName", "test/templates/deployment.yaml"} {
		if !strings.Contains(cond, want) {
			t.Fatalf("condition missing %q: %s", want, cond)
		}
	}
}
