package script

import "github.com/CodeMachine0121/go-trading/internal/domain/models/vo"

// indicatorValueWire is one indicator value as it crosses from a compartment back to
// the service. It exists because the encoding across that boundary cannot tell an
// empty series from no series at all — both arrive as nothing — and the difference
// matters: an empty series is a value the script produced, a missing one is a kind
// that does not hold that content. So each content says outright whether it is there.
type indicatorValueWire struct {
	IsList      bool
	Numbers     []float64
	HasNumbers  bool
	Booleans    []bool
	HasBooleans bool
	Signal      vo.SignalVo
}

// newIndicatorValueWire takes an indicator value apart for the trip across.
func newIndicatorValueWire(indicatorValue vo.IndicatorValueVo) indicatorValueWire {
	return indicatorValueWire{
		IsList:      indicatorValue.IsList,
		Numbers:     indicatorValue.Numbers,
		HasNumbers:  indicatorValue.Numbers != nil,
		Booleans:    indicatorValue.Booleans,
		HasBooleans: indicatorValue.Booleans != nil,
		Signal:      indicatorValue.Signal,
	}
}

// toIndicatorValue puts the value back together exactly as it was taken apart.
func (indicatorValueWire indicatorValueWire) toIndicatorValue() vo.IndicatorValueVo {
	indicatorValue := vo.IndicatorValueVo{IsList: indicatorValueWire.IsList, Signal: indicatorValueWire.Signal}
	if indicatorValueWire.HasNumbers {
		indicatorValue.Numbers = append(make([]float64, 0, len(indicatorValueWire.Numbers)), indicatorValueWire.Numbers...)
	}
	if indicatorValueWire.HasBooleans {
		indicatorValue.Booleans = append(make([]bool, 0, len(indicatorValueWire.Booleans)), indicatorValueWire.Booleans...)
	}

	return indicatorValue
}
