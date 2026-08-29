package template

import "time"

// _now is wrapped in a func var so tests can stub it for deterministic year
// values. Keep it private to the package.
var _now = func() time.Time {
	return time.Now()
}
