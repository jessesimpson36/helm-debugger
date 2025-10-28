package query

import (
	"strings"
	"strconv"
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
