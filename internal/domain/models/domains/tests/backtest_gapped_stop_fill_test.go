package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every spot case buys 100 units at 100 from 10,000.
func TestSpotStopGappedThroughAtTheOpenFillsAtTheOpen(t *testing.T) {
	testCases := []struct {
		name               string
		stopLoss           int64
		takeProfit         int64
		secondBar          bar
		wantExitReason     string
		wantExitPrice      string
		wantProfit         string
		wantFinalEquity    string
		wantStopLossExits  int
		wantNoClosedTrades bool
	}{
		{
			name:     "an open below the stop fills at the open",
			stopLoss: 5, secondBar: bar{open: 90, high: 91, low: 88, close: 89},
			wantExitReason: "stopLoss", wantExitPrice: "90", wantProfit: "-1000", wantFinalEquity: "9000", wantStopLossExits: 1,
		},
		{
			name:     "an open exactly at the stop fills at the stop",
			stopLoss: 5, secondBar: bar{open: 95, high: 96, low: 93, close: 94},
			wantExitReason: "stopLoss", wantExitPrice: "95", wantProfit: "-500", wantFinalEquity: "9500", wantStopLossExits: 1,
		},
		{
			name:     "a stop reached only within the bar still fills at the stop",
			stopLoss: 5, secondBar: bar{open: 96, high: 97, low: 94, close: 96},
			wantExitReason: "stopLoss", wantExitPrice: "95", wantProfit: "-500", wantFinalEquity: "9500", wantStopLossExits: 1,
		},
		{
			name:     "a gapped stop on a bar also reaching the take profit is still the stop at the open",
			stopLoss: 5, takeProfit: 5, secondBar: bar{open: 90, high: 106, low: 88, close: 100},
			wantExitReason: "stopLoss", wantExitPrice: "90", wantProfit: "-1000", wantFinalEquity: "9000", wantStopLossExits: 1,
		},
		{
			name:       "a take profit gapped through at the open still fills at the take profit",
			takeProfit: 5, secondBar: bar{open: 110, high: 112, low: 109, close: 111},
			wantExitReason: "takeProfit", wantExitPrice: "105", wantProfit: "500", wantFinalEquity: "10500",
		},
		{
			name:      "without a stop a gap down is simply held",
			secondBar: bar{open: 90, high: 91, low: 88, close: 89},
			// 100 units held at 89.
			wantFinalEquity: "8900", wantNoClosedTrades: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := replayWithExitsOf(t, testCase.stopLoss, testCase.takeProfit,
				[]bar{{high: 100, low: 100, close: 100}, testCase.secondBar},
				buySignal, holdSignal)

			assert.Equal(t, testCase.wantFinalEquity, result.Summary.FinalEquity.String())
			assert.Equal(t, testCase.wantStopLossExits, result.Summary.StopLossExitCount)
			if testCase.wantNoClosedTrades {
				assert.Empty(t, result.ClosedTrades)

				return
			}
			require.Len(t, result.ClosedTrades, 1)
			assert.Equal(t, testCase.wantExitReason, result.ClosedTrades[0].ExitReason)
			assert.Equal(t, testCase.wantExitPrice, result.ClosedTrades[0].ExitPrice.String())
			assert.Equal(t, testCase.wantProfit, result.ClosedTrades[0].Profit.String())
		})
	}
}

func TestSpotStopGappedThroughWhenFillingAtTheNextOpen(t *testing.T) {
	testCases := []struct {
		name          string
		bars          []shortTermBar
		wantExitPrice string
		wantProfit    string
	}{
		{
			name:          "the bar opening the position cannot gap through its own stop",
			bars:          []shortTermBar{{close: 99, signal: vo.SignalBuy}, {open: 100, low: 97, close: 99}},
			wantExitPrice: "98",
			wantProfit:    "-200",
		},
		{
			name:          "a later bar opening below the stop fills at its open",
			bars:          []shortTermBar{{close: 99, signal: vo.SignalBuy}, {open: 100, close: 100}, {open: 95, low: 94, close: 96}},
			wantExitPrice: "95",
			wantProfit:    "-500",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			requestDto := nextOpenRequest()
			requestDto.StopLossPercentage = decimal.NewFromInt(2)

			resultDto := replaySpotBars(t, requestDto, testCase.bars)

			require.Len(t, resultDto.ClosedTrades, 1)
			assert.Equal(t, "stopLoss", resultDto.ClosedTrades[0].ExitReason)
			assertDecimalEqual(t, "100", resultDto.ClosedTrades[0].EntryPrice)
			assertDecimalEqual(t, testCase.wantExitPrice, resultDto.ClosedTrades[0].ExitPrice)
			assertDecimalEqual(t, testCase.wantProfit, resultDto.ClosedTrades[0].Profit)
		})
	}
}

// Every one-times case puts 10,000 into 100 units at 100.
func TestContractStopGappedThroughAtTheOpenFillsAtTheOpen(t *testing.T) {
	testCases := []struct {
		name          string
		tradingMode   string
		fillTiming    string
		slippage      string
		bars          []contractReplayBar
		wantExitPrice string
		wantProfit    string
	}{
		{
			name:          "a long opening below its stop fills at the open",
			tradingMode:   "longOnly",
			bars:          []contractReplayBar{{close: 100, signal: vo.SignalBuy}, {open: 90, high: 91, low: 88, close: 89}},
			wantExitPrice: "90",
			wantProfit:    "-1000",
		},
		{
			name:          "a short opening above its stop fills at the open",
			tradingMode:   "shortOnly",
			bars:          []contractReplayBar{{close: 100, signal: vo.SignalSell}, {open: 110, high: 112, low: 109, close: 111}},
			wantExitPrice: "110",
			wantProfit:    "-1000",
		},
		{
			name:          "a short reaching its stop only within the bar fills at the stop",
			tradingMode:   "shortOnly",
			bars:          []contractReplayBar{{close: 100, signal: vo.SignalSell}, {open: 104, high: 106, low: 103, close: 104}},
			wantExitPrice: "105",
			wantProfit:    "-500",
		},
		{
			name:          "a gapped fill at the next open is the open too",
			tradingMode:   "longOnly",
			fillTiming:    "nextOpen",
			bars:          []contractReplayBar{{close: 99, signal: vo.SignalBuy}, {open: 100, close: 100}, {open: 90, low: 88, close: 89}},
			wantExitPrice: "90",
			wantProfit:    "-1000",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			requestDto := contractReplayRequest()
			requestDto.TradingMode = testCase.tradingMode
			requestDto.FillTiming = testCase.fillTiming
			requestDto.StopLossPercentage = decimal.NewFromInt(5)

			resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()), testCase.bars)

			require.Len(t, resultDto.ClosedTrades, 1)
			assert.Equal(t, "stopLoss", resultDto.ClosedTrades[0].ExitReason)
			assertDecimalEqual(t, testCase.wantExitPrice, resultDto.ClosedTrades[0].ExitPrice)
			assertDecimalEqual(t, testCase.wantProfit, resultDto.ClosedTrades[0].Profit)
		})
	}

	t.Run("a gapped stop still pays the slippage on the open", func(t *testing.T) {
		requestDto := contractReplayRequest()
		requestDto.TradingMode = "longOnly"
		requestDto.StopLossPercentage = decimal.NewFromInt(5)
		requestDto.SlippagePercentage = decimal.RequireFromString("0.1")

		resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, {open: 90, high: 91, low: 88, close: 89}})

		require.Len(t, resultDto.ClosedTrades, 1)
		// 90 less 0.1%.
		assertDecimalEqual(t, "89.91", resultDto.ClosedTrades[0].ExitPrice)
	})
}

// Ten times long: 10,000 of margin on 1,000 units at 100 puts the liquidation price near 90.45.
func TestContractOpenGappingPastTheStopAndTheLiquidationPrice(t *testing.T) {
	tenTimesWithStop := func(stopLoss int64) dto.ContractBacktestRequestDto {
		requestDto := contractReplayRequest()
		requestDto.Leverage = decimal.NewFromInt(10)
		requestDto.TradingMode = "longOnly"
		requestDto.StopLossPercentage = decimal.NewFromInt(stopLoss)

		return requestDto
	}

	testCases := []struct {
		name            string
		stopLoss        int64
		secondBar       contractReplayBar
		wantExitReason  string
		wantExitPrice   string
		wantProfit      string
		wantFinalEquity string
	}{
		{
			name:           "an open past the stop but short of liquidation stops at the open",
			stopLoss:       5,
			secondBar:      contractReplayBar{open: 93, high: 94, low: 92, close: 93, markLow: 92},
			wantExitReason: "stopLoss", wantExitPrice: "93", wantProfit: "-7000", wantFinalEquity: "3000",
		},
		{
			name:           "a mark open past liquidation liquidates even though the stop is nearer",
			stopLoss:       5,
			secondBar:      contractReplayBar{open: 88, close: 88},
			wantExitReason: "liquidation", wantProfit: "-10000", wantFinalEquity: "0",
		},
		{
			name:           "a traded open past liquidation with the mark short of it stops at the open and loses only the margin",
			stopLoss:       5,
			secondBar:      contractReplayBar{open: 89, markOpen: 91, high: 92, low: 89, close: 92, markLow: 90},
			wantExitReason: "stopLoss", wantExitPrice: "89", wantProfit: "-10000", wantFinalEquity: "0",
		},
		{
			name:           "an open past neither leaves the nearer liquidation price to be reached within the bar",
			stopLoss:       15,
			secondBar:      contractReplayBar{open: 92, low: 90, close: 92, markLow: 90},
			wantExitReason: "liquidation", wantProfit: "-10000", wantFinalEquity: "0",
		},
		{
			name:           "a mark open past both a farther stop and liquidation liquidates",
			stopLoss:       15,
			secondBar:      contractReplayBar{open: 84, close: 84},
			wantExitReason: "liquidation", wantProfit: "-10000", wantFinalEquity: "0",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			resultDto := replayContract(t, tenTimesWithStop(testCase.stopLoss),
				contractReplayRules(t, contractReplaySpecification()),
				[]contractReplayBar{{close: 100, signal: vo.SignalBuy}, testCase.secondBar})

			require.Len(t, resultDto.ClosedTrades, 1)
			assert.Equal(t, testCase.wantExitReason, resultDto.ClosedTrades[0].ExitReason)
			if testCase.wantExitPrice != "" {
				assertDecimalEqual(t, testCase.wantExitPrice, resultDto.ClosedTrades[0].ExitPrice)
			}
			assertDecimalEqual(t, testCase.wantProfit, resultDto.ClosedTrades[0].Profit)
			assertDecimalEqual(t, testCase.wantFinalEquity, resultDto.Summary.FinalEquity)
		})
	}

	t.Run("a short whose mark opens past its liquidation price liquidates even though the stop is nearer", func(t *testing.T) {
		requestDto := tenTimesWithStop(5)
		requestDto.TradingMode = "shortOnly"

		// (100,000 + 10,000) ÷ (1,000 × 1.005) puts the short's liquidation price near 109.45.
		resultDto := replayContract(t, requestDto, contractReplayRules(t, contractReplaySpecification()),
			[]contractReplayBar{{close: 100, signal: vo.SignalSell}, {open: 112, close: 112}})

		require.Len(t, resultDto.ClosedTrades, 1)
		assert.Equal(t, "liquidation", resultDto.ClosedTrades[0].ExitReason)
		assertDecimalEqual(t, "-10000", resultDto.ClosedTrades[0].Profit)
	})
}
