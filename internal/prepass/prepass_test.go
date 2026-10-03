package prepass

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jessesimpson36/helm-debugger/internal/settings"
)

// fixtureHelm writes an executable fake helm binary that prints a fixed set of
// Source comments, so the parser and invocation can be tested without a chart.
func fixtureHelm(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "helm")
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRenderedTemplatesParsesSourceComments(t *testing.T) {
	helmPath := fixtureHelm(t,
		"#!/bin/sh\n"+
			"cat <<'EOF'\n"+
			"---\n# Source: mychart/templates/deployment.yaml\nkind: Deployment\n"+
			"---\n# Source: mychart/templates/service.yaml\nkind: Service\n"+
			"---\n# Source: mychart/templates/deployment.yaml\nkind: Deployment\n"+
			"EOF\n")
	cfg := &settings.Settings{ChartName: "mychart", CompiledHelmPath: helmPath}

	got, err := RenderedTemplates(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RenderedTemplates: %v", err)
	}
	want := []string{"mychart/templates/deployment.yaml", "mychart/templates/service.yaml"}
	if len(got) != len(want) {
		t.Fatalf("RenderedTemplates = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("RenderedTemplates = %v, want %v", got, want)
		}
	}
}

func TestRenderedTemplatesFailsOnNonZeroExit(t *testing.T) {
	helmPath := fixtureHelm(t, "#!/bin/sh\necho boom >&2\nexit 1\n")
	cfg := &settings.Settings{ChartName: "x", CompiledHelmPath: helmPath}
	if _, err := RenderedTemplates(context.Background(), cfg); err == nil {
		t.Fatal("expected an error when helm exits non-zero")
	}
}
