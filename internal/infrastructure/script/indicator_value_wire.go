package script

import "github.com/CodeMachine0121/go-trading/internal/domain/models/vo"

// indicatorValueWire carries explicit presence flags because the encoding cannot tell an empty series from a missing one.
type indicatorValueWire struct {
	IsList      bool
	Numbers     []float64
	HasNumbers  bool
	Booleans    []bool
	HasBooleans bool
	Signal      vo.SignalVo
}

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
