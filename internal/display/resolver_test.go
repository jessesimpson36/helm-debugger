package display

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeTestTgz creates a gzipped tar containing a single file at innerPath.
func writeTestTgz(t *testing.T, path, innerPath, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)
	body := []byte(content)
	if err := tw.WriteHeader(&tar.Header{
		Name:     innerPath,
		Mode:     0o644,
		Size:     int64(len(body)),
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestResolveAndReadOneLineDirectPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plain.yaml")
	writeTestFile(t, path, "kind: Pod\nname: direct\n")

	got, err := ResolveAndReadOneLine(dir, path, 2)
	if err != nil {
		t.Fatalf("ResolveAndReadOneLine: %v", err)
	}
	if got != "name: direct" {
		t.Fatalf("line = %q, want %q", got, "name: direct")
	}
}

// TestResolveVersionedChartDirectory covers a chart whose directory name differs
// from its Chart.yaml name, which is the common versioned-directory layout.
func TestResolveVersionedChartDirectory(t *testing.T) {
	root := t.TempDir()
	chartDir := filepath.Join(root, "charts", "example-platform-8.9")
	writeTestFile(t, filepath.Join(chartDir, "Chart.yaml"), "apiVersion: v2\nname: \"example-platform\" # comment\n")
	writeTestFile(t, filepath.Join(chartDir, "templates", "frontend", "_config.yaml"), "security:\n  username: x\n")

	got, err := ResolveAndReadOneLine(chartDir, "example-platform/templates/frontend/_config.yaml", 2)
	if err != nil {
		t.Fatalf("ResolveAndReadOneLine: %v", err)
	}
	if got != "  username: x" {
		t.Fatalf("line = %q, want %q", got, "  username: x")
	}
}

func TestResolveDecompressedSubchart(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "charts", "sub", "Chart.yaml"), "name: sub\n")
	writeTestFile(t, filepath.Join(root, "charts", "sub", "templates", "svc.yaml"), "kind: Service\n")

	got, err := ResolveAndReadOneLine(root, "sub/templates/svc.yaml", 1)
	if err != nil {
		t.Fatalf("ResolveAndReadOneLine: %v", err)
	}
	if got != "kind: Service" {
		t.Fatalf("line = %q, want %q", got, "kind: Service")
	}
}

func TestResolveSubchartFromTgz(t *testing.T) {
	root := t.TempDir()
	writeTestTgz(t,
		filepath.Join(root, "charts", "subchart-1.2.3.tgz"),
		"subchart/templates/cm.yaml",
		"kind: ConfigMap\ndata:\n  k: v\n",
	)

	got, err := ResolveAndReadOneLine(root, "subchart/templates/cm.yaml", 3)
	if err != nil {
		t.Fatalf("ResolveAndReadOneLine: %v", err)
	}
	if got != "  k: v" {
		t.Fatalf("line = %q, want %q", got, "  k: v")
	}
}

func TestResolveMissingReturnsError(t *testing.T) {
	root := t.TempDir()
	_, err := ResolveAndReadOneLine(root, "nosuch/templates/missing.yaml", 1)
	if err == nil {
		t.Fatal("expected an error for a missing template")
	}
	if !strings.Contains(err.Error(), "could not resolve") {
		t.Fatalf("unexpected error: %v", err)
	}
}
