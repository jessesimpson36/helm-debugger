
package main

import (
	"context"
	"time"
	"github.com/jessesimpson36/helm-debugger/internal/dlvcontroller"
)

func main() {
	dlvController := &dlvcontroller.RPCDlvController{}
	ctx := context.Background()
	rpcClient, err := dlvController.StartSession(ctx)
	if err != nil {
		panic(err)
	}

	err = dlvController.Configure(ctx, rpcClient)
	if err != nil {
		panic(err)
	}

	_, err = rpcClient.Restart(false)
	if err != nil {
		panic(err)
	}

	for {
		state, err := rpcClient.GetState()
		if err != nil {
			panic(err)
		}
		if state.Exited {
			println("Program stopped")
			break
		}

		if state.Running {
			// sleep 1 second
			time.Sleep(1 * time.Second)
			continue
		}

		err = dlvController.DisplayVars(rpcClient)
		if err != nil {
			panic(err)
		}

		state = <-rpcClient.Continue()
	}
	err = dlvController.QuitSession(rpcClient)
	if err != nil {
		panic(err)
	}

}
