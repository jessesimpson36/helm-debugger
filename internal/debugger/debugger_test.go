package debugger

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jessesimpson36/helm-debugger/internal/frame"
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

func TestRunResolvesValues(t *testing.T) {
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
		ResolveValues:    true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	result, err := Run(ctx, cfg, io.Discard)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// The chart's serviceAccount.name defaults to "" in values.yaml; the helper
	// line reads it, so it must resolve to the empty string rather than being
	// reported as unresolved.
	found := false
	for _, flow := range result.Flows {
		for _, ref := range flow.ValuesReference {
			if ref.ValuesName != "serviceAccount.name" {
				continue
			}
			found = true
			if !ref.Resolved || !ref.Found {
				t.Fatalf("serviceAccount.name was not resolved: %+v", ref)
			}
			if ref.Values != `""` {
				t.Fatalf("serviceAccount.name = %q, want %q", ref.Values, `""`)
			}
		}
	}
	if !found {
		t.Fatal("expected a flow to reference serviceAccount.name")
	}
}

func TestRunRequiresChart(t *testing.T) {
	_, err := Run(context.Background(), &settings.Settings{CompiledHelmPath: "helm"}, io.Discard)
	if err == nil {
		t.Fatal("expected validation error for missing chart")
	}
}

func TestCollectSourceWarnings(t *testing.T) {
	events := []*frame.BindResult{
		nil,
		{ExecutionUnit: &frame.ExecutionUnit{FileName: "b/templates/b.yaml", SourceError: "boom"}},
		{ExecutionUnit: &frame.ExecutionUnit{FileName: "a/templates/a.yaml", SourceError: "missing a"}},
		// Duplicate file: only the first reason is kept.
		{ExecutionUnit: &frame.ExecutionUnit{FileName: "a/templates/a.yaml", SourceError: "missing a again"}},
		{ExecutionUnit: &frame.ExecutionUnit{FileName: "c/templates/c.yaml"}},
		{RenderedLine: &frame.RenderedLine{Content: "x"}},
	}

	got := collectSourceWarnings(events)
	want := []string{
		"a/templates/a.yaml: missing a",
		"b/templates/b.yaml: boom",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collectSourceWarnings = %#v, want %#v", got, want)
	}
	if collectSourceWarnings(nil) != nil {
		t.Fatal("expected nil warnings when there are no source errors")
	}
}

func TestCollectSourceWarningsCapsOutput(t *testing.T) {
	var events []*frame.BindResult
	for i := 0; i < maxSourceWarnings+5; i++ {
		events = append(events, &frame.BindResult{
			ExecutionUnit: &frame.ExecutionUnit{
				FileName:    fmt.Sprintf("templates/%02d.yaml", i),
				SourceError: "missing",
			},
		})
	}
	got := collectSourceWarnings(events)
	if len(got) != maxSourceWarnings+1 {
		t.Fatalf("got %d warnings, want %d", len(got), maxSourceWarnings+1)
	}
	if last := got[len(got)-1]; !strings.Contains(last, "more template sources") {
		t.Fatalf("last warning = %q, want a summary line", last)
	}
}
