package settings

import (
	"flag"
	"strings"
)

type Settings struct {
	ChartName          string
	CommandArgs        string
	Mode	           string
	RenderedQueryFiles []string
	TemplateQueryFiles []string
	HelpersQueryFiles  []string
	ValuesQuery        []string
}

func NewSettings() *Settings {
	settings := &Settings{}

	commaDelimitedRenderedQueryFiles := ""
	commaDelimitedTemplateQueryFiles := ""
	commaDelimitedHelpersQueryFiles := ""
	commaDelimitedValuesQuery := ""

	flag.StringVar(&settings.ChartName, "chart", "", "The name of the Helm chart to debug.")
	flag.StringVar(&commaDelimitedRenderedQueryFiles, "rendered-file", "", "Comma-delimited list of query files for rendered manifest.")
	flag.StringVar(&commaDelimitedTemplateQueryFiles, "template-file", "", "Comma-delimited list of query files for templates and helpers.")
	flag.StringVar(&commaDelimitedHelpersQueryFiles, "helper-file", "", "Comma-delimited list of query files for helpers.")
	flag.StringVar(&commaDelimitedValuesQuery, "values", "", "Comma-delimited list of values queries to capture.")
	flag.StringVar(&settings.CommandArgs, "extra-command-args", "", "Additional command line arguments to pass to 'helm template' command.")
	flag.StringVar(&settings.Mode, "mode", "all", "Mode of operation: model, branch, line")

	flag.Parse()

	if commaDelimitedRenderedQueryFiles != "" {
		for _, file := range strings.Split(commaDelimitedRenderedQueryFiles, ",") {
			settings.RenderedQueryFiles = append(settings.RenderedQueryFiles, strings.TrimSpace(file))
		}
	}
	if commaDelimitedTemplateQueryFiles != "" {
		for _, file := range strings.Split(commaDelimitedTemplateQueryFiles, ",") {
			settings.TemplateQueryFiles = append(settings.TemplateQueryFiles, strings.TrimSpace(file))
		}
	}
	if commaDelimitedHelpersQueryFiles != "" {
		for _, file := range strings.Split(commaDelimitedHelpersQueryFiles, ",") {
			settings.HelpersQueryFiles = append(settings.HelpersQueryFiles, strings.TrimSpace(file))
		}
	}
	if commaDelimitedValuesQuery != "" {
		for _, query := range strings.Split(commaDelimitedValuesQuery, ",") {
			settings.ValuesQuery = append(settings.ValuesQuery, strings.TrimSpace(query))
		}
	}
	return settings
}
