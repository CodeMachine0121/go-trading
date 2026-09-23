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

// contractReplayStart is where every replayed stretch below begins.
var contractReplayStart = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// contractReplayBar is one bar of a replay as a table row: only the figures a case is
// about are given, and every one left at zero falls back to the close — a flat bar.
type contractReplayBar struct {
	close     float64
	high      float64
	low       float64
	markHigh  float64
	markLow   float64
	markClose float64
	signal    vo.SignalVo
}

// contractReplaySpecification is the trading specification every case replays under
// unless it says otherwise.
func contractReplaySpecification() entities.ContractTradingSymbol {
	confirmedAt := contractReplayStart
	fundingIntervalHours := 8

	return entities.ContractTradingSymbol{
		Symbol:                 "BTCUSDT",
		IsWatched:              true,
		TickSize:               decimal.NewNullDecimal(decimal.RequireFromString("0.01")),
		QuantityStep:           decimal.NewNullDecimal(decimal.RequireFromString("0.001")),
		MinimumQuantity:        decimal.NewNullDecimal(decimal.RequireFromString("0.001")),
		MinimumNotional:        decimal.NewNullDecimal(decimal.RequireFromString("5")),
		MaintenanceMarginRate:  decimal.NewNullDecimal(decimal.RequireFromString("0.005")),
		LiquidationFeeRate:     decimal.NewNullDecimal(decimal.RequireFromString("0.005")),
		FundingIntervalHours:   &fundingIntervalHours,
		SpecificationUpdatedAt: &confirmedAt,
	}
}

func contractReplayTier(tier int, floor string, notionalCap string, rate string, maximumLeverage int) entities.ContractMaintenanceMarginTier {
	return entities.ContractMaintenanceMarginTier{
		Symbol:                "BTCUSDT",
		Tier:                  tier,
		NotionalFloor:         decimal.RequireFromString(floor),
		NotionalCap:           decimal.RequireFromString(notionalCap),
		MaintenanceMarginRate: decimal.RequireFromString(rate),
		MaintenanceAmount:     decimal.Zero,
		MaximumLeverage:       maximumLeverage,
		ConfirmedAt:           time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
	}
}

func contractReplayRules(
	t *testing.T,
	specification entities.ContractTradingSymbol,
	tiers ...entities.ContractMaintenanceMarginTier,
) domains.ContractTradingRulesDomain {
	t.Helper()

	tradingRules, err := domains.NewContractTradingRulesDomain(specification, true, tiers)
	require.NoError(t, err)

	return tradingRules
}

// contractReplayRequest is a one-minute replay of the whole first day, staking
// everything, one times leverage, long and short, nothing charged.
func contractReplayRequest() dto.ContractBacktestRequestDto {
	return dto.ContractBacktestRequestDto{
		Symbol:              "BTCUSDT",
		AggregationInterval: "1m",
		StartTime:           contractReplayStart,
		EndTime:             contractReplayStart.Add(24 * time.Hour),
		InitialCapital:      decimal.NewFromInt(10000),
	}
}

// replayContract replays the bars, one per interval from the start, with the
// settlements given.
func replayContract(
	t *testing.T,
	requestDto dto.ContractBacktestRequestDto,
	tradingRules domains.ContractTradingRulesDomain,
	bars []contractReplayBar,
	settlements ...entities.ContractFundingRateSettlement,
) dto.ContractBacktestResultDto {
	t.Helper()

	contractBacktestDomain, err := domains.NewContractBacktestDomain(
		requestDto, tradingRules, 100000, contractReplayStart.Add(30*24*time.Hour))
	require.NoError(t, err)

	alignment, err := contractBacktestDomain.SelectInput(contractReplayCandles(requestDto.AggregationInterval, bars))
	require.NoError(t, err)

	signals := make([]domains.SignalDomain, 0, len(bars))
	for _, bar := range bars {
		signal := bar.signal
		if signal == "" {
			signal = vo.SignalHold
		}
		signals = append(signals, domains.NewSignalDomainOf(signal))
	}

	return contractBacktestDomain.ReplayOver(alignment, signals, settlements)
}

// contractReplayBarDuration is how far apart consecutive bars of that coarseness open.
func contractReplayBarDuration(aggregationInterval string) time.Duration {
	switch aggregationInterval {
	case "1h":
		return time.Hour
	case "1d":
		return 24 * time.Hour
	}

	return time.Minute
}

func contractReplayCandles(aggregationInterval string, bars []contractReplayBar) []entities.KCandleContract {
	kCandleContracts := make([]entities.KCandleContract, 0, len(bars))
	for barIndex, bar := range bars {
		high, low := bar.high, bar.low
		if high == 0 {
			high = bar.close
		}
		if low == 0 {
			low = bar.close
		}
		markHigh, markLow, markClose := bar.markHigh, bar.markLow, bar.markClose
		if markHigh == 0 {
			markHigh = high
		}
		if markLow == 0 {
			markLow = low
		}
		if markClose == 0 {
			markClose = bar.close
		}

		kCandleContracts = append(kCandleContracts, entities.KCandleContract{
			Symbol:    "BTCUSDT",
			OpenTime:  contractReplayStart.Add(time.Duration(barIndex) * contractReplayBarDuration(aggregationInterval)),
			Open:      decimal.NewFromFloat(bar.close),
			High:      decimal.NewFromFloat(high),
			Low:       decimal.NewFromFloat(low),
			Close:     decimal.NewFromFloat(bar.close),
			MarkOpen:  decimal.NewFromFloat(bar.close),
			MarkHigh:  decimal.NewFromFloat(markHigh),
			MarkLow:   decimal.NewFromFloat(markLow),
			MarkClose: decimal.NewFromFloat(markClose),
		})
	}

	return kCandleContracts
}

func contractSettlementAt(settlementTime time.Time, rate string, markPrice string) entities.ContractFundingRateSettlement {
	settlement := entities.ContractFundingRateSettlement{
		Symbol:         "BTCUSDT",
		SettlementTime: settlementTime,
		FundingRate:    decimal.RequireFromString(rate),
	}
	if markPrice != "" {
		settlement.MarkPrice = decimal.NewNullDecimal(decimal.RequireFromString(markPrice))
	}

	return settlement
}

func assertDecimalEqual(t *testing.T, expected string, actual decimal.Decimal) {
	t.Helper()

	assert.True(t, decimal.RequireFromString(expected).Equal(actual), "expected %s, got %s", expected, actual)
}

func contractReplayRefusal(
	t *testing.T, requestDto dto.ContractBacktestRequestDto, tradingRules domains.ContractTradingRulesDomain,
) (string, string) {
	t.Helper()

	_, err := domains.NewContractBacktestDomain(requestDto, tradingRules, 100000, contractReplayStart.Add(30*24*time.Hour))
	require.Error(t, err)
	require.ErrorIs(t, err, domains.ErrBacktestValidation)

	fieldName, _ := domains.BacktestFieldName(err)

	return fieldName, err.Error()
}

func TestContractBacktestMarginAndLeverage(t *testing.T) {
	t.Run("five times leverage puts down the margin and carries five times the notional", func(t *testing.T) {
		requestDto := contractReplayRequest()
		requestDto.Leverage = decimal.NewFromInt(5)

		resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 110, signal: vo.SignalSell}})

		require.Len(t, resultDto.ClosedTrades, 1)
		closedTrade := resultDto.ClosedTrades[0]
		assert.Equal(t, "long", closedTrade.Direction)
		assertDecimalEqual(t, "10000", closedTrade.Margin)
		assertDecimalEqual(t, "500", closedTrade.Quantity)
		assertDecimalEqual(t, "5", closedTrade.Leverage)
		// 500 units × (110 − 100).
		assertDecimalEqual(t, "5000", closedTrade.Profit)
		// The same bar reversed into a short.
		assert.Equal(t, 2, resultDto.Summary.PositionOpenCount)
	})

	t.Run("leverage left blank is one times", func(t *testing.T) {
		resultDto := replayContract(t, contractReplayRequest(), contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 110, signal: vo.SignalSell}})

		require.NotEmpty(t, resultDto.ClosedTrades)
		assertDecimalEqual(t, "100", resultDto.ClosedTrades[0].Quantity)
		assertDecimalEqual(t, "1", resultDto.Leverage)
	})

	t.Run("leverage below one is refused", func(t *testing.T) {
		requestDto := contractReplayRequest()
		requestDto.Leverage = decimal.RequireFromString("0.5")

		fieldName, message := contractReplayRefusal(t, requestDto, contractReplayRules(t, contractReplaySpecification()))

		assert.Equal(t, "leverage", fieldName)
		assert.Contains(t, message, "槓桿倍數不得小於 1 倍")
	})

	t.Run("leverage above the symbol's highest tier is refused naming the limit", func(t *testing.T) {
		requestDto := contractReplayRequest()
		requestDto.Leverage = decimal.NewFromInt(150)

		fieldName, message := contractReplayRefusal(t, requestDto, contractReplayRules(t, contractReplaySpecification(),
			contractReplayTier(1, "0", "50000", "0.004", 125),
			contractReplayTier(2, "50000", "250000", "0.005", 100)))

		assert.Equal(t, "leverage", fieldName)
		assert.Contains(t, message, "125")
	})

	t.Run("an opening whose notional falls in a tier allowing less leverage is blocked", func(t *testing.T) {
		requestDto := contractReplayRequest()
		requestDto.Leverage = decimal.NewFromInt(20)

		// 10,000 × 20 = 200,000 of notional, in the tier allowing ten times.
		resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification(),
			contractReplayTier(1, "0", "50000", "0.004", 125),
			contractReplayTier(2, "50000", "250000", "0.005", 10)),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 100}})

		assert.Equal(t, 0, resultDto.Summary.PositionOpenCount)
		assert.Equal(t, 1, resultDto.Summary.BlockedOpeningCount)
	})

	t.Run("a maintenance margin rate typed in is refused", func(t *testing.T) {
		requestDto := contractReplayRequest()
		requestDto.MaintenanceMarginRate = decimal.RequireFromString("0.5")

		fieldName, _ := contractReplayRefusal(t, requestDto, contractReplayRules(t, contractReplaySpecification()))

		assert.Equal(t, "maintenanceMarginRate", fieldName)
	})
}

func TestContractBacktestTradingModes(t *testing.T) {
	testCases := []struct {
		name           string
		tradingMode    string
		bars           []contractReplayBar
		wantOpenCount  int
		wantDirections []string
	}{
		{
			name:           "long and short opens a short on a sell while flat",
			tradingMode:    "longShort",
			bars:           []contractReplayBar{{close: 100, signal: vo.SignalSell}, {close: 90, signal: vo.SignalBuy}},
			wantOpenCount:  2,
			wantDirections: []string{"short"},
		},
		{
			name:           "long only ignores a sell while flat",
			tradingMode:    "longOnly",
			bars:           []contractReplayBar{{close: 100, signal: vo.SignalSell}, {close: 90}},
			wantOpenCount:  0,
			wantDirections: []string{},
		},
		{
			name:           "long only closes a long on a sell and stays flat",
			tradingMode:    "longOnly",
			bars:           []contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 110, signal: vo.SignalSell}},
			wantOpenCount:  1,
			wantDirections: []string{"long"},
		},
		{
			name:           "short only opens a short on a sell while flat",
			tradingMode:    "shortOnly",
			bars:           []contractReplayBar{{close: 100, signal: vo.SignalSell}, {close: 90}},
			wantOpenCount:  1,
			wantDirections: []string{},
		},
		{
			name:           "short only ignores a buy while flat",
			tradingMode:    "shortOnly",
			bars:           []contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 90}},
			wantOpenCount:  0,
			wantDirections: []string{},
		},
		{
			name:           "short only closes a short on a buy and stays flat",
			tradingMode:    "shortOnly",
			bars:           []contractReplayBar{{close: 100, signal: vo.SignalSell}, {close: 90, signal: vo.SignalBuy}},
			wantOpenCount:  1,
			wantDirections: []string{"short"},
		},
		{
			name:           "a blank trading mode is long and short",
			tradingMode:    "",
			bars:           []contractReplayBar{{close: 100, signal: vo.SignalSell}, {close: 90}},
			wantOpenCount:  1,
			wantDirections: []string{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			requestDto := contractReplayRequest()
			requestDto.TradingMode = testCase.tradingMode

			resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()), testCase.bars)

			assert.Equal(t, testCase.wantOpenCount, resultDto.Summary.PositionOpenCount)
			directions := make([]string, 0, len(resultDto.ClosedTrades))
			for _, closedTrade := range resultDto.ClosedTrades {
				directions = append(directions, closedTrade.Direction)
			}
			assert.Equal(t, testCase.wantDirections, directions)
		})
	}

	t.Run("a blank trading mode reports long and short", func(t *testing.T) {
		resultDto := replayContract(t, contractReplayRequest(), contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100}, {close: 100}})

		assert.Equal(t, "longShort", resultDto.TradingMode)
	})

	t.Run("spot is not a contract trading mode", func(t *testing.T) {
		requestDto := contractReplayRequest()
		requestDto.TradingMode = "spot"

		fieldName, message := contractReplayRefusal(t, requestDto, contractReplayRules(t, contractReplaySpecification()))

		assert.Equal(t, "tradingMode", fieldName)
		assert.Contains(t, message, "longShort、longOnly、shortOnly")
	})

	t.Run("a reversal the account can no longer afford leaves it flat", func(t *testing.T) {
		requestDto := contractReplayRequest()
		requestDto.PositionSizingMode = "fixedAmount"
		requestDto.PositionSizingValue = decimal.NewFromInt(10000)

		// The long loses half, so the 10,000 the short would need is not there.
		resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 50, signal: vo.SignalSell}, {close: 50}})

		assert.Equal(t, 1, resultDto.Summary.PositionOpenCount)
		require.Len(t, resultDto.ClosedTrades, 1)
		assertDecimalEqual(t, "5000", resultDto.Summary.FinalEquity)
	})
}

func TestContractBacktestTradingRules(t *testing.T) {
	t.Run("the quantity steps down and the margin left over stays in the account", func(t *testing.T) {
		requestDto := contractReplayRequest()
		requestDto.InitialCapital = decimal.NewFromInt(1000)
		requestDto.TradingMode = "longOnly"

		resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 810, signal: vo.SignalBuy}, {close: 810, signal: vo.SignalSell}})

		require.Len(t, resultDto.ClosedTrades, 1)
		assertDecimalEqual(t, "1.234", resultDto.ClosedTrades[0].Quantity)
		assertDecimalEqual(t, "999.54", resultDto.ClosedTrades[0].Margin)
		// While it was open the 0.46 never left the account.
		assertDecimalEqual(t, "1000", resultDto.EquityCurve[0].Equity)
	})

	t.Run("an opening below the minimum notional is blocked", func(t *testing.T) {
		requestDto := contractReplayRequest()
		requestDto.PositionSizingMode = "fixedAmount"
		requestDto.PositionSizingValue = decimal.NewFromInt(3)

		resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 100}})

		assert.Equal(t, 0, resultDto.Summary.PositionOpenCount)
		assert.Equal(t, 1, resultDto.Summary.BlockedOpeningCount)
	})

	t.Run("the stop is placed on the nearest tick", func(t *testing.T) {
		specification := contractReplaySpecification()
		specification.TickSize = decimal.NewNullDecimal(decimal.RequireFromString("0.1"))
		requestDto := contractReplayRequest()
		requestDto.StopLossPercentage = decimal.RequireFromString("2.03")

		// 97.97 unrounded would not be reached by a low of 98.0; 98.0 on the tick is.
		resultDto := replayContract(t, requestDto, contractReplayRules(t, specification),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 99, low: 98.0}})

		require.Len(t, resultDto.ClosedTrades, 1)
		assert.Equal(t, "stopLoss", resultDto.ClosedTrades[0].ExitReason)
		assertDecimalEqual(t, "98", resultDto.ClosedTrades[0].ExitPrice)
	})

	t.Run("a symbol without a trading specification is refused", func(t *testing.T) {
		_, err := domains.NewContractTradingRulesDomain(entities.ContractTradingSymbol{Symbol: "BTCUSDT"}, true, nil)

		require.ErrorIs(t, err, domains.ErrBacktestValidation)
		assert.Contains(t, err.Error(), "還沒有交易規格")
	})

	t.Run("a symbol the system does not know is refused the same way", func(t *testing.T) {
		_, err := domains.NewContractTradingRulesDomain(entities.ContractTradingSymbol{}, false, nil)

		require.ErrorIs(t, err, domains.ErrBacktestValidation)
		assert.Contains(t, err.Error(), "還沒有交易規格")
	})
}

func TestContractBacktestSlippageAndCosts(t *testing.T) {
	testCases := []struct {
		name           string
		slippage       string
		signal         vo.SignalVo
		wantEntryPrice string
	}{
		{name: "a buy fills dearer", slippage: "0.1", signal: vo.SignalBuy, wantEntryPrice: "100.1"},
		{name: "a sell fills cheaper", slippage: "0.1", signal: vo.SignalSell, wantEntryPrice: "99.9"},
		{name: "no slippage fills at the close", slippage: "0", signal: vo.SignalBuy, wantEntryPrice: "100"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			requestDto := contractReplayRequest()
			requestDto.SlippagePercentage = decimal.RequireFromString(testCase.slippage)
			closingSignal := vo.SignalSell
			if testCase.signal == vo.SignalSell {
				closingSignal = vo.SignalBuy
			}

			resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
				[]contractReplayBar{{close: 100, signal: testCase.signal}, {close: 100, signal: closingSignal}})

			require.NotEmpty(t, resultDto.ClosedTrades)
			assertDecimalEqual(t, testCase.wantEntryPrice, resultDto.ClosedTrades[0].EntryPrice)
		})
	}

	t.Run("a negative slippage is refused", func(t *testing.T) {
		requestDto := contractReplayRequest()
		requestDto.SlippagePercentage = decimal.RequireFromString("-0.1")

		fieldName, message := contractReplayRefusal(t, requestDto, contractReplayRules(t, contractReplaySpecification()))

		assert.Equal(t, "slippage", fieldName)
		assert.Contains(t, message, "滑點不得為負")
	})

	t.Run("staking everything leaves room for the entry charge on the whole notional", func(t *testing.T) {
		requestDto := contractReplayRequest()
		requestDto.InitialCapital = decimal.NewFromInt(10025)
		requestDto.Leverage = decimal.NewFromInt(5)
		requestDto.EntryCostPercentage = decimal.RequireFromString("0.05")
		requestDto.TradingMode = "longOnly"

		// 10,025 ÷ (1 + 5 × 0.05%) = 10,000 of margin; its 25 charge is the rest.
		resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 100, signal: vo.SignalSell}})

		require.Len(t, resultDto.ClosedTrades, 1)
		assertDecimalEqual(t, "10000", resultDto.ClosedTrades[0].Margin)
		assertDecimalEqual(t, "25", resultDto.ClosedTrades[0].EntryCost)
	})

	t.Run("the entry charge is taken on the notional", func(t *testing.T) {
		requestDto := contractReplayRequest()
		requestDto.InitialCapital = decimal.NewFromInt(20000)
		requestDto.PositionSizingMode = "fixedAmount"
		requestDto.PositionSizingValue = decimal.NewFromInt(10000)
		requestDto.Leverage = decimal.NewFromInt(5)
		requestDto.EntryCostPercentage = decimal.RequireFromString("0.05")
		requestDto.TradingMode = "longOnly"

		resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 100}})

		assert.Empty(t, resultDto.ClosedTrades)
		// 50,000 of notional at 0.05%.
		assertDecimalEqual(t, "25", resultDto.Summary.TotalTransactionCost)
	})
}

func TestContractBacktestLiquidation(t *testing.T) {
	tenTimesLong := func() dto.ContractBacktestRequestDto {
		requestDto := contractReplayRequest()
		requestDto.Leverage = decimal.NewFromInt(10)
		requestDto.TradingMode = "longOnly"

		return requestDto
	}

	t.Run("a traded low through the liquidation price does not liquidate when the mark price holds", func(t *testing.T) {
		resultDto := replayContract(t, tenTimesLong(), contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 95, low: 90, markLow: 91}})

		assert.Empty(t, resultDto.ClosedTrades)
		assert.Equal(t, 0, resultDto.Summary.LiquidationExitCount)
	})

	t.Run("the mark price reaching the liquidation price loses the whole margin", func(t *testing.T) {
		// 10,000 of margin on 1,000 units at 100: the liquidation price is about 90.45.
		resultDto := replayContract(t, tenTimesLong(), contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 95, low: 90.4, markLow: 90.4}})

		require.Len(t, resultDto.ClosedTrades, 1)
		assert.Equal(t, "liquidation", resultDto.ClosedTrades[0].ExitReason)
		assertDecimalEqual(t, "-10000", resultDto.ClosedTrades[0].Profit)
		assertDecimalEqual(t, "0", resultDto.Summary.FinalEquity)
		assert.Equal(t, 1, resultDto.Summary.LiquidationExitCount)
	})

	t.Run("a liquidated position loses its margin and its entry charge, nothing more", func(t *testing.T) {
		requestDto := tenTimesLong()
		requestDto.InitialCapital = decimal.NewFromInt(20000)
		requestDto.PositionSizingMode = "fixedAmount"
		requestDto.PositionSizingValue = decimal.NewFromInt(10000)
		requestDto.EntryCostPercentage = decimal.RequireFromString("0.05")

		resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 95, markLow: 80}})

		require.Len(t, resultDto.ClosedTrades, 1)
		// 10,000 of margin and 50 charged on 100,000 of notional.
		assertDecimalEqual(t, "-10050", resultDto.ClosedTrades[0].Profit)
		assertDecimalEqual(t, "9950", resultDto.Summary.FinalEquity)
	})

	t.Run("a mark price just above the liquidation price holds", func(t *testing.T) {
		resultDto := replayContract(t, tenTimesLong(), contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 95, markLow: 90.46}})

		assert.Empty(t, resultDto.ClosedTrades)
	})

	t.Run("a short's liquidation price sits above its entry", func(t *testing.T) {
		requestDto := contractReplayRequest()
		requestDto.Leverage = decimal.NewFromInt(10)
		requestDto.TradingMode = "shortOnly"

		// (100,000 + 10,000) ÷ (1,000 × 1.005) ≈ 109.45.
		held := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalSell}, {close: 105, markHigh: 109.4}})
		liquidated := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalSell}, {close: 105, markHigh: 109.5}})

		assert.Empty(t, held.ClosedTrades)
		require.Len(t, liquidated.ClosedTrades, 1)
		assert.Equal(t, "liquidation", liquidated.ClosedTrades[0].ExitReason)
	})

	t.Run("a price gapping far past the liquidation price still loses only the margin", func(t *testing.T) {
		resultDto := replayContract(t, tenTimesLong(), contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 10, markLow: 10}})

		require.Len(t, resultDto.ClosedTrades, 1)
		assertDecimalEqual(t, "0", resultDto.Summary.FinalEquity)
	})

	t.Run("without a ladder the smallest tier stands in and the report card says so", func(t *testing.T) {
		resultDto := replayContract(t, tenTimesLong(), contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100}, {close: 100}})

		assert.Equal(t, "smallestTier", resultDto.Summary.MaintenanceMarginBasis.Kind)
		assert.Nil(t, resultDto.Summary.MaintenanceMarginBasis.ConfirmedAt)
	})

	t.Run("with a ladder the position's tier is used and its confirmation date reported", func(t *testing.T) {
		// A 2% rate in the position's tier: (100,000 − 10,000) ÷ (1,000 × 0.98) ≈ 91.84.
		tradingRules := contractReplayRules(t, contractReplaySpecification(),
			contractReplayTier(1, "0", "50000", "0.004", 125),
			contractReplayTier(2, "50000", "250000", "0.02", 20))

		resultDto := replayContract(t, tenTimesLong(), tradingRules,
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 95, markLow: 91.5}})

		require.Len(t, resultDto.ClosedTrades, 1)
		assert.Equal(t, "liquidation", resultDto.ClosedTrades[0].ExitReason)
		assert.Equal(t, "tiers", resultDto.Summary.MaintenanceMarginBasis.Kind)
		require.NotNil(t, resultDto.Summary.MaintenanceMarginBasis.ConfirmedAt)
		assert.Equal(t, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), *resultDto.Summary.MaintenanceMarginBasis.ConfirmedAt)
	})
}

func TestContractBacktestStopAgainstLiquidation(t *testing.T) {
	testCases := []struct {
		name           string
		stopLoss       string
		takeProfit     string
		bar            contractReplayBar
		wantExitReason string
		wantExitPrice  string
	}{
		{
			name:           "a stop nearer than the liquidation price is reached first",
			stopLoss:       "5",
			bar:            contractReplayBar{close: 96, low: 94, markLow: 90},
			wantExitReason: "stopLoss",
			wantExitPrice:  "95",
		},
		{
			name:           "a liquidation price nearer than the stop is reached first",
			stopLoss:       "15",
			bar:            contractReplayBar{close: 96, low: 90, markLow: 90},
			wantExitReason: "liquidation",
			wantExitPrice:  "",
		},
		{
			name:           "a bar reaching both the stop and the take profit is read as the stop",
			stopLoss:       "5",
			takeProfit:     "5",
			bar:            contractReplayBar{close: 100, high: 106, low: 94, markLow: 94},
			wantExitReason: "stopLoss",
			wantExitPrice:  "95",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			requestDto := contractReplayRequest()
			requestDto.Leverage = decimal.NewFromInt(10)
			requestDto.TradingMode = "longOnly"
			requestDto.StopLossPercentage = decimal.RequireFromString(testCase.stopLoss)
			if testCase.takeProfit != "" {
				requestDto.TakeProfitPercentage = decimal.RequireFromString(testCase.takeProfit)
			}

			resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
				[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, testCase.bar})

			require.Len(t, resultDto.ClosedTrades, 1)
			assert.Equal(t, testCase.wantExitReason, resultDto.ClosedTrades[0].ExitReason)
			if testCase.wantExitPrice != "" {
				assertDecimalEqual(t, testCase.wantExitPrice, resultDto.ClosedTrades[0].ExitPrice)
			}
		})
	}
}

func TestContractBacktestFunding(t *testing.T) {
	fiveTimesHourly := func(tradingMode string) dto.ContractBacktestRequestDto {
		requestDto := contractReplayRequest()
		requestDto.AggregationInterval = "1h"
		requestDto.Leverage = decimal.NewFromInt(5)
		requestDto.TradingMode = tradingMode

		return requestDto
	}
	// Bars open at 00:00, 01:00, … — the second one at 01:00.
	secondBarOpen := contractReplayStart.Add(time.Hour)

	testCases := []struct {
		name         string
		tradingMode  string
		signal       vo.SignalVo
		rate         string
		wantTotalFee string
	}{
		{name: "a long pays a positive rate", tradingMode: "longOnly", signal: vo.SignalBuy, rate: "0.0001", wantTotalFee: "5"},
		{name: "a short receives a positive rate", tradingMode: "shortOnly", signal: vo.SignalSell, rate: "0.0001", wantTotalFee: "-5"},
		{name: "a long receives a negative rate", tradingMode: "longOnly", signal: vo.SignalBuy, rate: "-0.0001", wantTotalFee: "-5"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// 500 units carried into the 01:00 bar, settled at 01:00 on a mark of 100.
			resultDto := replayContract(t, fiveTimesHourly(testCase.tradingMode),
				contractReplayRules(t, contractReplaySpecification()),
				[]contractReplayBar{{close: 100, signal: testCase.signal}, {close: 100}},
				contractSettlementAt(secondBarOpen, testCase.rate, "100"))

			assertDecimalEqual(t, testCase.wantTotalFee, resultDto.Summary.TotalFundingFee)
		})
	}

	t.Run("paying funding moves the margin and so the equity", func(t *testing.T) {
		resultDto := replayContract(t, fiveTimesHourly("longOnly"),
			contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 100}},
			contractSettlementAt(secondBarOpen, "0.0001", "100"))

		assertDecimalEqual(t, "9995", resultDto.Summary.FinalEquity)
	})

	t.Run("a settlement before the position was opened is not paid", func(t *testing.T) {
		// Opened at the close of the 01:00 bar; the 01:00 settlement came before it.
		resultDto := replayContract(t, fiveTimesHourly("longOnly"),
			contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100}, {close: 100, signal: vo.SignalBuy}, {close: 100}},
			contractSettlementAt(secondBarOpen, "0.0001", "100"))

		assertDecimalEqual(t, "0", resultDto.Summary.TotalFundingFee)
	})

	t.Run("every settlement inside a coarse bar is paid", func(t *testing.T) {
		requestDto := fiveTimesHourly("longOnly")
		requestDto.AggregationInterval = "1d"
		requestDto.EndTime = contractReplayStart.Add(5 * 24 * time.Hour)
		secondDay := contractReplayStart.Add(24 * time.Hour)

		resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 100}},
			contractSettlementAt(secondDay, "0.0001", "100"),
			contractSettlementAt(secondDay.Add(8*time.Hour), "0.0001", "100"),
			contractSettlementAt(secondDay.Add(16*time.Hour), "0.0001", "100"))

		assertDecimalEqual(t, "15", resultDto.Summary.TotalFundingFee)
	})

	t.Run("a settlement recorded without a mark price is valued at the bar's mark close", func(t *testing.T) {
		resultDto := replayContract(t, fiveTimesHourly("longOnly"),
			contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 100, markClose: 120}},
			contractSettlementAt(secondBarOpen, "0.0001", ""))

		// 500 × 120 × 0.0001.
		assertDecimalEqual(t, "6", resultDto.Summary.TotalFundingFee)
	})

	t.Run("a settlement on a bar's closing moment belongs to the next bar", func(t *testing.T) {
		// 02:00 closes the 01:00 bar and opens the 02:00 one; there is no 02:00 bar
		// here, so the settlement is never paid.
		resultDto := replayContract(t, fiveTimesHourly("longOnly"),
			contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 100}},
			contractSettlementAt(contractReplayStart.Add(2*time.Hour), "0.0001", "100"))

		assertDecimalEqual(t, "0", resultDto.Summary.TotalFundingFee)
	})

	t.Run("the total is what was paid net of what was received", func(t *testing.T) {
		resultDto := replayContract(t, fiveTimesHourly("longOnly"),
			contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 100}, {close: 100}},
			contractSettlementAt(secondBarOpen, "0.0006", "100"),
			contractSettlementAt(secondBarOpen.Add(time.Hour), "-0.00024", "100"))

		// 30 paid, 12 received.
		assertDecimalEqual(t, "18", resultDto.Summary.TotalFundingFee)
	})

	t.Run("paying funding walks the liquidation price towards the entry", func(t *testing.T) {
		requestDto := contractReplayRequest()
		requestDto.AggregationInterval = "1h"
		requestDto.Leverage = decimal.NewFromInt(10)
		requestDto.TradingMode = "longOnly"
		// 1,000 units: a 1% rate on a mark of 100 takes 1,000 out of the 10,000 margin,
		// and the liquidation price climbs from about 90.45 to about 91.46.
		bars := []contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 95, markLow: 91}}

		withoutFunding := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()), bars)
		withFunding := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()), bars,
			contractSettlementAt(secondBarOpen, "0.01", "100"))

		assert.Empty(t, withoutFunding.ClosedTrades)
		require.Len(t, withFunding.ClosedTrades, 1)
		assert.Equal(t, "liquidation", withFunding.ClosedTrades[0].ExitReason)
		assertDecimalEqual(t, "1000", withFunding.ClosedTrades[0].FundingFee)
	})
}

func TestContractBacktestReportCard(t *testing.T) {
	t.Run("long and short are counted apart", func(t *testing.T) {
		// Long +10, short −10, long +10, short −10, long −10; the last short stays open.
		resultDto := replayContract(t, contractReplayRequest(), contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{
				{close: 100, signal: vo.SignalBuy},
				{close: 110, signal: vo.SignalSell},
				{close: 120, signal: vo.SignalBuy},
				{close: 130, signal: vo.SignalSell},
				{close: 140, signal: vo.SignalBuy},
				{close: 130, signal: vo.SignalSell},
			})

		assert.Equal(t, 3, resultDto.Summary.LongTradeCount)
		require.NotNil(t, resultDto.Summary.LongWinRate)
		assert.InDelta(t, 2.0/3.0, *resultDto.Summary.LongWinRate, 1e-9)
		assert.Equal(t, 2, resultDto.Summary.ShortTradeCount)
		require.NotNil(t, resultDto.Summary.ShortWinRate)
		assert.InDelta(t, 0.0, *resultDto.Summary.ShortWinRate, 1e-9)
	})

	t.Run("a side that never traded has no win rate", func(t *testing.T) {
		requestDto := contractReplayRequest()
		requestDto.TradingMode = "longOnly"

		resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 110, signal: vo.SignalSell}})

		assert.Equal(t, 0, resultDto.Summary.ShortTradeCount)
		assert.Nil(t, resultDto.Summary.ShortWinRate)
	})

	t.Run("fewer than two bars is refused", func(t *testing.T) {
		contractBacktestDomain, err := domains.NewContractBacktestDomain(contractReplayRequest(),
			contractReplayRules(t, contractReplaySpecification()), 100000, contractReplayStart.Add(30*24*time.Hour))
		require.NoError(t, err)

		_, err = contractBacktestDomain.SelectInput(contractReplayCandles("1m", []contractReplayBar{{close: 100}}))

		require.ErrorIs(t, err, domains.ErrBacktestValidation)
		fieldName, _ := domains.BacktestFieldName(err)
		assert.Equal(t, "timeRange", fieldName)
	})
}

func TestContractBacktestShortExitsAndRemainingRules(t *testing.T) {
	shortOnly := func() dto.ContractBacktestRequestDto {
		requestDto := contractReplayRequest()
		requestDto.TradingMode = "shortOnly"
		requestDto.StopLossPercentage = decimal.NewFromInt(5)
		requestDto.TakeProfitPercentage = decimal.NewFromInt(5)

		return requestDto
	}

	t.Run("a short is stopped above its entry", func(t *testing.T) {
		resultDto := replayContract(t, shortOnly(), contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalSell}, {close: 102, high: 106}})

		require.Len(t, resultDto.ClosedTrades, 1)
		assert.Equal(t, "stopLoss", resultDto.ClosedTrades[0].ExitReason)
		assertDecimalEqual(t, "105", resultDto.ClosedTrades[0].ExitPrice)
	})

	t.Run("a short takes its profit below its entry", func(t *testing.T) {
		resultDto := replayContract(t, shortOnly(), contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalSell}, {close: 98, low: 94}})

		require.Len(t, resultDto.ClosedTrades, 1)
		assert.Equal(t, "takeProfit", resultDto.ClosedTrades[0].ExitReason)
		assertDecimalEqual(t, "95", resultDto.ClosedTrades[0].ExitPrice)
		assert.Equal(t, 1, resultDto.Summary.TakeProfitExitCount)
	})

	t.Run("a long takes its profit above its entry", func(t *testing.T) {
		requestDto := shortOnly()
		requestDto.TradingMode = "longOnly"

		resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 102, high: 106}})

		require.Len(t, resultDto.ClosedTrades, 1)
		assert.Equal(t, "takeProfit", resultDto.ClosedTrades[0].ExitReason)
		assertDecimalEqual(t, "105", resultDto.ClosedTrades[0].ExitPrice)
	})

	t.Run("hearing the side already held changes nothing", func(t *testing.T) {
		resultDto := replayContract(t, contractReplayRequest(), contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 110, signal: vo.SignalBuy}})

		assert.Equal(t, 1, resultDto.Summary.PositionOpenCount)
		assert.Empty(t, resultDto.ClosedTrades)
	})

	t.Run("a settlement before the first bar is not paid", func(t *testing.T) {
		requestDto := contractReplayRequest()
		requestDto.AggregationInterval = "1h"

		resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 100}},
			contractSettlementAt(contractReplayStart.Add(-8*time.Hour), "0.01", "100"))

		assertDecimalEqual(t, "0", resultDto.Summary.TotalFundingFee)
	})

	t.Run("a symbol naming no quantity step keeps the exact quantity", func(t *testing.T) {
		specification := contractReplaySpecification()
		specification.QuantityStep = decimal.NullDecimal{}
		requestDto := contractReplayRequest()
		requestDto.InitialCapital = decimal.NewFromInt(1000)
		requestDto.TradingMode = "longOnly"

		resultDto := replayContract(t, requestDto, contractReplayRules(t, specification),
			[]contractReplayBar{{close: 800, signal: vo.SignalBuy}, {close: 800, signal: vo.SignalSell}})

		require.Len(t, resultDto.ClosedTrades, 1)
		assertDecimalEqual(t, "1.25", resultDto.ClosedTrades[0].Quantity)
	})

	t.Run("an opening below the minimum quantity is blocked", func(t *testing.T) {
		specification := contractReplaySpecification()
		specification.MinimumQuantity = decimal.NewNullDecimal(decimal.NewFromInt(1))
		requestDto := contractReplayRequest()
		requestDto.InitialCapital = decimal.NewFromInt(50)

		resultDto := replayContract(t, requestDto, contractReplayRules(t, specification),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 100}})

		assert.Equal(t, 1, resultDto.Summary.BlockedOpeningCount)
	})

	refusals := []struct {
		name          string
		mutateRequest func(*dto.ContractBacktestRequestDto)
		wantField     string
	}{
		{
			name: "a slippage above a hundred percent is refused",
			mutateRequest: func(requestDto *dto.ContractBacktestRequestDto) {
				requestDto.SlippagePercentage = decimal.NewFromInt(101)
			},
			wantField: "slippage",
		},
		{
			name: "a percentage that with its charge on the notional can never be afforded is refused",
			mutateRequest: func(requestDto *dto.ContractBacktestRequestDto) {
				// 99.9% clears a spot replay's bar (100 ÷ 1.0005) but not the notional's
				// (100 ÷ 1.0025).
				requestDto.PositionSizingMode = "percentage"
				requestDto.PositionSizingValue = decimal.RequireFromString("99.9")
				requestDto.Leverage = decimal.NewFromInt(5)
				requestDto.EntryCostPercentage = decimal.RequireFromString("0.05")
			},
			wantField: "positionSizingValue",
		},
		{
			name: "what a spot replay refuses a contract replay refuses in the same words",
			mutateRequest: func(requestDto *dto.ContractBacktestRequestDto) {
				requestDto.InitialCapital = decimal.Zero
			},
			wantField: "initialCapital",
		},
	}

	for _, refusal := range refusals {
		t.Run(refusal.name, func(t *testing.T) {
			requestDto := contractReplayRequest()
			refusal.mutateRequest(&requestDto)

			fieldName, _ := contractReplayRefusal(t, requestDto, contractReplayRules(t, contractReplaySpecification()))

			assert.Equal(t, refusal.wantField, fieldName)
		})
	}
}

func TestContractBacktestContractFollowUps(t *testing.T) {
	t.Run("a long closed by a sell hands its money back to the account", func(t *testing.T) {
		requestDto := contractReplayRequest()
		requestDto.TradingMode = "longOnly"

		resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 110, signal: vo.SignalSell}, {close: 120}})

		// Flat after the sell: the rise to 120 no longer moves the account.
		assertDecimalEqual(t, "11000", resultDto.Summary.FinalEquity)
	})

	t.Run("the tier's maintenance amount pushes the liquidation price away", func(t *testing.T) {
		requestDto := contractReplayRequest()
		requestDto.Leverage = decimal.NewFromInt(10)
		requestDto.TradingMode = "longOnly"
		deductingTier := contractReplayTier(2, "50000", "250000", "0.02", 20)
		deductingTier.MaintenanceAmount = decimal.NewFromInt(500)
		tradingRules := contractReplayRules(t, contractReplaySpecification(),
			contractReplayTier(1, "0", "50000", "0.004", 125), deductingTier)

		// (100,000 − 10,000 − 500) ÷ (1,000 × 0.98) ≈ 91.33; without the amount it would be 91.84.
		held := replayContract(t, requestDto, tradingRules,
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 95, markLow: 91.4}})
		liquidated := replayContract(t, requestDto, tradingRules,
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 95, markLow: 91.3}})

		assert.Empty(t, held.ClosedTrades)
		require.Len(t, liquidated.ClosedTrades, 1)
		assert.Equal(t, "liquidation", liquidated.ClosedTrades[0].ExitReason)
	})

	t.Run("a bar reaching both the stop and a nearer liquidation price liquidates", func(t *testing.T) {
		requestDto := contractReplayRequest()
		requestDto.Leverage = decimal.NewFromInt(10)
		requestDto.TradingMode = "longOnly"
		requestDto.StopLossPercentage = decimal.NewFromInt(15)

		resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 90, low: 84, markLow: 84}})

		require.Len(t, resultDto.ClosedTrades, 1)
		assert.Equal(t, "liquidation", resultDto.ClosedTrades[0].ExitReason)
	})

	t.Run("the exit charge is taken on the money that changes hands", func(t *testing.T) {
		requestDto := contractReplayRequest()
		requestDto.TradingMode = "longOnly"
		requestDto.EntryCostPercentage = decimal.Zero
		requestDto.ExitCostPercentage = decimal.RequireFromString("0.1")

		resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 110, signal: vo.SignalSell}})

		require.Len(t, resultDto.ClosedTrades, 1)
		// 100 units × 110 × 0.1%.
		assertDecimalEqual(t, "11", resultDto.ClosedTrades[0].ExitCost)
		assertDecimalEqual(t, "989", resultDto.ClosedTrades[0].Profit)
	})

	t.Run("a trade's profit is net of both charges and of its funding", func(t *testing.T) {
		requestDto := contractReplayRequest()
		requestDto.AggregationInterval = "1h"
		requestDto.InitialCapital = decimal.NewFromInt(20000)
		requestDto.PositionSizingMode = "fixedAmount"
		requestDto.PositionSizingValue = decimal.NewFromInt(10000)
		requestDto.Leverage = decimal.NewFromInt(5)
		requestDto.TradingMode = "longOnly"
		requestDto.EntryCostPercentage = decimal.RequireFromString("0.05")

		resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 110, signal: vo.SignalSell}},
			contractSettlementAt(contractReplayStart.Add(time.Hour), "0.0001", "100"))

		require.Len(t, resultDto.ClosedTrades, 1)
		closedTrade := resultDto.ClosedTrades[0]
		assertDecimalEqual(t, "5", closedTrade.FundingFee)
		// 5,000 on the price, less 25 in, 27.5 out and 5 of funding.
		assertDecimalEqual(t, "4942.5", closedTrade.Profit)
	})

	slippedExits := []struct {
		name          string
		tradingMode   string
		stake         int64
		stopLoss      int64
		bars          []contractReplayBar
		wantExitPrice string
	}{
		{
			name: "a long closed by a signal sells a little cheaper", tradingMode: "longOnly", stake: 10100,
			bars:          []contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 110, signal: vo.SignalSell}},
			wantExitPrice: "108.9",
		},
		{
			name: "a short closed by a signal buys back a little dearer", tradingMode: "shortOnly", stake: 9900,
			bars:          []contractReplayBar{{close: 100, signal: vo.SignalSell}, {close: 90, signal: vo.SignalBuy}},
			wantExitPrice: "90.9",
		},
		{
			// Entered at 101, stopped at 95.95, filled 1% below it.
			name: "a stop fills on the wrong side of its price", tradingMode: "longOnly", stake: 10100, stopLoss: 5,
			bars:          []contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 96, low: 95}},
			wantExitPrice: "94.9905",
		},
	}

	for _, slippedExit := range slippedExits {
		t.Run(slippedExit.name, func(t *testing.T) {
			requestDto := contractReplayRequest()
			requestDto.InitialCapital = decimal.NewFromInt(20000)
			requestDto.PositionSizingMode = "fixedAmount"
			requestDto.PositionSizingValue = decimal.NewFromInt(slippedExit.stake)
			requestDto.TradingMode = slippedExit.tradingMode
			requestDto.SlippagePercentage = decimal.NewFromInt(1)
			requestDto.StopLossPercentage = decimal.NewFromInt(slippedExit.stopLoss)

			resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()), slippedExit.bars)

			require.Len(t, resultDto.ClosedTrades, 1)
			assertDecimalEqual(t, slippedExit.wantExitPrice, resultDto.ClosedTrades[0].ExitPrice)
		})
	}

	t.Run("the bar a position opens on never reaches its exit levels", func(t *testing.T) {
		requestDto := contractReplayRequest()
		requestDto.TradingMode = "longOnly"
		requestDto.StopLossPercentage = decimal.NewFromInt(5)

		// The opening bar's low of 90 happened before the close it was entered at.
		resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, low: 90, signal: vo.SignalBuy}, {close: 100}})

		assert.Empty(t, resultDto.ClosedTrades)
		assert.Equal(t, 1, resultDto.Summary.PositionOpenCount)
	})

	t.Run("a stretch needing more bars than one read allows is refused", func(t *testing.T) {
		_, err := domains.NewContractBacktestDomain(contractReplayRequest(),
			contractReplayRules(t, contractReplaySpecification()), 10, contractReplayStart.Add(30*24*time.Hour))

		require.ErrorIs(t, err, domains.ErrBacktestValidation)
		fieldName, _ := domains.BacktestFieldName(err)
		assert.Equal(t, "timeRange", fieldName)
	})

	t.Run("an explicit zero leverage is one times, the same as leaving it blank", func(t *testing.T) {
		requestDto := contractReplayRequest()
		requestDto.Leverage = decimal.Zero

		resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {close: 100}})

		assertDecimalEqual(t, "1", resultDto.Leverage)
	})
}
