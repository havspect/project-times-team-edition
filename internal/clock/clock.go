// Package clock provides the time source injected wherever "now" matters, so tests never sleep.
package clock

import "time"

// Clock is injected wherever "now" matters, so tests never sleep.
type Clock interface{ Now() time.Time }

// System is the real clock, used in production; tests pass a fake.
type System struct{}

// Now returns the current time.
func (System) Now() time.Time { return time.Now() }
