package executionflow

import (
	"github.com/jessesimpson36/helm-debugger/internal/frame"
	"strings"
)

type ExecutionFlow struct {
	Template         *frame.ExecutionUnit
	Helpers          []*frame.ExecutionUnit
	ValuesReference  []*ValuesReference
	RenderedManifest []*frame.RenderedLine
}

type ValuesReference struct {
	ExecutionUnit *frame.ExecutionUnit
	ValuesName    string
	Values        string
}

func ContainsValuesReference(execUnit *frame.ExecutionUnit) bool {
	if strings.Contains(execUnit.LineContent, "Values.") {
		return true
	}
	return false
}

func GetValuesReferences(execUnit *frame.ExecutionUnit) []string {
	valuesRefs := []string{}
	words := strings.Fields(execUnit.LineContent)
	for _, word := range words {
		if strings.HasPrefix(word, ".Values.") {
			cleaned := strings.TrimSuffix(strings.TrimPrefix(word, ".Values."), ",")
			valuesRefs = append(valuesRefs, cleaned)
		}
	}
	return valuesRefs
}

func FillValuesReferences(flow *ExecutionFlow, execUnit *frame.ExecutionUnit) {
	if ContainsValuesReference(execUnit) {
		valuesNames := GetValuesReferences(execUnit)
		for _, valName := range valuesNames {
			valRef := &ValuesReference{
				ExecutionUnit: execUnit,
				ValuesName:    valName,
				Values:        "", // Placeholder, actual value retrieval logic needed
			}
			flow.ValuesReference = append(flow.ValuesReference, valRef)
		}
	}
}

func IsTemplate(execUnit *frame.ExecutionUnit) bool {
	// if function name == filename then it's a template, not a helper
	if execUnit == nil {
		return false
	}
	return execUnit.FunctionName == execUnit.FileName
}
