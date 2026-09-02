package persistence_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// The aggregated series is the one place where the query range and the bucket grid
// disagree on purpose: a bucket is a whole hour, a range can start in the middle of
// one. Which candles end up in that half-covered bucket is decided by the range
// filter in storage, so proving it needs the real database — a test that hands
// candles straight to the domain cannot see the filter at all.
func TestGetKCandleSeriesAgainstStorage(t *testing.T) {
	newSeriesService := func(t *testing.T) *service.KCandleService {
		t.Helper()

		kCandleRepository := persistence.NewKCandleRepository(newTestDatabase(t))
		_, saveError := kCandleRepository.Save(kCandleAt("BTCUSDT", at(9, 55), "150"))
		require.NoError(t, saveError)
		_, saveError = kCandleRepository.Save(kCandleAt("BTCUSDT", at(10, 0), "250"))
		require.NoError(t, saveError)

		clockProxy := mocks.NewMockIClockProxy(gomock.NewController(t))
		clockProxy.EXPECT().Now().Return(at(12, 0)).AnyTimes()

		return service.NewKCandleService(kCandleRepository, clockProxy, 1000)
	}

	t.Run("a range covering both candles gives each its own hour", func(t *testing.T) {
		seriesDto, err := newSeriesService(t).GetKCandleSeries(dto.KCandleSeriesQueryDto{
			Symbol: "BTCUSDT", StartTime: at(9, 50), EndTime: at(10, 30), Interval: "1h",
		})

		require.NoError(t, err)
		require.Len(t, seriesDto.KCandles, 2)
		assert.Equal(t, at(9, 0), seriesDto.KCandles[0].OpenTime)
		assert.True(t, seriesDto.KCandles[0].Close.Equal(kCandleAt("", at(9, 55), "150").Close))
		assert.Equal(t, at(10, 0), seriesDto.KCandles[1].OpenTime)
		assert.True(t, seriesDto.KCandles[1].Close.Equal(kCandleAt("", at(10, 0), "250").Close))
	})

	t.Run("a range starting after a candle leaves that candle out of its bucket", func(t *testing.T) {
		seriesDto, err := newSeriesService(t).GetKCandleSeries(dto.KCandleSeriesQueryDto{
			Symbol: "BTCUSDT", StartTime: at(9, 58), EndTime: at(10, 30), Interval: "1h",
		})

		require.NoError(t, err)
		require.Len(t, seriesDto.KCandles, 1, "the 09:00 bucket held nothing inside the range")
		assert.Equal(t, at(10, 0), seriesDto.KCandles[0].OpenTime)
		assert.True(t, seriesDto.KCandles[0].Close.Equal(kCandleAt("", at(10, 0), "250").Close))
	})
}
