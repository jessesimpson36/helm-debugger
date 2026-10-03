// Package includegraph builds the call graph of Helm template helpers from the
// chart's source. It answers "which helper definitions can run while this helper
// runs?" by following literal `include "x"` / `template "x"` calls, so the
// debugger can scope its per-node walk breakpoint to exactly the definitions a
// queried helper reaches.
//
// Resolution is static and deliberately conservative. A body that calls a
// helper through a non-literal name (for example
// `include (printf "%s.fullname" .component)` or `include $helper`) cannot be
// resolved statically; such a definition is marked Unresolved and its callers
// are treated as possibly reaching anything, so the caller falls back to a
// wider capture instead of silently missing lines.
package includegraph

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Graph is the resolved helper call graph for one chart.
type Graph struct {
	// calls maps a defined template name to the set of defined names its body
	// resolves to (transitively via Closure; directly via directCalls).
	directCalls map[string]map[string]struct{}
	// unresolved records definitions whose body contains a call that could not
	// be resolved statically. A caller of such a definition cannot know what it
	// reaches.
	unresolved map[string]bool
	// defined is the set of every template name defined anywhere in the chart.
	defined map[string]bool
}

// defineRe matches `define "name"` in a template file.
var defineRe = regexp.MustCompile(`\{\{-?\s*define\s+"([^"]+)"`)

// callRe matches the start of a literal include/template call, capturing the
// literal name: `include "x"` or `template "x"`.
var callRe = regexp.MustCompile(`(?:include|template)\s+"([^"]+)"`)

// dynamicCallRe matches an include/template whose name is not a string literal,
// e.g. `include (printf ...)` or `include $helper`. It is used only to mark the
// enclosing definition as unresolved.
var dynamicCallRe = regexp.MustCompile(`(?:include|template)\s+[^"\s]`)

// Load parses every template source file under chartDir and returns the graph.
// Files that cannot be read are skipped; the graph simply does not know about
// their definitions, which makes callers conservative.
func Load(chartDir string) (*Graph, error) {
	g := &Graph{
		directCalls: map[string]map[string]struct{}{},
		unresolved:  map[string]bool{},
		defined:     map[string]bool{},
	}

	var files []string
	err := filepath.WalkDir(chartDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			// charts/ may hold .tgz dependencies, which are not parsed.
			if path != chartDir && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(d.Name())
		if ext == ".tpl" || ext == ".yaml" || ext == ".yml" || ext == ".txt" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	for _, path := range files {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}
		g.parseFile(string(data))
	}
	return g, nil
}

// parseFile registers every definition in a file and the literal calls in each
// definition's body.
func (g *Graph) parseFile(content string) {
	for _, seg := range splitDefinitions(content) {
		if seg.name == "" {
			continue
		}
		g.defined[seg.name] = true
		calls := map[string]struct{}{}
		for _, m := range callRe.FindAllStringSubmatch(seg.body, -1) {
			calls[m[1]] = struct{}{}
		}
		g.directCalls[seg.name] = calls
		if dynamicCallRe.MatchString(seg.body) {
			g.unresolved[seg.name] = true
		}
	}
}

// definition is one `define "name" ... end` block.
type definition struct {
	name string
	body string
}

// splitDefinitions returns the define blocks in content. It is a lightweight
// scanner rather than a template parser: real charts use `{{- define "x" -}}`
// ... `{{- end }}` and the block boundaries are unambiguous enough for a regex
// split, which keeps this dependency-free and robust to unrelated syntax.
func splitDefinitions(content string) []definition {
	locs := defineRe.FindAllStringSubmatchIndex(content, -1)
	var out []definition
	for i, loc := range locs {
		name := content[loc[2]:loc[3]]
		bodyStart := loc[1]
		bodyEnd := len(content)
		if i+1 < len(locs) {
			bodyEnd = locs[i+1][0]
		}
		out = append(out, definition{name: name, body: content[bodyStart:bodyEnd]})
	}
	return out
}

// Defined reports whether name is defined in the chart.
func (g *Graph) Defined(name string) bool {
	return g.defined[name]
}

// Unresolved reports whether name's body contains a call that could not be
// resolved statically, so its reach cannot be bounded.
func (g *Graph) Unresolved(name string) bool {
	return g.unresolved[name]
}

// Closure returns the set of definition names reachable from roots by following
// literal includes, including the roots themselves. A root whose body (or that
// of any reached definition) is unresolved is reported in the second return
// value as "unbounded", meaning the caller should widen its capture rather than
// trust the set.
func (g *Graph) Closure(roots []string) (names []string, unbounded bool) {
	seen := map[string]struct{}{}
	stack := append([]string(nil), roots...)
	for len(stack) > 0 {
		name := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		if g.unresolved[name] {
			unbounded = true
		}
		for callee := range g.directCalls[name] {
			stack = append(stack, callee)
		}
	}
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, unbounded
}

// Names loads the chart at chartDir and returns the transitive closure of
// template names reachable from roots by literal includes, plus whether any
// reached definition has an unresolvable (dynamic) call. It is a convenience
// over Load+Closure for callers that just want the set.
func Names(chartDir string, roots []string) ([]string, bool) {
	g, err := Load(chartDir)
	if err != nil {
		// Without a graph, be conservative: signal unbounded so the caller falls
		// back to a wider capture rather than trusting an empty set.
		return nil, true
	}
	return g.Closure(roots)
}

// ClosureCond builds a Delve breakpoint condition string that is true when the
// executing template's name matches any name in the closure. The caller uses it
// on text/template.(*Template).Execute; the returned string is empty when names
// is empty.
func ClosureCond(names []string) string {
	if len(names) == 0 {
		return ""
	}
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, `t.name == "`+strings.ReplaceAll(name, `"`, `\"`)+`"`)
	}
	return strings.Join(parts, " || ")
}
