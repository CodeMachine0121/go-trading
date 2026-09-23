package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// contractSourceNamed is a signal source naming a strategy script that eats contract
// bars.
func contractSourceNamed(label string, strategyScriptID uint) dto.TradingStrategySignalSourceWriteDto {
	source := sourceNamed(label, strategyScriptID)
	source.DeclaredMarketDataKind = string(vo.MarketDataKindContractKCandle)

	return source
}

func aContractTradingStrategyWriteDto() dto.TradingStrategyWriteDto {
	writeDto := aTradingStrategyWriteDto()
	writeDto.MarketDataKind = string(vo.MarketDataKindContractKCandle)
	writeDto.SignalSources = []dto.TradingStrategySignalSourceWriteDto{
		contractSourceNamed("A", 1), contractSourceNamed("B", 2),
	}

	return writeDto
}

func TestTradingStrategyMarketDataKind(t *testing.T) {
	t.Run("a contract trading strategy of contract sources is accepted", func(t *testing.T) {
		tradingStrategy, err := domains.NewTradingStrategyDomain(aContractTradingStrategyWriteDto())

		require.NoError(t, err)
		assert.Equal(t, "contractKCandle", tradingStrategy.ToEntity().MarketDataKind)
	})

	t.Run("saying nothing about the kind is the K candle, as every trading strategy was", func(t *testing.T) {
		tradingStrategy, err := domains.NewTradingStrategyDomain(aTradingStrategyWriteDto())

		require.NoError(t, err)
		assert.Equal(t, "kCandle", tradingStrategy.ToEntity().MarketDataKind)
		assert.Equal(t, "", tradingStrategy.ToEntity().TradingMode)
	})

	testCases := []struct {
		name     string
		writeDto func() dto.TradingStrategyWriteDto
	}{
		{
			name: "a contract trading strategy with a K candle source is refused naming it",
			writeDto: func() dto.TradingStrategyWriteDto {
				writeDto := aContractTradingStrategyWriteDto()
				writeDto.SignalSources[0] = sourceNamed("A", 1)

				return writeDto
			},
		},
		{
			name: "a K candle trading strategy with a contract source is refused naming it",
			writeDto: func() dto.TradingStrategyWriteDto {
				writeDto := aTradingStrategyWriteDto()
				writeDto.SignalSources[0] = contractSourceNamed("A", 1)

				return writeDto
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := domains.NewTradingStrategyDomain(testCase.writeDto())

			require.ErrorIs(t, err, domains.ErrTradingStrategyValidation)
			assert.Contains(t, err.Error(), `信號來源 "A"`)
			assert.Contains(t, err.Error(), "都要吃同一種行情")
		})
	}

	t.Run("a kind that does not exist is refused", func(t *testing.T) {
		writeDto := aTradingStrategyWriteDto()
		writeDto.MarketDataKind = "option"

		_, err := domains.NewTradingStrategyDomain(writeDto)

		require.ErrorIs(t, err, domains.ErrTradingStrategyValidation)
	})
}

func TestContractTradingStrategyTradingMode(t *testing.T) {
	testCases := []struct {
		name            string
		declaredMode    string
		wantStoredMode  string
		wantRefusalText string
	}{
		{name: "a contract trading strategy keeps short only", declaredMode: "shortOnly", wantStoredMode: "shortOnly"},
		{name: "a contract trading strategy with no mode is long and short", declaredMode: "", wantStoredMode: "longShort"},
		{name: "spot is not a contract trading mode", declaredMode: "spot", wantRefusalText: "longShort、longOnly、shortOnly"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writeDto := aContractTradingStrategyWriteDto()
			writeDto.TradingMode = testCase.declaredMode

			tradingStrategy, err := domains.NewTradingStrategyDomain(writeDto)

			if testCase.wantRefusalText != "" {
				require.ErrorIs(t, err, domains.ErrTradingStrategyValidation)
				assert.Contains(t, err.Error(), testCase.wantRefusalText)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, testCase.wantStoredMode, tradingStrategy.ToEntity().TradingMode)
		})
	}

	t.Run("a K candle trading strategy is still refused a trading mode, in the same words", func(t *testing.T) {
		writeDto := aTradingStrategyWriteDto()
		writeDto.TradingMode = "longShort"

		_, err := domains.NewTradingStrategyDomain(writeDto)

		require.ErrorIs(t, err, domains.ErrTradingStrategyValidation)
		assert.Contains(t, err.Error(), "只重演現貨")
	})
}

func TestMarketDataKindForTradingStrategiesAndBots(t *testing.T) {
	kCandleKind, err := domains.NewMarketDataKindDomain("kCandle")
	require.NoError(t, err)
	contractKind, err := domains.NewMarketDataKindDomain("contractKCandle")
	require.NoError(t, err)

	t.Run("a rewrite saying nothing keeps the stored kind", func(t *testing.T) {
		retained, retainError := contractKind.RetainingForTradingStrategy("")

		require.NoError(t, retainError)
		assert.Equal(t, vo.MarketDataKindContractKCandle, retained.Value())
	})

	t.Run("a rewrite restating the kind changes nothing", func(t *testing.T) {
		retained, retainError := kCandleKind.RetainingForTradingStrategy("kCandle")

		require.NoError(t, retainError)
		assert.Equal(t, vo.MarketDataKindKCandle, retained.Value())
	})

	t.Run("a rewrite naming the other kind is refused", func(t *testing.T) {
		_, retainError := kCandleKind.RetainingForTradingStrategy("contractKCandle")

		require.ErrorIs(t, retainError, domains.ErrTradingStrategyValidation)
		assert.Contains(t, retainError.Error(), "行情種類建立後不得更換")
	})

	t.Run("a bot may follow a K candle trading strategy", func(t *testing.T) {
		assert.NoError(t, kCandleKind.RequireFollowableByStrategyBot())
	})

	t.Run("a bot may not follow a contract trading strategy", func(t *testing.T) {
		followError := contractKind.RequireFollowableByStrategyBot()

		require.ErrorIs(t, followError, domains.ErrStrategyBotValidation)
		assert.Contains(t, followError.Error(), "策略機器人目前只跑 K 線")
	})

	t.Run("a script is replayable only over the kind it eats", func(t *testing.T) {
		assert.NoError(t, contractKind.RequireReplayableAs(vo.MarketDataKindContractKCandle))

		mismatch := contractKind.RequireReplayableAs(vo.MarketDataKindKCandle)
		require.ErrorIs(t, mismatch, domains.ErrStrategyScriptMarketDataKindMismatch)
		assert.Contains(t, mismatch.Error(), "吃的是合約行情")
	})
}

func TestSpotTradingStrategyReplayRefusesContractRules(t *testing.T) {
	t.Run("a contract trading strategy cannot be replayed on spot", func(t *testing.T) {
		requestDto := tradingStrategyBacktestRequest()
		requestDto.TradingStrategyMarketDataKind = "contractKCandle"

		_, err := domains.NewTradingStrategyBacktestDomain(requestDto, 1000, backtestStart.Add(30*24*time.Hour))

		require.ErrorIs(t, err, domains.ErrBacktestValidation)
		assert.Contains(t, err.Error(), "吃的是合約行情")
	})

	t.Run("a source saved eating contract bars is refused by name", func(t *testing.T) {
		requestDto := tradingStrategyBacktestRequest()
		requestDto.SignalSources[0].MarketDataKind = "contractKCandle"

		_, err := domains.NewTradingStrategyBacktestDomain(requestDto, 1000, backtestStart.Add(30*24*time.Hour))

		require.ErrorIs(t, err, domains.ErrBacktestValidation)
		assert.Contains(t, err.Error(), `信號來源 "A"`)
	})
}

// contractTradingStrategyReplayRequest replays a one-source contract trading strategy
// whose conditions are exactly that source's own opinion.
func contractTradingStrategyReplayRequest() dto.ContractTradingStrategyBacktestRequestDto {
	return dto.ContractTradingStrategyBacktestRequestDto{
		Symbol:    "BTCUSDT",
		StartTime: contractReplayStart,
		EndTime:   contractReplayStart.Add(24 * time.Hour),
		SignalSources: []dto.ResolvedSignalSourceDto{
			{Label: "A", AggregationInterval: "1m", Script: "//", MarketDataKind: "contractKCandle"},
		},
		BuyCondition:                  dto.TradingStrategyConditionDto{SourceLabel: "A", Signal: "buy"},
		SellCondition:                 dto.TradingStrategyConditionDto{SourceLabel: "A", Signal: "sell"},
		TradingStrategyMarketDataKind: "contractKCandle",
		TradingStrategyTradingMode:    "longOnly",
		InitialCapital:                decimal.NewFromInt(10000),
		Leverage:                      decimal.NewFromInt(5),
	}
}

func TestContractTradingStrategyBacktest(t *testing.T) {
	now := contractReplayStart.Add(30 * 24 * time.Hour)

	t.Run("a single source whose conditions are its own opinion replays exactly as the script", func(t *testing.T) {
		bars := []contractReplayBar{
			{close: 100, signal: vo.SignalBuy}, {close: 110, signal: vo.SignalSell},
			{close: 105, signal: vo.SignalSell}, {close: 120, signal: vo.SignalBuy},
		}
		tradingRules := contractReplayRules(t, contractReplaySpecification())
		strategyBacktest, err := domains.NewContractTradingStrategyBacktestDomain(
			contractTradingStrategyReplayRequest(), tradingRules, 100000, now)
		require.NoError(t, err)

		alignment, err := strategyBacktest.ContractBacktest().SelectInput(contractReplayCandles("1m", bars))
		require.NoError(t, err)
		sourceSignals := make([]domains.SignalDomain, 0, len(bars))
		for _, bar := range bars {
			sourceSignals = append(sourceSignals, domains.NewSignalDomainOf(bar.signal))
		}

		strategyResult := strategyBacktest.ReplayOver(alignment, [][]domains.SignalDomain{sourceSignals}, nil)

		scriptRequest := contractReplayRequest()
		scriptRequest.Leverage = decimal.NewFromInt(5)
		scriptRequest.TradingMode = "longOnly"
		scriptResult := replayContract(t, scriptRequest, tradingRules, bars)

		assert.Equal(t, scriptResult, strategyResult)
		assert.Equal(t, "longOnly", strategyResult.TradingMode)
	})

	t.Run("a bar both trees hold on is a hold and is counted", func(t *testing.T) {
		requestDto := contractTradingStrategyReplayRequest()
		requestDto.SignalSources = append(requestDto.SignalSources, dto.ResolvedSignalSourceDto{
			Label: "B", AggregationInterval: "1m", Script: "//", MarketDataKind: "contractKCandle",
		})
		requestDto.SellCondition = dto.TradingStrategyConditionDto{SourceLabel: "B", Signal: "sell"}
		strategyBacktest, err := domains.NewContractTradingStrategyBacktestDomain(
			requestDto, contractReplayRules(t, contractReplaySpecification()), 100000, now)
		require.NoError(t, err)

		bars := []contractReplayBar{{close: 100}, {close: 100}}
		alignment, err := strategyBacktest.ContractBacktest().SelectInput(contractReplayCandles("1m", bars))
		require.NoError(t, err)

		resultDto := strategyBacktest.ReplayOver(alignment, [][]domains.SignalDomain{
			{domains.NewSignalDomainOf(vo.SignalBuy), domains.NewSignalDomainOf(vo.SignalHold)},
			{domains.NewSignalDomainOf(vo.SignalSell), domains.NewSignalDomainOf(vo.SignalHold)},
		}, nil)

		assert.Equal(t, 1, resultDto.Summary.ConflictedCandleCount)
		assert.Equal(t, 0, resultDto.Summary.PositionOpenCount)
	})

	refusals := []struct {
		name          string
		mutateRequest func(*dto.ContractTradingStrategyBacktestRequestDto)
		wantField     string
		wantText      string
	}{
		{
			name: "a trading mode sent with the replay is refused",
			mutateRequest: func(requestDto *dto.ContractTradingStrategyBacktestRequestDto) {
				requestDto.TradingMode = "shortOnly"
			},
			wantField: "tradingMode",
			wantText:  "交易模式由交易策略自己決定",
		},
		{
			name: "a K candle trading strategy cannot be replayed on a contract account",
			mutateRequest: func(requestDto *dto.ContractTradingStrategyBacktestRequestDto) {
				requestDto.TradingStrategyMarketDataKind = "kCandle"
			},
			wantText: "吃的是 K 線",
		},
		{
			name: "a source eating K candles is refused by name",
			mutateRequest: func(requestDto *dto.ContractTradingStrategyBacktestRequestDto) {
				requestDto.SignalSources[0].MarketDataKind = "kCandle"
			},
			wantField: "signalSources",
			wantText:  `信號來源 "A"`,
		},
	}

	for _, refusal := range refusals {
		t.Run(refusal.name, func(t *testing.T) {
			requestDto := contractTradingStrategyReplayRequest()
			refusal.mutateRequest(&requestDto)

			_, err := domains.NewContractTradingStrategyBacktestDomain(
				requestDto, contractReplayRules(t, contractReplaySpecification()), 100000, now)

			require.ErrorIs(t, err, domains.ErrBacktestValidation)
			assert.Contains(t, err.Error(), refusal.wantText)
			if refusal.wantField != "" {
				fieldName, _ := domains.BacktestFieldName(err)
				assert.Equal(t, refusal.wantField, fieldName)
			}
		})
	}
}

func TestUnrecognisedKindsAndBrokenSourcesAreRefused(t *testing.T) {
	now := contractReplayStart.Add(30 * 24 * time.Hour)
	tradingRules := func(t *testing.T) domains.ContractTradingRulesDomain {
		return contractReplayRules(t, contractReplaySpecification())
	}

	contractRefusals := []struct {
		name          string
		mutateRequest func(*dto.ContractTradingStrategyBacktestRequestDto)
	}{
		{name: "a trading strategy of an unknown kind", mutateRequest: func(requestDto *dto.ContractTradingStrategyBacktestRequestDto) {
			requestDto.TradingStrategyMarketDataKind = "option"
		}},
		{name: "a source of an unknown kind", mutateRequest: func(requestDto *dto.ContractTradingStrategyBacktestRequestDto) {
			requestDto.SignalSources[0].MarketDataKind = "option"
		}},
		{name: "a source whose knobs cannot be declared", mutateRequest: func(requestDto *dto.ContractTradingStrategyBacktestRequestDto) {
			requestDto.SignalSources[0].Parameters = []dto.StrategyScriptParameterWriteDto{{Name: "", Kind: "lookbackCount"}}
		}},
		{name: "a source given a value for a knob it never declared", mutateRequest: func(requestDto *dto.ContractTradingStrategyBacktestRequestDto) {
			requestDto.SignalSources[0].ParameterValues = []dto.StrategyScriptParameterValueDto{{Name: "週期", Value: 20}}
		}},
		{name: "a buy condition naming an undeclared source", mutateRequest: func(requestDto *dto.ContractTradingStrategyBacktestRequestDto) {
			requestDto.BuyCondition = dto.TradingStrategyConditionDto{SourceLabel: "Z", Signal: "buy"}
		}},
		{name: "a sell condition naming an undeclared source", mutateRequest: func(requestDto *dto.ContractTradingStrategyBacktestRequestDto) {
			requestDto.SellCondition = dto.TradingStrategyConditionDto{SourceLabel: "Z", Signal: "sell"}
		}},
		{name: "a replay the contract account refuses", mutateRequest: func(requestDto *dto.ContractTradingStrategyBacktestRequestDto) {
			requestDto.InitialCapital = decimal.Zero
		}},
	}

	for _, refusal := range contractRefusals {
		t.Run(refusal.name, func(t *testing.T) {
			requestDto := contractTradingStrategyReplayRequest()
			refusal.mutateRequest(&requestDto)

			_, err := domains.NewContractTradingStrategyBacktestDomain(requestDto, tradingRules(t), 100000, now)

			assert.Error(t, err)
		})
	}

	t.Run("a spot replay of a trading strategy of an unknown kind", func(t *testing.T) {
		requestDto := tradingStrategyBacktestRequest()
		requestDto.TradingStrategyMarketDataKind = "option"

		_, err := domains.NewTradingStrategyBacktestDomain(requestDto, 1000, now)

		require.ErrorIs(t, err, domains.ErrBacktestValidation)
	})

	t.Run("a trading strategy saved with a source of an unknown kind", func(t *testing.T) {
		writeDto := aTradingStrategyWriteDto()
		writeDto.SignalSources[0].DeclaredMarketDataKind = "option"

		_, err := domains.NewTradingStrategyDomain(writeDto)

		require.ErrorIs(t, err, domains.ErrTradingStrategyValidation)
	})

	t.Run("a rewrite naming an unknown kind", func(t *testing.T) {
		kCandleKind, err := domains.NewMarketDataKindDomain("")
		require.NoError(t, err)

		_, retainError := kCandleKind.RetainingForTradingStrategy("option")

		require.ErrorIs(t, retainError, domains.ErrTradingStrategyValidation)
	})
}
