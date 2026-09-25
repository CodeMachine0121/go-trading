package persistence_test

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// Which candles fall in a half-covered bucket is decided by the storage range filter, so this needs the real database.
func TestGetKCandleSeriesAgainstStorage(t *testing.T) {
	newSeriesService := func(t *testing.T) *service.KCandleService {
		t.Helper()

		database := newTestDatabase(t)
		kCandleRepository := persistence.NewKCandleRepository(database)
		_, saveError := kCandleRepository.Save(t.Context(), kCandleAt("BTCUSDT", at(9, 55), "150"))
		require.NoError(t, saveError)
		_, saveError = kCandleRepository.Save(t.Context(), kCandleAt("BTCUSDT", at(10, 0), "250"))
		require.NoError(t, saveError)

		clockProxy := mocks.NewMockIClockProxy(gomock.NewController(t))
		clockProxy.EXPECT().Now().Return(at(12, 0)).AnyTimes()

		// 未登錄的交易標的視為永不收盤的市場，所以每一分鐘都算數。
		return service.NewKCandleService(
			kCandleRepository, persistence.NewTradingSymbolRepository(database), clockProxy,
			domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}),
			1000)
	}

	t.Run("a range covering both candles gives each its own hour", func(t *testing.T) {
		seriesDto, err := newSeriesService(t).GetKCandleSeries(t.Context(), dto.KCandleSeriesQueryDto{
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
		seriesDto, err := newSeriesService(t).GetKCandleSeries(t.Context(), dto.KCandleSeriesQueryDto{
			Symbol: "BTCUSDT", StartTime: at(9, 58), EndTime: at(10, 30), Interval: "1h",
		})

		require.NoError(t, err)
		require.Len(t, seriesDto.KCandles, 1, "the 09:00 bucket held nothing inside the range")
		assert.Equal(t, at(10, 0), seriesDto.KCandles[0].OpenTime)
		assert.True(t, seriesDto.KCandles[0].Close.Equal(kCandleAt("", at(10, 0), "250").Close))
	})
}
