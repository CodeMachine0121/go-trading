package clock

import "time"

// SystemClockProxy reads the current time from the machine's clock.
type SystemClockProxy struct{}

func NewSystemClockProxy() *SystemClockProxy {
	return &SystemClockProxy{}
}

func (systemClockProxy *SystemClockProxy) Now() time.Time {
	return time.Now().UTC()
}
