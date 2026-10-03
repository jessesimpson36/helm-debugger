package settings

import (
	"strings"
)

// SplitArgs splits a command line fragment into individual arguments using
// POSIX-ish shell quoting rules. It supports single quotes (literal), double
// quotes, and backslash escapes.
//
// It exists so the CLI's -extra-command-args flag can be converted into the
// argument slice passed to the helm subprocess without invoking a shell. The
// MCP server passes arguments as a real slice and does not use this.
func SplitArgs(input string) []string {
	var (
		args    []string
		current strings.Builder
		started bool

		inSingle bool
		inDouble bool
		escaped  bool
	)

	flush := func() {
		if started {
			args = append(args, current.String())
			current.Reset()
			started = false
		}
	}

	for _, r := range input {
		switch {
		case escaped:
			current.WriteRune(r)
			started = true
			escaped = false
		case r == '\\' && !inSingle:
			escaped = true
			started = true
		case r == '\'' && !inDouble:
			inSingle = !inSingle
			started = true
		case r == '"' && !inSingle:
			inDouble = !inDouble
			started = true
		case (r == ' ' || r == '\t' || r == '\n') && !inSingle && !inDouble:
			flush()
		default:
			current.WriteRune(r)
			started = true
		}
	}
	flush()

	return args
}
