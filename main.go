package main

import (
	"github.com/jessesimpson36/helm-debugger/internal/alternativemain/branch"
	"github.com/jessesimpson36/helm-debugger/internal/alternativemain/line"
	"github.com/jessesimpson36/helm-debugger/internal/alternativemain/model"
	"github.com/jessesimpson36/helm-debugger/internal/settings"
)

func main() {
	settings := settings.NewSettings()

	if settings.Mode == "branch" {
		err := branch.Main(settings)
		if err != nil {
			panic(err)
		}
	} else if settings.Mode == "line" {
		err := line.Main(settings)
		if err != nil {
			panic(err)
		}
	} else if settings.Mode == "model" {
		err := model.Main(settings)
		if err != nil {
			panic(err)
		}
	} else {
		println("No valid command provided. Use 'branch' to run the branch mode. (prints every if/else condition)")
	}
}
