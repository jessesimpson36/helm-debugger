package breakpoints

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// LineNumbers holds the resolved line numbers within text/template/exec.go that
// the debugger sets breakpoints on. Hard-coding these was fragile: they change
// whenever the Go standard library is edited or the debugger is run against a
// helm binary compiled with a different Go toolchain.
//
// The values are resolved from the Go source tree (GOROOT) that was used to
// compile the helm binary under debug. This keeps the project working across
// Go patch/minor releases without a per-version table.
type LineNumbers struct {
	// LineStart is the first statement of (*state).walk and fires for every
	// node the template engine walks.
	LineStart int
	// RenderedManifest is the statement in (*state).walk that writes a text node
	// to the output buffer, used to snapshot the rendered manifest.
	RenderedManifest int
}

// FallbackLines are the line numbers for the Go 1.25.x/1.26.x standard library
// (verified against go1.25.3 and go1.26.7). They are only used when the standard
// library source cannot be located.
var FallbackLines = LineNumbers{
	LineStart:        262,
	RenderedManifest: 287,
}

// DefaultGOROOT returns the GOROOT to resolve standard library sources from.
// It prefers the GOROOT the debugger binary was built with, falling back to the
// GOROOT reported by the go tool, and finally the GOROOT environment variable.
func DefaultGOROOT() string {
	if root := runtime.GOROOT(); root != "" {
		return root
	}
	if root := os.Getenv("GOROOT"); root != "" {
		return root
	}
	if out, err := exec.Command("go", "env", "GOROOT").Output(); err == nil {
		if root := strings.TrimSpace(string(out)); root != "" {
			return root
		}
	}
	return ""
}

// Resolve locates the text/template/exec.go file under goroot and computes the
// line numbers the debugger needs. When goroot is empty, DefaultGOROOT is used.
func Resolve(goroot string) (LineNumbers, error) {
	if goroot == "" {
		goroot = DefaultGOROOT()
	}
	if goroot == "" {
		return LineNumbers{}, fmt.Errorf("unable to determine GOROOT; pass the go root explicitly")
	}

	execPath := filepath.Join(goroot, "src", "text", "template", "exec.go")
	lines, err := resolveFile(execPath)
	if err != nil {
		return LineNumbers{}, fmt.Errorf("resolving breakpoints from %s: %w", execPath, err)
	}
	return lines, nil
}

// ResolveOrDefault resolves the line numbers, returning FallbackLines when the
// standard library source cannot be found.
func ResolveOrDefault(goroot string) LineNumbers {
	lines, err := Resolve(goroot)
	if err != nil {
		return FallbackLines
	}
	return lines
}

// resolveFile parses a text/template/exec.go source file and extracts the line
// numbers the debugger relies on. It is intentionally a tiny, dependency-free
// scanner rather than a full parser so it keeps working if the surrounding
// source changes.
func resolveFile(path string) (LineNumbers, error) {
	f, err := os.Open(path)
	if err != nil {
		return LineNumbers{}, err
	}
	defer f.Close()

	var (
		result  LineNumbers
		scanner = bufio.NewScanner(f)
		lineNo  int
		inWalk  bool
	)

	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Track which function body we are in. Only (*state).walk is relevant;
		// the leading "func " case resets the flag for every other function.
		switch {
		case strings.HasPrefix(trimmed, "func (s *state) walk("):
			inWalk = true
		case strings.HasPrefix(trimmed, "func "):
			inWalk = false
		}

		if inWalk {
			if result.LineStart == 0 && trimmed == "s.at(node)" {
				result.LineStart = lineNo
			}
			if result.RenderedManifest == 0 && strings.Contains(trimmed, "s.wr.Write(node.Text)") {
				result.RenderedManifest = lineNo
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return LineNumbers{}, err
	}

	var missing []string
	if result.LineStart == 0 {
		missing = append(missing, "walk line start (s.at(node))")
	}
	if result.RenderedManifest == 0 {
		missing = append(missing, "rendered manifest (s.wr.Write)")
	}
	if len(missing) > 0 {
		return LineNumbers{}, fmt.Errorf("could not find %s in %s", strings.Join(missing, ", "), path)
	}

	return result, nil
}
