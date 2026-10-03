package templateframe

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/jessesimpson36/helm-debugger/internal/frame"
	"github.com/jessesimpson36/helm-debugger/internal/templatepath"
)

const (
	varFunctionName = "node.Pipe.tr.Name"
	varLineNumber   = "node.Pipe.Line"
	varFileName     = "node.Pipe.tr.ParseName"
)

func testFrame(workingDir string, resolver *templatepath.Resolver) *TemplateFrame {
	return &TemplateFrame{
		Mapper: frame.Mapper{
			"FunctionName": varFunctionName,
			"LineNumber":   varLineNumber,
			"FileName":     varFileName,
		},
		WorkingDir: workingDir,
		Resolver:   resolver,
	}
}

func respVars(fileName string, line int) map[string]string {
	return map[string]string{
		varFunctionName: fileName,
		varLineNumber:   strconv.Itoa(line),
		varFileName:     fileName,
	}
}

func TestBindReadsLineThroughResolver(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "templates", "deployment.yaml"), []byte("kind: Deployment\nname: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Chart.yaml name differs from the directory name.
	if err := os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte("name: chart-runtime\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tf := testFrame("", templatepath.New("", dir))
	result, err := tf.Bind(respVars("chart-runtime/templates/deployment.yaml", 2))
	if err != nil {
		t.Fatalf("Bind returned an error: %v", err)
	}
	unit := result.ExecutionUnit
	if unit.SourceError != "" {
		t.Fatalf("unexpected SourceError: %s", unit.SourceError)
	}
	if unit.LineContent != "name: x" {
		t.Fatalf("LineContent = %q, want %q", unit.LineContent, "name: x")
	}
}

func TestBindKeepsUnitWhenSourceMissing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte("name: chart-runtime\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tf := testFrame("", templatepath.New("", dir))
	result, err := tf.Bind(respVars("chart-runtime/templates/missing.yaml", 3))
	if err != nil {
		t.Fatalf("Bind must not fail on a missing source, got: %v", err)
	}
	unit := result.ExecutionUnit
	if unit == nil {
		t.Fatal("expected an execution unit")
	}
	if unit.SourceError == "" {
		t.Fatal("expected SourceError for a missing source")
	}
	if unit.LineContent != "" {
		t.Fatalf("LineContent = %q, want empty", unit.LineContent)
	}
}

func TestBindWithoutResolverUsesWorkingDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "templates", "plain.yaml"), []byte("kind: Pod\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tf := testFrame(dir, nil)
	result, err := tf.Bind(respVars("templates/plain.yaml", 1))
	if err != nil {
		t.Fatalf("Bind returned an error: %v", err)
	}
	if got := result.ExecutionUnit.LineContent; got != "kind: Pod" {
		t.Fatalf("LineContent = %q, want %q", got, "kind: Pod")
	}
}
