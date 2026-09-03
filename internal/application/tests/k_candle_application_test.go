package application_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

const queryMaxResults = 1000

func at(hour int, minute int) time.Time {
	return time.Date(2026, 8, 29, hour, minute, 0, 0, time.UTC)
}

func kCandleAt(openTime time.Time, closePrice string) entities.KCandle {
	return entities.KCandle{
		Symbol:              "BTCUSDT",
		OpenTime:            openTime,
		Open:                decimal.RequireFromString("100"),
		High:                decimal.RequireFromString("120"),
		Low:                 decimal.RequireFromString("90"),
		Close:               decimal.RequireFromString(closePrice),
		Volume:              decimal.RequireFromString("11"),
		QuoteVolume:         decimal.RequireFromString("1200"),
		TakerBuyBaseVolume:  decimal.RequireFromString("5"),
		TakerBuyQuoteVolume: decimal.RequireFromString("600"),
	}
}

func writeDtoAt(openTime time.Time, closePrice string) dto.KCandleWriteDto {
	kCandle := kCandleAt(openTime, closePrice)
	return dto.KCandleWriteDto{
		Symbol: kCandle.Symbol, OpenTime: kCandle.OpenTime,
		Open: kCandle.Open, High: kCandle.High, Low: kCandle.Low, Close: kCandle.Close,
		Volume: kCandle.Volume, QuoteVolume: kCandle.QuoteVolume,
		TakerBuyBaseVolume: kCandle.TakerBuyBaseVolume, TakerBuyQuoteVolume: kCandle.TakerBuyQuoteVolume,
	}
}

type applicationUnderTest struct {
	kCandleApplication *application.KCandleApplication
	kCandleRepository  *mocks.MockIKCandleRepository
}

// newApplicationUnderTest wires the real domain service and real domain models,
// mocking only the outermost boundaries: storage and the clock.
func newApplicationUnderTest(t *testing.T) applicationUnderTest {
	controller := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(at(12, 0)).AnyTimes()

	kCandleService := service.NewKCandleService(kCandleRepository, clockProxy, queryMaxResults)

	return applicationUnderTest{
		kCandleApplication: application.NewKCandleApplication(kCandleService),
		kCandleRepository:  kCandleRepository,
	}
}

func TestKCandleApplicationSave(t *testing.T) {
	t.Run("stores a valid candle and hands back what was stored", func(t *testing.T) {
		fixture := newApplicationUnderTest(t)
		fixture.kCandleRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Return(kCandleAt(at(9, 0), "120"), nil)

		kCandleDto, err := fixture.kCandleApplication.SaveKCandle(t.Context(), writeDtoAt(at(9, 0), "120"))

		assert.NoError(t, err)
		assert.Equal(t, "BTCUSDT", kCandleDto.Symbol)
		assert.True(t, decimal.RequireFromString("120").Equal(kCandleDto.Close))
	})

	t.Run("refuses a candle whose open time is off the five minute mark", func(t *testing.T) {
		fixture := newApplicationUnderTest(t)

		_, err := fixture.kCandleApplication.SaveKCandle(t.Context(), writeDtoAt(at(9, 3), "120"))

		assert.ErrorIs(t, err, domains.ErrKCandleValidation)
		assert.Contains(t, err.Error(), "起始時間必須落在5分鐘刻度上")
	})

	t.Run("refuses a candle whose open time points into the future", func(t *testing.T) {
		fixture := newApplicationUnderTest(t)

		_, err := fixture.kCandleApplication.SaveKCandle(t.Context(), writeDtoAt(at(12, 5), "120"))

		assert.ErrorIs(t, err, domains.ErrKCandleValidation)
		assert.Contains(t, err.Error(), "起始時間不得指向未來")
	})
}

func TestKCandleApplicationGetInRange(t *testing.T) {
	t.Run("hands back the candles earliest first", func(t *testing.T) {
		fixture := newApplicationUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindInRange(gomock.Any(), gomock.Any(), queryMaxResults+1).
			Return([]entities.KCandle{kCandleAt(at(9, 0), "100"), kCandleAt(at(9, 5), "101")}, nil)

		kCandleDtos, err := fixture.kCandleApplication.GetKCandlesInRange(t.Context(), dto.KCandleQueryDto{
			Symbol: "BTCUSDT", StartTime: at(9, 0), EndTime: at(9, 10),
		})

		assert.NoError(t, err)
		assert.Len(t, kCandleDtos, 2)
		assert.Equal(t, at(9, 0), kCandleDtos[0].OpenTime)
	})

	t.Run("refuses a query with no trading symbol", func(t *testing.T) {
		fixture := newApplicationUnderTest(t)

		_, err := fixture.kCandleApplication.GetKCandlesInRange(t.Context(), dto.KCandleQueryDto{
			Symbol: "", StartTime: at(9, 0), EndTime: at(9, 10),
		})

		assert.ErrorIs(t, err, domains.ErrKCandleValidation)
		assert.Contains(t, err.Error(), "必須指定交易標的")
	})
}

func TestKCandleApplicationGetSeries(t *testing.T) {
	t.Run("hands back one candle per bucket, earliest first", func(t *testing.T) {
		fixture := newApplicationUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindInRange(gomock.Any(), gomock.Any(), 2*12).
			Return([]entities.KCandle{
				kCandleAt(at(9, 55), "150"),
				kCandleAt(at(9, 0), "100"),
				kCandleAt(at(10, 0), "200"),
			}, nil)

		seriesDto, err := fixture.kCandleApplication.GetKCandleSeries(t.Context(), dto.KCandleSeriesQueryDto{
			Symbol: "BTCUSDT", StartTime: at(9, 0), EndTime: at(10, 30), Interval: "1h",
		})

		assert.NoError(t, err)
		assert.Equal(t, "1h", seriesDto.Interval)
		assert.Len(t, seriesDto.KCandles, 2)
		assert.Equal(t, at(9, 0), seriesDto.KCandles[0].OpenTime)
		assert.True(t, decimal.RequireFromString("150").Equal(seriesDto.KCandles[0].Close))
		assert.Equal(t, at(10, 0), seriesDto.KCandles[1].OpenTime)
	})

	t.Run("refuses a range cut into more buckets than one query may answer with", func(t *testing.T) {
		fixture := newApplicationUnderTest(t)

		_, err := fixture.kCandleApplication.GetKCandleSeries(t.Context(), dto.KCandleSeriesQueryDto{
			Symbol:    "BTCUSDT",
			StartTime: at(0, 0),
			EndTime:   at(0, 0).Add(time.Duration(queryMaxResults) * 5 * time.Minute),
			Interval:  "5m",
		})

		assert.ErrorIs(t, err, domains.ErrKCandleValidation)
		assert.Contains(t, err.Error(), "請縮小區間或改用更長的彙總刻度")
	})
}

func TestKCandleApplicationGetUpdateDelete(t *testing.T) {
	t.Run("hands back the named candle", func(t *testing.T) {
		fixture := newApplicationUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindOne(gomock.Any(), "BTCUSDT", at(9, 0)).Return(kCandleAt(at(9, 0), "110"), nil)

		kCandleDto, err := fixture.kCandleApplication.GetKCandle(t.Context(), "BTCUSDT", at(9, 0))

		assert.NoError(t, err)
		assert.True(t, decimal.RequireFromString("110").Equal(kCandleDto.Close))
	})

	t.Run("hands back the updated figures", func(t *testing.T) {
		fixture := newApplicationUnderTest(t)
		fixture.kCandleRepository.EXPECT().Update(gomock.Any(), gomock.Any()).Return(kCandleAt(at(9, 0), "120"), nil)

		kCandleDto, err := fixture.kCandleApplication.UpdateKCandle(t.Context(), writeDtoAt(at(9, 0), "120"))

		assert.NoError(t, err)
		assert.True(t, decimal.RequireFromString("120").Equal(kCandleDto.Close))
	})

	t.Run("reports a candle that does not exist as not found", func(t *testing.T) {
		fixture := newApplicationUnderTest(t)
		fixture.kCandleRepository.EXPECT().Delete(gomock.Any(), "BTCUSDT", at(9, 0)).Return(domains.ErrKCandleNotFound)

		err := fixture.kCandleApplication.DeleteKCandle(t.Context(), "BTCUSDT", at(9, 0))

		assert.ErrorIs(t, err, domains.ErrKCandleNotFound)
	})

	t.Run("removes the named candle", func(t *testing.T) {
		fixture := newApplicationUnderTest(t)
		fixture.kCandleRepository.EXPECT().Delete(gomock.Any(), "BTCUSDT", at(9, 0)).Return(nil)

		err := fixture.kCandleApplication.DeleteKCandle(t.Context(), "BTCUSDT", at(9, 0))

		assert.NoError(t, err)
	})
}
