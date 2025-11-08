package interactive

import (
	"context"
	"time"
	"github.com/jessesimpson36/helm-debugger/internal/dlvcontroller"
	"github.com/jessesimpson36/helm-debugger/internal/breakpoints"
	"github.com/jessesimpson36/helm-debugger/internal/frame"
)

func Main() error {
	dlvController := &dlvcontroller.RPCDlvController{}
	ctx := context.Background()
	rpcClient, err := dlvController.StartSession(ctx)
	if err != nil {
		return err
	}

	frames := []*frame.Frame{breakpoints.GetLineStartFrame()}
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
				// println("Error binding variables: " + err.Error())
			} else {
				err = execUnit.Display(false)
				if err != nil {
					// do nothing since this is noisy
					// println("Error displaying execution unit: " + err.Error())
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
