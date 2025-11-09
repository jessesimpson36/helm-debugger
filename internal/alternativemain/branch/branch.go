package branch

import (
	"context"
	"github.com/jessesimpson36/helm-debugger/internal/breakpoints"
	"github.com/jessesimpson36/helm-debugger/internal/dlvcontroller"
	"github.com/jessesimpson36/helm-debugger/internal/frame/delegate"
	"time"
)

func Main() error {
	dlvController := &dlvcontroller.RPCDlvController{}
	ctx := context.Background()
	rpcClient, err := dlvController.StartSession(ctx)
	if err != nil {
		return err
	}

	frames := []*delegate.DelegateFrame{breakpoints.GetConditionalFrame()}
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

		// go from state to frame
		var currentFrame *delegate.DelegateFrame
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
			println("Error gathering variables: " + err.Error())
		} else {
			execUnit, err := currentFrame.Bind(respVars)
			if err != nil {
				println("Error binding variables: " + err.Error())
			} else {
				err = execUnit.Display(false)
				if err != nil {
					println("Error displaying execution unit: " + err.Error())
				}
			}
		}

		state = <-rpcClient.Continue()
	}
	err = dlvController.QuitSession(rpcClient)
	if err != nil {
		return err
	}
	return nil
}
