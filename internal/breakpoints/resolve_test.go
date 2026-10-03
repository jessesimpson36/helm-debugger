package breakpoints

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// findLine returns the 1-based line number of the first line containing marker.
func findLine(t *testing.T, src, marker string) int {
	t.Helper()
	for i, line := range strings.Split(src, "\n") {
		if strings.Contains(line, marker) {
			return i + 1
		}
	}
	t.Fatalf("marker %q not found in fixture", marker)
	return 0
}

func TestResolveFileFindsMarkersInCorrectFunctions(t *testing.T) {
	// The fixture intentionally contains decoy occurrences of s.at(node) in an
	// unrelated function, so the parser must track function context.
	fixture := `package template

func (s *state) other() {
	s.at(node)
}

func (s *state) walk(dot reflect.Value, node parse.Node) {
	s.at(node)
	switch node := node.(type) {
	case *parse.ActionNode:
		val := s.evalPipeline(dot, node.Pipe)
		_ = val
	case *parse.TextNode:
		if _, err := s.wr.Write(node.Text); err != nil {
			s.writeError(err)
		}
	}
}

func (s *state) walkIfOrWith(typ parse.NodeType, dot reflect.Value, pipe *parse.PipeNode, list, elseList *parse.ListNode) {
	defer s.pop(s.mark())
	val := s.evalPipeline(dot, pipe)
	truth, ok := isTrue(indirectInterface(val))
	if !ok {
		s.errorf("if/with can't use %v", val)
	}
	if truth {
		s.walk(dot, list)
	} else if elseList != nil {
		s.walk(dot, elseList)
	}
}
`
	dir := t.TempDir()
	path := filepath.Join(dir, "exec.go")
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveFile(path)
	if err != nil {
		t.Fatalf("resolveFile: %v", err)
	}

	want := LineNumbers{
		LineStart:        findLine(t, fixture, "func (s *state) walk(dot") + 1, // s.at(node) on next line
		RenderedManifest: findLine(t, fixture, "s.wr.Write(node.Text)"),
		ConditionalStart: findLine(t, fixture, "s.evalPipeline(dot, pipe)"),
		ConditionalTrue:  findLine(t, fixture, "if truth {"),
		ConditionalFalse: findLine(t, fixture, "s.walk(dot, elseList)"),
	}
	if got != want {
		t.Fatalf("resolveFile = %+v, want %+v", got, want)
	}
}

func TestResolveFileReportsMissingMarkers(t *testing.T) {
	fixture := "package template\n\nfunc (s *state) walk(dot reflect.Value, node parse.Node) {\n}\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "exec.go")
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveFile(path); err == nil {
		t.Fatal("expected error for missing markers, got nil")
	}
}

func TestResolveMissingFile(t *testing.T) {
	if _, err := Resolve(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Fatal("expected error for missing GOROOT, got nil")
	}
}

func TestResolveFromRuntimeGOROOT(t *testing.T) {
	goroot := DefaultGOROOT()
	if goroot == "" {
		t.Skip("no GOROOT available")
	}
	if _, err := os.Stat(filepath.Join(goroot, "src", "text", "template", "exec.go")); err != nil {
		t.Skipf("no text/template source under %s: %v", goroot, err)
	}
	lines, err := Resolve(goroot)
	if err != nil {
		t.Fatalf("Resolve(%s): %v", goroot, err)
	}
	if lines.LineStart == 0 || lines.RenderedManifest == 0 || lines.ConditionalStart == 0 ||
		lines.ConditionalTrue == 0 || lines.ConditionalFalse == 0 {
		t.Fatalf("incomplete line numbers: %+v", lines)
	}
	// The conditional branch lines must be ordered and inside the file.
	if !(lines.LineStart < lines.RenderedManifest) {
		t.Fatalf("expected walk lines ordered, got %+v", lines)
	}
	if !(lines.ConditionalStart < lines.ConditionalTrue && lines.ConditionalTrue <= lines.ConditionalFalse) {
		t.Fatalf("expected conditional lines ordered, got %+v", lines)
	}
}

func TestResolveOrDefaultFallsBack(t *testing.T) {
	lines := ResolveOrDefault(filepath.Join(t.TempDir(), "missing"))
	if lines != FallbackLines {
		t.Fatalf("ResolveOrDefault = %+v, want fallback %+v", lines, FallbackLines)
	}
}
