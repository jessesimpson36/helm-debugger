package breakpointevent

import (
	"github.com/jessesimpson36/helm-debugger/internal/executionflow"
	"github.com/jessesimpson36/helm-debugger/internal/frame"
)

// Process turns captured breakpoint events into execution flows.
//
// Flows are grouped by the explicit Owner stamped on each event: the runtime
// name of the top-level rendered template the event belongs to. Grouping by an
// explicit owner, rather than inferring a boundary from "the next event happens
// to be a template", keeps the output correct when the debugger does not stop on
// every node (the walk breakpoint is gated to the queried helper's subtree, so
// events for other templates are absent).
//
// Within an owner's group the first template execution becomes the flow's
// Template and each helper execution becomes a Helpers entry. Rendered buffer
// snapshots are attached to the group they belong to.
func Process(breakpointEvents []*frame.BindResult) []*executionflow.ExecutionFlow {
	var order []string
	flows := map[string]*executionflow.ExecutionFlow{}
	// helpersSeen dedupes helper frames within a flow. A helper line can be
	// reported by both the walk breakpoint and the scoped execute breakpoint.
	helpersSeen := map[string]map[unitKey]struct{}{}

	flowFor := func(owner string) *executionflow.ExecutionFlow {
		if flow, ok := flows[owner]; ok {
			return flow
		}
		flow := &executionflow.ExecutionFlow{Owner: owner}
		flows[owner] = flow
		helpersSeen[owner] = map[unitKey]struct{}{}
		order = append(order, owner)
		return flow
	}

	for _, breakpointEvent := range breakpointEvents {
		if breakpointEvent == nil {
			continue
		}
		if breakpointEvent.ExecutionUnit != nil {
			execUnit := breakpointEvent.ExecutionUnit
			flow := flowFor(execUnit.Owner)
			if flow.Template == nil && executionflow.IsTemplate(execUnit) {
				flow.Template = execUnit
				executionflow.FillValuesReferences(flow, execUnit)
				continue
			}
			if executionflow.IsTemplate(execUnit) {
				// A second top-level execution within the same owner: Helm can
				// execute the same rendered template more than once, so keep the
				// first as the flow's anchor and treat the rest as helpers so no
				// lines are dropped.
			}
			key := unitKey{execUnit.FileName, execUnit.LineNumber, execUnit.FunctionName}
			if _, seen := helpersSeen[execUnit.Owner][key]; seen {
				continue
			}
			helpersSeen[execUnit.Owner][key] = struct{}{}
			executionflow.FillValuesReferences(flow, execUnit)
			flow.Helpers = append(flow.Helpers, execUnit)
			continue
		}
		if breakpointEvent.RenderedLine != nil {
			renderedLine := breakpointEvent.RenderedLine
			flow := flowFor(renderedLine.Owner)
			flow.RenderedManifest = append(flow.RenderedManifest, renderedLine)
		}
	}

	result := make([]*executionflow.ExecutionFlow, 0, len(order))
	for _, owner := range order {
		flow := flows[owner]
		// Keep a flow when it has anything to show: a captured template unit, a
		// helper, or a rendered buffer. A gated run can produce a flow with no
		// template unit of its own.
		if flow.Template != nil || len(flow.Helpers) > 0 || len(flow.RenderedManifest) > 0 {
			result = append(result, flow)
		}
	}
	return result
}

// unitKey identifies an execution unit for deduplication within a flow.
type unitKey struct {
	file     string
	line     int
	function string
}
