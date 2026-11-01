package process

import (
	"os"
	"time"
)

// Windows has no Unix signal forwarding; retain the existing tree shutdown.
func TerminateTree(parent *os.Process, _ time.Duration) error {
	return StopTree(parent)
}
