package domains

import "github.com/CodeMachine0121/go-trading/internal/domain/models/vo"

// SignalDomain is one candle's buy/sell/hold opinion from a signal-kind script; the runner has already rejected any other value.
type SignalDomain struct {
	value vo.SignalVo
}

// NewSignalDomain reads the signal from the script result's reserved signal key.
func NewSignalDomain(indicatorValues map[string]vo.IndicatorValueVo) SignalDomain {
	return SignalDomain{value: indicatorValues[vo.SignalIndicatorKey].Signal}
}

// NewSignalDomainOf wraps a signal that was already concluded, e.g. a combined strategy replay or a bot round.
func NewSignalDomainOf(signal vo.SignalVo) SignalDomain {
	return SignalDomain{value: signal}
}

func (signalDomain SignalDomain) Value() vo.SignalVo {
	return signalDomain.value
}

// InWords keeps the script's own vocabulary so readers can trace a conclusion back to its sources; unknown values are shown verbatim rather than guessed.
func (signalDomain SignalDomain) InWords() string {
	switch signalDomain.value {
	case vo.SignalBuy:
		return "買入"
	case vo.SignalSell:
		return "賣出"
	case vo.SignalHold:
		return "持有"
	}

	return string(signalDomain.value)
}

// TargetPosition maps buy to long and sell to flat; hold and unrecognised values leave the position unchanged so an unknown signal never closes positions.
func (signalDomain SignalDomain) TargetPosition() vo.TargetPositionVo {
	switch signalDomain.value {
	case vo.SignalBuy:
		return vo.TargetPositionLong
	case vo.SignalSell:
		return vo.TargetPositionFlat
	}

	return vo.TargetPositionUnchanged
}

// HeadlineVerb renders sell as 出場 because the reader is often already flat and cannot act on 賣出.
func (signalDomain SignalDomain) HeadlineVerb() string {
	if signalDomain.value == vo.SignalSell {
		return "出場"
	}

	return signalDomain.InWords()
}
