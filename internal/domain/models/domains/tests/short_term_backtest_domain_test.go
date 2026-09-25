package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// shortTermBar is an hourly spot bar, flat at its close unless open, high or low are given.
type shortTermBar struct {
	open   float64
	high   float64
	low    float64
	close  float64
	signal vo.SignalVo
}

func shortTermCandles(bars []shortTermBar) ([]vo.KCandleVo, []domains.SignalDomain) {
	candles := make([]vo.KCandleVo, 0, len(bars))
	signals := make([]domains.SignalDomain, 0, len(bars))
	for barIndex, bar := range bars {
		open, high, low := bar.open, bar.high, bar.low
		if open == 0 {
			open = bar.close
		}
		if high == 0 {
			high = max(open, bar.close)
		}
		if low == 0 {
			low = min(open, bar.close)
		}
		signal := bar.signal
		if signal == "" {
			signal = vo.SignalHold
		}

		candles = append(candles, vo.KCandleVo{
			Symbol:              "BTCUSDT",
			OpenTimeUnixSeconds: backtestStart.Add(time.Duration(barIndex) * time.Hour).Unix(),
			Open:                open,
			High:                high,
			Low:                 low,
			Close:               bar.close,
		})
		signals = append(signals, domains.NewSignalDomainOf(signal))
	}

	return candles, signals
}

func replaySpotBars(
	t *testing.T, requestDto dto.BacktestRequestDto, bars []shortTermBar,
) dto.BacktestResultDto {
	t.Helper()

	backtestDomain, err := domains.NewBacktestDomain(requestDto, backtestMaxCandleCount, backtestNow)
	require.NoError(t, err)
	candles, signals := shortTermCandles(bars)

	return backtestDomain.ReplayOver(candles, signals, nil)
}

func nextOpenRequest() dto.BacktestRequestDto {
	requestDto := backtestRequest()
	requestDto.FillTiming = "nextOpen"
	requestDto.EndTime = backtestStart.Add(24 * time.Hour)

	return requestDto
}

func TestSpotReplayFillTiming(t *testing.T) {
	t.Run("filling at the close is what it always was", func(t *testing.T) {
		requestDto := backtestRequest()
		requestDto.EndTime = backtestStart.Add(24 * time.Hour)

		resultDto := replaySpotBars(t, requestDto, []shortTermBar{
			{close: 100, signal: vo.SignalBuy}, {open: 101, close: 102, signal: vo.SignalSell}})

		require.Len(t, resultDto.ClosedTrades, 1)
		assertDecimalEqual(t, "100", resultDto.ClosedTrades[0].EntryPrice)
		assert.Equal(t, "close", resultDto.FillTiming)
	})

	t.Run("filling at the next open waits for the next bar", func(t *testing.T) {
		resultDto := replaySpotBars(t, nextOpenRequest(), []shortTermBar{
			{close: 100, signal: vo.SignalBuy}, {open: 101, close: 102, signal: vo.SignalSell}, {open: 103, close: 108}})

		require.Len(t, resultDto.ClosedTrades, 1)
		assertDecimalEqual(t, "101", resultDto.ClosedTrades[0].EntryPrice)
		assert.Equal(t, backtestStart.Add(time.Hour), resultDto.ClosedTrades[0].EntryTime)
		assertDecimalEqual(t, "103", resultDto.ClosedTrades[0].ExitPrice)
		assert.Equal(t, "nextOpen", resultDto.FillTiming)
	})

	t.Run("the last bar's signal is never filled", func(t *testing.T) {
		resultDto := replaySpotBars(t, nextOpenRequest(), []shortTermBar{
			{close: 100}, {close: 100, signal: vo.SignalBuy}})

		assert.Equal(t, 0, resultDto.Summary.PositionOpenCount)
	})

	t.Run("the bar filled at its open can stop the position it just opened", func(t *testing.T) {
		requestDto := nextOpenRequest()
		requestDto.StopLossPercentage = decimal.NewFromInt(2)

		resultDto := replaySpotBars(t, requestDto, []shortTermBar{
			{close: 99, signal: vo.SignalBuy}, {open: 100, low: 97, close: 99}})

		require.Len(t, resultDto.ClosedTrades, 1)
		assert.Equal(t, "stopLoss", resultDto.ClosedTrades[0].ExitReason)
		assertDecimalEqual(t, "98", resultDto.ClosedTrades[0].ExitPrice)
	})

	t.Run("an unknown fill timing is refused naming the field", func(t *testing.T) {
		requestDto := backtestRequest()
		requestDto.FillTiming = "intraday"

		_, err := domains.NewBacktestDomain(requestDto, backtestMaxCandleCount, backtestNow)

		require.ErrorIs(t, err, domains.ErrBacktestValidation)
		fieldName, _ := domains.BacktestFieldName(err)
		assert.Equal(t, "fillTiming", fieldName)
		assert.Contains(t, err.Error(), "close")
		assert.Contains(t, err.Error(), "nextOpen")
	})
}

func TestContractReplayFillTiming(t *testing.T) {
	nextOpenContractRequest := func() dto.ContractBacktestRequestDto {
		requestDto := contractReplayRequest()
		requestDto.AggregationInterval = "1h"
		requestDto.Leverage = decimal.NewFromInt(5)
		requestDto.TradingMode = "longOnly"
		requestDto.FillTiming = "nextOpen"

		return requestDto
	}

	t.Run("a settlement at the fill bar's open is not paid by the position it opens", func(t *testing.T) {
		resultDto := replayContract(t, nextOpenContractRequest(), contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 100}},
			contractSettlementAt(contractReplayStart.Add(time.Hour), "0.0001", "100"))

		assert.Equal(t, 1, resultDto.Summary.PositionOpenCount)
		assertDecimalEqual(t, "0", resultDto.Summary.TotalFundingFee)
	})

	t.Run("a later settlement in the fill bar is paid", func(t *testing.T) {
		requestDto := nextOpenContractRequest()
		requestDto.AggregationInterval = "1d"
		requestDto.EndTime = contractReplayStart.Add(5 * 24 * time.Hour)
		secondDay := contractReplayStart.Add(24 * time.Hour)

		// Opened at the second day's open with 500 units: the midnight settlement precedes the fill, the 08:00 one follows it.
		resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 100}},
			contractSettlementAt(secondDay, "0.0001", "100"),
			contractSettlementAt(secondDay.Add(8*time.Hour), "0.0001", "100"))

		assertDecimalEqual(t, "5", resultDto.Summary.TotalFundingFee)
	})

	t.Run("a position carried into the bar pays the settlement at its open", func(t *testing.T) {
		resultDto := replayContract(t, nextOpenContractRequest(), contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 100}, {close: 100}},
			contractSettlementAt(contractReplayStart.Add(2*time.Hour), "0.0001", "100"))

		assertDecimalEqual(t, "5", resultDto.Summary.TotalFundingFee)
	})

	t.Run("slippage still applies to the open it fills at", func(t *testing.T) {
		requestDto := nextOpenContractRequest()
		requestDto.SlippagePercentage = decimal.RequireFromString("0.1")

		resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 99, signal: vo.SignalBuy}, {open: 100, close: 104, signal: vo.SignalSell}, {close: 104}})

		require.Len(t, resultDto.ClosedTrades, 1)
		// Filled at the second bar's open of 100, not its close of 104.
		assertDecimalEqual(t, "100.1", resultDto.ClosedTrades[0].EntryPrice)
		assert.Equal(t, "nextOpen", resultDto.FillTiming)
	})
}

func outcomeOf(netProfit string) vo.TradeOutcomeVo {
	return vo.TradeOutcomeVo{
		NetProfit:   decimal.RequireFromString(netProfit),
		GrossProfit: decimal.RequireFromString(netProfit),
		EntryTime:   backtestStart,
		ExitTime:    backtestStart.Add(time.Hour),
	}
}

func TestBacktestTradeStatistics(t *testing.T) {
	t.Run("winners over losers, profit per trade and the longest losing run", func(t *testing.T) {
		statisticsDto := domains.NewBacktestTradeStatisticsDomain([]vo.TradeOutcomeVo{
			outcomeOf("300"), outcomeOf("-100"), outcomeOf("200"), outcomeOf("-100"),
		}).ToDto()

		require.NotNil(t, statisticsDto.ProfitFactor)
		assert.InDelta(t, 2.5, *statisticsDto.ProfitFactor, 1e-9)
		require.True(t, statisticsDto.Expectancy.Valid)
		assertDecimalEqual(t, "75", statisticsDto.Expectancy.Decimal)
		assert.Equal(t, 1, statisticsDto.MaximumConsecutiveLossCount)
	})

	t.Run("nothing lost leaves the profit factor inapplicable", func(t *testing.T) {
		statisticsDto := domains.NewBacktestTradeStatisticsDomain([]vo.TradeOutcomeVo{
			outcomeOf("100"), outcomeOf("50"),
		}).ToDto()

		assert.Nil(t, statisticsDto.ProfitFactor)
		assertDecimalEqual(t, "75", statisticsDto.Expectancy.Decimal)
	})

	t.Run("no finished trade leaves every figure inapplicable", func(t *testing.T) {
		statisticsDto := domains.NewBacktestTradeStatisticsDomain(nil).ToDto()

		assert.Nil(t, statisticsDto.ProfitFactor)
		assert.False(t, statisticsDto.Expectancy.Valid)
		assert.Nil(t, statisticsDto.AverageHoldingSeconds)
		assert.Nil(t, statisticsDto.CostToGrossProfitRatio)
		assert.Equal(t, 0, statisticsDto.MaximumConsecutiveLossCount)
	})

	t.Run("breaking even ends a losing run", func(t *testing.T) {
		statisticsDto := domains.NewBacktestTradeStatisticsDomain([]vo.TradeOutcomeVo{
			outcomeOf("-10"), outcomeOf("-20"), outcomeOf("0"), outcomeOf("-5"),
		}).ToDto()

		assert.Equal(t, 2, statisticsDto.MaximumConsecutiveLossCount)
	})

	t.Run("the charges over what the trades made before them", func(t *testing.T) {
		costly := vo.TradeOutcomeVo{
			NetProfit: decimal.NewFromInt(75), GrossProfit: decimal.NewFromInt(100),
			TransactionCost: decimal.NewFromInt(25), EntryTime: backtestStart, ExitTime: backtestStart.Add(time.Hour),
		}

		statisticsDto := domains.NewBacktestTradeStatisticsDomain([]vo.TradeOutcomeVo{costly, costly}).ToDto()

		require.NotNil(t, statisticsDto.CostToGrossProfitRatio)
		assert.InDelta(t, 0.25, *statisticsDto.CostToGrossProfitRatio, 1e-9)
	})

	t.Run("nothing made before the charges leaves the cost share inapplicable", func(t *testing.T) {
		statisticsDto := domains.NewBacktestTradeStatisticsDomain([]vo.TradeOutcomeVo{{
			NetProfit: decimal.NewFromInt(-50), GrossProfit: decimal.NewFromInt(-40),
			TransactionCost: decimal.NewFromInt(10), EntryTime: backtestStart, ExitTime: backtestStart,
		}}).ToDto()

		assert.Nil(t, statisticsDto.CostToGrossProfitRatio)
	})

	t.Run("the average holding time", func(t *testing.T) {
		oneHour := outcomeOf("10")
		threeHours := outcomeOf("10")
		threeHours.ExitTime = backtestStart.Add(3 * time.Hour)

		statisticsDto := domains.NewBacktestTradeStatisticsDomain([]vo.TradeOutcomeVo{oneHour, threeHours}).ToDto()

		require.NotNil(t, statisticsDto.AverageHoldingSeconds)
		assert.Equal(t, int64(2*60*60), *statisticsDto.AverageHoldingSeconds)
	})

	t.Run("a spot trade's charges are what it paid before its net profit", func(t *testing.T) {
		outcome := vo.ClosedTradeVo{
			Profit: decimal.NewFromInt(80), EntryCost: decimal.NewFromInt(10), ExitCost: decimal.NewFromInt(10),
		}.ToOutcomeVo()

		assertDecimalEqual(t, "100", outcome.GrossProfit)
		assertDecimalEqual(t, "20", outcome.TransactionCost)
	})

	t.Run("a contract trade's funding is put back but is not a charge", func(t *testing.T) {
		outcome := vo.ContractClosedTradeVo{
			Profit: decimal.NewFromInt(75), EntryCost: decimal.NewFromInt(10), ExitCost: decimal.NewFromInt(10),
			FundingFee: decimal.NewFromInt(5),
		}.ToOutcomeVo()

		assertDecimalEqual(t, "100", outcome.GrossProfit)
		assertDecimalEqual(t, "20", outcome.TransactionCost)
	})

	t.Run("the report card counts only finished trades", func(t *testing.T) {
		requestDto := backtestRequest()
		requestDto.EndTime = backtestStart.Add(24 * time.Hour)

		// One round trip of +100, then a position still open at the end.
		resultDto := replaySpotBars(t, requestDto, []shortTermBar{
			{close: 100, signal: vo.SignalBuy}, {close: 101, signal: vo.SignalSell},
			{close: 101, signal: vo.SignalBuy}, {close: 50}})

		require.Len(t, resultDto.ClosedTrades, 1)
		assertDecimalEqual(t, "100", resultDto.Summary.Expectancy.Decimal)
		assert.Equal(t, 0, resultDto.Summary.MaximumConsecutiveLossCount)
	})

	t.Run("the contract report card carries the same figures", func(t *testing.T) {
		requestDto := contractReplayRequest()
		requestDto.TradingMode = "longOnly"

		resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 110, signal: vo.SignalSell}})

		require.True(t, resultDto.Summary.Expectancy.Valid)
		assertDecimalEqual(t, "1000", resultDto.Summary.Expectancy.Decimal)
		assert.Nil(t, resultDto.Summary.ProfitFactor)
	})
}

func splitRequest(validationStartHour int) dto.BacktestRequestDto {
	requestDto := backtestRequest()
	requestDto.EndTime = backtestStart.Add(24 * time.Hour)
	requestDto.ValidationStartTime = backtestStart.Add(time.Duration(validationStartHour) * time.Hour)

	return requestDto
}

func TestSpotReplayValidationSplit(t *testing.T) {
	sixBars := []shortTermBar{
		{close: 100, signal: vo.SignalBuy}, {close: 110}, {close: 120},
		{close: 120, signal: vo.SignalBuy}, {close: 130, signal: vo.SignalSell}, {close: 130},
	}

	t.Run("a split replay hands back the whole and both parts", func(t *testing.T) {
		resultDto := replaySpotBars(t, splitRequest(3), sixBars)

		require.NotNil(t, resultDto.InSample)
		require.NotNil(t, resultDto.Validation)
		assert.Equal(t, 6, resultDto.UsedCandleCount)
		assert.Equal(t, 3, resultDto.InSample.UsedCandleCount)
		assert.Equal(t, 3, resultDto.Validation.UsedCandleCount)
		assert.Equal(t, backtestStart.Add(3*time.Hour), resultDto.Validation.StartTime)
		require.NotNil(t, resultDto.ValidationStartTime)
		assert.Equal(t, backtestStart.Add(3*time.Hour), *resultDto.ValidationStartTime)
	})

	t.Run("the validation part starts flat from the initial capital", func(t *testing.T) {
		resultDto := replaySpotBars(t, splitRequest(3), sixBars)

		// The in-sample long at 100 never reaches validation, where the only trade is 120 → 130.
		require.NotNil(t, resultDto.Validation)
		require.Len(t, resultDto.Validation.ClosedTrades, 1)
		assertDecimalEqual(t, "120", resultDto.Validation.ClosedTrades[0].EntryPrice)
		assertDecimalEqual(t, "10000", resultDto.Validation.Summary.InitialCapital)
		assert.Empty(t, resultDto.InSample.ClosedTrades)
		assert.Equal(t, 1, resultDto.InSample.Summary.PositionOpenCount)
	})

	t.Run("each part counts its own conflicted bars", func(t *testing.T) {
		backtestDomain, err := domains.NewBacktestDomain(splitRequest(3), backtestMaxCandleCount, backtestNow)
		require.NoError(t, err)
		candles, signals := shortTermCandles(sixBars)

		resultDto := backtestDomain.ReplayOver(candles, signals, []bool{true, false, false, true, true, false})

		assert.Equal(t, 3, resultDto.Summary.ConflictedCandleCount)
		assert.Equal(t, 1, resultDto.InSample.Summary.ConflictedCandleCount)
		assert.Equal(t, 2, resultDto.Validation.Summary.ConflictedCandleCount)
	})

	t.Run("no validation start is no split", func(t *testing.T) {
		requestDto := backtestRequest()
		requestDto.EndTime = backtestStart.Add(24 * time.Hour)

		resultDto := replaySpotBars(t, requestDto, sixBars)

		assert.Nil(t, resultDto.InSample)
		assert.Nil(t, resultDto.Validation)
		assert.Nil(t, resultDto.ValidationStartTime)
	})

	refusals := []struct {
		name            string
		validationStart time.Time
	}{
		{name: "a validation start at the stretch's start", validationStart: backtestStart},
		{name: "a validation start after the stretch's end", validationStart: backtestStart.Add(48 * time.Hour)},
	}
	for _, refusal := range refusals {
		t.Run(refusal.name+" is refused", func(t *testing.T) {
			requestDto := splitRequest(0)
			requestDto.ValidationStartTime = refusal.validationStart

			_, err := domains.NewBacktestDomain(requestDto, backtestMaxCandleCount, backtestNow)

			require.ErrorIs(t, err, domains.ErrBacktestValidation)
			fieldName, _ := domains.BacktestFieldName(err)
			assert.Equal(t, "validationStartTime", fieldName)
			assert.Contains(t, err.Error(), "驗證起點必須落在這次期間之內")
		})
	}

	t.Run("a validation part with no bar is refused before anything runs", func(t *testing.T) {
		backtestDomain, err := domains.NewBacktestDomain(splitRequest(10), backtestMaxCandleCount, backtestNow)
		require.NoError(t, err)

		_, err = backtestDomain.SelectInputCandles(storedBacktestCandlesForHours(0, 1, 2))

		require.ErrorIs(t, err, domains.ErrBacktestValidation)
		assert.Contains(t, err.Error(), "驗證段湊不出任何一格")
	})
}

func storedBacktestCandlesForHours(hours ...int) []entities.KCandle {
	kCandles := make([]entities.KCandle, 0, len(hours))
	for _, hour := range hours {
		kCandles = append(kCandles, storedBacktestCandleAt(hour, 100))
	}

	return kCandles
}

func TestContractReplayValidationSplit(t *testing.T) {
	requestDto := contractReplayRequest()
	requestDto.TradingMode = "longOnly"
	requestDto.ValidationStartTime = contractReplayStart.Add(2 * time.Minute)

	resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
		[]contractReplayBar{
			{close: 100, signal: vo.SignalBuy}, {close: 110},
			{close: 110, signal: vo.SignalBuy}, {close: 120, signal: vo.SignalSell},
		})

	require.NotNil(t, resultDto.InSample)
	require.NotNil(t, resultDto.Validation)
	assert.Equal(t, 2, resultDto.InSample.UsedCandleCount)
	assert.Equal(t, 2, resultDto.Validation.UsedCandleCount)
	require.Len(t, resultDto.Validation.ClosedTrades, 1)
	assertDecimalEqual(t, "110", resultDto.Validation.ClosedTrades[0].EntryPrice)
	assert.Equal(t, "longOnly", resultDto.Validation.TradingMode)
}

func TestValidationSplitEdges(t *testing.T) {
	t.Run("an in-sample part with no bar is refused", func(t *testing.T) {
		backtestDomain, err := domains.NewBacktestDomain(splitRequest(2), backtestMaxCandleCount, backtestNow)
		require.NoError(t, err)

		// The stored candles only begin after the validation start.
		_, err = backtestDomain.SelectInputCandles(storedBacktestCandlesForHours(5, 6, 7))

		require.ErrorIs(t, err, domains.ErrBacktestValidation)
		assert.Contains(t, err.Error(), "調參段湊不出任何一格")
	})

	t.Run("candles that cannot be split are replayed whole", func(t *testing.T) {
		backtestDomain, err := domains.NewBacktestDomain(splitRequest(10), backtestMaxCandleCount, backtestNow)
		require.NoError(t, err)
		candles, signals := shortTermCandles([]shortTermBar{{close: 100}, {close: 100}})

		resultDto := backtestDomain.ReplayOver(candles, signals, nil)

		assert.Nil(t, resultDto.Validation)
		assert.Equal(t, 2, resultDto.UsedCandleCount)
	})

	t.Run("an unsplit replay has no validation start", func(t *testing.T) {
		segments, err := domains.NewBacktestSegmentsDomain(time.Time{}, backtestStart, backtestStart.Add(time.Hour))
		require.NoError(t, err)

		assert.False(t, segments.IsSplit())
		assert.Nil(t, segments.ValidationStartTime())
	})

	t.Run("contract bars with none after the validation start are refused", func(t *testing.T) {
		requestDto := contractReplayRequest()
		requestDto.ValidationStartTime = contractReplayStart.Add(10 * time.Minute)
		contractBacktestDomain, err := domains.NewContractBacktestDomain(requestDto,
			contractReplayRules(t, contractReplaySpecification()), 100000, contractReplayStart.Add(30*24*time.Hour))
		require.NoError(t, err)

		_, err = contractBacktestDomain.SelectInput(contractReplayCandles("1m", []contractReplayBar{{close: 100}, {close: 100}}))

		require.ErrorIs(t, err, domains.ErrBacktestValidation)
		assert.Contains(t, err.Error(), "驗證段湊不出任何一格")
	})

	t.Run("contract bars that cannot be split are replayed whole", func(t *testing.T) {
		requestDto := contractReplayRequest()
		unsplitDomain, err := domains.NewContractBacktestDomain(requestDto,
			contractReplayRules(t, contractReplaySpecification()), 100000, contractReplayStart.Add(30*24*time.Hour))
		require.NoError(t, err)
		alignment, err := unsplitDomain.SelectInput(contractReplayCandles("1m", []contractReplayBar{{close: 100}, {close: 100}}))
		require.NoError(t, err)
		requestDto.ValidationStartTime = contractReplayStart.Add(10 * time.Minute)
		splitDomain, err := domains.NewContractBacktestDomain(requestDto,
			contractReplayRules(t, contractReplaySpecification()), 100000, contractReplayStart.Add(30*24*time.Hour))
		require.NoError(t, err)

		resultDto := splitDomain.ReplayOver(alignment,
			[]domains.SignalDomain{domains.NewSignalDomainOf(vo.SignalHold), domains.NewSignalDomainOf(vo.SignalHold)}, nil, nil)

		assert.Nil(t, resultDto.Validation)
		assert.Equal(t, 2, resultDto.UsedCandleCount)
	})
}

func TestReplayCeilingIsTheReplaysOwn(t *testing.T) {
	requestDto := backtestRequest()
	requestDto.AggregationInterval = "1m"
	requestDto.EndTime = backtestStart.Add(1999 * time.Minute)

	t.Run("a replay within its own ceiling is accepted beyond a single query's", func(t *testing.T) {
		_, err := domains.NewBacktestDomain(requestDto, 50000, backtestNow)

		assert.NoError(t, err)
	})

	t.Run("a replay beyond its ceiling is refused saying both numbers", func(t *testing.T) {
		_, err := domains.NewBacktestDomain(requestDto, 1000, backtestNow)

		require.ErrorIs(t, err, domains.ErrBacktestValidation)
		assert.Contains(t, err.Error(), "2000 根")
		assert.Contains(t, err.Error(), "最多 1000 根")
	})
}

func TestContractNextOpenFillBarReachesItsOwnExitLevels(t *testing.T) {
	requestDto := contractReplayRequest()
	requestDto.TradingMode = "longOnly"
	requestDto.FillTiming = "nextOpen"
	requestDto.StopLossPercentage = decimal.NewFromInt(2)

	// Filled at the second bar's open of 100; its low of 97 hits the 98 stop and the curve records the bar's close.
	resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
		[]contractReplayBar{{close: 99, signal: vo.SignalBuy}, {open: 100, low: 97, close: 99}})

	require.Len(t, resultDto.ClosedTrades, 1)
	assert.Equal(t, "stopLoss", resultDto.ClosedTrades[0].ExitReason)
	assertDecimalEqual(t, "98", resultDto.ClosedTrades[0].ExitPrice)
	assertDecimalEqual(t, "9800", resultDto.EquityCurve[1].Equity)
}
