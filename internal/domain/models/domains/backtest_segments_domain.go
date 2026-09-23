package domains

import (
	"time"
)

// BacktestSegmentsDomain is whether a replay is split for validation, and where.
//
// A split replay is still replayed as a whole, and additionally in two parts — the
// in-sample part before the validation start, where parameters are tuned, and the
// validation part from it on, which only ever answers "does this work on market it
// has not seen". Each part is replayed on its own from the initial capital and flat,
// so a position left open at the end of one says nothing about the other.
//
// Its zero value is no split: every replay made before there was one.
type BacktestSegmentsDomain struct {
	validationStartTime time.Time
	isSplit             bool
}

// NewBacktestSegmentsDomain reads the validation start against the stretch it has to
// fall strictly inside. Declaring none is no split.
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

// IsSplit is whether the replay has a validation part at all.
func (segmentsDomain BacktestSegmentsDomain) IsSplit() bool {
	return segmentsDomain.isSplit
}

// ValidationStartTime is where the replay is split, or nothing when it is not.
func (segmentsDomain BacktestSegmentsDomain) ValidationStartTime() *time.Time {
	if !segmentsDomain.isSplit {
		return nil
	}

	validationStartTime := segmentsDomain.validationStartTime

	return &validationStartTime
}

// SplitIndex is where the bars, opening at those times earliest first, divide: the
// first bar opening at or after the validation start begins the validation part.
// Either part holding no bar at all is refused — there would be nothing to judge.
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
