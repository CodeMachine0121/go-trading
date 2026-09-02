package dto

import "encoding/json"

// IndicatorValueDto is one indicator's value as it leaves the domain. It knows the
// shape it promised — a series or a lone value — so it can be written out as that
// shape rather than as the way it happens to be stored. Exactly one of Numbers and
// Booleans holds the content, and a lone value occupies the first slot of it.
type IndicatorValueDto struct {
	IsList   bool
	Numbers  []float64
	Booleans []bool
}

// MarshalJSON writes the value itself: a series as a series, a lone value on its own.
// Without it a reader would receive the storage — two slices — instead of the value.
// A lone value that was never filled in reports zero rather than bringing the whole
// response down.
func (indicatorValueDto IndicatorValueDto) MarshalJSON() ([]byte, error) {
	if indicatorValueDto.IsList {
		if indicatorValueDto.Numbers != nil {
			return json.Marshal(indicatorValueDto.Numbers)
		}

		return json.Marshal(indicatorValueDto.Booleans)
	}

	if indicatorValueDto.Numbers != nil {
		loneNumber := 0.0
		if len(indicatorValueDto.Numbers) > 0 {
			loneNumber = indicatorValueDto.Numbers[0]
		}

		return json.Marshal(loneNumber)
	}

	loneAnswer := false
	if len(indicatorValueDto.Booleans) > 0 {
		loneAnswer = indicatorValueDto.Booleans[0]
	}

	return json.Marshal(loneAnswer)
}
