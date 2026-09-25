package domains

import (
	"errors"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// ErrIndicatorCalculationValidation marks a request the caller got wrong.
var ErrIndicatorCalculationValidation = errors.New("indicator calculation validation failed")

// ErrIndicatorCalculationCandleCountExceeded is distinguishable so callers can suggest a shorter span or a coarser interval without parsing the message.
var ErrIndicatorCalculationCandleCountExceeded = errors.New("indicator calculation candle count exceeded")

// CandleCountExceeded matches both ErrIndicatorCalculationValidation and ErrIndicatorCalculationCandleCountExceeded.
func CandleCountExceeded(neededCandleCount int, maxCandleCount int) error {
	return &candleCountExceededError{
		neededCandleCount: neededCandleCount,
		maxCandleCount:    maxCandleCount,
	}
}

type candleCountExceededError struct {
	neededCandleCount int
	maxCandleCount    int
}

func (exceeded *candleCountExceededError) Error() string {
	return fmt.Sprintf("%v: 這一段配上回看根數要用到 %d 根，超過單次可用的最大根數（最多 %d 根）",
		ErrIndicatorCalculationValidation, exceeded.neededCandleCount, exceeded.maxCandleCount)
}

func (exceeded *candleCountExceededError) Unwrap() []error {
	return []error{ErrIndicatorCalculationValidation, ErrIndicatorCalculationCandleCountExceeded}
}

// ErrIndicatorCalculationCandleCoverageTooThin marks buckets that do not reach the declared look-back; it is separate from the too-wide error because the remedies are opposite.
var ErrIndicatorCalculationCandleCoverageTooThin = errors.New(
	"indicator calculation candle coverage too thin")

// CandleCoverageTooThin matches both ErrIndicatorCalculationValidation and ErrIndicatorCalculationCandleCoverageTooThin.
func CandleCoverageTooThin(availableCandleCount int, minimumCandleCount int) error {
	return &candleCoverageTooThinError{
		availableCandleCount: availableCandleCount,
		minimumCandleCount:   minimumCandleCount,
	}
}

// CandleCoverageShortfall extracts the available and minimum counts so callers need not parse the message.
func CandleCoverageShortfall(err error) (int, int, bool) {
	var tooThin *candleCoverageTooThinError
	if !errors.As(err, &tooThin) {
		return 0, 0, false
	}

	return tooThin.availableCandleCount, tooThin.minimumCandleCount, true
}

type candleCoverageTooThinError struct {
	availableCandleCount int
	minimumCandleCount   int
}

func (tooThin *candleCoverageTooThinError) Error() string {
	return fmt.Sprintf(
		"%v: K 線不足，走完的刻度區間目前湊得出 %d 根，但這支算法至少要 %d 根才算得出一個值",
		ErrIndicatorCalculationValidation,
		tooThin.availableCandleCount, tooThin.minimumCandleCount)
}

func (tooThin *candleCoverageTooThinError) Unwrap() []error {
	return []error{ErrIndicatorCalculationValidation, ErrIndicatorCalculationCandleCoverageTooThin}
}

// ErrObservationWindowHoldsNoTrading marks a window with no overlap with the venue's trading hours, whose remedy (a different time) is unrelated to changing the interval.
var ErrObservationWindowHoldsNoTrading = errors.New("observation window holds no trading")

// ObservationWindowHoldsNoTrading matches both ErrIndicatorCalculationValidation and ErrObservationWindowHoldsNoTrading.
func ObservationWindowHoldsNoTrading(market vo.MarketVo) error {
	return &observationWindowHoldsNoTradingError{market: market}
}

type observationWindowHoldsNoTradingError struct {
	market vo.MarketVo
}

func (holdsNoTrading *observationWindowHoldsNoTradingError) Error() string {
	return fmt.Sprintf("%v: 要看的這一段時間裡，%s 沒有交易——換彙總刻度沒有用，請改看有交易的時間",
		ErrIndicatorCalculationValidation, holdsNoTrading.market)
}

func (holdsNoTrading *observationWindowHoldsNoTradingError) Unwrap() []error {
	return []error{ErrIndicatorCalculationValidation, ErrObservationWindowHoldsNoTrading}
}

// ErrIndicatorScriptFailed marks a well-formed request whose script could not compile, failed at runtime, or used something forbidden.
var ErrIndicatorScriptFailed = errors.New("indicator script failed")

// ErrIndicatorParameterNotDeclared is separate from a script failure so a renamed-but-not-updated knob is reported as a name mismatch, not a broken algorithm.
var ErrIndicatorParameterNotDeclared = errors.New("indicator parameter not declared")

// UndeclaredParameterName extracts the knob name so callers need not parse the message.
func UndeclaredParameterName(err error) (string, bool) {
	var undeclared *undeclaredParameterError
	if !errors.As(err, &undeclared) {
		return "", false
	}

	return undeclared.name, true
}

func UndeclaredParameter(name string) error {
	return &undeclaredParameterError{name: name}
}

type undeclaredParameterError struct {
	name string
}

func (undeclared *undeclaredParameterError) Error() string {
	return fmt.Sprintf("%v: 算式取用了參數 %q，但這一次沒有宣告這個名字",
		ErrIndicatorParameterNotDeclared, undeclared.name)
}

func (undeclared *undeclaredParameterError) Unwrap() error {
	return ErrIndicatorParameterNotDeclared
}
