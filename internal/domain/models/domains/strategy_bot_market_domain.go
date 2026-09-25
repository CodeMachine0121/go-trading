package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// strategyBotContractSymbolSuffix keeps a contract message from being mistaken for the same-named spot market.
const strategyBotContractSymbolSuffix = " 永續合約"

// contractMarketStalenessAllowance is the slack for late one-minute candle writes before a contract counts as no longer ingesting.
const contractMarketStalenessAllowance = 5 * time.Minute

// StrategyBotMarketDomain is the single place a bot's wording (target, headline verb, colour mark, symbol label) depends on spot vs contract and the contract trading mode.
type StrategyBotMarketDomain struct {
	isContract bool
	// hasTradingMode is false when the stored mode is unreadable (the save gate prevents this); such a bot then names no act.
	tradingMode    ContractTradingModeDomain
	hasTradingMode bool
}

func NewStrategyBotMarketDomain(
	marketDataKind string, contractTradingMode string,
) StrategyBotMarketDomain {
	if marketDataKind != string(vo.MarketDataKindContractKCandle) {
		return StrategyBotMarketDomain{}
	}

	tradingMode, tradingModeError := NewContractTradingModeDomain(contractTradingMode)

	return StrategyBotMarketDomain{
		isContract:     true,
		tradingMode:    tradingMode,
		hasTradingMode: tradingModeError == nil,
	}
}

func (strategyBotMarketDomain StrategyBotMarketDomain) IsContract() bool {
	return strategyBotMarketDomain.isContract
}

// TargetFor defers to the signal for spot and to the trading mode for contracts.
func (strategyBotMarketDomain StrategyBotMarketDomain) TargetFor(signal SignalDomain) vo.TargetPositionVo {
	if !strategyBotMarketDomain.isContract {
		return signal.TargetPosition()
	}

	if !strategyBotMarketDomain.hasTradingMode {
		return vo.TargetPositionUnchanged
	}

	return strategyBotMarketDomain.tradingMode.TargetFor(signal)
}

// HeadlineVerb names which side a contract close applies to so a holder of the other side doesn't act on it.
func (strategyBotMarketDomain StrategyBotMarketDomain) HeadlineVerb(signal SignalDomain) string {
	if !strategyBotMarketDomain.isContract {
		return signal.HeadlineVerb()
	}

	switch strategyBotMarketDomain.TargetFor(signal) {
	case vo.TargetPositionLong:
		return "做多"
	case vo.TargetPositionShort:
		return "做空"
	case vo.TargetPositionFlat:
		if strategyBotMarketDomain.tradingMode.Value() == vo.ContractTradingModeShortOnly {
			return "平空"
		}

		return "平多"
	}

	return signal.InWords()
}

// HeadlineMark uses the neutral mark for closes and unrecognised values rather than guessing a direction.
func (strategyBotMarketDomain StrategyBotMarketDomain) HeadlineMark(signal SignalDomain) string {
	if !strategyBotMarketDomain.isContract {
		switch signal.Value() {
		case vo.SignalBuy:
			return "🟢"
		case vo.SignalSell:
			return "🔴"
		}

		return "⚪"
	}

	switch strategyBotMarketDomain.TargetFor(signal) {
	case vo.TargetPositionLong:
		return "🟢"
	case vo.TargetPositionShort:
		return "🔴"
	}

	return "⚪"
}

func (strategyBotMarketDomain StrategyBotMarketDomain) SymbolLabel(symbol string) string {
	if !strategyBotMarketDomain.isContract {
		return symbol
	}

	return symbol + strategyBotContractSymbolSuffix
}

// RequireCurrentMarket refuses a contract round whose newest one-minute candle is stale (sources' own bars can be legitimately old); spot is exempt because spot markets close.
func (strategyBotMarketDomain StrategyBotMarketDomain) RequireCurrentMarket(
	newestCandleOpenTime time.Time, hasNewestCandle bool, now time.Time,
) error {
	if !strategyBotMarketDomain.isContract {
		return nil
	}

	if !hasNewestCandle {
		return fmt.Errorf("%w: 這個合約標的還沒有任何一分鐘合約 K 線", ErrStrategyBotMarketDataStale)
	}

	if newestCandleOpenTime.Before(now.Add(-contractMarketStalenessAllowance)) {
		return fmt.Errorf("%w: 最新一根合約 K 線停在 %s，行情沒有再進來——這個合約標的可能已不在合約追蹤名單上",
			ErrStrategyBotMarketDataStale, newestCandleOpenTime.UTC().Format(time.RFC3339))
	}

	return nil
}

// ReferenceCandleWords names the candle kind a reference price comes from; the spot variant's leading blank is intentional spacing.
func (strategyBotMarketDomain StrategyBotMarketDomain) ReferenceCandleWords() string {
	if !strategyBotMarketDomain.isContract {
		return " K 線"
	}

	return "合約 K 線"
}

// TradingModeInWords is empty for spot bots and unreadable modes.
func (strategyBotMarketDomain StrategyBotMarketDomain) TradingModeInWords() string {
	if !strategyBotMarketDomain.isContract || !strategyBotMarketDomain.hasTradingMode {
		return ""
	}

	switch strategyBotMarketDomain.tradingMode.Value() {
	case vo.ContractTradingModeLongOnly:
		return "只做多"
	case vo.ContractTradingModeShortOnly:
		return "只做空"
	}

	return "多空反手"
}
