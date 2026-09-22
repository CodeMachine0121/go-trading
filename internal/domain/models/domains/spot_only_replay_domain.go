package domains

import (
	"fmt"
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

// SpotOnlyReplayDomain is the sentence "this system only ever replays spot", held in
// one place so that the four callers who have to say it say it identically.
//
// **Nothing survives it, and that is the point.** It reads what a caller declared,
// refuses anything only a contract account could have meant, and hands back a value
// with nothing on it. A version that answered "which rules is this" would leave the
// choice alive — something downstream would ask, and the question this whole slice
// exists to delete would still be there, just quieter.
//
// The two refusals it owns are the only thing a user can see that is new here. The
// feature itself is a removal; these sentences are what stands where the removed thing
// was, and four copies of them would drift apart the first time somebody improved one.
// Its predecessor — a trading mode's BorrowingRefusal — existed for exactly this
// reason, and this inherits the job word for word.
//
// **It is a gate, not a foundation.** When a replay of contracts lands, those
// declarations mean something again, and that slice decides what — at which point
// this goes. Its four callers are already asking the question it would answer.
type SpotOnlyReplayDomain struct{}

// NewSpotOnlyReplayDomain reads what the caller declared and settles both rules.
//
// The judgement is the same for both: **refuse what would change the answer, accept
// what already means spot.** Nothing, "spot", nothing borrowed — all three describe
// the replay this system performs, so saying them is free. Anything else describes a
// replay it does not perform, and the alternative to refusing is handing back a report
// card of a different run that looks exactly like a correct one. Nothing on that page
// would say so, which is the one failure this system's own rules refuse everywhere
// else.
func NewSpotOnlyReplayDomain(
	declaredTradingMode string,
	declaredLeverageMultiplier decimal.Decimal,
) (SpotOnlyReplayDomain, error) {
	normalizedDeclaration := strings.TrimSpace(declaredTradingMode)
	if normalizedDeclaration != "" &&
		!strings.EqualFold(normalizedDeclaration, spotDeclaration) {
		return SpotOnlyReplayDomain{}, fmt.Errorf(
			"這個系統只重演現貨，沒有交易模式可以指定——" +
				"借錢、做空與強制平倉是合約帳戶的事，那是另外一件事")
	}

	// Below one is refused before the spot rule is reached, and in the words it has
	// always been refused in. Somebody who typed 0.5 meant half a position, and the
	// sentence about not being able to borrow would send them to look for a loan they
	// never asked for.
	if !declaredLeverageMultiplier.IsZero() &&
		declaredLeverageMultiplier.LessThan(noBorrowing) {
		return SpotOnlyReplayDomain{}, fmt.Errorf("槓桿倍數不得小於 1 倍")
	}

	// Nothing at all and one times are both a position paid for in full. There is no
	// lender either way, so there is nothing to refuse.
	if declaredLeverageMultiplier.GreaterThan(noBorrowing) {
		return SpotOnlyReplayDomain{}, fmt.Errorf(
			"這個系統只重演現貨，開不了槓桿——現貨是拿現金換東西，沒有人借錢給你")
	}

	return SpotOnlyReplayDomain{}, nil
}
