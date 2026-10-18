package application_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// A paces at one hour and B reads four hours: B's first bar finishes with the fourth hour, so the
// strategy can only buy there, never on the earlier hours B had not yet spoken for.
func TestTradingStrategyBacktestHearsACoarserSourceOnlyOnceItsBarHasFinished(t *testing.T) {
	fixture := newTradingStrategyBacktestUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), replayedTradingStrategyID).
		Return(aReplayedTradingStrategy("1h", "4h"), nil)

	hourlyCandles := []entities.KCandle{}
	for hour := range 8 {
		hourlyCandles = append(hourlyCandles, storedHourlyCandle(hour, fmt.Sprintf("%d", 100+10*hour)))
	}
	fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(hourlyCandles, nil)

	candleCountsBySource := []int{}
	answers := [][]vo.SignalVo{
		{vo.SignalBuy, vo.SignalBuy, vo.SignalBuy, vo.SignalBuy, vo.SignalBuy, vo.SignalBuy, vo.SignalBuy, vo.SignalSell},
		{vo.SignalBuy, vo.SignalSell},
	}
	fixture.indicatorScriptProxy.EXPECT().
		ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ string, _ domains.IndicatorResultTypeDomain, kCandles []vo.KCandleVo,
			_ domains.StrategyScriptParametersDomain, _ domaininterface.IReplayProgressSink,
		) ([]map[string]vo.IndicatorValueVo, error) {
			answer := answers[len(candleCountsBySource)]
			candleCountsBySource = append(candleCountsBySource, len(kCandles))

			return signalsSaying(answer...), nil
		}).Times(2)

	requestDto := tradingStrategyBacktestRequestDto()
	requestDto.EndTime = backtestStart.Add(8 * time.Hour)

	resultDto, runError := fixture.application.RunTradingStrategyBacktest(
		context.Background(), backtestViewerID, replayedTradingStrategyID, requestDto)

	require.NoError(t, runError)
	assert.Equal(t, "1h", resultDto.Interval)
	assert.Equal(t, []int{8, 2}, candleCountsBySource)
	require.Len(t, resultDto.ClosedTrades, 1)
	assert.Equal(t, backtestStart.Add(3*time.Hour), resultDto.ClosedTrades[0].EntryTime)
	assert.True(t, decimal.RequireFromString("130").Equal(resultDto.ClosedTrades[0].EntryPrice))
	assert.True(t, decimal.RequireFromString("170").Equal(resultDto.ClosedTrades[0].ExitPrice))
}

// Two read hours finish no four-hour bar, so B is never run and holds; A alone never opens.
func TestTradingStrategyBacktestDoesNotRunACoarserSourceWithNoFinishedBar(t *testing.T) {
	fixture := newTradingStrategyBacktestUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), replayedTradingStrategyID).
		Return(aReplayedTradingStrategy("1h", "4h"), nil)
	fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entities.KCandle{storedHourlyCandle(0, "100"), storedHourlyCandle(1, "110")}, nil)
	fixture.indicatorScriptProxy.EXPECT().
		ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(signalsSaying(vo.SignalBuy, vo.SignalBuy), nil).Times(1)

	requestDto := tradingStrategyBacktestRequestDto()
	requestDto.EndTime = backtestStart.Add(2 * time.Hour)

	resultDto, runError := fixture.application.RunTradingStrategyBacktest(
		context.Background(), backtestViewerID, replayedTradingStrategyID, requestDto)

	require.NoError(t, runError)
	assert.Equal(t, 0, resultDto.Summary.PositionOpenCount)
}
