package _interface

import "time"

//go:generate go tool mockgen -source=i_clock_proxy.go -destination=mocks/mock_i_clock_proxy.go -package=mocks

// IClockProxy reads the time and waits, so "now" rules and retry backoffs are testable without the wall clock.
type IClockProxy interface {
	Now() time.Time
	Sleep(duration time.Duration)
}
