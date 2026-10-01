package domains

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
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

// RequireEnableable checks that a key exists before checking its markets; blank bot kinds are spot, as every bot stored before kinds existed is a spot bot.
func (autoOrderDomain StrategyBotAutoOrderDomain) RequireEnableable(
	binanceTradingKeyStatus dto.BinanceTradingKeyStatusDto,
) error {
	if !binanceTradingKeyStatus.Configured {
		return fmt.Errorf("%w: 請先完成幣安交易金鑰設定，才能打開自動下單",
			ErrStrategyBotAutoOrderKeyNotConfigured)
	}

	requiredMarket, requiredMarketLabel := vo.TradableMarketSpot, "現貨"
	if vo.MarketDataKindVo(autoOrderDomain.bot.MarketDataKind) == vo.MarketDataKindContractKCandle {
		requiredMarket, requiredMarketLabel = vo.TradableMarketContract, "合約"
	}

	if !NewTradableMarketsDomainOf(binanceTradingKeyStatus.TradableMarkets).Covers(requiredMarket) {
		return fmt.Errorf("%w: 這組幣安交易金鑰沒有%s交易權限",
			ErrStrategyBotAutoOrderMarketNotCovered, requiredMarketLabel)
	}

	return nil
}
