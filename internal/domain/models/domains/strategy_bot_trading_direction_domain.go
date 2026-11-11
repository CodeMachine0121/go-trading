package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// StrategyBotTradingDirectionDomain is a bot's settled trading direction: the directions a contract bot faces, translated once into the contract trading mode its rounds and auto orders read; spot bots have none.
type StrategyBotTradingDirectionDomain struct {
	isContract  bool
	tradingMode ContractTradingModeDomain
}

// NewStrategyBotTradingDirectionDomain settles what was declared; nil means nothing was said, which is long and short for a contract bot.
func NewStrategyBotTradingDirectionDomain(
	marketDataKind MarketDataKindDomain, declared *[]string,
) (StrategyBotTradingDirectionDomain, error) {
	if !marketDataKind.IsContract() {
		if declared != nil && len(*declared) > 0 {
			return StrategyBotTradingDirectionDomain{}, fmt.Errorf(
				"%w: 現貨機器人沒有交易方向——現貨只能做多", ErrStrategyBotValidation)
		}

		return StrategyBotTradingDirectionDomain{}, nil
	}

	if declared == nil {
		return StrategyBotTradingDirectionDomain{
			isContract: true, tradingMode: NewContractTradingModeDomainFacing(true, true),
		}, nil
	}

	if len(*declared) == 0 {
		return StrategyBotTradingDirectionDomain{}, fmt.Errorf(
			"%w: 至少要選一個交易方向（%s、%s）",
			ErrStrategyBotValidation, vo.TradingDirectionLong, vo.TradingDirectionShort)
	}

	facesLong, facesShort := false, false
	for _, declaredDirection := range *declared {
		switch {
		case strings.EqualFold(strings.TrimSpace(declaredDirection), string(vo.TradingDirectionLong)):
			facesLong = true
		case strings.EqualFold(strings.TrimSpace(declaredDirection), string(vo.TradingDirectionShort)):
			facesShort = true
		default:
			return StrategyBotTradingDirectionDomain{}, fmt.Errorf(
				"%w: 交易方向只能是 %s 或 %s（做多、做空），不認得 %q",
				ErrStrategyBotValidation, vo.TradingDirectionLong, vo.TradingDirectionShort, declaredDirection)
		}
	}

	return StrategyBotTradingDirectionDomain{
		isContract: true, tradingMode: NewContractTradingModeDomainFacing(facesLong, facesShort),
	}, nil
}

// ContractTradingMode is what the bot stores and its rounds read; blank for spot bots.
func (tradingDirectionDomain StrategyBotTradingDirectionDomain) ContractTradingMode() string {
	if !tradingDirectionDomain.isContract {
		return ""
	}

	return string(tradingDirectionDomain.tradingMode.Value())
}

// Differs tells whether this direction is not the one the bot stores; a contract bot storing none reads as long-short.
func (tradingDirectionDomain StrategyBotTradingDirectionDomain) Differs(storedContractTradingMode string) bool {
	if !tradingDirectionDomain.isContract {
		return false
	}

	storedTradingMode, storedError := NewContractTradingModeDomain(storedContractTradingMode)
	if storedError != nil {
		return true
	}

	return storedTradingMode.Value() != tradingDirectionDomain.tradingMode.Value()
}
