// Package prepass discovers which templates a chart actually renders, without a
// debugger. It runs `helm template` once and reads the "# Source: <name>"
// comments, which use the same runtime template name (chart-prefixed path) that
// text/template reports as t.name at (*Template).Execute.
//
// This lets the debugger install a single conditional breakpoint on Execute
// matching the rendered templates plus any helper the caller asked about, and
// drop the per-node walk breakpoint. See internal/debugger.
package prepass

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/jessesimpson36/helm-debugger/internal/settings"
)

// sourceComment matches a rendered manifest's origin, e.g. "# Source: mychart/templates/deployment.yaml".
var sourceComment = regexp.MustCompile(`(?m)^# Source: (.+?)\s*$`)

// withoutShowOnly returns args with every `--show-only <value>` pair removed, so
// the pre-pass renders and reports all templates regardless of which ones the
// caller wants rendered output for. It handles `--show-only=x` and
// `--show-only x` spellings.
func withoutShowOnly(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--show-only":
			// Skip the flag and its value.
			i++
			continue
		case strings.HasPrefix(arg, "--show-only="):
			continue
		default:
			out = append(out, arg)
		}
	}
	return out
}

// resolveHelmBinary finds the helm binary the way dlvcontroller does. The path
// is resolved against the debugger process's working directory (not the chart's
// working_dir, which only affects where helm runs), so both call sites agree.
func resolveHelmBinary(cfg *settings.Settings) (string, error) {
	helmBinary, err := exec.LookPath(cfg.CompiledHelmPath)
	if err != nil {
		return "", fmt.Errorf("locating helm binary %q: %w", cfg.CompiledHelmPath, err)
	}
	if !filepath.IsAbs(helmBinary) {
		abs, absErr := filepath.Abs(helmBinary)
		if absErr != nil {
			return "", fmt.Errorf("resolving helm binary path: %w", absErr)
		}
		helmBinary = abs
	}
	return helmBinary, nil
}

// RenderedTemplates runs `helm template` and returns the sorted, unique
// template names that produced output. It returns an error when helm fails; the
// caller falls back to the walk-based capture in that case.
func RenderedTemplates(ctx context.Context, cfg *settings.Settings) ([]string, error) {
	if cfg == nil {
		return nil, fmt.Errorf("settings are required")
	}
	helmBinary, err := resolveHelmBinary(cfg)
	if err != nil {
		return nil, err
	}

	// The pre-pass must see every template that renders, but --show-only limits
	// what helm prints. Strip it so the discovered set matches what the debuggee
	// actually executes; the debuggee still runs with the caller's full args.
	args := append([]string{"template", cfg.ChartName}, withoutShowOnly(cfg.CommandArgs)...)
	cmd := exec.CommandContext(ctx, helmBinary, args...)
	cmd.Dir = cfg.EffectiveWorkingDir()
	// Rendering twice (pre-pass then debuggee) must not mutate shared state.
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("helm template pre-pass: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	seen := map[string]struct{}{}
	for _, match := range sourceComment.FindAllStringSubmatch(stdout.String(), -1) {
		name := strings.TrimSpace(match[1])
		if name != "" {
			seen[name] = struct{}{}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}
