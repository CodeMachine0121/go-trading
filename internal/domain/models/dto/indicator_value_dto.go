package dto

import "encoding/json"

// IndicatorValueDto is one indicator's value as it leaves the domain. It knows the
// shape it promised — a series or a lone value — so it can be written out as that
// shape rather than as the way it happens to be stored.
type IndicatorValueDto struct {
	IsList   bool
	Numbers  []float64
	Booleans []bool
}

// MarshalJSON writes the value itself: an array for a series, the bare number or
// answer otherwise. Without it a reader would receive the storage — two slices —
// instead of the value.
func (indicatorValueDto IndicatorValueDto) MarshalJSON() ([]byte, error) {
	if indicatorValueDto.Numbers != nil {
		if indicatorValueDto.IsList {
			return json.Marshal(indicatorValueDto.Numbers)
		}

		return json.Marshal(indicatorValueDto.firstNumber())
	}

	if indicatorValueDto.IsList {
		return json.Marshal(indicatorValueDto.Booleans)
	}

	return json.Marshal(indicatorValueDto.firstBoolean())
}

// firstNumber and firstBoolean keep a lone value writable even when nothing was put
// in it, so a malformed value reports zero instead of bringing the response down.
func (indicatorValueDto IndicatorValueDto) firstNumber() float64 {
	if len(indicatorValueDto.Numbers) == 0 {
		return 0
	}

	return indicatorValueDto.Numbers[0]
}

func (indicatorValueDto IndicatorValueDto) firstBoolean() bool {
	if len(indicatorValueDto.Booleans) == 0 {
		return false
	}

	return indicatorValueDto.Booleans[0]
}
