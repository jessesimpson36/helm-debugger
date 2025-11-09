package model

import (
	"context"
	"fmt"
	"github.com/jessesimpson36/helm-debugger/internal/breakpointevent"
	"github.com/jessesimpson36/helm-debugger/internal/breakpoints"
	"github.com/jessesimpson36/helm-debugger/internal/dlvcontroller"
	"github.com/jessesimpson36/helm-debugger/internal/frame"
	"github.com/jessesimpson36/helm-debugger/internal/frame/delegate"
	"github.com/jessesimpson36/helm-debugger/internal/query"
	"time"
)

func Main() error {
	dlvController := &dlvcontroller.RPCDlvController{}
	ctx := context.Background()
	rpcClient, err := dlvController.StartSession(ctx)
	if err != nil {
		return err
	}

	frames := []*delegate.DelegateFrame{
		breakpoints.GetLineStartFrame(),
		breakpoints.GetConditionalFrame(),
	}
	err = dlvController.Configure(ctx, rpcClient, frames)
	if err != nil {
		return err
	}

	_, err = rpcClient.Restart(false)
	if err != nil {
		return err
	}

	state, err := rpcClient.GetState()
	if err != nil {
		return err
	}

	breakpointEvents := []*frame.BindResult{}
	for {
		if state.Exited {
			println("Program stopped")
			break
		}

		if state.Running {
			// sleep 1 second
			time.Sleep(1 * time.Second)
			continue
		}

		var currentFrame *delegate.DelegateFrame
		for i, frame := range frames {
			if state.CurrentThread.Breakpoint != nil && state.CurrentThread.Breakpoint.Name == frame.Breakpoints[i].Name {
				currentFrame = frame
			}
			if state.CurrentThread.Breakpoint != nil && (state.CurrentThread.Breakpoint.Name == "conditionalevaluatedtrue" || state.CurrentThread.Breakpoint.Name == "conditionalevaluatedfalse") {
				// not doing anything with these breakpoints yet
				state = <-rpcClient.Continue()
				continue
			}

		}
		if currentFrame == nil {
			state = <-rpcClient.Continue()
			continue
		}

		respVars, err := currentFrame.Gather(rpcClient)
		if err != nil {
			// do nothing since this is noisy
			// println("Error gathering variables: " + err.Error())
		} else {
			breakpointEvent, err := currentFrame.Bind(respVars)
			if err != nil {
				// do nothing since this is noisy
			}
			breakpointEvents = append(breakpointEvents, breakpointEvent)
		}

		state = <-rpcClient.Continue()
	}
	err = dlvController.QuitSession(rpcClient)
	if err != nil {
		return err
	}

	executionFlows := breakpointevent.Process(breakpointEvents)

	fmt.Println("================= VALUES QUERY =================")
	valuesQuery := []string{
		"image.tag",
	}

	afterQueryValuesReferences := query.QueryValuesReference(executionFlows, valuesQuery)

	for _, flow := range afterQueryValuesReferences {
		flow.Template.Display(false)
		for _, helper := range flow.Helpers {
			helper.Display(true)
		}
		fmt.Println("Relevant Values")
		for _, valRef := range flow.ValuesReference {
			fmt.Printf("- %s\n", valRef.ValuesName)
		}
		fmt.Println("--------------------------------------------------")
	}

	fmt.Println("================= HELPERS QUERY =================")
	afterQueryHelpers := query.QueryHelpers(executionFlows, []string{"test.serviceAccountName"})

	for _, flow := range afterQueryHelpers {
		flow.Template.Display(false)
		for _, helper := range flow.Helpers {
			helper.Display(true)
		}
		fmt.Println("Relevant Values")
		for _, valRef := range flow.ValuesReference {
			fmt.Printf("- %s\n", valRef.ValuesName)
		}
		fmt.Println("--------------------------------------------------")
	}

	fmt.Println("================= TEMPLATE QUERY =================")
	afterQueryTemplate := query.QueryTemplate(executionFlows, []string{"test/templates/deployment.yaml:42"})

	for _, flow := range afterQueryTemplate {
		flow.Template.Display(false)
		for _, helper := range flow.Helpers {
			helper.Display(true)
		}
		fmt.Println("Relevant Values")
		for _, valRef := range flow.ValuesReference {
			fmt.Printf("- %s\n", valRef.ValuesName)
		}
		fmt.Println("--------------------------------------------------")
	}

	return nil
}
