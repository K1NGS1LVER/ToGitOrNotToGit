// Package logx provides tocommit's opt-in debug logging. It stays silent unless
// explicitly enabled (via TOCOMMIT_DEBUG or the --debug flag), so the git hook
// never pollutes normal commit output.
package logx

import (
	"io"
	"log"
	"os"
)

var logger = log.New(io.Discard, "tocommit: ", 0)

// Enabled reports whether debug logging is currently active.
var Enabled bool

// Enable turns on debug logging to stderr.
func Enable() {
	Enabled = true
	logger.SetOutput(os.Stderr)
}

// EnableFromEnv enables logging when TOCOMMIT_DEBUG is set to a truthy value.
func EnableFromEnv() {
	if isTruthy(os.Getenv("TOCOMMIT_DEBUG")) {
		Enable()
	}
}

// Debugf logs a debug line, but only when logging is enabled.
func Debugf(format string, args ...any) {
	if !Enabled {
		return
	}
	logger.Printf(format, args...)
}

func isTruthy(v string) bool {
	switch v {
	case "", "0", "false", "no", "off":
		return false
	default:
		return true
	}
}
