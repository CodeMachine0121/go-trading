package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// selectablePositionSizingModes is in the order offered back when a declaration is not recognised.
var selectablePositionSizingModes = []vo.PositionSizingModeVo{
	vo.PositionSizingModeAllIn,
	vo.PositionSizingModePercentage,
	vo.PositionSizingModeFixedAmount,
}

// oneHundredPercent is shared with the position plan, whose exit distances use the same percentage units.
var oneHundredPercent = decimal.NewFromInt(100)

// PositionSizingDomain settles a mode and its figure together, so no mode exists without a usable figure; its zero value is unusable.
type PositionSizingDomain struct {
	mode  vo.PositionSizingModeVo
	value decimal.Decimal
}

// NewPositionSizingDomain defaults to staking everything and refuses figures that could never stake anything, since a tradeless replay looks like a broken system.
// A fixed amount above starting capital is allowed because the balance changes during a replay.
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

	return PositionSizingDomain{}, positionSizingModeFailure(fmt.Sprintf(
		"每次開倉押多少只能是 %s 其中之一", strings.Join(selectableSpellings, "、")))
}

// newPositionSizingDomain is the only constructor, so a mode never coexists with a figure its rule refuses.
func newPositionSizingDomain(
	mode vo.PositionSizingModeVo, value decimal.Decimal,
) (PositionSizingDomain, error) {
	if mode == vo.PositionSizingModePercentage &&
		(!value.IsPositive() || value.GreaterThan(oneHundredPercent)) {
		return PositionSizingDomain{}, positionSizingFigureFailure("百分比必須大於零且不超過一百")
	}

	if mode == vo.PositionSizingModeFixedAmount && !value.IsPositive() {
		return PositionSizingDomain{}, positionSizingFigureFailure("固定金額必須大於零")
	}

	return PositionSizingDomain{mode: mode, value: value}, nil
}

func (positionSizingDomain PositionSizingDomain) Mode() vo.PositionSizingModeVo {
	return positionSizingDomain.mode
}

// Value is the declared figure (zero for all-in), kept so storage reads it back as it arrived rather than deriving it from StakeFor.
func (positionSizingDomain PositionSizingDomain) Value() decimal.Decimal {
	return positionSizingDomain.value
}

// NeverStakesUnder reports a percentage larger than the share the costs leave affordable, which could never stake anything at any balance; other modes cannot be judged in advance.
func (positionSizingDomain PositionSizingDomain) NeverStakesUnder(
	transactionCosts BacktestTransactionCostsDomain,
) bool {
	if positionSizingDomain.mode != vo.PositionSizingModePercentage {
		return false
	}

	return positionSizingDomain.value.GreaterThan(
		transactionCosts.MaximumStakeFrom(oneHundredPercent))
}

// StakeFor returns the stake and whether stake plus entry charge fit the cash; unaffordable just skips this opening.
// All-in shrinks to fit the costs, while typed figures are honoured or skipped, never shrunk.
func (positionSizingDomain PositionSizingDomain) StakeFor(
	availableCash decimal.Decimal,
	transactionCosts BacktestTransactionCostsDomain,
) (decimal.Decimal, bool) {
	maximumStake := transactionCosts.MaximumStakeFrom(availableCash)

	if positionSizingDomain.mode == vo.PositionSizingModeFixedAmount {
		return positionSizingDomain.value,
			positionSizingDomain.value.LessThanOrEqual(maximumStake)
	}

	stake := maximumStake
	if positionSizingDomain.mode == vo.PositionSizingModePercentage {
		stake = availableCash.Mul(positionSizingDomain.value).Div(oneHundredPercent)
	}

	return stake, stake.IsPositive() && stake.LessThanOrEqual(maximumStake)
}
