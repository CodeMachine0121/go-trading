package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// declarableMarketDataKinds is the entire set a strategy script may declare, in the
// order it is offered back when a declaration is not recognised.
var declarableMarketDataKinds = []vo.MarketDataKindVo{
	vo.MarketDataKindKCandle,
	vo.MarketDataKindContractKCandle,
}

// MarketDataKindDomain is which kind of market a strategy script eats, and the two
// rules that follow from it: a script keeps the kind it was created with, and it runs
// only where that kind of market is what gets handed over.
//
// The kind is part of what the algorithm *is*. Its entry point receives one shape or
// the other, so a script switched to the other kind is not the same script reading
// different data — it is a script that no longer fits its own entry point. That is why
// the kind is settled once and never changed, rather than being a setting of a run.
//
// Its zero value is not a usable kind; it is only ever returned alongside an error.
type MarketDataKindDomain struct {
	value vo.MarketDataKindVo
}

// NewMarketDataKindDomain reads what was declared. Declaring nothing is the spot K
// candle, which is what every strategy script was before there was a choice — so a
// caller written before the choice existed keeps working untouched. Spelling is
// forgiving about surrounding blanks and letter case; anything else is refused,
// naming what could have been declared instead.
//
// The refusal carries the reason alone, with no sentinel of its own: which kind of
// failure an unrecognised kind counts as belongs to whoever asked.
func NewMarketDataKindDomain(declared string) (MarketDataKindDomain, error) {
	normalizedDeclaration := strings.TrimSpace(declared)
	if normalizedDeclaration == "" {
		return MarketDataKindDomain{value: vo.MarketDataKindKCandle}, nil
	}

	for _, declarableMarketDataKind := range declarableMarketDataKinds {
		if strings.EqualFold(string(declarableMarketDataKind), normalizedDeclaration) {
			return MarketDataKindDomain{value: declarableMarketDataKind}, nil
		}
	}

	declarableSpellings := make([]string, 0, len(declarableMarketDataKinds))
	for _, declarableMarketDataKind := range declarableMarketDataKinds {
		declarableSpellings = append(declarableSpellings, string(declarableMarketDataKind))
	}

	return MarketDataKindDomain{}, fmt.Errorf(
		"行情種類只能是 %s 其中之一", strings.Join(declarableSpellings, "、"))
}

func (marketDataKindDomain MarketDataKindDomain) Value() vo.MarketDataKindVo {
	return marketDataKindDomain.value
}

// Retaining is the kind a rewrite ends up with, this being the kind the strategy
// script already has.
//
// A rewrite that says nothing about the kind keeps it — the caller changing a name has
// no reason to know the kind at all. One that restates the same kind is not changing
// anything either. One that names the other kind is refused: see the type's comment
// for why a script never changes the market it eats.
func (marketDataKindDomain MarketDataKindDomain) Retaining(requested string) (MarketDataKindDomain, error) {
	if strings.TrimSpace(requested) == "" {
		return marketDataKindDomain, nil
	}

	requestedKind, declarationError := NewMarketDataKindDomain(requested)
	if declarationError != nil {
		return MarketDataKindDomain{}, fmt.Errorf("%w: %w", ErrStrategyScriptValidation, declarationError)
	}

	if requestedKind.value != marketDataKindDomain.value {
		return MarketDataKindDomain{}, fmt.Errorf(
			"%w: 行情種類建立後不得更換——這支策略腳本吃的是%s；要吃%s請另建一支",
			ErrStrategyScriptValidation, marketDataKindDomain.label(), requestedKind.label())
	}

	return marketDataKindDomain, nil
}

// RequireRunnableAs refuses to run this strategy script where the other kind of market
// is what gets handed over. Letting it through would only move the failure: the script
// would be fed a shape its entry point does not take, and the caller would read
// "the script is written wrong" about a script that is written exactly right for the
// market it was made for.
func (marketDataKindDomain MarketDataKindDomain) RequireRunnableAs(expected vo.MarketDataKindVo) error {
	if marketDataKindDomain.value == expected {
		return nil
	}

	return fmt.Errorf("%w: 這支策略腳本吃的是%s，不能拿來做這一種指標計算",
		ErrStrategyScriptMarketDataKindMismatch, marketDataKindDomain.label())
}

// label is how the kind reads in a sentence meant for a person. Both refusals above
// name the kind, and they must name it the same way.
func (marketDataKindDomain MarketDataKindDomain) label() string {
	if marketDataKindDomain.value == vo.MarketDataKindContractKCandle {
		return "合約行情"
	}

	return " K 線"
}
