package mcpserver

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jessesimpson36/helm-debugger/internal/breakpoints"
	"github.com/jessesimpson36/helm-debugger/internal/debugger"
	"github.com/jessesimpson36/helm-debugger/internal/report"
	"github.com/jessesimpson36/helm-debugger/internal/settings"
)

func registerTools(server *mcp.Server, logger *log.Logger) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "helm_template",
		Description: "Render a Helm chart with `helm template` and return the rendered manifests. " +
			"Use this for a quick, non-debugging verification of what a chart produces.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input helmTemplateInput) (*mcp.CallToolResult, helmTemplateOutput, error) {
		return handleHelmTemplate(ctx, input)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "debug_helm",
		Description: "Debug a Helm chart with delve and return the template/helper execution flows behind the " +
			"rendered manifests, including the values.yaml options involved. Requires a helm binary built with " +
			"debug symbols (the bundled Docker image provides one).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input debugHelmInput) (*mcp.CallToolResult, debugHelmOutput, error) {
		return handleDebugHelm(ctx, input, logger)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "resolve_breakpoints",
		Description: "Report the text/template/exec.go line numbers the debugger will set breakpoints on for a " +
			"given GOROOT. Useful to verify the debugger is compatible with the Go version a helm binary was built with.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input resolveBreakpointsInput) (*mcp.CallToolResult, resolveBreakpointsOutput, error) {
		return handleResolveBreakpoints(input)
	})
}

type helmTemplateInput struct {
	Chart      string   `json:"chart,omitempty" jsonschema:"name or path of the helm chart to render, relative to working_dir"`
	ChartPath  string   `json:"chart_path,omitempty" jsonschema:"path to the chart directory; alternative to chart+working_dir"`
	ExtraArgs  []string `json:"extra_args,omitempty" jsonschema:"extra arguments passed to 'helm template'"`
	HelmPath   string   `json:"helm_path,omitempty" jsonschema:"path to the helm binary (default: helm)"`
	WorkingDir string   `json:"working_dir,omitempty" jsonschema:"directory chart paths are relative to"`
}

type helmTemplateOutput struct {
	Rendered  string `json:"rendered" jsonschema:"the rendered manifests"`
	Stderr    string `json:"stderr,omitempty" jsonschema:"helm's standard error output"`
	ExitCode  int    `json:"exit_code" jsonschema:"the helm process exit code"`
	Command   string `json:"command" jsonschema:"the full helm command that was executed"`
	Succeeded bool   `json:"succeeded" jsonschema:"whether helm exited successfully"`
}

func handleHelmTemplate(ctx context.Context, input helmTemplateInput) (*mcp.CallToolResult, helmTemplateOutput, error) {
	chart, workingDir := resolveChartFields(input.Chart, input.ChartPath, input.WorkingDir)
	if chart == "" {
		return nil, helmTemplateOutput{}, fmt.Errorf("chart or chart_path is required")
	}
	helmPath := input.HelmPath
	if helmPath == "" {
		helmPath = "helm"
	}
	args := append([]string{"template", chart}, input.ExtraArgs...)

	cmd := exec.CommandContext(ctx, helmPath, args...)
	cmd.Dir = workingDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	exitCode := 0
	runErr := cmd.Run()
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, helmTemplateOutput{}, fmt.Errorf("running helm template: %w", runErr)
		}
	}

	out := helmTemplateOutput{
		Rendered:  stdout.String(),
		Stderr:    stderr.String(),
		ExitCode:  exitCode,
		Command:   fmt.Sprintf("%s %v", helmPath, args),
		Succeeded: runErr == nil,
	}

	text := out.Rendered
	if !out.Succeeded {
		text = fmt.Sprintf("helm template failed (exit %d)\n%s\n%s", out.ExitCode, out.Rendered, out.Stderr)
	}
	return textResult(text), out, nil
}

type debugHelmInput struct {
	Chart          string   `json:"chart,omitempty" jsonschema:"name or path of the helm chart to debug, relative to working_dir"`
	ChartPath      string   `json:"chart_path,omitempty" jsonschema:"path to the chart directory; alternative to chart+working_dir"`
	ExtraArgs      []string `json:"extra_args,omitempty" jsonschema:"extra arguments passed to 'helm template'"`
	Values         []string `json:"values,omitempty" jsonschema:"values.yaml option paths to filter execution flows by, e.g. image.tag"`
	Helpers        []string `json:"helpers,omitempty" jsonschema:"helper/template names to filter by, e.g. mychart.fullname"`
	Templates      []string `json:"templates,omitempty" jsonschema:"template file:line filters, e.g. mychart/templates/deployment.yaml:42"`
	Rendered       []string `json:"rendered,omitempty" jsonschema:"rendered manifest file:line filters"`
	HelmPath       string   `json:"helm_path,omitempty" jsonschema:"path to the debug-enabled helm binary"`
	GoRoot         string   `json:"goroot,omitempty" jsonschema:"GOROOT whose text/template source should be used for breakpoints"`
	WorkingDir     string   `json:"working_dir,omitempty" jsonschema:"directory chart paths are relative to"`
	DebugPort      int      `json:"debug_port,omitempty" jsonschema:"port for the headless delve server; 0 picks a free port"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty" jsonschema:"maximum seconds to wait for the debug run"`
}

type breakpointLines struct {
	LineStart        int `json:"line_start" jsonschema:"(*state).walk line start"`
	RenderedManifest int `json:"rendered_manifest" jsonschema:"rendered manifest write line"`
	ConditionalStart int `json:"conditional_start" jsonschema:"if/with pipeline evaluation line"`
	ConditionalTrue  int `json:"conditional_true" jsonschema:"truthy branch line"`
	ConditionalFalse int `json:"conditional_false" jsonschema:"falsy branch line"`
}

type debugHelmOutput struct {
	Report       string          `json:"report" jsonschema:"human-readable execution flow report"`
	FlowCount    int             `json:"flow_count" jsonschema:"number of execution flows captured"`
	SectionCount int             `json:"section_count" jsonschema:"number of report sections"`
	LineNumbers  breakpointLines `json:"line_numbers" jsonschema:"resolved text/template breakpoint lines"`
	SectionNames []string        `json:"section_names" jsonschema:"names of the report sections"`
}

func handleDebugHelm(ctx context.Context, input debugHelmInput, logger *log.Logger) (*mcp.CallToolResult, debugHelmOutput, error) {
	chart, workingDir := resolveChartFields(input.Chart, input.ChartPath, input.WorkingDir)
	cfg := &settings.Settings{
		ChartName:          chart,
		CommandArgs:        input.ExtraArgs,
		CompiledHelmPath:   input.HelmPath,
		GoRoot:             input.GoRoot,
		WorkingDir:         workingDir,
		DebugPort:          input.DebugPort,
		ValuesQuery:        input.Values,
		HelpersQueryFiles:  input.Helpers,
		TemplateQueryFiles: input.Templates,
		RenderedQueryFiles: input.Rendered,
	}
	if cfg.CompiledHelmPath == "" {
		cfg.CompiledHelmPath = "helm"
	}
	if err := cfg.Validate(); err != nil {
		return nil, debugHelmOutput{}, err
	}

	runCtx, cancel := newTimeoutContext(ctx, input.TimeoutSeconds, 5*time.Minute)
	defer cancel()

	result, err := debugger.Run(runCtx, cfg, logger.Writer())
	if err != nil {
		return nil, debugHelmOutput{}, err
	}

	sections := report.Sections(result.Flows, cfg)
	text := report.Text(sections)

	lines := result.LineNumbers
	out := debugHelmOutput{
		Report:       text,
		FlowCount:    len(result.Flows),
		SectionCount: len(sections),
		LineNumbers: breakpointLines{
			LineStart:        lines.LineStart,
			RenderedManifest: lines.RenderedManifest,
			ConditionalStart: lines.ConditionalStart,
			ConditionalTrue:  lines.ConditionalTrue,
			ConditionalFalse: lines.ConditionalFalse,
		},
	}
	for _, section := range sections {
		out.SectionNames = append(out.SectionNames, section.Name)
	}

	return textResult(text), out, nil
}

type resolveBreakpointsInput struct {
	GoRoot string `json:"goroot,omitempty" jsonschema:"GOROOT to resolve breakpoints from; defaults to the server's own GOROOT"`
}

type resolveBreakpointsOutput struct {
	GoRoot     string          `json:"goroot" jsonschema:"the GOROOT that was inspected"`
	Resolved   bool            `json:"resolved" jsonschema:"whether the line numbers were found in the standard library source"`
	Lines      breakpointLines `json:"lines" jsonschema:"resolved breakpoint line numbers"`
	Error      string          `json:"error,omitempty" jsonschema:"error encountered while resolving"`
	SourceFile string          `json:"source_file" jsonschema:"path to the text/template/exec.go that was inspected"`
}

func handleResolveBreakpoints(input resolveBreakpointsInput) (*mcp.CallToolResult, resolveBreakpointsOutput, error) {
	goroot := input.GoRoot
	if goroot == "" {
		goroot = breakpoints.DefaultGOROOT()
	}
	sourceFile := filepath.Join(goroot, "src", "text", "template", "exec.go")

	resolved, err := breakpoints.Resolve(goroot)
	if err != nil {
		out := resolveBreakpointsOutput{
			GoRoot:     goroot,
			Resolved:   false,
			Lines:      toBreakpointLines(breakpoints.FallbackLines),
			Error:      err.Error(),
			SourceFile: sourceFile,
		}
		return textResult(fmt.Sprintf("could not resolve breakpoints: %v\nusing fallback: %+v", err, out.Lines)), out, nil
	}

	out := resolveBreakpointsOutput{
		GoRoot:     goroot,
		Resolved:   true,
		Lines:      toBreakpointLines(resolved),
		SourceFile: sourceFile,
	}
	return textResult(fmt.Sprintf("resolved breakpoints from %s: %+v", sourceFile, out.Lines)), out, nil
}

func toBreakpointLines(lines breakpoints.LineNumbers) breakpointLines {
	return breakpointLines{
		LineStart:        lines.LineStart,
		RenderedManifest: lines.RenderedManifest,
		ConditionalStart: lines.ConditionalStart,
		ConditionalTrue:  lines.ConditionalTrue,
		ConditionalFalse: lines.ConditionalFalse,
	}
}

func effectiveDir(dir string) string {
	if dir == "" {
		return "."
	}
	return dir
}

// resolveChartFields turns either (chart, working_dir) or chart_path into a
// chart name and a working directory. A relative chart_path is resolved
// against working_dir.
func resolveChartFields(chart, chartPath, workingDir string) (string, string) {
	base := effectiveDir(workingDir)
	if chartPath != "" {
		p := chartPath
		if !filepath.IsAbs(p) {
			p = filepath.Join(base, p)
		}
		p = filepath.Clean(p)
		return filepath.Base(p), filepath.Dir(p)
	}
	return chart, base
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}
