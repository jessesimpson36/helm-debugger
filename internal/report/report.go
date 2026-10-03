// Package report turns captured execution flows into the human-readable
// sections printed by the CLI and returned by the MCP server.
package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/jessesimpson36/helm-debugger/internal/executionflow"
	"github.com/jessesimpson36/helm-debugger/internal/frame"
	"github.com/jessesimpson36/helm-debugger/internal/query"
	"github.com/jessesimpson36/helm-debugger/internal/settings"
)

// Section is a named group of flows, usually the result of one query type.
type Section struct {
	Name  string
	Flows []*executionflow.ExecutionFlow
}

// Sections applies the queries in cfg to flows. When no query is configured it
// returns a single section containing every flow. The order matches the
// historical CLI output: values, helpers, template, rendered.
func Sections(flows []*executionflow.ExecutionFlow, cfg *settings.Settings) []Section {
	var sections []Section
	if len(cfg.ValuesQuery) > 0 {
		sections = append(sections, Section{
			Name:  "VALUES QUERY",
			Flows: query.QueryValuesReference(flows, cfg.ValuesQuery),
		})
	}
	if len(cfg.HelpersQueryFiles) > 0 {
		sections = append(sections, Section{
			Name:  "HELPERS QUERY",
			Flows: query.QueryHelpers(flows, cfg.HelpersQueryFiles),
		})
	}
	if len(cfg.TemplateQueryFiles) > 0 {
		sections = append(sections, Section{
			Name:  "TEMPLATE QUERY",
			Flows: query.QueryTemplate(flows, cfg.TemplateQueryFiles),
		})
	}
	if len(cfg.RenderedQueryFiles) > 0 {
		sections = append(sections, Section{
			Name:  "RENDERED QUERY",
			Flows: query.QueryRendered(flows, cfg.RenderedQueryFiles),
		})
	}
	if len(sections) == 0 {
		sections = append(sections, Section{Name: "EXECUTION FLOWS", Flows: flows})
	}
	return sections
}

// Write renders all sections to w.
func Write(w io.Writer, sections []Section) {
	for _, section := range sections {
		WriteSection(w, section)
	}
}

// WriteSection renders one section to w.
func WriteSection(w io.Writer, section Section) {
	fmt.Fprintf(w, "================= %s =================\n", section.Name)
	if len(section.Flows) == 0 {
		fmt.Fprintln(w, "  (no flows matched this query)")
	}
	for _, flow := range section.Flows {
		writeFlow(w, flow)
	}
}

// Text renders all sections to a string.
func Text(sections []Section) string {
	var b strings.Builder
	Write(&b, sections)
	return b.String()
}

// WarningsText renders source-resolution warnings as a section. It returns an
// empty string when there are none, so callers can concatenate it
// unconditionally.
func WarningsText(warnings []string) string {
	if len(warnings) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintln(&b, "================= WARNINGS =================")
	for _, warning := range warnings {
		fmt.Fprintf(&b, "- %s\n", warning)
	}
	return b.String()
}

// dedupeValuesReferences collapses repeated options within a flow, keeping the
// first occurrence (and its resolved value). A gated run can report the same
// option from more than one captured line.
func dedupeValuesReferences(refs []*executionflow.ValuesReference) []*executionflow.ValuesReference {
	seen := map[string]struct{}{}
	out := make([]*executionflow.ValuesReference, 0, len(refs))
	for _, ref := range refs {
		key := ref.ValuesName
		if ref.Root {
			key = "$." + key
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, ref)
	}
	return out
}

func writeFlow(w io.Writer, flow *executionflow.ExecutionFlow) {
	if flow == nil {
		return
	}
	if flow.Template != nil {
		_ = flow.Template.Display(w, false)
	} else if flow.Owner != "" {
		// The walk breakpoint was gated, so the template's own lines were not
		// captured; still name the template the flow belongs to.
		fmt.Fprintf(w, "%s\n", flow.Owner)
	}
	for _, helper := range flow.Helpers {
		_ = helper.Display(w, true)
	}
	fmt.Fprintln(w, "Relevant Values")
	for _, valRef := range dedupeValuesReferences(flow.ValuesReference) {
		name := valRef.ValuesName
		if valRef.Root {
			name = "$.Values." + valRef.ValuesName
		}
		switch {
		case valRef.Resolved && valRef.Found:
			fmt.Fprintf(w, "- %s = %s\n", name, valRef.Values)
		case valRef.Resolved:
			fmt.Fprintf(w, "- %s = <unset>\n", name)
		default:
			fmt.Fprintf(w, "- %s\n", name)
		}
	}
	fmt.Fprintln(w, "WriteBuffer")
	var prevBuffer *frame.RenderedLine
	for _, capturedBuffer := range flow.RenderedManifest {
		if prevBuffer != nil {
			if strings.HasPrefix(capturedBuffer.Content, prevBuffer.Content) {
				added := strings.TrimPrefix(capturedBuffer.Content, prevBuffer.Content)
				// A leading newline terminates the previous line rather than
				// starting a new, empty one.
				added = strings.TrimPrefix(added, "\n")
				for _, line := range strings.Split(added, "\n") {
					fmt.Fprintf(w, "+     %s\n", line)
				}
			}
		} else {
			for i, line := range strings.Split(capturedBuffer.Content, "\n") {
				fmt.Fprintf(w, "%4d  %s\n", i, line)
			}
		}
		prevBuffer = capturedBuffer
	}
	fmt.Fprintln(w, "--------------------------------------------------")
}
