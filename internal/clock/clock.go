package clock

import "time"

// Clock is injected wherever "now" matters, so tests never sleep.
type Clock interface{ Now() time.Time }

type System struct{}

func (System) Now() time.Time { return time.Now() }
