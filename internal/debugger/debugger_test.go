package debugger

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jessesimpson36/helm-debugger/internal/report"
	"github.com/jessesimpson36/helm-debugger/internal/settings"
)

// repoRoot returns the repository root relative to this package directory.
func repoRoot() string {
	return filepath.Join("..", "..")
}

// localDebugHelm returns the path to a debug-enabled helm binary, or "".
func localDebugHelm() string {
	if p := os.Getenv("HELM_DEBUGGER_HELM"); p != "" {
		return p
	}
	candidate := filepath.Join(repoRoot(), "helm", "bin", "helm")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

// TestRunAgainstLocalHelm exercises the full delve pipeline. It only runs when
// a debug-enabled helm binary and the dlv CLI are available, so it is skipped in
// most environments and CI.
func TestRunAgainstLocalHelm(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	helmPath := localDebugHelm()
	if helmPath == "" {
		t.Skip("no debug-enabled helm binary; set HELM_DEBUGGER_HELM to enable")
	}
	if _, err := exec.LookPath("dlv"); err != nil {
		t.Skip("dlv not found in PATH")
	}

	cfg := &settings.Settings{
		ChartName:        "test",
		CompiledHelmPath: helmPath,
		WorkingDir:       repoRoot(),
		CommandArgs:      []string{"--show-only", "templates/deployment.yaml"},
		ValuesQuery:      []string{"image.tag"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	result, err := Run(ctx, cfg, io.Discard)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(result.Flows) == 0 {
		t.Fatal("expected at least one execution flow")
	}
	if result.LineNumbers.LineStart == 0 || result.LineNumbers.RenderedManifest == 0 {
		t.Fatalf("breakpoints were not resolved: %+v", result.LineNumbers)
	}

	text := report.Text(report.Sections(result.Flows, cfg))
	for _, want := range []string{"VALUES QUERY", "image.tag", "WriteBuffer"} {
		if !strings.Contains(text, want) {
			t.Fatalf("report missing %q:\n%s", want, text)
		}
	}
}

func TestRunRequiresChart(t *testing.T) {
	_, err := Run(context.Background(), &settings.Settings{CompiledHelmPath: "helm"}, io.Discard)
	if err == nil {
		t.Fatal("expected validation error for missing chart")
	}
}
