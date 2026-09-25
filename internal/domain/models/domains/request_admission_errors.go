package domains

import (
	"errors"
	"fmt"
	"math"
	"time"
)

var ErrRequestRateExceeded = errors.New("請求太頻繁")

var ErrLiveStreamCapacityReached = errors.New("同時開著的即時跟盤已達上限，請先關掉一些再開")

// RequestRateExceededError carries how long until the next request would be admitted.
type RequestRateExceededError struct {
	RetryAfter time.Duration
}

// RetryAfterSeconds rounds up so a caller that waits exactly this long is admitted.
func (requestRateExceededError RequestRateExceededError) RetryAfterSeconds() int {
	return max(1, int(math.Ceil(requestRateExceededError.RetryAfter.Seconds())))
}

func (requestRateExceededError RequestRateExceededError) Error() string {
	return fmt.Sprintf("%s，請 %d 秒後再試",
		ErrRequestRateExceeded.Error(), requestRateExceededError.RetryAfterSeconds())
}

func (requestRateExceededError RequestRateExceededError) Is(target error) bool {
	return target == ErrRequestRateExceeded
}
