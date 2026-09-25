package domains

import (
	"errors"
	"strings"

	"github.com/shopspring/decimal"
)

// spotDeclaration is still accepted because declaring "spot" asks for the same replay as declaring nothing.
const spotDeclaration = "spot"

// noBorrowing treats zero and one-times leverage alike: a fully paid position.
var noBorrowing = decimal.NewFromInt(1)

// Sentinels for declarations this spot-only system refuses, kept here as the single source of their wording.
var (
	ErrSpotOnlyTradingMode = errors.New(
		"這個系統只重演現貨，沒有交易模式可以指定——" +
			"借錢、做空與強制平倉是合約帳戶的事，那是另外一件事")
	ErrSpotOnlyBorrowing = errors.New(
		"這個系統只重演現貨，開不了槓桿——現貨是拿現金換東西，沒有人借錢給你")
	// ErrSpotOnlyMaintenanceMarginRate is refused rather than ignored because the rate only means something to a borrowing account.
	ErrSpotOnlyMaintenanceMarginRate = errors.New(
		"這個系統只重演現貨，沒有維持保證金率——強制平倉是合約帳戶的事，那是另外一件事")
	// ErrLeverageMultiplierBelowOne keeps its own wording so 0.5 reads as half a position, not a loan.
	ErrLeverageMultiplierBelowOne = errors.New("槓桿倍數不得小於 1 倍")
)

// SpotOnlyReplayDomain is a temporary gate that refuses contract-only declarations and carries no state, so no downstream code can branch on a trading mode.
type SpotOnlyReplayDomain struct{}

// NewSpotOnlyReplayDomain accepts empty/"spot"/unleveraged declarations and refuses anything that would silently replay a different kind of run; zero values mean "not declared".
func NewSpotOnlyReplayDomain(
	declaredTradingMode string,
	declaredLeverageMultiplier decimal.Decimal,
	declaredMaintenanceMarginRate decimal.Decimal,
) (SpotOnlyReplayDomain, error) {
	normalizedDeclaration := strings.TrimSpace(declaredTradingMode)
	if normalizedDeclaration != "" &&
		!strings.EqualFold(normalizedDeclaration, spotDeclaration) {
		return SpotOnlyReplayDomain{}, ErrSpotOnlyTradingMode
	}

	// Checked before the borrowing rule so half a position gets its own message.
	if !declaredLeverageMultiplier.IsZero() &&
		declaredLeverageMultiplier.LessThan(noBorrowing) {
		return SpotOnlyReplayDomain{}, ErrLeverageMultiplierBelowOne
	}

	if declaredLeverageMultiplier.GreaterThan(noBorrowing) {
		return SpotOnlyReplayDomain{}, ErrSpotOnlyBorrowing
	}

	if !declaredMaintenanceMarginRate.IsZero() {
		return SpotOnlyReplayDomain{}, ErrSpotOnlyMaintenanceMarginRate
	}

	return SpotOnlyReplayDomain{}, nil
}
