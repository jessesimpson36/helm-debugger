package query

import (
	"sort"
	"strconv"
	"strings"

	"github.com/jessesimpson36/helm-debugger/internal/executionflow"
)

// This module filters out execution flows based on a query

func QueryValuesReference(flows []*executionflow.ExecutionFlow, selectedValues []string) []*executionflow.ExecutionFlow {
	filteredFlows := []*executionflow.ExecutionFlow{}
	for _, flow := range flows {
		found := false
		for _, selectedValue := range selectedValues {
			for _, valRef := range flow.ValuesReference {
				if strings.HasPrefix(valRef.ValuesName, selectedValue) {
					filteredFlows = append(filteredFlows, flow)
					found = true
					break
				}
			}
			if found {
				break
			}
		}
	}
	return filteredFlows
}

func QueryHelpers(flows []*executionflow.ExecutionFlow, selectedHelpers []string) []*executionflow.ExecutionFlow {
	filteredFlows := []*executionflow.ExecutionFlow{}
	for _, flow := range flows {
		found := false
		for _, selectedHelper := range selectedHelpers {
			for _, helper := range flow.Helpers {
				if strings.HasPrefix(helper.FunctionName, selectedHelper) {
					filteredFlows = append(filteredFlows, flow)
					found = true
					break
				}
			}
			if found {
				break
			}
		}
	}
	return filteredFlows
}

// selectedTemplateLineNumbers are line numbers in string format filename:lineNumber (e.g., "templates/deployment.yaml:300")
func QueryTemplate(flows []*executionflow.ExecutionFlow, selectedTemplateLineNumbers []string) []*executionflow.ExecutionFlow {
	filteredFlows := []*executionflow.ExecutionFlow{}
	for _, flow := range flows {
		found := false
		for _, fileLineCombo := range selectedTemplateLineNumbers {
			splitOutput := strings.Split(fileLineCombo, ":")
			if len(splitOutput) != 2 {
				continue
			}
			selectedFile := splitOutput[0]
			selectedLineStr := splitOutput[1]
			selectedLine, err := strconv.Atoi(selectedLineStr)
			if err != nil {
				continue
			}
			if strings.HasPrefix(flow.Template.FileName, selectedFile) && flow.Template.LineNumber == selectedLine {
				filteredFlows = append(filteredFlows, flow)
				found = true
				break
			}
			for _, helper := range flow.Helpers {
				if strings.HasPrefix(helper.FileName, selectedFile) && helper.LineNumber == selectedLine {
					filteredFlows = append(filteredFlows, flow)
					found = true
					break
				}
			}
			if found {
				break
			}
		}
	}
	return filteredFlows
}

// selectedRenderedTemplateLineNumbers are line numbers in string format filename:lineNumber (e.g., "templates/deployment.yaml:300")
func QueryRenderedTemplate(flows []*executionflow.ExecutionFlow, selectedRenderedTemplateLineNumbers []string) []*executionflow.ExecutionFlow {
	filteredFlows := []*executionflow.ExecutionFlow{}
	for _, flow := range flows {
		found := false
		for _, fileLineCombo := range selectedRenderedTemplateLineNumbers {
			splitOutput := strings.Split(fileLineCombo, ":")
			if len(splitOutput) != 2 {
				continue
			}
			selectedFile := splitOutput[0]
			selectedLineStr := splitOutput[1]
			selectedLine, err := strconv.Atoi(selectedLineStr)
			if err != nil {
				continue
			}
			if strings.HasPrefix(flow.Template.FileName, selectedFile) {
				if len(flow.RenderedManifest) > 1 {
					first := flow.RenderedManifest[0]
					last := flow.RenderedManifest[len(flow.RenderedManifest)-1]
					lowerBound := len(strings.Split(first.Content, "\n"))
					upperBound := len(strings.Split(last.Content, "\n"))
					if selectedLine > lowerBound && selectedLine < upperBound {
						filteredFlows = append(filteredFlows, flow)
						found = true
						break
					}

				} else if len(flow.RenderedManifest) == 1 {
					if len(strings.Split(flow.RenderedManifest[0].Content, "\n")) > selectedLine {
						filteredFlows = append(filteredFlows, flow)
						found = true
						break
					}
				}
			}
		}
		if found {
			break
		}
	}
	return filteredFlows
}

// IsFileLine reports whether selector is a "file:line" reference as accepted by
// QueryTemplate and QueryRenderedTemplate, for example
// "my chart/templates/deployment.yaml:42".
func IsFileLine(selector string) bool {
	parts := strings.Split(selector, ":")
	if len(parts) != 2 || parts[0] == "" {
		return false
	}
	if _, err := strconv.Atoi(parts[1]); err != nil {
		return false
	}
	return true
}

// QueryRendered matches flows against rendered-output selectors. A selector
// that looks like "file:line" matches the template or helper source that wrote
// that rendered line. Any other selector is treated as a substring needle and
// matched against the rendered output captured by the flow, which lets a caller
// paste a line of wrong output instead of a source location.
func QueryRendered(flows []*executionflow.ExecutionFlow, selectors []string) []*executionflow.ExecutionFlow {
	var fileLines, needles []string
	for _, selector := range selectors {
		if IsFileLine(selector) {
			fileLines = append(fileLines, selector)
		} else if strings.TrimSpace(selector) != "" {
			needles = append(needles, selector)
		}
	}
	matched := QueryRenderedTemplate(flows, fileLines)
	matched = append(matched, QueryRenderedNeedle(flows, needles)...)
	return dedupeFlows(matched)
}

// QueryRenderedNeedle matches flows whose captured rendered output contains any
// of the given substrings.
func QueryRenderedNeedle(flows []*executionflow.ExecutionFlow, needles []string) []*executionflow.ExecutionFlow {
	if len(needles) == 0 {
		return nil
	}
	filteredFlows := []*executionflow.ExecutionFlow{}
	for _, flow := range flows {
		if flowRenderedContains(flow, needles) {
			filteredFlows = append(filteredFlows, flow)
		}
	}
	return filteredFlows
}

func flowRenderedContains(flow *executionflow.ExecutionFlow, needles []string) bool {
	if flow == nil || len(flow.RenderedManifest) == 0 {
		return false
	}
	added := flowRenderedAdditions(flow)
	for _, needle := range needles {
		if needle != "" && strings.Contains(added, needle) {
			return true
		}
	}
	return false
}

// flowRenderedAdditions returns the output a flow contributed: the growth of its
// rendered buffer from the first captured state to the last. Matching against
// this instead of the whole buffer keeps a needle from matching a line that a
// previous flow wrote and that merely persists in later starting buffers.
func flowRenderedAdditions(flow *executionflow.ExecutionFlow) string {
	first := flow.RenderedManifest[0]
	if len(flow.RenderedManifest) == 1 {
		return first.Content
	}
	last := flow.RenderedManifest[len(flow.RenderedManifest)-1]
	if strings.HasPrefix(last.Content, first.Content) {
		return strings.TrimPrefix(last.Content, first.Content)
	}
	return last.Content
}

func dedupeFlows(flows []*executionflow.ExecutionFlow) []*executionflow.ExecutionFlow {
	seen := make(map[*executionflow.ExecutionFlow]struct{}, len(flows))
	out := make([]*executionflow.ExecutionFlow, 0, len(flows))
	for _, flow := range flows {
		if flow == nil {
			continue
		}
		if _, ok := seen[flow]; ok {
			continue
		}
		seen[flow] = struct{}{}
		out = append(out, flow)
	}
	return out
}

// KnownValueNames returns the sorted, unique values.yaml paths referenced by
// any flow. It is used to suggest alternatives when a query matches nothing.
func KnownValueNames(flows []*executionflow.ExecutionFlow) []string {
	names := map[string]struct{}{}
	for _, flow := range flows {
		if flow == nil {
			continue
		}
		for _, valRef := range flow.ValuesReference {
			if valRef.ValuesName != "" {
				names[valRef.ValuesName] = struct{}{}
			}
		}
	}
	return sortedKeys(names)
}

// KnownHelperNames returns the sorted, unique helper function names invoked by
// any flow.
func KnownHelperNames(flows []*executionflow.ExecutionFlow) []string {
	names := map[string]struct{}{}
	for _, flow := range flows {
		if flow == nil {
			continue
		}
		for _, helper := range flow.Helpers {
			if helper.FunctionName != "" && helper.FunctionName != helper.FileName {
				names[helper.FunctionName] = struct{}{}
			}
		}
	}
	return sortedKeys(names)
}

// KnownTemplateFiles returns the sorted, unique template file names that
// produced any flow.
func KnownTemplateFiles(flows []*executionflow.ExecutionFlow) []string {
	names := map[string]struct{}{}
	for _, flow := range flows {
		if flow == nil || flow.Template == nil {
			continue
		}
		if flow.Template.FileName != "" {
			names[flow.Template.FileName] = struct{}{}
		}
	}
	return sortedKeys(names)
}

func sortedKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
