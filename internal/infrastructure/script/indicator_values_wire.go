package script

import "github.com/CodeMachine0121/go-trading/internal/domain/models/vo"

// indicatorValuesWire is one run's whole set of indicator values as it crosses from a
// compartment back to the service — every named value taken apart for the trip, and
// put back together on arrival.
type indicatorValuesWire map[string]indicatorValueWire

// newIndicatorValuesWire takes one run's values apart for the trip across.
func newIndicatorValuesWire(indicatorValues map[string]vo.IndicatorValueVo) indicatorValuesWire {
	valuesWire := make(indicatorValuesWire, len(indicatorValues))
	for indicatorName, indicatorValue := range indicatorValues {
		valuesWire[indicatorName] = newIndicatorValueWire(indicatorValue)
	}

	return valuesWire
}

// toIndicatorValues puts one run's values back together exactly as they were taken
// apart. A set that arrives empty comes back as an empty set, never as none: an empty
// set is a valid result a script produced.
func (indicatorValuesWire indicatorValuesWire) toIndicatorValues() map[string]vo.IndicatorValueVo {
	indicatorValues := make(map[string]vo.IndicatorValueVo, len(indicatorValuesWire))
	for indicatorName, indicatorValue := range indicatorValuesWire {
		indicatorValues[indicatorName] = indicatorValue.toIndicatorValue()
	}

	return indicatorValues
}
