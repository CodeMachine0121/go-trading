package domains

import (
	"errors"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// ErrIndicatorCalculationValidation marks a request the caller got wrong: the candle
// count, or a stretch of market too thin to yield even one value.
var ErrIndicatorCalculationValidation = errors.New("indicator calculation validation failed")

// ErrIndicatorCalculationCandleCountExceeded marks the one validation failure a
// caller can act on by changing two specific inputs: the span asked for, together
// with the look-back the algorithm declares, needs more candles than one call may
// read.
//
// It is told apart from the other validation failures for the same reason the
// mismatched-knob failure is told apart from a broken script: the caller has two
// concrete ways out — ask for a shorter span, or aggregate more coarsely — and a
// caller can only offer them if it knows this is the failure it got. Reading that
// out of the sentence would be matching on prose written for a person.
var ErrIndicatorCalculationCandleCountExceeded = errors.New("indicator calculation candle count exceeded")

// CandleCountExceeded builds that failure. It answers to both sentinels: it is still
// a validation failure to everything that only cares about that, and it is the
// candle-count one to whoever can offer the two ways out.
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

// ErrIndicatorCalculationCandleCoverageTooThin marks the one shortfall that cannot be
// answered at all: the finished buckets do not even reach the look-back the algorithm
// declares, so not a single indicator value can come out of them.
//
// Coming up short of the count asked for is no longer a refusal — the calculation
// answers over whatever is there. This is the floor below that, and it is told apart
// from every other validation failure for the same reason the over-wide one is: the
// caller has concrete ways out, and they are the *opposite* ways out. Too thin is
// fixed by reading the market more finely or by filling in the missing history; too
// wide is fixed by asking for less or reading more coarsely. Answered as one failure,
// a caller would send people to turn the dial the wrong way.
var ErrIndicatorCalculationCandleCoverageTooThin = errors.New(
	"indicator calculation candle coverage too thin")

// CandleCoverageTooThin builds that failure. It answers to both sentinels: still a
// validation failure to everything that only cares about that, and the too-thin one
// to whoever can offer the way out.
func CandleCoverageTooThin(availableCandleCount int, minimumCandleCount int) error {
	return &candleCoverageTooThinError{
		availableCandleCount: availableCandleCount,
		minimumCandleCount:   minimumCandleCount,
	}
}

// CandleCoverageShortfall digs the two counts out of a too-thin failure, so that
// whoever answers the caller can hand them over as values of their own.
//
// They travel as numbers rather than only inside the sentence for the same reason the
// undeclared knob's name does: a caller reading them out of the message would be
// parsing prose written for a person, which changes whenever the wording improves.
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

// ErrObservationWindowHoldsNoTrading marks a stretch of market that holds no market:
// the window a caller is looking at and the hours its venue trades do not overlap at
// all — a Taiwan night, a weekend, the wrong side of the closing bell.
//
// It is told apart from a stretch that is merely too thin because the ways out are
// not just different but unrelated. Too thin is answered by reading more finely or
// waiting for history to fill in; this one is answered by looking at a time the
// market was open. Offered as the same failure, a caller would send people to turn a
// dial that changes nothing.
var ErrObservationWindowHoldsNoTrading = errors.New("observation window holds no trading")

// ObservationWindowHoldsNoTrading builds that failure. It answers to both sentinels:
// still a validation failure to everything that only cares about that, and the
// no-trading one to whoever can offer the way out.
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

// ErrIndicatorScriptFailed marks a well-formed request whose script could not run:
// unreadable, failed while running, or reaching for something it may not use.
var ErrIndicatorScriptFailed = errors.New("indicator script failed")

// ErrIndicatorParameterNotDeclared is what a script reaching for a knob nobody
// declared comes back as.
//
// It is its own sentinel rather than a script failure, and that distinction is the
// whole point: renaming a knob and forgetting to change the line that reads it is an
// easy mistake and an invisible one, and reporting it as "your algorithm is broken"
// sends the person to read the wrong thing. What went wrong is that a name does not
// match — so that is what it says.
var ErrIndicatorParameterNotDeclared = errors.New("indicator parameter not declared")

// UndeclaredParameterName digs the name out of a mismatched-knob failure, so that
// whoever answers the caller can hand it over as a field of its own.
//
// A caller telling this failure apart by reading the message would be matching on
// prose written for a person — it changes whenever the wording improves. The name
// travels as a value instead.
func UndeclaredParameterName(err error) (string, bool) {
	var undeclared *undeclaredParameterError
	if !errors.As(err, &undeclared) {
		return "", false
	}

	return undeclared.name, true
}

// UndeclaredParameter builds the failure for a knob nobody declared, carrying both
// the sentence a person reads and the name a caller acts on.
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
