package vo

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// IndicatorValueVo is one indicator's value; exactly one of Numbers, Booleans and Signal is set, and a lone value occupies the first slot of its slice.
type IndicatorValueVo struct {
	// IsList distinguishes a series of one from a lone value, since both are stored alike.
	IsList   bool
	Numbers  []float64
	Booleans []bool
	// Signal is never a series.
	Signal SignalVo
}

func (indicatorValueVo IndicatorValueVo) ToDto() dto.IndicatorValueDto {
	return dto.IndicatorValueDto{
		IsList:   indicatorValueVo.IsList,
		Numbers:  indicatorValueVo.Numbers,
		Booleans: indicatorValueVo.Booleans,
	}
}
