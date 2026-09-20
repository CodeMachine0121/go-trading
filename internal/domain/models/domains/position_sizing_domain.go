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

// oneHundredPercent is the whole of something, written as the percentage a caller
// would type.
//
// Shared with the position plan beside it, which measures its two exits in the same
// units: a distance of the whole price. Two names for a hundred in one package is one
// name too many — and the day somebody decides percentages are written as fractions,
// two of them would be two places to change.
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

	return PositionSizingDomain{}, positionSizingModeFailure(fmt.Sprintf(
		"每次開倉押多少只能是 %s 其中之一", strings.Join(selectableSpellings, "、")))
}

// newPositionSizingDomain is the only way an instance is built, so a mode can never
// exist alongside a figure its own rule would have refused.
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

// Value is the figure that was declared beside the mode. Staking everything needs
// none, and answers zero.
//
// It is here beside Mode so that whoever has to store these two can read them back
// the way they arrived. Working the figure back out of StakeFor is possible and
// wrong: it would depend on what the mode does with it, so the day a fourth mode
// exists, storing the third one silently changes.
func (positionSizingDomain PositionSizingDomain) Value() decimal.Decimal {
	return positionSizingDomain.value
}

// StakeFor is what this opening puts down given the cash on hand and what trading
// costs, and whether it can be put down at all.
//
// The second answer is not an error. A fixed amount the account cannot currently
// cover means this one opening does not happen — the replay carries on, and the
// account may well afford the next one. Reporting it as a failure would end a run
// over something that is a perfectly ordinary way for a strategy script to behave.
//
// Affording it means affording the stake **and** the charge for putting it down, and
// that is one comparison rather than three because the costs hand over a single
// ceiling to measure every mode against. Staking everything means staking that
// ceiling: the cash becomes the trade, not the position, so the position ends up a
// little smaller than the cash and nothing is left owing. The other two modes name a
// figure the caller typed, and a typed figure is honoured or skipped, never quietly
// shrunk — which is why a hundred percent and "everything" part ways once there is a
// charge, exactly as a fixed amount equal to the cash already does.
//
// It takes the costs rather than a ceiling already worked out so that the one caller
// with nothing to pay says so in its own words: a bot's position plan hands over the
// zero value, and that line is the only place the decision to leave live advice alone
// is visible. A number would have made the same thing read like a coincidence.
// NeverStakesUnder says whether this sizing could never put anything down under those
// costs, whatever the account happens to hold at the time.
//
// It exists for the reason the constructor's own refusals exist, in the same words: a
// figure that can never stake anything produces a replay with no trades at all, which
// reads on screen exactly like a broken system. The only thing new here is that the
// figure became unstakeable by meeting a *second* input — a percentage that was
// perfectly usable until somebody said what trading costs.
//
// Only the percentage mode can be settled in advance. It stakes a share of whatever is
// on hand, so if that share is bigger than the share the costs leave affordable, it is
// bigger on every candle and at every balance — the two scale together. Staking
// everything shrinks to fit by definition, and a fixed amount depends on a balance
// that moves, so neither can be judged before the walk begins.
//
// The affordable share is the ceiling worked out from a hundred: what proportion of
// the cash can actually be staked once its own entry charge has to fit beside it. A
// run paying nothing gets a hundred back, and no percentage may exceed that, so this
// answers false for every replay made before there were costs to declare.
func (positionSizingDomain PositionSizingDomain) NeverStakesUnder(
	transactionCosts BacktestTransactionCostsDomain,
) bool {
	if positionSizingDomain.mode != vo.PositionSizingModePercentage {
		return false
	}

	return positionSizingDomain.value.GreaterThan(
		transactionCosts.MaximumStakeFrom(oneHundredPercent))
}

func (positionSizingDomain PositionSizingDomain) StakeFor(
	availableCash decimal.Decimal, transactionCosts BacktestTransactionCostsDomain,
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
