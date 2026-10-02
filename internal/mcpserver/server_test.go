package mcpserver

import (
	"context"
	"strings"
	"testing"

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
