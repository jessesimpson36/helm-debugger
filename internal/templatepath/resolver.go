// Package templatepath maps Go text/template names back to on-disk source
// files.
//
// Helm names templates by the chart's Chart.yaml name, not by the directory the
// chart lives in. For a versioned chart layout such as example-platform-8.9/
// (chart name "example-platform") the template name reported by Go at runtime is
// "example-platform/templates/...", which does not exist relative to the working
// directory. The Resolver rewrites those names to the chart's actual directory
// so the debugger can read the source line behind a breakpoint.
package templatepath

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Resolver maps runtime template names to files on disk.
type Resolver struct {
	workingDir string
	chartDir   string
	chartName  string
}

// New builds a Resolver for the chart at chart. The chart may be an absolute
// path, a path relative to workingDir, or a bare chart name. A missing
// Chart.yaml is tolerated: chartName then falls back to the directory base name.
func New(workingDir, chart string) *Resolver {
	r := &Resolver{workingDir: workingDir}
	if chart == "" {
		return r
	}

	dir := chart
	if !filepath.IsAbs(dir) {
		base := workingDir
		if base == "" {
			base = "."
		}
		dir = filepath.Join(base, chart)
	}
	r.chartDir = dir
	r.chartName = readChartName(dir)
	if r.chartName == "" {
		r.chartName = filepath.Base(dir)
	}
	return r
}

// ChartDir returns the on-disk chart directory the resolver was built for.
func (r *Resolver) ChartDir() string { return r.chartDir }

// ChartName returns the runtime chart name read from Chart.yaml.
func (r *Resolver) ChartName() string { return r.chartName }

// Resolve returns the on-disk path for a runtime template name. When name is
// already an absolute path it is returned as-is. A missing file is reported as
// an error rather than silently returning a path that does not exist.
func (r *Resolver) Resolve(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty template name")
	}
	if filepath.IsAbs(name) {
		if fileExists(name) {
			return name, nil
		}
		return name, fmt.Errorf("template source %s does not exist", name)
	}

	for _, candidate := range r.candidates(filepath.ToSlash(name)) {
		if fileExists(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf(
		"no file matched template %q (chart directory %q, chart name %q)",
		name, r.chartDir, r.chartName,
	)
}

// candidates returns the paths to try for a template name, in priority order.
// The first two cover the common cases: the name as a path relative to the
// working directory, and the name with the chart-name prefix replaced by the
// chart directory. The remaining entries are best-effort fallbacks for a chart
// whose Chart.yaml name could not be read and for vendored subchart templates.
func (r *Resolver) candidates(name string) []string {
	var out []string
	add := func(p string) {
		if p == "" {
			return
		}
		for _, existing := range out {
			if existing == p {
				return
			}
		}
		out = append(out, p)
	}

	if r.workingDir != "" {
		add(filepath.Join(r.workingDir, name))
	}
	if r.chartDir != "" {
		if r.chartName != "" && strings.HasPrefix(name, r.chartName+"/") {
			add(filepath.Join(r.chartDir, strings.TrimPrefix(name, r.chartName+"/")))
		}
		if i := strings.IndexByte(name, '/'); i >= 0 {
			add(filepath.Join(r.chartDir, name[i+1:]))
		}
		add(filepath.Join(r.chartDir, name))
	}
	return out
}

// readChartName extracts the top-level name field from a chart's Chart.yaml. It
// is a tiny scanner rather than a YAML dependency, matching the approach used
// for the standard library breakpoints.
func readChartName(chartDir string) string {
	file, err := os.Open(filepath.Join(chartDir, "Chart.yaml"))
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		if !strings.HasPrefix(line, "name:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "name:"))
		if i := strings.Index(value, " #"); i >= 0 {
			value = strings.TrimSpace(value[:i])
		}
		value = strings.Trim(value, `"'`)
		if value != "" {
			return value
		}
	}
	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
