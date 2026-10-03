package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jessesimpson36/helm-debugger/internal/debugger"
	"github.com/jessesimpson36/helm-debugger/internal/executionflow"
	"github.com/jessesimpson36/helm-debugger/internal/frame"
	"github.com/jessesimpson36/helm-debugger/internal/settings"
	"github.com/jessesimpson36/helm-debugger/internal/templatepath"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// connectTestClient wires a client to a fresh server over an in-memory
// transport and returns the connected client session.
func connectTestClient(t *testing.T) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	server := NewServer(nil)
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)

	st, ct := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	clientSession, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })
	return clientSession
}

func resultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func TestListTools(t *testing.T) {
	session := connectTestClient(t)
	res, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	names := map[string]bool{}
	for _, tool := range res.Tools {
		names[tool.Name] = true
		if tool.Description == "" {
			t.Errorf("tool %s has no description", tool.Name)
		}
	}
	for _, want := range []string{"helm_template", "debug_helm", "resolve_breakpoints"} {
		if !names[want] {
			t.Errorf("missing tool %s", want)
		}
	}
}

// TestToolDescriptionsGuidePhases locks in the wording that steers a caller to
// debug_helm while reproducing and before editing, rather than only at the end.
func TestToolDescriptionsGuidePhases(t *testing.T) {
	session := connectTestClient(t)
	res, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	descriptions := map[string]string{}
	for _, tool := range res.Tools {
		descriptions[tool.Name] = tool.Description
	}

	for _, want := range []string{"REPRODUCING", "BEFORE editing", "rendered"} {
		if !strings.Contains(descriptions["debug_helm"], want) {
			t.Errorf("debug_helm description should contain %q:\n%s", want, descriptions["debug_helm"])
		}
	}
	if !strings.Contains(descriptions["helm_template"], "reproduce") {
		t.Errorf("helm_template description should mention reproducing output:\n%s", descriptions["helm_template"])
	}
}

// fakeFlow models an empty-username bug: a template line that reads a value, a
// helper that resolves it, and rendered output containing the result.
func fakeFlow() *executionflow.ExecutionFlow {
	read := &frame.ExecutionUnit{
		FunctionName: "frontend.effectiveDbUsername",
		FileName:     "example-platform/templates/frontend/_helpers.tpl",
		LineNumber:   158,
		LineContent:  `{{- .Values.frontend.database.auth.username | default .Values.global.database.auth.username -}}`,
	}
	return &executionflow.ExecutionFlow{
		Template: &frame.ExecutionUnit{
			FunctionName: "example-platform/templates/frontend/files/_config.yaml",
			FileName:     "example-platform/templates/frontend/files/_config.yaml",
			LineNumber:   42,
			LineContent:  `username: {{ .Values.frontend.database.auth.username | quote }}`,
		},
		Helpers: []*frame.ExecutionUnit{read},
		ValuesReference: []*executionflow.ValuesReference{
			{ExecutionUnit: read, ValuesName: "frontend.database.auth.username"},
			{ExecutionUnit: read, ValuesName: "global.database.auth.username"},
		},
		RenderedManifest: []*frame.RenderedLine{
			{Content: "db:\n  security:"},
			{Content: "db:\n  security:\n    username: \"\""},
		},
	}
}

func TestSummarizeLocateByDefault(t *testing.T) {
	cfg := &settings.Settings{ValuesQuery: []string{"frontend.database.auth.username"}}
	out := summarize(&debugger.Result{Flows: []*executionflow.ExecutionFlow{fakeFlow()}}, cfg, "")

	if out.Mode != modeLocate {
		t.Fatalf("default mode = %q, want %q", out.Mode, modeLocate)
	}
	if !strings.Contains(out.Report, "_config.yaml:42") {
		t.Fatalf("locate report missing the template site:\n%s", out.Report)
	}
	if strings.Contains(out.Report, "WriteBuffer") {
		t.Fatalf("locate report must be compact (no write buffers):\n%s", out.Report)
	}
	if len(out.Sites) == 0 {
		t.Fatalf("expected structured sites, got none")
	}
	if !strings.Contains(strings.Join(out.RelevantValues, ","), "frontend.database.auth.username") {
		t.Fatalf("expected relevant values, got %v", out.RelevantValues)
	}
}

func TestSummarizeFullKeepsWriteBuffers(t *testing.T) {
	out := summarize(&debugger.Result{Flows: []*executionflow.ExecutionFlow{fakeFlow()}}, &settings.Settings{}, modeFull)
	if out.Mode != modeFull {
		t.Fatalf("mode = %q, want %q", out.Mode, modeFull)
	}
	if !strings.Contains(out.Report, "WriteBuffer") {
		t.Fatalf("full report should include write buffers:\n%s", out.Report)
	}
}

func TestSummarizeEmptyQuerySuggestsNearbyNames(t *testing.T) {
	// "database.auth" is not a prefix of any value, so no flow matches,
	// but it is a substring of the real option, so it should be suggested.
	cfg := &settings.Settings{ValuesQuery: []string{"database.auth"}}
	out := summarize(&debugger.Result{Flows: []*executionflow.ExecutionFlow{fakeFlow()}}, cfg, modeLocate)

	if len(out.Sites) != 0 {
		t.Fatalf("expected no sites, got %+v", out.Sites)
	}
	if !strings.Contains(strings.Join(out.Suggestions, "\n"), "values: frontend.database.auth.username") {
		t.Fatalf("expected a value suggestion, got %v", out.Suggestions)
	}
	if !strings.Contains(out.Report, "No source sites matched the query.") {
		t.Fatalf("empty query should explain itself:\n%s", out.Report)
	}
}

func TestResolveBreakpointsTool(t *testing.T) {
	session := connectTestClient(t)
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "resolve_breakpoints",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(t, res))
	}
	text := resultText(t, res)
	if !strings.Contains(text, "resolved breakpoints") && !strings.Contains(text, "could not resolve") {
		t.Fatalf("unexpected text: %s", text)
	}
}

func TestHelmTemplateTool(t *testing.T) {
	session := connectTestClient(t)
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "helm_template",
		Arguments: map[string]any{
			"chart":     "mychart",
			"helm_path": "echo",
			"extra_args": []any{
				"--show-only",
				"templates/deployment.yaml",
			},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(t, res))
	}
	if got := resultText(t, res); !strings.Contains(got, "template mychart --show-only templates/deployment.yaml") {
		t.Fatalf("unexpected rendered output: %q", got)
	}
}

func TestResolveChartFields(t *testing.T) {
	tests := []struct {
		name       string
		chart      string
		chartPath  string
		workingDir string
		wantChart  string
		wantDir    string
	}{
		{"chart and default dir", "test", "", "", "test", "."},
		{"chart and explicit dir", "mychart", "", "/workspace", "mychart", "/workspace"},
		{"relative chart_path", "", "./charts/mychart", "", "mychart", "charts"},
		{"relative chart_path with dir", "", "mychart", "/workspace", "mychart", "/workspace"},
		{"absolute chart_path", "", "/workspace/test", "", "test", "/workspace"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotChart, gotDir := resolveChartFields(tt.chart, tt.chartPath, tt.workingDir)
			if gotChart != tt.wantChart || gotDir != tt.wantDir {
				t.Fatalf("resolveChartFields(%q,%q,%q) = (%q,%q), want (%q,%q)",
					tt.chart, tt.chartPath, tt.workingDir, gotChart, gotDir, tt.wantChart, tt.wantDir)
			}
		})
	}
}

func TestDebugHelmResolvesVersionedChartPaths(t *testing.T) {
	root := t.TempDir()
	chartDir := filepath.Join(root, "charts", "example-platform-8.9")
	if err := os.MkdirAll(filepath.Join(chartDir, "templates", "frontend"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("name: example-platform\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(chartDir, "templates", "frontend", "_config.yaml")
	if err := os.WriteFile(source, []byte("security:\n  username: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Mirrors what handleDebugHelm does with chart_path.
	chart, workingDir := resolveChartFields("", "charts/example-platform-8.9", root)
	resolver := templatepath.New(workingDir, chart)
	got, err := resolver.Resolve("example-platform/templates/frontend/_config.yaml")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != source {
		t.Fatalf("Resolve = %q, want %q", got, source)
	}
}

func TestDebugHelmRequiresChart(t *testing.T) {
	session := connectTestClient(t)
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "debug_helm",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool returned protocol error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected a tool error for missing chart, got: %s", resultText(t, res))
	}
}

// debugHelmBinary returns a path to a debug-enabled helm binary, or "".
func debugHelmBinary() string {
	if p := os.Getenv("HELM_DEBUGGER_HELM"); p != "" {
		return p
	}
	candidate := filepath.Join("..", "..", "helm", "bin", "helm")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

// decodeDebugOutput converts the tool's structured content into its typed form.
func decodeDebugOutput(t *testing.T, res *mcp.CallToolResult) debugHelmOutput {
	t.Helper()
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	var out debugHelmOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}
	return out
}

// TestDebugHelmToolVersionedChart is an end-to-end test of the MCP tool against
// a chart whose directory name differs from its Chart.yaml name. It only runs
// when a debug-enabled helm binary and dlv are available.
func TestDebugHelmToolVersionedChart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	helmPath := debugHelmBinary()
	if helmPath == "" {
		t.Skip("no debug-enabled helm binary; set HELM_DEBUGGER_HELM to enable")
	}
	if _, err := exec.LookPath("dlv"); err != nil {
		t.Skip("dlv not found in PATH")
	}

	root := t.TempDir()
	chartDir := filepath.Join(root, "charts", "mychart-1.2.3")
	files := map[string]string{
		"Chart.yaml":  "apiVersion: v2\nname: mychart\nversion: 1.2.3\n",
		"values.yaml": "image:\n  tag: v1\n",
		"templates/_helpers.tpl": `{{- define "mychart.fullname" -}}
{{- printf "%s-%s" .Release.Name .Chart.Name -}}
{{- end -}}`,
		"templates/deployment.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ include "mychart.fullname" . }}
data:
  tag: {{ .Values.image.tag | quote }}
`,
	}
	for rel, content := range files {
		path := filepath.Join(chartDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	session := connectTestClient(t)
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "debug_helm",
		Arguments: map[string]any{
			"chart_path":  "charts/mychart-1.2.3",
			"working_dir": root,
			"helm_path":   helmPath,
			"values":      []any{"image.tag"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool returned protocol error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(t, res))
	}

	out := decodeDebugOutput(t, res)
	if out.FlowCount == 0 {
		t.Fatalf("expected execution flows for a versioned chart, got none (warnings: %v)", out.Warnings)
	}
	if len(out.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", out.Warnings)
	}
	if out.Mode != modeLocate {
		t.Fatalf("default mode = %q, want %q", out.Mode, modeLocate)
	}
	if len(out.Sites) == 0 {
		t.Fatalf("expected structured source sites, got none (report:\n%s)", out.Report)
	}
	if strings.Contains(out.Report, "WriteBuffer") {
		t.Fatalf("locate report must be compact, got:\n%s", out.Report)
	}
	if !strings.Contains(out.Report, "mychart/templates/deployment.yaml") {
		t.Fatalf("report does not mention the deployment template:\n%s", out.Report)
	}
	if !strings.Contains(out.Report, "image.tag") {
		t.Fatalf("report does not mention the queried value:\n%s", out.Report)
	}

	// The rendered-output needle finds the template that wrote the line without
	// the caller knowing the source file.
	res, err = session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "debug_helm",
		Arguments: map[string]any{
			"chart_path":  "charts/mychart-1.2.3",
			"working_dir": root,
			"helm_path":   helmPath,
			"rendered":    []any{`tag: "v1"`},
		},
	})
	if err != nil {
		t.Fatalf("CallTool (rendered needle) returned protocol error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error (rendered needle): %s", resultText(t, res))
	}
	needleOut := decodeDebugOutput(t, res)
	if len(needleOut.Sites) == 0 {
		t.Fatalf("rendered needle found no sites (report:\n%s)", needleOut.Report)
	}
	if !strings.Contains(needleOut.Report, "mychart/templates/deployment.yaml") {
		t.Fatalf("rendered needle did not locate the deployment template:\n%s", needleOut.Report)
	}
}
