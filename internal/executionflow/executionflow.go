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
	// Owner is the runtime name of the top-level rendered template this flow
	// belongs to. It is always set, even when no execution unit was captured for
	// the template itself (for example when the walk breakpoint is gated to a
	// helper's subtree), so the report still knows the anchor.
	Owner string
}

type ValuesReference struct {
	ExecutionUnit *frame.ExecutionUnit
	ValuesName    string
	Values        string
	// Root is true when the line reads "$.Values.<name>" (the root data) rather
	// than ".Values.<name>" (the current dot).
	Root bool
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
	if !ContainsValuesReference(execUnit) {
		return
	}
	for _, ref := range templatevalues.ParseReferences(execUnit.LineContent) {
		valRef := &ValuesReference{
			ExecutionUnit: execUnit,
			ValuesName:    ref.Path,
			Root:          ref.Root,
		}
		if execUnit.ResolvedValues != nil {
			valRef.Resolved = true
			valRef.Values, valRef.Found = execUnit.ResolvedValues[ref.Expr()]
		}
		flow.ValuesReference = append(flow.ValuesReference, valRef)
	}
}

func IsTemplate(execUnit *frame.ExecutionUnit) bool {
	// if function name == filename then it's a template, not a helper
	if execUnit == nil {
		return false
	}
	return execUnit.FunctionName == execUnit.FileName
}
