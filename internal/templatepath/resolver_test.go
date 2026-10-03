package templatepath

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile creates a file (and its parent directories) under dir.
func writeFile(t *testing.T, dir, rel, content string) string {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// versionedChart builds a chart whose directory name differs from its Chart.yaml
// name, mirroring a versioned chart layout.
func versionedChart(t *testing.T) (dir, name string) {
	t.Helper()
	dir = t.TempDir()
	name = "example-platform"
	writeFile(t, dir, "Chart.yaml", "apiVersion: v2\nname: "+name+"\nversion: 1.0.0\n")
	writeFile(t, dir, "templates/deployment.yaml", "kind: Deployment\n")
	writeFile(t, dir, "templates/frontend/_helpers.tpl", "{{- define \"frontend.fullname\" -}}\n")
	// A vendored subchart template, named by the parent chart prefix.
	writeFile(t, dir, "charts/sub/templates/service.yaml", "kind: Service\n")
	return dir, name
}

func TestNewReadsChartName(t *testing.T) {
	dir, want := versionedChart(t)
	r := New("", dir)
	if r.ChartName() != want {
		t.Fatalf("ChartName() = %q, want %q", r.ChartName(), want)
	}
	if r.ChartDir() != dir {
		t.Fatalf("ChartDir() = %q, want %q", r.ChartDir(), dir)
	}
}

func TestNewBareNameResolvesUnderWorkingDir(t *testing.T) {
	parent := t.TempDir()
	writeFile(t, parent, "mychart/Chart.yaml", "name: mychart\n")
	r := New(parent, "mychart")
	want := filepath.Join(parent, "mychart")
	if r.ChartDir() != want {
		t.Fatalf("ChartDir() = %q, want %q", r.ChartDir(), want)
	}
	if r.ChartName() != "mychart" {
		t.Fatalf("ChartName() = %q, want mychart", r.ChartName())
	}
}

func TestResolveRewritesChartNamePrefix(t *testing.T) {
	dir, name := versionedChart(t)
	r := New("", dir)

	got, err := r.Resolve(name + "/templates/deployment.yaml")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := filepath.Join(dir, "templates", "deployment.yaml")
	if got != want {
		t.Fatalf("Resolve = %q, want %q", got, want)
	}
}

func TestResolveHelperAndSubchartPaths(t *testing.T) {
	dir, name := versionedChart(t)
	r := New("", dir)

	for _, rel := range []string{
		"templates/frontend/_helpers.tpl",
		"charts/sub/templates/service.yaml",
	} {
		got, err := r.Resolve(name + "/" + rel)
		if err != nil {
			t.Fatalf("Resolve(%q): %v", rel, err)
		}
		want := filepath.Join(dir, filepath.FromSlash(rel))
		if got != want {
			t.Fatalf("Resolve(%q) = %q, want %q", rel, got, want)
		}
	}
}

func TestResolveFallsBackToWorkingDirRelativePath(t *testing.T) {
	parent := t.TempDir()
	writeFile(t, parent, "templates/plain.yaml", "kind: Pod\n")

	r := New(parent, "unrelated")
	got, err := r.Resolve("templates/plain.yaml")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if want := filepath.Join(parent, "templates", "plain.yaml"); got != want {
		t.Fatalf("Resolve = %q, want %q", got, want)
	}
}

func TestResolveAbsolutePath(t *testing.T) {
	dir, _ := versionedChart(t)
	abs := filepath.Join(dir, "templates", "deployment.yaml")
	r := New("", dir)
	got, err := r.Resolve(abs)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != abs {
		t.Fatalf("Resolve = %q, want %q", got, abs)
	}
}

func TestResolveMissingReturnsError(t *testing.T) {
	dir, name := versionedChart(t)
	r := New("", dir)
	if _, err := r.Resolve(name + "/templates/nope.yaml"); err == nil {
		t.Fatal("expected an error for a missing template")
	}
}

func TestReadChartNameIgnoresNestedNames(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Chart.yaml", "apiVersion: v2\nname: real-name\nmaintainers:\n  - name: someone\n")
	if got := readChartName(dir); got != "real-name" {
		t.Fatalf("readChartName = %q, want real-name", got)
	}
}

func TestReadChartNameQuotedAndCommented(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Chart.yaml", `name: "quoted-chart" # a comment`+"\n")
	if got := readChartName(dir); got != "quoted-chart" {
		t.Fatalf("readChartName = %q, want quoted-chart", got)
	}
}
