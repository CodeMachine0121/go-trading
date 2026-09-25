package clock

import "time"

type SystemClockProxy struct{}

func NewSystemClockProxy() *SystemClockProxy {
	return &SystemClockProxy{}
}

func (systemClockProxy *SystemClockProxy) Now() time.Time {
	return time.Now().UTC()
}

func (systemClockProxy *SystemClockProxy) Sleep(duration time.Duration) {
	time.Sleep(duration)
}
