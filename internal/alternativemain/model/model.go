package model

import (
	"context"
	"os"

	"github.com/jessesimpson36/helm-debugger/internal/debugger"
	"github.com/jessesimpson36/helm-debugger/internal/report"
	"github.com/jessesimpson36/helm-debugger/internal/settings"
)

func Main(settings *settings.Settings) error {
	result, err := debugger.Run(context.Background(), settings, os.Stderr)
	if err != nil {
		return err
	}
	report.Write(os.Stdout, report.Sections(result.Flows, settings))
	return nil
}
