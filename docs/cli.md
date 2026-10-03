# CLI reference

```text
  -chart string
    	The name of the Helm chart to debug.
  -debug-port int
    	Port for the headless delve server. 0 picks a free port.
  -extra-command-args string
    	Additional command line arguments to pass to 'helm template' command.
  -goroot string
    	GOROOT used to resolve text/template breakpoints. Defaults to the debugger's own GOROOT.
  -helm-path string
    	Path to the compiled Helm binary. (default "helm")
  -helper-file string
    	Comma-delimited list of query files for helpers.
  -mode string
    	Mode of operation: model, mcp (default "model")
  -rendered-file string
    	Comma-delimited list of query files for rendered manifest.
  -template-file string
    	Comma-delimited list of query files for templates and helpers.
  -values string
    	Comma-delimited list of values queries to capture.
  -version
    	Print build and toolchain version information, then exit.
  -working-dir string
    	Directory the helm chart paths are relative to. Defaults to the current directory.
```

`helm-debugger --version` prints the release version, commit, build date, and
the Go/Helm/Delve versions baked into the build. The same version is reported by
the MCP server in its `initialize` response, so a client can confirm what it is
talking to.

## Examples

The Makefile targets are the quickest way to see the flags in use. Query
types map to flags like this:

| Query | Flag | Example |
| --- | --- | --- |
| Values | `--values` | `--values image.tag` |
| Helper | `--helper-file` | `--helper-file test.serviceAccountName` |
| Template | `--template-file` | `--template-file test/templates/deployment.yaml:42` |
| Rendered manifest | `--rendered-file` | `--rendered-file test/templates/deployment.yaml:32` |

```bash
go run . --mode model \
  --helm-path ./helm/bin/helm \
  --rendered-file test/templates/deployment.yaml:32 \
  --template-file test/templates/deployment.yaml:42 \
  --helper-file test.serviceAccountName \
  --values image.tag \
  --chart test \
  --extra-command-args '--show-only templates/deployment.yaml'
```

See [Concepts](concepts.md) for what each query type means and
[Understanding output](output.md) for how to read the result.
