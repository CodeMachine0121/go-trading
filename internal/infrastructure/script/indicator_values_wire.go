package script

import "github.com/CodeMachine0121/go-trading/internal/domain/models/vo"

type indicatorValuesWire map[string]indicatorValueWire

func newIndicatorValuesWire(indicatorValues map[string]vo.IndicatorValueVo) indicatorValuesWire {
	valuesWire := make(indicatorValuesWire, len(indicatorValues))
	for indicatorName, indicatorValue := range indicatorValues {
		valuesWire[indicatorName] = newIndicatorValueWire(indicatorValue)
	}

	return valuesWire
}

// toIndicatorValues returns an empty map, never nil, for an empty set.
func (indicatorValuesWire indicatorValuesWire) toIndicatorValues() map[string]vo.IndicatorValueVo {
	indicatorValues := make(map[string]vo.IndicatorValueVo, len(indicatorValuesWire))
	for indicatorName, indicatorValue := range indicatorValuesWire {
		indicatorValues[indicatorName] = indicatorValue.toIndicatorValue()
	}

	return indicatorValues
}
