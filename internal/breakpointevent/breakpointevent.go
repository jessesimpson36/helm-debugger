package breakpointevent

import (
	"github.com/jessesimpson36/helm-debugger/internal/executionflow"
	"github.com/jessesimpson36/helm-debugger/internal/frame"
)

func Process(breakpointEvents []*frame.BindResult) []*executionflow.ExecutionFlow {
	flows := []*executionflow.ExecutionFlow{}
	first := true
	executionFlow := &executionflow.ExecutionFlow{}
	//prevFlow := executionFlow
	for _, breakpointEvent := range breakpointEvents {
		// if for whatever reason we hit breakpoints in helpers before hitting a template, then skip
		if breakpointEvent == nil {
			continue
		}
		if breakpointEvent.ExecutionUnit != nil {
			execUnit := breakpointEvent.ExecutionUnit
			if first {
				if !executionflow.IsTemplate(execUnit) {
					continue
				}
				first = false
				executionFlow.Template = execUnit
				executionflow.FillValuesReferences(executionFlow, execUnit)
				continue
			}
			if executionflow.IsTemplate(execUnit) {
				flows = append(flows, executionFlow)
				//prevFlow = executionFlow
				executionFlow = &executionflow.ExecutionFlow{}
				executionFlow.Template = execUnit
				executionflow.FillValuesReferences(executionFlow, execUnit)
			} else {
				executionflow.FillValuesReferences(executionFlow, execUnit)
				executionFlow.Helpers = append(executionFlow.Helpers, execUnit)
			}
		} else if breakpointEvent.RenderedLine != nil {
			renderedLine := breakpointEvent.RenderedLine
			// we keep track of the previous flow because we want the before and after the lines were rendered.
//			if prevFlow != nil {
//				prevFlow.RenderedManifest = append(prevFlow.RenderedManifest, renderedLine)
//				prevFlow = nil
//			}
			executionFlow.RenderedManifest = append(executionFlow.RenderedManifest, renderedLine)
		}
	}
	// sanity check in case last flow may not have been saved
	if executionFlow.Template != nil {
		flows = append(flows, executionFlow)
	}
	return flows
}
