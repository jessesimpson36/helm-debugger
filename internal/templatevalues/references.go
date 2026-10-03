// Package templatevalues extracts .Values.* references from rendered template
// lines and resolves their runtime values against a stopped Helm debuggee.
//
// Helm evaluates .Values.<path> through its Go template data, so the value is
// not present in the template text. At the text/template walk breakpoint the
// current data is available as the `dot` local (and the root data as the `$`
// template variable), and Delve can call their Interface() methods to
// materialize them. This package walks that materialized data (an api.Variable
// tree) along the reference path to report the value Helm actually saw.
package templatevalues

import "strings"

// valuesPrefix is the template data field that holds the merged values.
const valuesPrefix = ".Values."

// Reference is a .Values.* path read by a template line.
type Reference struct {
	// Path is the option path without the leading ".Values.", for example
	// "serviceAccount.name".
	Path string
	// Root is true when the reference is written "$.Values.x" (the root data
	// passed to the template), false for ".Values.x" (the current dot).
	Root bool
}

// Expr renders the reference as it appears in a template. It is also the
// resolved-values map key, so root and dot references to the same path stay
// distinct when a line reads both.
func (r Reference) Expr() string {
	if r.Root {
		return "$.Values." + r.Path
	}
	return ".Values." + r.Path
}

// Contains reports whether line references .Values at all. It is a cheap gate
// so callers can avoid the more expensive reference scan.
func Contains(line string) bool {
	return strings.Contains(line, valuesPrefix)
}

// References returns the distinct .Values.* paths referenced by a template
// source line, with the leading ".Values." removed and without regard to
// whether they are root or dot references. Prefer ParseReferences when the
// distinction matters.
func References(line string) []string {
	var paths []string
	seen := map[string]struct{}{}
	for _, ref := range ParseReferences(line) {
		if _, ok := seen[ref.Path]; ok {
			continue
		}
		seen[ref.Path] = struct{}{}
		paths = append(paths, ref.Path)
	}
	return paths
}

// ParseReferences returns the distinct .Values.* references on a template
// source line, preserving root versus dot. A bare "$" immediately before
// ".Values." marks a root reference ("$.Values.x"); anything else is treated as
// a dot reference.
//
// The scan intentionally works on the raw line rather than a parsed template:
// callers hand it a single source line, which is not a complete template, and
// it must also cope with values referenced inside arguments or pipelines.
func ParseReferences(line string) []Reference {
	var refs []Reference
	seen := map[string]struct{}{}
	for i := 0; ; {
		j := strings.Index(line[i:], valuesPrefix)
		if j < 0 {
			break
		}
		dotIdx := i + j
		start := dotIdx + len(valuesPrefix)
		end := start
		for end < len(line) && isPathChar(line[end]) {
			end++
		}
		i = end
		if end == start {
			continue
		}
		name := strings.TrimRight(line[start:end], ".")
		if name == "" {
			continue
		}
		// A bare "$" immediately before the ".Values." dot means the root data.
		// "$x.Values." (a variable) leaves the preceding byte as an identifier
		// character, so it is not misread as root.
		ref := Reference{Path: name, Root: dotIdx > 0 && line[dotIdx-1] == '$'}
		key := ref.Expr()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		refs = append(refs, ref)
	}
	return refs
}

// isPathChar reports whether c can appear in a .Values path segment. Keys are
// dot-separated identifiers; letters, digits, underscores, and dots cover the
// conventions Helm charts use in template text.
func isPathChar(c byte) bool {
	return c == '.' || c == '_' ||
		(c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9')
}
