package settings

import (
	"flag"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

type Settings struct {
	ChartName          string
	CommandArgs        []string
	CompiledHelmPath   string
	Mode               string
	GoRoot             string
	WorkingDir         string
	DebugPort          int
	RenderedQueryFiles []string
	TemplateQueryFiles []string
	HelpersQueryFiles  []string
	ValuesQuery        []string
	ResolveValues      bool
	ShowVersion        bool
}

// NewSettings builds settings from command line flags. It is used by the CLI
// modes. The MCP server constructs a Settings value directly instead of going
// through flags.
func NewSettings() *Settings {
	settings := &Settings{}

	commaDelimitedRenderedQueryFiles := ""
	commaDelimitedTemplateQueryFiles := ""
	commaDelimitedHelpersQueryFiles := ""
	commaDelimitedValuesQuery := ""
	rawCommandArgs := ""

	flag.StringVar(&settings.ChartName, "chart", "", "The name of the Helm chart to debug.")
	flag.StringVar(&commaDelimitedRenderedQueryFiles, "rendered-file", "", "Comma-delimited list of query files for rendered manifest.")
	flag.StringVar(&commaDelimitedTemplateQueryFiles, "template-file", "", "Comma-delimited list of query files for templates and helpers.")
	flag.StringVar(&commaDelimitedHelpersQueryFiles, "helper-file", "", "Comma-delimited list of query files for helpers.")
	flag.StringVar(&commaDelimitedValuesQuery, "values", "", "Comma-delimited list of values queries to capture.")
	flag.StringVar(&rawCommandArgs, "extra-command-args", "", "Additional command line arguments to pass to 'helm template' command.")
	flag.StringVar(&settings.Mode, "mode", "model", "Mode of operation: model, mcp")
	flag.StringVar(&settings.CompiledHelmPath, "helm-path", "helm", "Path to the compiled Helm binary.")
	flag.StringVar(&settings.GoRoot, "goroot", "", "GOROOT used to resolve text/template breakpoints. Defaults to the debugger's own GOROOT.")
	flag.StringVar(&settings.WorkingDir, "working-dir", "", "Directory the helm chart paths are relative to. Defaults to the current directory.")
	flag.IntVar(&settings.DebugPort, "debug-port", 0, "Port for the headless delve server. 0 picks a free port.")
	flag.BoolVar(&settings.ResolveValues, "resolve-values", false, "Resolve .Values.* references on each captured line to the value Helm rendered with.")
	flag.BoolVar(&settings.ShowVersion, "version", false, "Print build and toolchain version information, then exit.")

	flag.Parse()

	settings.RenderedQueryFiles = SplitCommaDelimited(commaDelimitedRenderedQueryFiles)
	settings.TemplateQueryFiles = SplitCommaDelimited(commaDelimitedTemplateQueryFiles)
	settings.HelpersQueryFiles = SplitCommaDelimited(commaDelimitedHelpersQueryFiles)
	settings.ValuesQuery = SplitCommaDelimited(commaDelimitedValuesQuery)
	settings.CommandArgs = SplitArgs(rawCommandArgs)

	return settings
}

// SplitCommaDelimited splits a comma-delimited flag value, trimming whitespace
// from each entry and dropping empty entries.
func SplitCommaDelimited(value string) []string {
	if value == "" {
		return nil
	}
	var out []string
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// Validate checks that the settings are sufficient to run the debugger.
func (s *Settings) Validate() error {
	if s.ChartName == "" {
		return fmt.Errorf("chart is required")
	}
	if s.CompiledHelmPath == "" {
		return fmt.Errorf("helm-path is required")
	}
	return nil
}

// ScopedTemplateNames returns the template/helper names that a filter selects,
// suitable for a Delve breakpoint condition on (*Template).Execute. It is empty
// when no filter names a template or helper, in which case no scoped breakpoint
// is installed and the debugger falls back to the walk breakpoint alone.
//
// Helm prefixes define names with the chart name (e.g. "test.serviceAccountName")
// and file templates with the chart path (e.g. "test/templates/deployment.yaml").
// The provided filters may be the bare helper name, the full defined name, or a
// file path; every spelling that could match is included so the condition is a
// safe superset. Exactness is not required because the debugger still applies
// the ordinary query filter to the captured flows.
func (s *Settings) ScopedTemplateNames() []string {
	if s == nil {
		return nil
	}
	var names []string
	names = append(names, s.HelpersQueryFiles...)
	names = append(names, s.TemplateQueryFiles...)
	return names
}

// ScopedOnly reports whether the query can be served entirely by the scoped
// Execute breakpoint, so the per-node walk breakpoint can be dropped.
//
// The walk breakpoint is the only source of two things: rendered write buffers
// and value reads on lines whose source is not known until the run executes.
// A query needs neither when it names helpers/templates and does not ask for a
// rendered substring or a values option. Exact file:line rendered selectors are
// fine: the scoped frame reports the same source locations.
func (s *Settings) ScopedOnly() bool {
	if s == nil {
		return false
	}
	if len(s.ScopedTemplateNames()) == 0 {
		return false
	}
	if len(s.ValuesQuery) > 0 {
		return false
	}
	for _, sel := range s.RenderedQueryFiles {
		if !IsFileLine(sel) {
			return false
		}
	}
	return true
}

// IsFileLine reports whether selector is a "file:line" reference, for example
// "mychart/templates/deployment.yaml:42".
func IsFileLine(selector string) bool {
	parts := strings.Split(selector, ":")
	if len(parts) != 2 || parts[0] == "" {
		return false
	}
	_, err := strconv.Atoi(parts[1])
	return err == nil
}

// EffectiveWorkingDir returns the directory chart paths are resolved from.
func (s *Settings) EffectiveWorkingDir() string {
	if s.WorkingDir != "" {
		return s.WorkingDir
	}
	return "."
}

// ChartDirectory returns the on-disk path of the chart being debugged. The
// resolver uses it to map runtime template names (which helm prefixes with the
// Chart.yaml name) back to source files, including subcharts and .tgz
// dependencies.
func (s *Settings) ChartDirectory() string {
	if s.ChartName == "" {
		return s.EffectiveWorkingDir()
	}
	if filepath.IsAbs(s.ChartName) {
		return filepath.Clean(s.ChartName)
	}
	return filepath.Join(s.EffectiveWorkingDir(), s.ChartName)
}

// Clone returns a shallow copy of the settings with its slice fields copied.
// It is used to derive per-request settings without mutating shared state.
func (s *Settings) Clone() *Settings {
	clone := *s
	clone.CommandArgs = append([]string(nil), s.CommandArgs...)
	clone.RenderedQueryFiles = append([]string(nil), s.RenderedQueryFiles...)
	clone.TemplateQueryFiles = append([]string(nil), s.TemplateQueryFiles...)
	clone.HelpersQueryFiles = append([]string(nil), s.HelpersQueryFiles...)
	clone.ValuesQuery = append([]string(nil), s.ValuesQuery...)
	return &clone
}
