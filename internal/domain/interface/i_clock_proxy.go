package _interface

import "time"

//go:generate go tool mockgen -source=i_clock_proxy.go -destination=mocks/mock_i_clock_proxy.go -package=mocks

// IClockProxy reads the current time. It exists so that rules about "now"
// stay verifiable instead of drifting with the wall clock.
type IClockProxy interface {
	Now() time.Time
}
