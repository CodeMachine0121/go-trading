package dto

import "encoding/json"

// IndicatorValueDto holds content in exactly one of Numbers or Booleans; a lone value
// occupies the first slot.
type IndicatorValueDto struct {
	IsList   bool
	Numbers  []float64
	Booleans []bool
}

// MarshalJSON writes a series as an array and a lone value as a scalar, reporting zero for
// an unfilled lone value.
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
