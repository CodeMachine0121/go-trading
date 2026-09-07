package vo

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// IndicatorValueVo is one indicator's value as the script runner produced it, in the
// shape its calculation declared. Exactly one of Numbers, Booleans and Signal holds
// the content — the one the declared kind calls for — and a lone number or answer
// occupies the first slot of its slice, so that a series and a single value are
// stored the same way.
type IndicatorValueVo struct {
	// IsList tells a series apart from a lone value. The two are stored alike, so
	// without it "one number" and "a series of one number" would be the same thing.
	IsList bool
	// Numbers is the content when the declared kind holds numbers; nil otherwise.
	Numbers []float64
	// Booleans is the content when the declared kind holds true/false answers;
	// nil otherwise.
	Booleans []bool
	// Signal is the content when the declared kind is the signal kind; empty
	// otherwise. It is a lone value, never a series.
	Signal SignalVo
}

// ToDto hands the value on in the shape it leaves the domain in.
func (indicatorValueVo IndicatorValueVo) ToDto() dto.IndicatorValueDto {
	return dto.IndicatorValueDto{
		IsList:   indicatorValueVo.IsList,
		Numbers:  indicatorValueVo.Numbers,
		Booleans: indicatorValueVo.Booleans,
	}
}
