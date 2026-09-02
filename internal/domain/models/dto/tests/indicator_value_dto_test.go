package dto_test

import (
	"encoding/json"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIndicatorValueDtoIsWrittenInTheShapeItPromised(t *testing.T) {
	testCases := []struct {
		name            string
		indicatorValue  dto.IndicatorValueDto
		expectedWritten string
	}{
		{
			name:            "a lone number is written on its own",
			indicatorValue:  dto.IndicatorValueDto{Numbers: []float64{110}},
			expectedWritten: "110",
		},
		{
			name:            "a series of numbers is written as a series",
			indicatorValue:  dto.IndicatorValueDto{IsList: true, Numbers: []float64{100, 105, 110}},
			expectedWritten: "[100,105,110]",
		},
		{
			name:            "a lone answer is written on its own",
			indicatorValue:  dto.IndicatorValueDto{Booleans: []bool{true}},
			expectedWritten: "true",
		},
		{
			name:            "a negative answer is still an answer",
			indicatorValue:  dto.IndicatorValueDto{Booleans: []bool{false}},
			expectedWritten: "false",
		},
		{
			name:            "a series of answers is written as a series",
			indicatorValue:  dto.IndicatorValueDto{IsList: true, Booleans: []bool{true, false, true}},
			expectedWritten: "[true,false,true]",
		},
		{
			name:            "an empty series of numbers is still a series",
			indicatorValue:  dto.IndicatorValueDto{IsList: true, Numbers: []float64{}},
			expectedWritten: "[]",
		},
		{
			name:            "an empty series of answers is still a series",
			indicatorValue:  dto.IndicatorValueDto{IsList: true, Booleans: []bool{}},
			expectedWritten: "[]",
		},
		{
			name:            "a lone number with nothing in it reports zero",
			indicatorValue:  dto.IndicatorValueDto{Numbers: []float64{}},
			expectedWritten: "0",
		},
		{
			name:            "a lone answer with nothing in it reports the negative",
			indicatorValue:  dto.IndicatorValueDto{},
			expectedWritten: "false",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			written, err := json.Marshal(testCase.indicatorValue)

			require.NoError(t, err)
			assert.JSONEq(t, testCase.expectedWritten, string(written))
		})
	}
}
