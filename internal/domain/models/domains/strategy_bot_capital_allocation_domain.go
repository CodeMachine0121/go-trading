package domains

import (
	"fmt"
	"slices"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// strategyBotAllocationDecimalPlaces keeps shares readable and makes the equal split's rounding predictable.
const strategyBotAllocationDecimalPlaces = 2

// StrategyBotCapitalAllocationDomain is how a bot's total capital is shared among its watched symbols: fixed shares, never borrowed, summing to at most 100%.
type StrategyBotCapitalAllocationDomain struct {
	percentages map[string]decimal.Decimal
}

// NewStrategyBotCapitalAllocationDomain checks the owner's shares when any are given; given none, it keeps the retained shares if they still cover exactly these symbols validly, and otherwise splits equally, so it never refuses an owner who gave nothing.
// The symbols must already be normalized.
func NewStrategyBotCapitalAllocationDomain(
	symbols []string, requested []dto.StrategyBotCapitalAllocationDto, retained []dto.StrategyBotCapitalAllocationDto,
) (StrategyBotCapitalAllocationDomain, error) {
	ownerGave := len(requested) > 0
	given := requested
	if !ownerGave {
		given = retained
	}

	percentages := map[string]decimal.Decimal{}
	refusal := error(nil)
	for _, allocation := range given {
		tradingSymbol, symbolError := NewTradingSymbolDomain(strings.TrimSpace(allocation.Symbol))
		_, alreadyGiven := percentages[tradingSymbol.Value()]
		switch {
		case symbolError != nil:
			refusal = fmt.Errorf("%w: 資金分配裡的交易標的有誤：%w", ErrStrategyBotValidation, symbolError)
		case !slices.Contains(symbols, tradingSymbol.Value()):
			refusal = fmt.Errorf("%w: %s 不是這台機器人的監聽標的", ErrStrategyBotValidation, tradingSymbol.Value())
		case alreadyGiven:
			refusal = fmt.Errorf("%w: %s 的分配比例給了兩次", ErrStrategyBotValidation, tradingSymbol.Value())
		}
		if refusal != nil {
			break
		}
		percentages[tradingSymbol.Value()] = allocation.AllocationPercentage
	}

	for _, symbol := range symbols {
		if _, hasPercentage := percentages[symbol]; refusal == nil && !hasPercentage {
			refusal = fmt.Errorf("%w: %s 沒有分配比例——要嘛每個監聽標的都給，要嘛都不給",
				ErrStrategyBotValidation, symbol)
		}
	}

	total := decimal.Zero
	for _, symbol := range symbols {
		percentage := percentages[symbol]
		switch {
		case refusal != nil:
		case !percentage.IsPositive() || percentage.GreaterThan(oneHundredPercent):
			refusal = fmt.Errorf("%w: %s 的分配比例必須大於 0、不超過 100（單位：%%）", ErrStrategyBotValidation, symbol)
		case !percentage.Equal(percentage.Truncate(strategyBotAllocationDecimalPlaces)):
			refusal = fmt.Errorf("%w: %s 的分配比例最多到小數點後兩位", ErrStrategyBotValidation, symbol)
		}
		total = total.Add(percentage)
	}

	if refusal == nil && total.GreaterThan(oneHundredPercent) {
		refusal = fmt.Errorf("%w: 分配比例加起來是 %s%%，不得超過 100%%", ErrStrategyBotValidation, total.String())
	}

	if refusal == nil {
		return StrategyBotCapitalAllocationDomain{percentages: percentages}, nil
	}

	if ownerGave {
		return StrategyBotCapitalAllocationDomain{}, refusal
	}

	// Truncated, with the remainder left unallocated, so an equal split never sums past 100%.
	equalShare := decimal.Zero
	if len(symbols) > 0 {
		equalShare = oneHundredPercent.Div(decimal.NewFromInt(int64(len(symbols)))).
			Truncate(strategyBotAllocationDecimalPlaces)
	}
	equalPercentages := map[string]decimal.Decimal{}
	for _, symbol := range symbols {
		equalPercentages[symbol] = equalShare
	}

	return StrategyBotCapitalAllocationDomain{percentages: equalPercentages}, nil
}

// PercentageOf is the symbol's share in percent; zero for a symbol the bot does not watch.
func (capitalAllocationDomain StrategyBotCapitalAllocationDomain) PercentageOf(symbol string) decimal.Decimal {
	return capitalAllocationDomain.percentages[symbol]
}
