package _interface

import "time"

//go:generate go tool mockgen -source=i_clock_proxy.go -destination=mocks/mock_i_clock_proxy.go -package=mocks

// IClockProxy reads the current time and waits. It exists so that rules about "now"
// stay verifiable instead of drifting with the wall clock — and waiting belongs here
// for the same reason: a test that has to sit out a real backoff is a test nobody
// runs.
type IClockProxy interface {
	Now() time.Time
	// Sleep waits out a stretch. It is on the clock rather than reached for directly
	// so that a retry with a pause in it can be exercised without one.
	Sleep(duration time.Duration)
}
