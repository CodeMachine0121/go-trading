package domains

import (
	"time"
)

// BacktestSegmentsDomain is an optional validation split; each part is replayed independently from the initial capital and flat, in addition to the whole.
type BacktestSegmentsDomain struct {
	validationStartTime time.Time
	isSplit             bool
}

// NewBacktestSegmentsDomain requires the validation start to fall strictly inside the stretch; zero means no split.
func NewBacktestSegmentsDomain(
	validationStartTime time.Time, stretchStart time.Time, stretchEnd time.Time,
) (BacktestSegmentsDomain, error) {
	if validationStartTime.IsZero() {
		return BacktestSegmentsDomain{}, nil
	}

	validationStartTime = validationStartTime.UTC()
	if !validationStartTime.After(stretchStart) || !validationStartTime.Before(stretchEnd) {
		return BacktestSegmentsDomain{}, BacktestValidationFailure(BacktestValidationStartTimeField,
			"驗證起點必須落在這次期間之內——晚於起點、早於終點")
	}

	return BacktestSegmentsDomain{validationStartTime: validationStartTime, isSplit: true}, nil
}

func (segmentsDomain BacktestSegmentsDomain) IsSplit() bool {
	return segmentsDomain.isSplit
}

// ValidationStartTime is nil when the replay is not split.
func (segmentsDomain BacktestSegmentsDomain) ValidationStartTime() *time.Time {
	if !segmentsDomain.isSplit {
		return nil
	}

	validationStartTime := segmentsDomain.validationStartTime

	return &validationStartTime
}

// SplitIndex is the first bar opening at or after the validation start; either part being empty is refused.
func (segmentsDomain BacktestSegmentsDomain) SplitIndex(barOpenTimes []time.Time) (int, error) {
	splitIndex := len(barOpenTimes)
	for barIndex, barOpenTime := range barOpenTimes {
		if !barOpenTime.Before(segmentsDomain.validationStartTime) {
			splitIndex = barIndex
			break
		}
	}

	if splitIndex == 0 {
		return 0, BacktestValidationFailure(BacktestValidationStartTimeField,
			"調參段湊不出任何一格走完的刻度區間——把驗證起點往後挪")
	}
	if splitIndex == len(barOpenTimes) {
		return 0, BacktestValidationFailure(BacktestValidationStartTimeField,
			"驗證段湊不出任何一格走完的刻度區間——把驗證起點往前挪，或把期間拉長")
	}

	return splitIndex, nil
}
