// Package exit names the error classes the apex binary maps to process exit codes.
package exit

import "errors"

var (
	Usage      = errors.New("usage")              // 64
	NotFound   = errors.New("not found")          // 1
	Transition = errors.New("illegal transition") // 3
	Corrupt    = errors.New("corrupt")            // 4
	Conflict   = errors.New("merge conflict")     // 5
)

// Code maps an error to the exit code of its class; any other non-nil error is the environment's fault (2).
func Code(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, Usage):
		return 64
	case errors.Is(err, NotFound):
		return 1
	case errors.Is(err, Transition):
		return 3
	case errors.Is(err, Corrupt):
		return 4
	case errors.Is(err, Conflict):
		return 5
	default:
		return 2
	}
}
