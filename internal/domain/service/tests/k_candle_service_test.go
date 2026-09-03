package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

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

var currentTime = time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)

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
		Symbol:              kCandle.Symbol,
		OpenTime:            kCandle.OpenTime,
		Open:                kCandle.Open,
		High:                kCandle.High,
		Low:                 kCandle.Low,
		Close:               kCandle.Close,
		Volume:              kCandle.Volume,
		QuoteVolume:         kCandle.QuoteVolume,
		TakerBuyBaseVolume:  kCandle.TakerBuyBaseVolume,
		TakerBuyQuoteVolume: kCandle.TakerBuyQuoteVolume,
	}
}

type serviceUnderTest struct {
	kCandleService    *service.KCandleService
	kCandleRepository *mocks.MockIKCandleRepository
}

func newServiceUnderTest(t *testing.T) serviceUnderTest {
	controller := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(currentTime).AnyTimes()

	return serviceUnderTest{
		kCandleService:    service.NewKCandleService(kCandleRepository, clockProxy, queryMaxResults),
		kCandleRepository: kCandleRepository,
	}
}

func TestGetKCandlesInRange(t *testing.T) {
	t.Run("returns every candle in the range, earliest first", func(t *testing.T) {
		fixture := newServiceUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindInRange(gomock.Any(), gomock.Any(), queryMaxResults+1).
			Return([]entities.KCandle{
				kCandleAt(at(9, 0), "100"),
				kCandleAt(at(9, 5), "101"),
				kCandleAt(at(9, 10), "102"),
			}, nil)

		kCandleDtos, err := fixture.kCandleService.GetKCandlesInRange(t.Context(), dto.KCandleQueryDto{
			Symbol: "BTCUSDT", StartTime: at(9, 0), EndTime: at(9, 10),
		})

		assert.NoError(t, err)
		assert.Len(t, kCandleDtos, 3)
		assert.Equal(t, at(9, 0), kCandleDtos[0].OpenTime)
		assert.Equal(t, at(9, 5), kCandleDtos[1].OpenTime)
		assert.Equal(t, at(9, 10), kCandleDtos[2].OpenTime)
	})

	t.Run("returns an empty list rather than an error when nothing falls in the range", func(t *testing.T) {
		fixture := newServiceUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindInRange(gomock.Any(), gomock.Any(), queryMaxResults+1).
			Return([]entities.KCandle{}, nil)

		kCandleDtos, err := fixture.kCandleService.GetKCandlesInRange(t.Context(), dto.KCandleQueryDto{
			Symbol: "BTCUSDT", StartTime: at(11, 0), EndTime: at(12, 0),
		})

		assert.NoError(t, err)
		assert.Empty(t, kCandleDtos)
	})

	t.Run("passes the trading symbol and range through to storage", func(t *testing.T) {
		fixture := newServiceUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindInRange(gomock.Any(), gomock.Any(), queryMaxResults+1).
			DoAndReturn(func(_ context.Context, query domains.KCandleQueryDomain, limit int) ([]entities.KCandle, error) {
				assert.Equal(t, "ETHUSDT", query.Symbol())
				assert.Equal(t, at(9, 1), query.StartTime())
				assert.Equal(t, at(9, 9), query.EndTime())
				return []entities.KCandle{}, nil
			})

		kCandleDtos, err := fixture.kCandleService.GetKCandlesInRange(t.Context(), dto.KCandleQueryDto{
			Symbol: "ETHUSDT", StartTime: at(9, 1), EndTime: at(9, 9),
		})

		assert.NoError(t, err)
		assert.Empty(t, kCandleDtos)
	})

	t.Run("returns the full page when the range holds exactly the maximum", func(t *testing.T) {
		fixture := newServiceUnderTest(t)
		storedKCandles := make([]entities.KCandle, 0, queryMaxResults)
		for index := range queryMaxResults {
			storedKCandles = append(storedKCandles, kCandleAt(at(9, 0).Add(time.Duration(index)*5*time.Minute), "100"))
		}
		fixture.kCandleRepository.EXPECT().
			FindInRange(gomock.Any(), gomock.Any(), queryMaxResults+1).
			Return(storedKCandles, nil)

		kCandleDtos, err := fixture.kCandleService.GetKCandlesInRange(t.Context(), dto.KCandleQueryDto{
			Symbol: "BTCUSDT", StartTime: at(9, 0), EndTime: at(23, 0),
		})

		assert.NoError(t, err)
		assert.Len(t, kCandleDtos, queryMaxResults)
	})

	t.Run("refuses a range holding more than the maximum", func(t *testing.T) {
		fixture := newServiceUnderTest(t)
		storedKCandles := make([]entities.KCandle, 0, queryMaxResults+1)
		for index := range queryMaxResults + 1 {
			storedKCandles = append(storedKCandles, kCandleAt(at(9, 0).Add(time.Duration(index)*5*time.Minute), "100"))
		}
		fixture.kCandleRepository.EXPECT().
			FindInRange(gomock.Any(), gomock.Any(), queryMaxResults+1).
			Return(storedKCandles, nil)

		kCandleDtos, err := fixture.kCandleService.GetKCandlesInRange(t.Context(), dto.KCandleQueryDto{
			Symbol: "BTCUSDT", StartTime: at(9, 0), EndTime: at(23, 0),
		})

		assert.ErrorIs(t, err, domains.ErrKCandleValidation)
		assert.Contains(t, err.Error(), "時間區間過大")
		assert.Nil(t, kCandleDtos)
	})

	t.Run("never reaches storage when the query breaks a rule", func(t *testing.T) {
		fixture := newServiceUnderTest(t)

		_, err := fixture.kCandleService.GetKCandlesInRange(t.Context(), dto.KCandleQueryDto{
			Symbol: "", StartTime: at(9, 0), EndTime: at(9, 10),
		})

		assert.ErrorIs(t, err, domains.ErrKCandleValidation)
	})

	t.Run("reports a storage failure", func(t *testing.T) {
		fixture := newServiceUnderTest(t)
		storageFailure := errors.New("storage unreachable")
		fixture.kCandleRepository.EXPECT().
			FindInRange(gomock.Any(), gomock.Any(), queryMaxResults+1).
			Return(nil, storageFailure)

		_, err := fixture.kCandleService.GetKCandlesInRange(t.Context(), dto.KCandleQueryDto{
			Symbol: "BTCUSDT", StartTime: at(9, 0), EndTime: at(9, 10),
		})

		assert.ErrorIs(t, err, storageFailure)
	})
}

func TestGetKCandleSeries(t *testing.T) {
	t.Run("merges the candles of each bucket into one, earliest first", func(t *testing.T) {
		fixture := newServiceUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindInRange(gomock.Any(), gomock.Any(), 3*12).
			Return([]entities.KCandle{
				kCandleAt(at(9, 0), "100"),
				kCandleAt(at(9, 55), "150"),
				kCandleAt(at(11, 0), "200"),
			}, nil)

		seriesDto, err := fixture.kCandleService.GetKCandleSeries(t.Context(), dto.KCandleSeriesQueryDto{
			Symbol: "BTCUSDT", StartTime: at(9, 0), EndTime: at(11, 30), Interval: "1h",
		})

		assert.NoError(t, err)
		assert.Equal(t, "BTCUSDT", seriesDto.Symbol)
		assert.Equal(t, "1h", seriesDto.Interval)
		assert.Len(t, seriesDto.KCandles, 2)
		assert.Equal(t, at(9, 0), seriesDto.KCandles[0].OpenTime)
		assert.True(t, decimal.RequireFromString("150").Equal(seriesDto.KCandles[0].Close))
		assert.Equal(t, at(11, 0), seriesDto.KCandles[1].OpenTime)
		assert.True(t, decimal.RequireFromString("200").Equal(seriesDto.KCandles[1].Close))
	})

	t.Run("leaves out a bucket nothing fell into", func(t *testing.T) {
		fixture := newServiceUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindInRange(gomock.Any(), gomock.Any(), 3*12).
			Return([]entities.KCandle{kCandleAt(at(10, 5), "100"), kCandleAt(at(12, 30), "200")}, nil)

		seriesDto, err := fixture.kCandleService.GetKCandleSeries(t.Context(), dto.KCandleSeriesQueryDto{
			Symbol: "BTCUSDT", StartTime: at(10, 0), EndTime: at(12, 59), Interval: "1h",
		})

		assert.NoError(t, err)
		assert.Len(t, seriesDto.KCandles, 2)
		assert.Equal(t, at(10, 0), seriesDto.KCandles[0].OpenTime)
		assert.Equal(t, at(12, 0), seriesDto.KCandles[1].OpenTime)
	})

	t.Run("returns an empty series rather than an error when nothing falls in the range", func(t *testing.T) {
		fixture := newServiceUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandle{}, nil)

		seriesDto, err := fixture.kCandleService.GetKCandleSeries(t.Context(), dto.KCandleSeriesQueryDto{
			Symbol: "BTCUSDT", StartTime: at(9, 0), EndTime: at(10, 0), Interval: "1h",
		})

		assert.NoError(t, err)
		assert.Empty(t, seriesDto.KCandles)
		assert.Equal(t, "BTCUSDT", seriesDto.Symbol)
		assert.Equal(t, "1h", seriesDto.Interval)
	})

	t.Run("aggregating at five minutes leaves every candle as it was", func(t *testing.T) {
		fixture := newServiceUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindInRange(gomock.Any(), gomock.Any(), 3).
			Return([]entities.KCandle{
				kCandleAt(at(9, 0), "100"),
				kCandleAt(at(9, 5), "101"),
				kCandleAt(at(9, 10), "102"),
			}, nil)

		seriesDto, err := fixture.kCandleService.GetKCandleSeries(t.Context(), dto.KCandleSeriesQueryDto{
			Symbol: "BTCUSDT", StartTime: at(9, 0), EndTime: at(9, 10), Interval: "5m",
		})

		assert.NoError(t, err)
		assert.Len(t, seriesDto.KCandles, 3)
		assert.Equal(t, at(9, 5), seriesDto.KCandles[1].OpenTime)
		assert.True(t, decimal.RequireFromString("101").Equal(seriesDto.KCandles[1].Close))
	})

	t.Run("never reaches storage when the range is cut into too many buckets", func(t *testing.T) {
		fixture := newServiceUnderTest(t)

		_, err := fixture.kCandleService.GetKCandleSeries(t.Context(), dto.KCandleSeriesQueryDto{
			Symbol:    "BTCUSDT",
			StartTime: at(0, 0),
			EndTime:   at(0, 0).Add(time.Duration(queryMaxResults) * 5 * time.Minute),
			Interval:  "5m",
		})

		assert.ErrorIs(t, err, domains.ErrKCandleValidation)
		assert.Contains(t, err.Error(), "時間區間過大，請縮小區間或改用更長的彙總刻度")
	})

	t.Run("answers the same range at a longer interval", func(t *testing.T) {
		fixture := newServiceUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandle{kCandleAt(at(9, 0), "100")}, nil)

		seriesDto, err := fixture.kCandleService.GetKCandleSeries(t.Context(), dto.KCandleSeriesQueryDto{
			Symbol:    "BTCUSDT",
			StartTime: at(0, 0),
			EndTime:   at(0, 0).Add(time.Duration(queryMaxResults) * 5 * time.Minute),
			Interval:  "1d",
		})

		assert.NoError(t, err)
		assert.Len(t, seriesDto.KCandles, 1)
	})

	t.Run("never reaches storage when the interval is one nobody offers", func(t *testing.T) {
		fixture := newServiceUnderTest(t)

		_, err := fixture.kCandleService.GetKCandleSeries(t.Context(), dto.KCandleSeriesQueryDto{
			Symbol: "BTCUSDT", StartTime: at(9, 0), EndTime: at(10, 0), Interval: "7m",
		})

		assert.ErrorIs(t, err, domains.ErrKCandleValidation)
		assert.Contains(t, err.Error(), "彙總刻度只能是")
	})

	t.Run("never reaches storage when the query breaks a range rule", func(t *testing.T) {
		fixture := newServiceUnderTest(t)

		_, err := fixture.kCandleService.GetKCandleSeries(t.Context(), dto.KCandleSeriesQueryDto{
			Symbol: "", StartTime: at(9, 0), EndTime: at(10, 0), Interval: "1h",
		})

		assert.ErrorIs(t, err, domains.ErrKCandleValidation)
		assert.Contains(t, err.Error(), "必須指定交易標的")
	})

	t.Run("reports a storage failure", func(t *testing.T) {
		fixture := newServiceUnderTest(t)
		storageFailure := errors.New("storage unreachable")
		fixture.kCandleRepository.EXPECT().
			FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, storageFailure)

		_, err := fixture.kCandleService.GetKCandleSeries(t.Context(), dto.KCandleSeriesQueryDto{
			Symbol: "BTCUSDT", StartTime: at(9, 0), EndTime: at(10, 0), Interval: "1h",
		})

		assert.ErrorIs(t, err, storageFailure)
	})
}

func TestSaveKCandle(t *testing.T) {
	t.Run("stores the candle and returns what was stored", func(t *testing.T) {
		fixture := newServiceUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			Save(gomock.Any(), gomock.Any()).
			Return(kCandleAt(at(9, 0), "120"), nil)

		kCandleDto, err := fixture.kCandleService.SaveKCandle(t.Context(), writeDtoAt(at(9, 0), "120"))

		assert.NoError(t, err)
		assert.Equal(t, "BTCUSDT", kCandleDto.Symbol)
		assert.Equal(t, at(9, 0), kCandleDto.OpenTime)
		assert.True(t, decimal.RequireFromString("120").Equal(kCandleDto.Close))
	})

	t.Run("never reaches storage when the candle breaks a rule", func(t *testing.T) {
		fixture := newServiceUnderTest(t)

		_, err := fixture.kCandleService.SaveKCandle(t.Context(), writeDtoAt(at(9, 3), "120"))

		assert.ErrorIs(t, err, domains.ErrKCandleValidation)
	})

	t.Run("reports a storage failure", func(t *testing.T) {
		fixture := newServiceUnderTest(t)
		storageFailure := errors.New("storage unreachable")
		fixture.kCandleRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Return(entities.KCandle{}, storageFailure)

		_, err := fixture.kCandleService.SaveKCandle(t.Context(), writeDtoAt(at(9, 0), "120"))

		assert.ErrorIs(t, err, storageFailure)
	})
}

func TestGetKCandle(t *testing.T) {
	t.Run("returns the named candle", func(t *testing.T) {
		fixture := newServiceUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindOne(gomock.Any(), "BTCUSDT", at(9, 0)).
			Return(kCandleAt(at(9, 0), "110"), nil)

		kCandleDto, err := fixture.kCandleService.GetKCandle(t.Context(), "BTCUSDT", at(9, 0))

		assert.NoError(t, err)
		assert.Equal(t, at(9, 0), kCandleDto.OpenTime)
		assert.True(t, decimal.RequireFromString("110").Equal(kCandleDto.Close))
	})

	t.Run("reports that a candle which does not exist was not found", func(t *testing.T) {
		fixture := newServiceUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindOne(gomock.Any(), "BTCUSDT", at(9, 0)).
			Return(entities.KCandle{}, domains.ErrKCandleNotFound)

		_, err := fixture.kCandleService.GetKCandle(t.Context(), "BTCUSDT", at(9, 0))

		assert.ErrorIs(t, err, domains.ErrKCandleNotFound)
	})
}

func TestUpdateKCandle(t *testing.T) {
	t.Run("replaces the figures of an existing candle", func(t *testing.T) {
		fixture := newServiceUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			Update(gomock.Any(), gomock.Any()).
			Return(kCandleAt(at(9, 0), "120"), nil)

		kCandleDto, err := fixture.kCandleService.UpdateKCandle(t.Context(), writeDtoAt(at(9, 0), "120"))

		assert.NoError(t, err)
		assert.True(t, decimal.RequireFromString("120").Equal(kCandleDto.Close))
	})

	t.Run("reports that a candle which does not exist was not found", func(t *testing.T) {
		fixture := newServiceUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			Update(gomock.Any(), gomock.Any()).
			Return(entities.KCandle{}, domains.ErrKCandleNotFound)

		_, err := fixture.kCandleService.UpdateKCandle(t.Context(), writeDtoAt(at(9, 0), "120"))

		assert.ErrorIs(t, err, domains.ErrKCandleNotFound)
	})

	t.Run("leaves the stored candle untouched when the new figures break a rule", func(t *testing.T) {
		fixture := newServiceUnderTest(t)
		brokenWriteDto := writeDtoAt(at(9, 0), "120")
		brokenWriteDto.High = decimal.RequireFromString("90")
		brokenWriteDto.Low = decimal.RequireFromString("100")

		_, err := fixture.kCandleService.UpdateKCandle(t.Context(), brokenWriteDto)

		assert.ErrorIs(t, err, domains.ErrKCandleValidation)
		assert.Contains(t, err.Error(), "最高價不得低於最低價")
	})
}

func TestDeleteKCandle(t *testing.T) {
	t.Run("removes the named candle", func(t *testing.T) {
		fixture := newServiceUnderTest(t)
		fixture.kCandleRepository.EXPECT().Delete(gomock.Any(), "BTCUSDT", at(9, 0)).Return(nil)

		err := fixture.kCandleService.DeleteKCandle(t.Context(), "BTCUSDT", at(9, 0))

		assert.NoError(t, err)
	})

	t.Run("reports that a candle which does not exist was not found", func(t *testing.T) {
		fixture := newServiceUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			Delete(gomock.Any(), "BTCUSDT", at(9, 0)).
			Return(domains.ErrKCandleNotFound)

		err := fixture.kCandleService.DeleteKCandle(t.Context(), "BTCUSDT", at(9, 0))

		assert.ErrorIs(t, err, domains.ErrKCandleNotFound)
	})
}
