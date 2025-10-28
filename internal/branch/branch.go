package branch

import (
	"context"
	"time"
	"github.com/jessesimpson36/helm-debugger/internal/dlvcontroller"
	"github.com/jessesimpson36/helm-debugger/internal/display"
	"github.com/jessesimpson36/helm-debugger/internal/breakpoints"
)

func Main() error {
	dlvController := &dlvcontroller.RPCDlvController{}
	ctx := context.Background()
	rpcClient, err := dlvController.StartSession(ctx)
	if err != nil {
		return err
	}

	breakpoints := breakpoints.GetConditionalBreakpoints()
	err = dlvController.Configure(ctx, rpcClient, breakpoints)
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

		err = display.DisplayVars(rpcClient)
		if err != nil {
			return err
		}

		state = <-rpcClient.Continue()
	}
	err = dlvController.QuitSession(rpcClient)
	if err != nil {
		return err
	}
	return nil
}
