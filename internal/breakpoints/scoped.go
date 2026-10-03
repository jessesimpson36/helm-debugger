package breakpoints

import (
	"sort"
	"strings"

	"github.com/go-delve/delve/service/api"
	"github.com/jessesimpson36/helm-debugger/internal/frame"
	"github.com/jessesimpson36/helm-debugger/internal/frame/delegate"
)

// includeFuncNames are the Helm template functions whose calls become helper
// frames. `include` and `tpl` both execute a named template, which at
// text/template.(*Template).Execute entry carries the target name in t.name.
var includeFuncNames = []string{"include", "tpl"}

// TemplateExecuteCond builds a Delve breakpoint condition that is true only when
// the executing template's name matches one of names. The debugger installs it
// on text/template.(*Template).Execute, which is reached once per template/helper
// invocation instead of once per node (walk), so Delve filters out almost every
// stop in the debugger process and only notifies the client for matches.
//
// Delve evaluates Cond where `t` is the Execute receiver and t.name is a plain
// string field, so the expression is valid for every invocation. No method calls
// (Delve rejects them in conditions) and no type assertions (Delve has no
// short-circuit, so a failing assertion would stop anyway) are used.
func TemplateExecuteCond(names []string) string {
	names = cleanNames(names)
	if len(names) == 0 {
		return ""
	}
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, `t.name == "`+escapeCondString(name)+`"`)
	}
	return strings.Join(parts, " || ")
}

// GetTemplateExecuteFrame returns a frame that captures one execution unit per
// matched template/helper invocation. It reads only fields valid at Execute
// entry, all through t (the template being executed):
//
//   - FunctionName: t.name, e.g. "test.serviceAccountName"
//   - FileName:     t.ParseName, e.g. "test/templates/_helpers.tpl"
//   - LineNumber:   t.Tree.Root's source line, derived from t.Tree.text and
//     t.Tree.Root.Pos by the frame's resolver
//
// The breakpoint is scoped by cond so it fires only for names the caller asked
// about.
func GetTemplateExecuteFrame(lines LineNumbers, cond string) *delegate.DelegateFrame {
	bp := &api.Breakpoint{
		Name: "templateexecute",
		File: "text/template/exec.go",
		// Execute is the name lookup + call into execute; breaking at its first
		// statement keeps the receiver t live.
		Line: lines.Execute,
		Cond: cond,
	}

	reqVars := []string{
		"t.name",
		"t.ParseName",
		"t.Tree.text",
		"t.Tree.Root.Pos",
	}

	mapper := frame.Mapper{
		"FunctionName": "t.name",
		"FileName":     "t.ParseName",
		"RootOffset":   "t.Tree.Root.Pos",
		"TreeText":     "t.Tree.text",
	}

	return &delegate.DelegateFrame{
		Breakpoints: []*api.Breakpoint{bp},
		ReqVars:     reqVars,
		Mapper:      mapper,
	}
}

// cleanNames trims, drops empties, and dedupes names while preserving order.
func cleanNames(names []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

// escapeCondString escapes a value for inclusion in a Delve double-quoted string
// literal. The names are template/helper identifiers in practice, but this keeps
// the generated condition valid regardless.
func escapeCondString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `"`, `\"`)
}

// SortedNames returns names sorted, for deterministic conditions and tests.
func SortedNames(names []string) []string {
	out := cleanNames(names)
	sort.Strings(out)
	return out
}
