package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// strategyBotContractSymbolSuffix is what follows a contract bot's symbol wherever the
// symbol is written for its owner, so that a message about a perpetual contract can
// never be read as one about the spot market of the same name.
const strategyBotContractSymbolSuffix = " 永續合約"

// StrategyBotMarketDomain is which kind of account a bot speaks about, and everything
// that follows from it: what a conclusion asks that account to be holding, which word
// the headline uses for it, which colour marks it, and how the symbol is labelled.
//
// It is the one place a bot's words depend on its kind. A spot bot speaks of buying
// and getting out, which is all cash for goods can do; a contract bot speaks of going
// long, going short and closing either — and which of those a sell means depends on the
// trading mode of the rules it follows. Scattered across the message, the plan and the
// lifecycle lines, that choice would be three switches that have to agree forever.
type StrategyBotMarketDomain struct {
	isContract bool
	// tradingMode is only read on a contract bot. hasTradingMode is false when the
	// stored mode could not be read, which the save gate makes unreachable; such a
	// bot names no act rather than guessing which way somebody should trade.
	tradingMode    ContractTradingModeDomain
	hasTradingMode bool
}

// NewStrategyBotMarketDomain reads the bot's kind and, for a contract bot, the trading
// mode of the contract trading strategy it follows.
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

// TargetFor is what this conclusion asks the account to be holding. A spot account
// asks the signal itself; a contract account asks its trading mode.
func (strategyBotMarketDomain StrategyBotMarketDomain) TargetFor(signal SignalDomain) vo.TargetPositionVo {
	if !strategyBotMarketDomain.isContract {
		return signal.TargetPosition()
	}

	if !strategyBotMarketDomain.hasTradingMode {
		return vo.TargetPositionUnchanged
	}

	return strategyBotMarketDomain.tradingMode.TargetFor(signal)
}

// HeadlineVerb is what a message about this conclusion asks its reader to go and do.
//
// On a contract account a conclusion is an act on a position: go long, go short, or
// close the one side these rules can hold. The word says which side is closed, because
// a reader holding the other side must not read "close" as being about theirs.
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

// HeadlineMark is the coloured mark that opens the headline, so that the direction
// survives being skimmed. Anything the system did not recognise gets the neutral one,
// because a guess here is a guess about which way somebody should trade — and so does
// closing a position, which faces neither way.
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

// SymbolLabel is the symbol as its owner reads it: a contract bot's carries the words
// that say it is a perpetual contract.
func (strategyBotMarketDomain StrategyBotMarketDomain) SymbolLabel(symbol string) string {
	if !strategyBotMarketDomain.isContract {
		return symbol
	}

	return symbol + strategyBotContractSymbolSuffix
}

// RequireCurrentBars refuses a round whose source judged by bars that have stopped
// arriving. The bars are those the source's script saw, oldest first, at the source's
// own coarseness.
//
// Only a contract account asks it. A contract trades round the clock, so the newest bar
// should always be the bucket that just finished; one bucket behind is allowed, because
// ingestion writes a candle a little after the minute it covers. Anything further back
// is bars no longer coming in — typically a contract taken off the watchlist — and a
// conclusion read from them would be about the past, sent as though it were now.
//
// A spot account is not asked: spot markets close, and a newest bar from before the
// weekend is exactly what an honest reading of a closed market looks like.
func (strategyBotMarketDomain StrategyBotMarketDomain) RequireCurrentBars(
	interval AggregationIntervalDomain, barOpenTimes []time.Time, now time.Time,
) error {
	if !strategyBotMarketDomain.isContract || len(barOpenTimes) == 0 {
		return nil
	}

	latestFinishedBucket := interval.BucketStart(interval.BucketStart(now).Add(-time.Nanosecond))
	oldestAcceptableBucket := interval.BucketStart(latestFinishedBucket.Add(-time.Nanosecond))
	newestBar := barOpenTimes[len(barOpenTimes)-1]

	if newestBar.Before(oldestAcceptableBucket) {
		return fmt.Errorf("%w: 最新一格合約行情停在 %s，行情沒有再進來——這個合約標的可能已不在合約追蹤名單上",
			ErrStrategyBotMarketDataStale, newestBar.UTC().Format(time.RFC3339))
	}

	return nil
}

// ReferenceCandleWords is what the candle a reference price is read from is called: a
// spot bot quotes a K candle, a contract bot a contract K candle. The leading blank is
// the spacing the sentence needs before a Latin letter.
func (strategyBotMarketDomain StrategyBotMarketDomain) ReferenceCandleWords() string {
	if !strategyBotMarketDomain.isContract {
		return " K 線"
	}

	return "合約 K 線"
}

// TradingModeInWords is the contract trading mode as a person reads it. A spot bot, or
// a mode that could not be read, has nothing to say here.
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
