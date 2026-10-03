package executionflow

import (
	"github.com/jessesimpson36/helm-debugger/internal/frame"
	"github.com/jessesimpson36/helm-debugger/internal/templatevalues"
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
	// Resolved is true when value resolution ran for this line, so Values (even
	// when empty) is meaningful.
	Resolved bool
	// Found is true when the option was present in the rendered values.
	Found bool
}

func ContainsValuesReference(execUnit *frame.ExecutionUnit) bool {
	if execUnit == nil {
		return false
	}
	return templatevalues.Contains(execUnit.LineContent)
}

func GetValuesReferences(execUnit *frame.ExecutionUnit) []string {
	if execUnit == nil {
		return nil
	}
	return templatevalues.References(execUnit.LineContent)
}

func FillValuesReferences(flow *ExecutionFlow, execUnit *frame.ExecutionUnit) {
	if ContainsValuesReference(execUnit) {
		valuesNames := GetValuesReferences(execUnit)
		for _, valName := range valuesNames {
			valRef := &ValuesReference{
				ExecutionUnit: execUnit,
				ValuesName:    valName,
			}
			if execUnit.ResolvedValues != nil {
				valRef.Resolved = true
				valRef.Values, valRef.Found = execUnit.ResolvedValues[valName]
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
