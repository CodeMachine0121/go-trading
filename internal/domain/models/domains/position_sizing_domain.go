package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// selectablePositionSizingModes is the entire set a caller may declare, in the order
// it is offered back when a declaration is not recognised.
var selectablePositionSizingModes = []vo.PositionSizingModeVo{
	vo.PositionSizingModeAllIn,
	vo.PositionSizingModePercentage,
	vo.PositionSizingModeFixedAmount,
}

// oneHundredPercent is the whole of the cash on hand, written as the percentage a
// caller would type.
var oneHundredPercent = decimal.NewFromInt(100)

// PositionSizingDomain is how much one opening stakes, and every rule about it.
//
// Building one settles the mode and its figure together, so a mode can never exist
// without a usable figure and nothing downstream checks again.
//
// Its zero value is not a usable mode; it is only ever returned alongside an error.
type PositionSizingDomain struct {
	mode  vo.PositionSizingModeVo
	value decimal.Decimal
}

// NewPositionSizingDomain reads what the caller declared. Declaring nothing means
// staking everything, which is the simplest thing a person can mean by "just try it".
//
// A percentage must be above zero and no more than a hundred, and a fixed amount must
// be above zero. Both refusals are about the same thing: a figure that can never
// stake anything produces a replay with no trades at all, which reads on screen
// exactly like a broken system. Refusing at the door is the honest version of that.
//
// A fixed amount larger than the starting capital is allowed. What the account holds
// changes as the replay runs, so judging the figure against where it started would be
// a guard that guesses; and "not enough cash, skip this opening" is already a rule,
// so the case is covered without one.
func NewPositionSizingDomain(
	declaredMode string, declaredValue decimal.Decimal,
) (PositionSizingDomain, error) {
	normalizedDeclaration := strings.TrimSpace(declaredMode)
	if normalizedDeclaration == "" {
		return PositionSizingDomain{mode: vo.PositionSizingModeAllIn}, nil
	}

	for _, selectableMode := range selectablePositionSizingModes {
		if !strings.EqualFold(string(selectableMode), normalizedDeclaration) {
			continue
		}

		return newPositionSizingDomain(selectableMode, declaredValue)
	}

	selectableSpellings := make([]string, 0, len(selectablePositionSizingModes))
	for _, selectableMode := range selectablePositionSizingModes {
		selectableSpellings = append(selectableSpellings, string(selectableMode))
	}

	return PositionSizingDomain{}, fmt.Errorf(
		"%w: 每次開倉押多少只能是 %s 其中之一",
		ErrBacktestValidation, strings.Join(selectableSpellings, "、"))
}

// newPositionSizingDomain is the only way an instance is built, so a mode can never
// exist alongside a figure its own rule would have refused.
func newPositionSizingDomain(
	mode vo.PositionSizingModeVo, value decimal.Decimal,
) (PositionSizingDomain, error) {
	if mode == vo.PositionSizingModePercentage &&
		(!value.IsPositive() || value.GreaterThan(oneHundredPercent)) {
		return PositionSizingDomain{}, fmt.Errorf(
			"%w: 百分比必須大於零且不超過一百", ErrBacktestValidation)
	}

	if mode == vo.PositionSizingModeFixedAmount && !value.IsPositive() {
		return PositionSizingDomain{}, fmt.Errorf(
			"%w: 固定金額必須大於零", ErrBacktestValidation)
	}

	return PositionSizingDomain{mode: mode, value: value}, nil
}

func (positionSizingDomain PositionSizingDomain) Mode() vo.PositionSizingModeVo {
	return positionSizingDomain.mode
}

// StakeFor is what this opening puts down given the cash on hand, and whether it can
// be put down at all.
//
// The second answer is not an error. A fixed amount the account cannot currently
// cover means this one opening does not happen — the replay carries on, and the
// account may well afford the next one. Reporting it as a failure would end a run
// over something that is a perfectly ordinary way for a strategy to behave.
func (positionSizingDomain PositionSizingDomain) StakeFor(
	availableCash decimal.Decimal,
) (decimal.Decimal, bool) {
	if positionSizingDomain.mode == vo.PositionSizingModeFixedAmount {
		return positionSizingDomain.value,
			positionSizingDomain.value.LessThanOrEqual(availableCash)
	}

	stake := availableCash
	if positionSizingDomain.mode == vo.PositionSizingModePercentage {
		stake = availableCash.Mul(positionSizingDomain.value).Div(oneHundredPercent)
	}

	return stake, stake.IsPositive()
}
