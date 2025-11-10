package model

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jessesimpson36/helm-debugger/internal/breakpointevent"
	"github.com/jessesimpson36/helm-debugger/internal/breakpoints"
	"github.com/jessesimpson36/helm-debugger/internal/dlvcontroller"
	"github.com/jessesimpson36/helm-debugger/internal/executionflow"
	"github.com/jessesimpson36/helm-debugger/internal/frame"
	"github.com/jessesimpson36/helm-debugger/internal/frame/delegate"
	"github.com/jessesimpson36/helm-debugger/internal/query"
)

func PrintFilteredExecutionFlows(flows []*executionflow.ExecutionFlow) {
	for _, flow := range flows {
		flow.Template.Display(false)
		for _, helper := range flow.Helpers {
			helper.Display(true)
		}
		fmt.Println("Relevant Values")
		for _, valRef := range flow.ValuesReference {
			fmt.Printf("- %s\n", valRef.ValuesName)
		}
		fmt.Println("WriteBuffer")
		var prevBuffer *frame.RenderedLine
		for _, capturedBuffer := range flow.RenderedManifest {
			//fmt.Println("- " + capturedBuffer.FileName)
			if prevBuffer != nil {
				if strings.HasPrefix(capturedBuffer.Content, prevBuffer.Content) {
					for _, line := range strings.Split(strings.TrimPrefix(capturedBuffer.Content, prevBuffer.Content), "\n") {
						fmt.Printf("+     %s\n", line)
					}
				}
			} else {
				for i, line := range strings.Split(capturedBuffer.Content, "\n") {
					fmt.Printf("%4d  %s\n", i, line)
				}
			}
			prevBuffer = capturedBuffer
		}
		fmt.Println("--------------------------------------------------")
	}
}

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
		breakpoints.GetRenderedManifestFrame(),
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
		for _, frame := range frames {
			for _, breakpoint := range frame.Breakpoints {
				if state.CurrentThread.Breakpoint != nil && state.CurrentThread.Breakpoint.Name == breakpoint.Name {
					currentFrame = frame
					break
				}
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
			println("Error gathering variables: " + err.Error())
		} else {
			breakpointEvent, err := currentFrame.Bind(respVars)
			if err != nil {
				// do nothing since this is noisy
				if strings.HasPrefix(currentFrame.Breakpoints[0].Name, "rendered") {
					println("Error binding: " + err.Error())
				}
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
	PrintFilteredExecutionFlows(afterQueryValuesReferences)

	fmt.Println("================= HELPERS QUERY =================")
	afterQueryHelpers := query.QueryHelpers(executionFlows, []string{"test.serviceAccountName"})
	PrintFilteredExecutionFlows(afterQueryHelpers)

	fmt.Println("================= TEMPLATE QUERY =================")
	afterQueryTemplate := query.QueryTemplate(executionFlows, []string{"test/templates/deployment.yaml:42"})
	PrintFilteredExecutionFlows(afterQueryTemplate)

	fmt.Println("================= RENDERED QUERY =================")
	afterRenderedQueryTemplate := query.QueryRenderedTemplate(executionFlows, []string{"test/templates/deployment.yaml:32"})
	PrintFilteredExecutionFlows(afterRenderedQueryTemplate)

	return nil
}
