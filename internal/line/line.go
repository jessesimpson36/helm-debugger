package line

import (
	"context"
	"time"
	"github.com/jessesimpson36/helm-debugger/internal/dlvcontroller"
	"github.com/jessesimpson36/helm-debugger/internal/breakpoints"
)

func Main() error {
	dlvController := &dlvcontroller.RPCDlvController{}
	ctx := context.Background()
	rpcClient, err := dlvController.StartSession(ctx)
	if err != nil {
		return err
	}

	frame := breakpoints.GetLineStartFrame()
	err = dlvController.Configure(ctx, rpcClient, frame)
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

		respVars, err := frame.Gather(rpcClient)
		if err != nil {
			// do nothing since this is noisy
			// println("Error gathering variables: " + err.Error())
		} else {
			execUnit, err := frame.Bind(respVars)
			if err != nil {
				// do nothing since this is noisy
				// println("Error binding variables: " + err.Error())
			} else {
				err = execUnit.Display()
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
