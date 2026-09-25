package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// declarableMarketDataKinds is in the order offered back when a declaration is not recognised.
var declarableMarketDataKinds = []vo.MarketDataKindVo{
	vo.MarketDataKindKCandle,
	vo.MarketDataKindContractKCandle,
}

// MarketDataKindDomain is the market shape a strategy script's entry point takes, fixed at creation because a script switched to the other kind no longer fits its own entry point; its zero value is unusable.
type MarketDataKindDomain struct {
	value vo.MarketDataKindVo
}

// NewMarketDataKindDomain defaults a blank declaration to spot K candles for backward compatibility and matches case-insensitively; the refusal carries no sentinel because the caller decides its category.
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

// Retaining keeps the kind when a rewrite omits or restates it and refuses a switch.
func (marketDataKindDomain MarketDataKindDomain) Retaining(requested string) (MarketDataKindDomain, error) {
	return marketDataKindDomain.retaining(requested, ErrStrategyScriptValidation, "這支策略腳本", "一支")
}

// RequireRunnableAs refuses a mismatched kind up front, rather than letting the script fail as if it were written wrong.
func (marketDataKindDomain MarketDataKindDomain) RequireRunnableAs(expected vo.MarketDataKindVo) error {
	if marketDataKindDomain.value == expected {
		return nil
	}

	return fmt.Errorf("%w: 這支策略腳本吃的是%s，不能拿來做這一種指標計算",
		ErrStrategyScriptMarketDataKindMismatch, marketDataKindDomain.label())
}

// RequireReplayableAs is RequireRunnableAs worded for a replay.
func (marketDataKindDomain MarketDataKindDomain) RequireReplayableAs(expected vo.MarketDataKindVo) error {
	if marketDataKindDomain.value == expected {
		return nil
	}

	return fmt.Errorf("%w: 這支策略腳本吃的是%s，不能拿來做這一種重演",
		ErrStrategyScriptMarketDataKindMismatch, marketDataKindDomain.label())
}

// RetainingForTradingStrategy is Retaining for a trading strategy, whose signal sources all eat its kind.
func (marketDataKindDomain MarketDataKindDomain) RetainingForTradingStrategy(
	requested string,
) (MarketDataKindDomain, error) {
	return marketDataKindDomain.retaining(requested, ErrTradingStrategyValidation, "這份交易策略", "一份")
}

// RetainingForStrategyBot is Retaining for a strategy bot, which reads one kind of market every round.
func (marketDataKindDomain MarketDataKindDomain) RetainingForStrategyBot(
	requested string,
) (MarketDataKindDomain, error) {
	return marketDataKindDomain.retaining(requested, ErrStrategyBotValidation, "這台機器人", "一台")
}

// retaining is the rule shared by the Retaining methods; only the sentinel and the named subject differ.
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

// RequireFollowableByStrategyBotOf refuses a trading strategy whose kind differs from the bot's, which would otherwise silently feed the wrong market shape every round.
func (marketDataKindDomain MarketDataKindDomain) RequireFollowableByStrategyBotOf(
	botMarketDataKind MarketDataKindDomain,
) error {
	if marketDataKindDomain.value == botMarketDataKind.value {
		return nil
	}

	return fmt.Errorf("%w: 這台機器人吃的是%s，那份交易策略吃的是%s——機器人只能引用行情種類相同的交易策略",
		ErrStrategyBotValidation, botMarketDataKind.label(), marketDataKindDomain.label())
}

// LeverageForStrategyBot applies spot-replay rules to a spot bot and contract-replay rules (blank is 1x, below 1x refused) to a contract bot; the symbol ceiling is checked by ContractStrategyBotMarketDomain.
// It runs at save time rather than in the per-round position plan so bots saved before this rule are not silently stopped.
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

// IsContract reports whether this is the perpetual contract bar rather than the spot K candle.
func (marketDataKindDomain MarketDataKindDomain) IsContract() bool {
	return marketDataKindDomain.value == vo.MarketDataKindContractKCandle
}

// label is the kind's user-facing wording, shared by every refusal.
func (marketDataKindDomain MarketDataKindDomain) label() string {
	if marketDataKindDomain.value == vo.MarketDataKindContractKCandle {
		return "合約行情"
	}

	return " K 線"
}
