package executionflow

import (
	"github.com/jessesimpson36/helm-debugger/internal/frame"
)

type ExecutionFlow struct {
	Template        *frame.ExecutionUnit
	Helpers         []*frame.ExecutionUnit
	ValuesReference []*ValuesReference
}

type ValuesReference struct {
	ExecutionUnit *frame.ExecutionUnit
	ValuesName    string
	Values        string
}
