package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
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
	return marketDataKindDomain.retaining(requested, ErrStrategyScriptValidation, "這支策略腳本", "一支")
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

// RequireReplayableAs refuses to replay this strategy script where the other kind of
// market is what the replay walks over — the same refusal RequireRunnableAs gives a
// calculation, in the words of a replay.
func (marketDataKindDomain MarketDataKindDomain) RequireReplayableAs(expected vo.MarketDataKindVo) error {
	if marketDataKindDomain.value == expected {
		return nil
	}

	return fmt.Errorf("%w: 這支策略腳本吃的是%s，不能拿來做這一種重演",
		ErrStrategyScriptMarketDataKindMismatch, marketDataKindDomain.label())
}

// RetainingForTradingStrategy is Retaining for a trading strategy: a rewrite that says
// nothing about the kind keeps it, one that restates it changes nothing, and one that
// names the other kind is refused — for the reason a strategy script's kind never
// changes: every one of its signal sources eats the kind it was written for.
func (marketDataKindDomain MarketDataKindDomain) RetainingForTradingStrategy(
	requested string,
) (MarketDataKindDomain, error) {
	return marketDataKindDomain.retaining(requested, ErrTradingStrategyValidation, "這份交易策略", "一份")
}

// RetainingForStrategyBot is Retaining for a strategy bot: a rewrite that says nothing
// about the kind keeps it, one that restates it changes nothing, and one that names the
// other kind is refused — a bot reads one kind of market every round, and a bot that
// switched would be following rules written for the other kind.
func (marketDataKindDomain MarketDataKindDomain) RetainingForStrategyBot(
	requested string,
) (MarketDataKindDomain, error) {
	return marketDataKindDomain.retaining(requested, ErrStrategyBotValidation, "這台機器人", "一台")
}

// retaining is the one rule the three Retaining methods share, in the words of whoever
// is asking: saying nothing keeps the kind, restating it changes nothing, and naming the
// other kind is refused. What differs between them is only the sentinel the refusal
// counts as and the thing it names — one strategy script, one trading strategy, one bot.
func (marketDataKindDomain MarketDataKindDomain) retaining(
	requested string, validationSentinel error, subject string, anotherOne string,
) (MarketDataKindDomain, error) {
	if strings.TrimSpace(requested) == "" {
		return marketDataKindDomain, nil
	}

	requestedKind, declarationError := NewMarketDataKindDomain(requested)
	if declarationError != nil {
		return MarketDataKindDomain{}, fmt.Errorf("%w: %w", validationSentinel, declarationError)
	}

	if requestedKind.value != marketDataKindDomain.value {
		return MarketDataKindDomain{}, fmt.Errorf(
			"%w: 行情種類建立後不得更換——%s吃的是%s；要吃%s請另建%s",
			validationSentinel, subject, marketDataKindDomain.label(), requestedKind.label(), anotherOne)
	}

	return marketDataKindDomain, nil
}

// RequireFollowableByStrategyBotOf refuses a trading strategy of this kind to a bot of
// the other kind. A bot reads its own kind of market every round; handed rules written
// for the other kind it would feed their scripts the wrong shape of market, round after
// round, where nobody is reading.
func (marketDataKindDomain MarketDataKindDomain) RequireFollowableByStrategyBotOf(
	botMarketDataKind MarketDataKindDomain,
) error {
	if marketDataKindDomain.value == botMarketDataKind.value {
		return nil
	}

	return fmt.Errorf("%w: 這台機器人吃的是%s，那份交易策略吃的是%s——機器人只能引用行情種類相同的交易策略",
		ErrStrategyBotValidation, botMarketDataKind.label(), marketDataKindDomain.label())
}

// LeverageForStrategyBot is the leverage a bot of this kind stores for what its caller
// declared.
//
// A spot bot lends nothing, so it may only ever suggest what a spot replay could have
// modelled; the sentence comes from the model a replay asks, so the same figure typed
// into either comes back with the same words. A contract bot borrows by the rules a
// contract replay reads a leverage by: nothing at all is one times, and under one is
// refused. Whether the symbol allows that much is answered where the symbol's ladder
// is read — see ContractStrategyBotMarketDomain.
//
// Asked here rather than inside the position plan because the plan is also built
// every round, from settings already stored. Refusing there would stop bots that were
// saved before this rule existed — and it would stop them silently, one round at a
// time, where nobody is reading.
func (marketDataKindDomain MarketDataKindDomain) LeverageForStrategyBot(
	declaredLeverage decimal.Decimal,
) (decimal.Decimal, error) {
	if !marketDataKindDomain.IsContract() {
		if _, borrowingRefusal := NewSpotOnlyReplayDomain(
			"", declaredLeverage, decimal.Zero); borrowingRefusal != nil {
			return decimal.Zero, fmt.Errorf("%w: %s", ErrStrategyBotValidation, borrowingRefusal)
		}

		return decimal.Zero, nil
	}

	if declaredLeverage.IsZero() {
		return oneWhole, nil
	}

	if declaredLeverage.LessThan(oneWhole) {
		return decimal.Zero, fmt.Errorf(
			"%w: %s", ErrStrategyBotValidation, ErrLeverageMultiplierBelowOne)
	}

	return declaredLeverage, nil
}

// IsContract is whether this is the perpetual contract bar rather than the spot K
// candle.
func (marketDataKindDomain MarketDataKindDomain) IsContract() bool {
	return marketDataKindDomain.value == vo.MarketDataKindContractKCandle
}

// label is how the kind reads in a sentence meant for a person. Every refusal above
// names the kind, and they must all name it the same way.
func (marketDataKindDomain MarketDataKindDomain) label() string {
	if marketDataKindDomain.value == vo.MarketDataKindContractKCandle {
		return "合約行情"
	}

	return " K 線"
}
