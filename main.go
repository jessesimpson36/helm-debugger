
package main

import (
	"os"
	"github.com/jessesimpson36/helm-debugger/internal/branch"
	"github.com/jessesimpson36/helm-debugger/internal/line"
	"github.com/jessesimpson36/helm-debugger/internal/model"
)

func main() {
	args := os.Args
	if len(args) > 1 && args[1] == "branch" {
		err := branch.Main()
		if err != nil {
			panic(err)
		}
	} else if len(args) > 1 && args[1] == "line" {
		err := line.Main()
		if err != nil {
			panic(err)
		}
	} else if len(args) > 1 && args[1] == "model" {
		err := model.Main()
		if err != nil {
			panic(err)
		}
	} else {
		println("No valid command provided. Use 'branch' to run the branch mode. (prints every if/else condition)")
	}
}
