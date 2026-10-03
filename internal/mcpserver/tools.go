package mcpserver

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jessesimpson36/helm-debugger/internal/breakpoints"
	"github.com/jessesimpson36/helm-debugger/internal/debugger"
	"github.com/jessesimpson36/helm-debugger/internal/executionflow"
	"github.com/jessesimpson36/helm-debugger/internal/query"
	"github.com/jessesimpson36/helm-debugger/internal/report"
	"github.com/jessesimpson36/helm-debugger/internal/settings"
)

// Report modes accepted by debug_helm.
const (
	modeLocate = "locate"
	modeFull   = "full"
)

func registerTools(server *mcp.Server, logger *log.Logger) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "helm_template",
		Description: "Render a Helm chart with `helm template` and return the rendered manifests. " +
			"Use this to reproduce wrong or surprising output; then pass the offending value path " +
			"or a snippet of the rendered output to `debug_helm` to find the template line that produced it.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input helmTemplateInput) (*mcp.CallToolResult, helmTemplateOutput, error) {
		return handleHelmTemplate(ctx, input)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "debug_helm",
		Description: "Find where a Helm value or a rendered line comes from. Use this while REPRODUCING a " +
			"render problem and BEFORE editing templates, _helpers.tpl, or values.yaml: pass `values` for " +
			"an option that is not taking effect to get the exact template/helper file:line that reads it, " +
			"or pass `rendered` with a snippet of the wrong output to find the template that wrote it. " +
			"Pass `resolve_values` to also report what each .Values.* option on a matched line evaluated to " +
			"at render time (for example `serviceAccount.name = \"\"`). " +
			"Re-run after editing to confirm the flow changed. Returns compact source sites by default; " +
			"set mode=\"full\" for complete execution flows and rendered write buffers. Requires a helm " +
			"binary built with debug symbols (the bundled Docker image provides one).",
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
	Rendered       []string `json:"rendered,omitempty" jsonschema:"rendered output selectors: a file:line source selector, or any other string treated as a substring of the rendered output (e.g. a snippet of the wrong output)"`
	ResolveValues  bool     `json:"resolve_values,omitempty" jsonschema:"also resolve each .Values.* option on matched lines to the value Helm rendered with, e.g. serviceAccount.name = \"\""`
	HelmPath       string   `json:"helm_path,omitempty" jsonschema:"path to the debug-enabled helm binary"`
	GoRoot         string   `json:"goroot,omitempty" jsonschema:"GOROOT whose text/template source should be used for breakpoints"`
	WorkingDir     string   `json:"working_dir,omitempty" jsonschema:"directory chart paths are relative to"`
	DebugPort      int      `json:"debug_port,omitempty" jsonschema:"port for the headless delve server; 0 picks a free port"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty" jsonschema:"maximum seconds to wait for the debug run"`
	Mode           string   `json:"mode,omitempty" jsonschema:"locate (default) returns compact source sites for the query; full returns complete execution flows with rendered write buffers"`
}

type breakpointLines struct {
	LineStart        int `json:"line_start" jsonschema:"(*state).walk line start"`
	RenderedManifest int `json:"rendered_manifest" jsonschema:"rendered manifest write line"`
}

type debugHelmOutput struct {
	Mode           string          `json:"mode" jsonschema:"report mode used: locate or full"`
	Report         string          `json:"report" jsonschema:"human-readable execution flow report"`
	FlowCount      int             `json:"flow_count" jsonschema:"number of execution flows captured"`
	SectionCount   int             `json:"section_count" jsonschema:"number of report sections"`
	LineNumbers    breakpointLines `json:"line_numbers" jsonschema:"resolved text/template breakpoint lines"`
	SectionNames   []string        `json:"section_names" jsonschema:"names of the report sections"`
	Sites          []report.Site   `json:"sites,omitempty" jsonschema:"template/helper source locations behind the matched flows (locate mode)"`
	RelevantValues []string        `json:"relevant_values,omitempty" jsonschema:"values.yaml options referenced by the matched flows"`
	Suggestions    []string        `json:"suggestions,omitempty" jsonschema:"when a query matches nothing: nearest known value/helper/template names to retry with"`
	Warnings       []string        `json:"warnings,omitempty" jsonschema:"template sources that could not be resolved to files; explains an empty or partial flow set"`
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
		ResolveValues:      input.ResolveValues,
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

	out := summarize(result, cfg, input.Mode)
	return textResult(out.Report), out, nil
}

// summarize turns a debug run into the MCP response. mode=locate (the default)
// returns compact source sites so the caller can cheaply answer "where is this
// value read / what writes this output" before editing; mode=full returns the
// complete execution flows with rendered write buffers.
func summarize(result *debugger.Result, cfg *settings.Settings, mode string) debugHelmOutput {
	if mode != modeFull {
		mode = modeLocate
	}
	lines := result.LineNumbers
	out := debugHelmOutput{
		Mode:      mode,
		FlowCount: len(result.Flows),
		Warnings:  result.Warnings,
		LineNumbers: breakpointLines{
			LineStart:        lines.LineStart,
			RenderedManifest: lines.RenderedManifest,
		},
	}

	if mode == modeFull {
		sections := report.Sections(result.Flows, cfg)
		out.SectionCount = len(sections)
		out.SectionNames = sectionNames(sections)
		out.Report = report.WarningsText(result.Warnings) + report.Text(sections)
		return out
	}

	located := report.Locate(result.Flows, cfg)
	out.SectionCount = len(located.Sections)
	out.SectionNames = sectionNames(located.Sections)
	out.Sites = located.Sites
	out.RelevantValues = located.RelevantValues
	// A query that matches nothing is the easy case to get silently wrong; say
	// so and suggest nearby names instead of returning an empty report.
	if len(located.Sites) == 0 && hasQuery(cfg) {
		out.Suggestions = suggestions(result.Flows, cfg)
	}
	out.Report = report.WarningsText(result.Warnings) + report.LocateText(located, out.Suggestions)
	return out
}

func sectionNames(sections []report.Section) []string {
	if len(sections) == 0 {
		return nil
	}
	names := make([]string, 0, len(sections))
	for _, section := range sections {
		names = append(names, section.Name)
	}
	return names
}

func hasQuery(cfg *settings.Settings) bool {
	if cfg == nil {
		return false
	}
	return len(cfg.ValuesQuery) > 0 || len(cfg.HelpersQueryFiles) > 0 ||
		len(cfg.TemplateQueryFiles) > 0 || len(cfg.RenderedQueryFiles) > 0
}

// suggestions returns "did you mean" hints for a query that matched nothing. It
// matches the query terms against every value, helper, and template the chart
// actually reads.
func suggestions(flows []*executionflow.ExecutionFlow, cfg *settings.Settings) []string {
	terms := collectTerms(cfg)
	if len(terms) == 0 {
		return nil
	}
	var out []string
	out = append(out, matchTerms(terms, query.KnownValueNames(flows), "values")...)
	out = append(out, matchTerms(terms, query.KnownHelperNames(flows), "helper")...)
	out = append(out, matchTerms(terms, query.KnownTemplateFiles(flows), "template")...)
	out = dedupeStrings(out)
	if len(out) == 0 {
		out = append(out, "no similar values.yaml options, helpers, or templates are read by this chart; check the chart name, extra_args, and the query")
	}
	return out
}

func collectTerms(cfg *settings.Settings) []string {
	if cfg == nil {
		return nil
	}
	var terms []string
	terms = append(terms, cfg.ValuesQuery...)
	terms = append(terms, cfg.HelpersQueryFiles...)
	terms = append(terms, cfg.TemplateQueryFiles...)
	return terms
}

func matchTerms(terms, known []string, kind string) []string {
	var out []string
	for _, term := range terms {
		lower := strings.ToLower(strings.TrimSpace(term))
		if lower == "" {
			continue
		}
		for _, name := range known {
			if strings.Contains(strings.ToLower(name), lower) {
				out = append(out, fmt.Sprintf("%s: %s", kind, name))
			}
		}
	}
	return out
}

func dedupeStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
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
