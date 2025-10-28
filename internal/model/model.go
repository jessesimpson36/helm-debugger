package model

import (
	"context"
	"fmt"
	"time"
	"github.com/jessesimpson36/helm-debugger/internal/dlvcontroller"
	"github.com/jessesimpson36/helm-debugger/internal/breakpoints"
	"github.com/jessesimpson36/helm-debugger/internal/executionflow"
	"github.com/jessesimpson36/helm-debugger/internal/frame"
	"github.com/jessesimpson36/helm-debugger/internal/query"
)

func Main() error {
	dlvController := &dlvcontroller.RPCDlvController{}
	ctx := context.Background()
	rpcClient, err := dlvController.StartSession(ctx)
	if err != nil {
		return err
	}

	frames := []*frame.Frame{
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

	execUnits := []*frame.ExecutionUnit{}
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

		var currentFrame *frame.Frame
		for i, frame := range frames {
			if state.CurrentThread.Breakpoint != nil && state.CurrentThread.Breakpoint.Name == frame.Breakpoints[i].Name {
				currentFrame = frame
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
			execUnit, err := currentFrame.Bind(respVars)
			if err != nil {
				// do nothing since this is noisy
			}
			execUnits = append(execUnits, execUnit)
		}

		state = <-rpcClient.Continue()
	}
	err = dlvController.QuitSession(rpcClient)
	if err != nil {
		return err
	}


	executionFlows := executionflow.Process(execUnits)

	fmt.Println("================= VALUES QUERY =================")
	valuesQuery := []string{
		"image.tag",
	}

	afterQueryValuesReferences := query.QueryValuesReference(executionFlows, valuesQuery)

	for _, flow := range afterQueryValuesReferences {
		flow.Template.Display()
		for _, helper := range flow.Helpers {
			helper.Display()
		}
		fmt.Println("--------------------------------------------------")
	}

	fmt.Println("================= HELPERS QUERY =================")
	afterQueryHelpers := query.QueryHelpers(executionFlows, []string{"test.serviceAccountName"})

	for _, flow := range afterQueryHelpers {
		flow.Template.Display()
		for _, helper := range flow.Helpers {
			helper.Display()
		}
		fmt.Println("--------------------------------------------------")
	}


	fmt.Println("================= TEMPLATE QUERY =================")
	afterQueryTemplate := query.QueryTemplate(executionFlows, []string{"test/templates/deployment.yaml:42"})

	for _, flow := range afterQueryTemplate {
		flow.Template.Display()
		for _, helper := range flow.Helpers {
			helper.Display()
		}
		fmt.Println("--------------------------------------------------")
	}

	return nil
}
