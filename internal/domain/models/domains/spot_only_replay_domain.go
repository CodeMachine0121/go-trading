package domains

import (
	"errors"
	"strings"

	"github.com/shopspring/decimal"
)

// spotDeclaration is the one thing a caller may still say about which rules to trade
// by, and saying it changes nothing: it is what they get anyway.
//
// It is kept as a spelling a caller may send rather than dropped along with the rest,
// because a request that says "spot" and a request that says nothing are asking for
// the same replay. Refusing the first would refuse somebody for being explicit.
const spotDeclaration = "spot"

// noBorrowing is a position worth exactly what was put down for it — the only kind
// this system replays. Nothing at all and one times are the same thing said twice.
var noBorrowing = decimal.NewFromInt(1)

// The two things a caller can declare that this system no longer does, each its own
// sentinel so that whoever asked can point at the right box without matching on prose
// written for a person.
//
// Both sentences live here and nowhere else. Every caller refuses in these words, and
// a second copy would drift apart on the first day somebody improved one of them.
var (
	// ErrSpotOnlyTradingMode is any set of rules other than spot being asked for.
	ErrSpotOnlyTradingMode = errors.New(
		"這個系統只重演現貨，沒有交易模式可以指定——" +
			"借錢、做空與強制平倉是合約帳戶的事，那是另外一件事")
	// ErrSpotOnlyBorrowing is a multiplier bigger than one — somebody else's money in
	// a system where there is nobody to borrow from.
	ErrSpotOnlyBorrowing = errors.New(
		"這個系統只重演現貨，開不了槓桿——現貨是拿現金換東西，沒有人借錢給你")
	// ErrSpotOnlyMaintenanceMarginRate is a rate at which a position would be closed
	// out for running out of collateral. It is refused rather than ignored for the
	// same reason a multiplier is: it only means something to an account that
	// borrowed, so somebody who sent one was picturing a different system.
	ErrSpotOnlyMaintenanceMarginRate = errors.New(
		"這個系統只重演現貨，沒有維持保證金率——強制平倉是合約帳戶的事，那是另外一件事")
	// ErrLeverageMultiplierBelowOne is a figure under one, refused in the words it has
	// always been refused in: somebody who typed 0.5 meant half a position, and the
	// sentence about borrowing would send them looking for a loan they never asked for.
	ErrLeverageMultiplierBelowOne = errors.New("槓桿倍數不得小於 1 倍")
)

// SpotOnlyReplayDomain is the sentence "this system only ever replays spot", held in
// one place so that every caller who has to say it says it identically.
//
// **Nothing survives it, and that is the point.** It reads what a caller declared,
// refuses anything only a contract account could have meant, and hands back a value
// with nothing on it. A version that answered "which rules is this" would leave the
// choice alive — something downstream would ask, and the question this whole slice
// exists to delete would still be there, just quieter.
//
// The refusals it owns are the only thing a user can see that is new here. The feature
// itself is a removal; these sentences are what stands where the removed thing was.
// Its predecessor — a trading mode's BorrowingRefusal — existed for exactly this
// reason, and this inherits the job word for word.
//
// **It is a gate, not a foundation.** When a replay of contracts lands, those
// declarations mean something again, and that slice decides what — at which point
// this goes. Its callers are already asking the question it would answer.
type SpotOnlyReplayDomain struct{}

// NewSpotOnlyReplayDomain reads what the caller declared and settles every rule about
// it.
//
// The judgement is the same throughout: **refuse what would change the answer, accept
// what already means spot.** Nothing, "spot", nothing borrowed — all three describe
// the replay this system performs, so saying them is free. Anything else describes a
// replay it does not perform, and the alternative to refusing is handing back a report
// card of a different run that looks exactly like a correct one. Nothing on that page
// would say so, which is the one failure this system's own rules refuse everywhere
// else.
//
// A caller with only one of the three declarations to make passes nothing for the
// others, and nothing is what all three of them most often are.
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

	// Below one is refused before the spot rule is reached, so that half a position is
	// answered as itself rather than as a loan.
	if !declaredLeverageMultiplier.IsZero() &&
		declaredLeverageMultiplier.LessThan(noBorrowing) {
		return SpotOnlyReplayDomain{}, ErrLeverageMultiplierBelowOne
	}

	// Nothing at all and one times are both a position paid for in full. There is no
	// lender either way, so there is nothing to refuse.
	if declaredLeverageMultiplier.GreaterThan(noBorrowing) {
		return SpotOnlyReplayDomain{}, ErrSpotOnlyBorrowing
	}

	// Asked last because it is the one of the three that cannot be meant innocently:
	// nothing and zero are what every caller sends, and a figure means somebody was
	// describing an account that can be closed out for running low on collateral.
	if !declaredMaintenanceMarginRate.IsZero() {
		return SpotOnlyReplayDomain{}, ErrSpotOnlyMaintenanceMarginRate
	}

	return SpotOnlyReplayDomain{}, nil
}
