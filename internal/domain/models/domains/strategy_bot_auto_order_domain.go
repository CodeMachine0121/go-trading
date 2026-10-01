package domains

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// StrategyBotAutoOrderDomain decides whether a bot may switch auto order on; switching it off needs no decision, so it is not here.
type StrategyBotAutoOrderDomain struct {
	bot entities.StrategyBot
}

func NewStrategyBotAutoOrderDomain(bot entities.StrategyBot) StrategyBotAutoOrderDomain {
	return StrategyBotAutoOrderDomain{bot: bot}
}

func (autoOrderDomain StrategyBotAutoOrderDomain) IsEnabled() bool {
	return autoOrderDomain.bot.AutoOrderEnabled
}

// RequireEnableable checks that a key exists before checking its markets, since without a key the markets question has no answer.
func (autoOrderDomain StrategyBotAutoOrderDomain) RequireEnableable(
	binanceTradingKeyStatus dto.BinanceTradingKeyStatusDto,
) error {
	if !binanceTradingKeyStatus.Configured {
		return fmt.Errorf("%w: 請先完成幣安交易金鑰設定，才能打開自動下單",
			ErrStrategyBotAutoOrderKeyNotConfigured)
	}

	return NewTradableMarketsDomainOf(binanceTradingKeyStatus.TradableMarkets).
		RequireCovering(autoOrderDomain.bot.MarketDataKind)
}
