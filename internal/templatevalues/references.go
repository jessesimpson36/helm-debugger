// Package templatevalues extracts .Values.* references from rendered template
// lines and resolves their runtime values against a stopped Helm debuggee.
//
// Helm evaluates .Values.<path> through its Go template data, so the value is
// not present in the template text. At the text/template walk breakpoint the
// current data is available as the `dot` local, and Delve can call
// dot.Interface() to materialize it. This package walks that materialized data
// (an api.Variable tree) along the reference path to report the value Helm
// actually saw.
package templatevalues

import "strings"

// valuesPrefix is the template data field that holds the merged values.
const valuesPrefix = ".Values."

// Contains reports whether line references .Values at all. It is a cheap gate
// so callers can avoid the more expensive reference scan.
func Contains(line string) bool {
	return strings.Contains(line, valuesPrefix)
}

// References returns the distinct .Values.* paths referenced by a template
// source line, with the leading ".Values." removed. For example
//
//	image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
//
// yields ["image.repository", "image.tag"].
//
// The scan intentionally works on the raw line rather than a parsed template:
// callers hand it a single source line, which is not a complete template, and
// it must also cope with values referenced inside arguments or pipelines.
func References(line string) []string {
	var refs []string
	seen := map[string]struct{}{}
	for i := 0; ; {
		j := strings.Index(line[i:], valuesPrefix)
		if j < 0 {
			break
		}
		start := i + j + len(valuesPrefix)
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
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		refs = append(refs, name)
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
