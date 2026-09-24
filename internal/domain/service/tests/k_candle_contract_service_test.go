package service_test

import (
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const contractQueryMaxResults = 3

func contractTradeCount(tradeCount int64) *int64 {
	return &tradeCount
}

func contractWriteDto() dto.KCandleContractWriteDto {
	return dto.KCandleContractWriteDto{
		Symbol:              "BTCUSDT",
		OpenTime:            ingestionAt(9, 0, 0),
		Open:                decimal.RequireFromString("100"),
		High:                decimal.RequireFromString("120"),
		Low:                 decimal.RequireFromString("90"),
		Close:               decimal.RequireFromString("110"),
		Volume:              decimal.RequireFromString("11"),
		QuoteVolume:         decimal.NewNullDecimal(decimal.RequireFromString("1200")),
		TakerBuyBaseVolume:  decimal.NewNullDecimal(decimal.RequireFromString("5")),
		TakerBuyQuoteVolume: decimal.NewNullDecimal(decimal.RequireFromString("600")),
		TradeCount:          contractTradeCount(7),
		MarkOpen:            decimal.NewNullDecimal(decimal.RequireFromString("101")),
		MarkHigh:            decimal.NewNullDecimal(decimal.RequireFromString("121")),
		MarkLow:             decimal.NewNullDecimal(decimal.RequireFromString("91")),
		MarkClose:           decimal.NewNullDecimal(decimal.RequireFromString("111")),
		IndexOpen:           decimal.NewNullDecimal(decimal.RequireFromString("102")),
		IndexHigh:           decimal.NewNullDecimal(decimal.RequireFromString("122")),
		IndexLow:            decimal.NewNullDecimal(decimal.RequireFromString("92")),
		IndexClose:          decimal.NewNullDecimal(decimal.RequireFromString("112")),
		PremiumIndexOpen:    decimal.NewNullDecimal(decimal.RequireFromString("-0.0001")),
		PremiumIndexHigh:    decimal.NewNullDecimal(decimal.RequireFromString("0.0002")),
		PremiumIndexLow:     decimal.NewNullDecimal(decimal.RequireFromString("-0.0003")),
		PremiumIndexClose:   decimal.NewNullDecimal(decimal.RequireFromString("0.0001")),
	}
}

func storedContractCandle(openTime time.Time, closePrice string) entities.KCandleContract {
	return entities.KCandleContract{
		Symbol:     "BTCUSDT",
		OpenTime:   openTime,
		Close:      decimal.RequireFromString(closePrice),
		MarkClose:  decimal.RequireFromString("111"),
		TradeCount: 7,
	}
}

type contractServiceUnderTest struct {
	service    *service.KCandleContractService
	repository *mocks.MockIKCandleContractRepository
}

func newContractServiceUnderTest(t *testing.T) contractServiceUnderTest {
	t.Helper()

	mockController := gomock.NewController(t)
	repository := mocks.NewMockIKCandleContractRepository(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(ingestionAt(9, 7, 30)).AnyTimes()

	return contractServiceUnderTest{
		// A market that closes sits in the catalogue beside the round-the-clock one, so a
		// contract series that consulted the wrong calendar would lose the hours it shuts.
		service: service.NewKCandleContractService(repository, clockProxy,
			domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
				vo.MarketCrypto: {},
				vo.MarketTaiwanStock: {TradingSession: vo.TradingSessionVo{
					Location:   time.FixedZone("Asia/Taipei", 8*60*60),
					DailyStart: 9 * time.Hour, DailyEnd: 13*time.Hour + 30*time.Minute,
					Weekdays: []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday},
				}},
			}), contractQueryMaxResults),
		repository: repository,
	}
}

func TestKCandleContractServiceStoresACandleWithoutConsultingTheWatchlist(t *testing.T) {
	underTest := newContractServiceUnderTest(t)
	underTest.repository.EXPECT().Save(gomock.Any(), gomock.Any()).
		Return(storedContractCandle(ingestionAt(9, 0, 0), "110"), nil)

	savedCandle, saveError := underTest.service.SaveKCandleContract(t.Context(), contractWriteDto())

	require.NoError(t, saveError)
	assert.Equal(t, "BTCUSDT", savedCandle.Symbol)
	assert.True(t, decimal.RequireFromString("110").Equal(savedCandle.Close))
	assert.True(t, decimal.RequireFromString("111").Equal(savedCandle.MarkClose))
	assert.Equal(t, int64(7), savedCandle.TradeCount)
}

func TestKCandleContractServiceRefusesACandleWithoutItsMarkPrice(t *testing.T) {
	underTest := newContractServiceUnderTest(t)
	writeDto := contractWriteDto()
	writeDto.MarkClose = decimal.NullDecimal{}
	underTest.repository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)

	_, saveError := underTest.service.SaveKCandleContract(t.Context(), writeDto)

	assert.ErrorIs(t, saveError, domains.ErrKCandleContractValidation)
}

func TestKCandleContractServiceReadsARangeAndRefusesOneTooWide(t *testing.T) {
	underTest := newContractServiceUnderTest(t)
	queryDto := dto.KCandleQueryDto{
		Symbol: "BTCUSDT", StartTime: ingestionAt(9, 0, 0), EndTime: ingestionAt(9, 9, 0),
	}
	underTest.repository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), contractQueryMaxResults+1).
		Return([]entities.KCandleContract{
			storedContractCandle(ingestionAt(9, 0, 0), "100"),
			storedContractCandle(ingestionAt(9, 1, 0), "101"),
		}, nil)

	contractCandles, findError := underTest.service.GetKCandleContractsInRange(t.Context(), queryDto)

	require.NoError(t, findError)
	require.Len(t, contractCandles, 2)
	assert.True(t, decimal.RequireFromString("111").Equal(contractCandles[0].MarkClose))

	underTest.repository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), contractQueryMaxResults+1).
		Return(make([]entities.KCandleContract, contractQueryMaxResults+1), nil)

	_, tooWideError := underTest.service.GetKCandleContractsInRange(t.Context(), queryDto)

	assert.ErrorIs(t, tooWideError, domains.ErrKCandleContractValidation)
	assert.Contains(t, tooWideError.Error(), "3")
}

func TestKCandleContractServiceAnswersAnEmptyRangeWithNothingRatherThanAFailure(t *testing.T) {
	underTest := newContractServiceUnderTest(t)
	underTest.repository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entities.KCandleContract{}, nil)

	contractCandles, findError := underTest.service.GetKCandleContractsInRange(
		t.Context(), dto.KCandleQueryDto{
			Symbol: "BTCUSDT", StartTime: ingestionAt(9, 0, 0), EndTime: ingestionAt(9, 9, 0),
		})

	require.NoError(t, findError)
	assert.Empty(t, contractCandles)
}

func TestKCandleContractServiceReadsUpdatesAndDeletesOneNamedCandle(t *testing.T) {
	underTest := newContractServiceUnderTest(t)
	underTest.repository.EXPECT().FindOne(gomock.Any(), "BTCUSDT", ingestionAt(9, 0, 0)).
		Return(storedContractCandle(ingestionAt(9, 0, 0), "110"), nil)
	underTest.repository.EXPECT().Update(gomock.Any(), gomock.Any()).
		Return(storedContractCandle(ingestionAt(9, 0, 0), "120"), nil)
	underTest.repository.EXPECT().Delete(gomock.Any(), "BTCUSDT", ingestionAt(9, 0, 0)).Return(nil)

	readCandle, readError := underTest.service.GetKCandleContract(
		t.Context(), "BTCUSDT", ingestionAt(9, 0, 0))
	updatedCandle, updateError := underTest.service.UpdateKCandleContract(
		t.Context(), contractWriteDto())
	deleteError := underTest.service.DeleteKCandleContract(
		t.Context(), "BTCUSDT", ingestionAt(9, 0, 0))

	require.NoError(t, readError)
	require.NoError(t, updateError)
	require.NoError(t, deleteError)
	assert.True(t, decimal.RequireFromString("110").Equal(readCandle.Close))
	assert.True(t, decimal.RequireFromString("120").Equal(updatedCandle.Close))
}

func TestKCandleContractServiceRefusesANameItCannotRead(t *testing.T) {
	underTest := newContractServiceUnderTest(t)

	_, readError := underTest.service.GetKCandleContract(t.Context(), "", ingestionAt(9, 0, 0))
	deleteError := underTest.service.DeleteKCandleContract(t.Context(), "", ingestionAt(9, 0, 0))

	assert.ErrorIs(t, readError, domains.ErrKCandleContractValidation)
	assert.ErrorIs(t, deleteError, domains.ErrKCandleContractValidation)
}

func TestKCandleContractServiceRefusesAnUpdateThatBreaksARule(t *testing.T) {
	underTest := newContractServiceUnderTest(t)
	writeDto := contractWriteDto()
	writeDto.TradeCount = nil
	underTest.repository.EXPECT().Update(gomock.Any(), gomock.Any()).Times(0)

	_, updateError := underTest.service.UpdateKCandleContract(t.Context(), writeDto)

	assert.ErrorIs(t, updateError, domains.ErrKCandleContractValidation)
}

func TestKCandleContractServicePassesStorageFailuresOn(t *testing.T) {
	underTest := newContractServiceUnderTest(t)
	underTest.repository.EXPECT().Save(gomock.Any(), gomock.Any()).
		Return(entities.KCandleContract{}, sourceUnreachable)
	underTest.repository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, sourceUnreachable)
	underTest.repository.EXPECT().FindOne(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entities.KCandleContract{}, sourceUnreachable)
	underTest.repository.EXPECT().Update(gomock.Any(), gomock.Any()).
		Return(entities.KCandleContract{}, sourceUnreachable)

	_, saveError := underTest.service.SaveKCandleContract(t.Context(), contractWriteDto())
	_, rangeError := underTest.service.GetKCandleContractsInRange(t.Context(), dto.KCandleQueryDto{
		Symbol: "BTCUSDT", StartTime: ingestionAt(9, 0, 0), EndTime: ingestionAt(9, 9, 0),
	})
	_, readError := underTest.service.GetKCandleContract(t.Context(), "BTCUSDT", ingestionAt(9, 0, 0))
	_, updateError := underTest.service.UpdateKCandleContract(t.Context(), contractWriteDto())

	for _, storageError := range []error{saveError, rangeError, readError, updateError} {
		assert.ErrorIs(t, storageError, sourceUnreachable)
	}
}

func TestKCandleContractServiceRefusesAQueryItCannotRead(t *testing.T) {
	underTest := newContractServiceUnderTest(t)

	_, queryError := underTest.service.GetKCandleContractsInRange(t.Context(), dto.KCandleQueryDto{
		Symbol: "", StartTime: ingestionAt(9, 0, 0), EndTime: ingestionAt(9, 9, 0),
	})

	assert.Error(t, queryError)
}

func TestKCandleContractServiceMergesAStretchByInterval(t *testing.T) {
	underTest := newContractServiceUnderTest(t)
	underTest.repository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, query domains.KCandleQueryDomain, limit int) ([]entities.KCandleContract, error) {
			assert.Equal(t, "BTCUSDT", query.Symbol())
			// Enough to read every minute of the stretch asked about.
			assert.GreaterOrEqual(t, limit, 10)

			first, second := storedContractCandle(ingestionAt(9, 0, 0), "110"), storedContractCandle(ingestionAt(9, 1, 0), "111")
			first.TradeCount, second.TradeCount = 7, 8

			return []entities.KCandleContract{first, second}, nil
		})

	series, seriesError := underTest.service.GetKCandleContractSeries(t.Context(), dto.KCandleSeriesQueryDto{
		Symbol: "btcusdt", StartTime: ingestionAt(9, 0, 0), EndTime: ingestionAt(9, 9, 0), Interval: "5m",
	})

	require.NoError(t, seriesError)
	assert.Equal(t, "5m", series.Interval)
	require.Len(t, series.KCandles, 1)
	assert.Equal(t, int64(15), series.KCandles[0].TradeCount)
	assert.True(t, decimal.RequireFromString("111").Equal(series.KCandles[0].Close))
}

func TestKCandleContractServiceMergesAWeekendStretchBecauseContractsNeverClose(t *testing.T) {
	underTest := newContractServiceUnderTest(t)
	// The hours asked about are on a Sunday: a market that closes has no trading time in them.
	require.Equal(t, time.Sunday, ingestionAt(3, 0, 0).Weekday())
	underTest.repository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, _ domains.KCandleQueryDomain, limit int) ([]entities.KCandleContract, error) {
			// Every minute of all three hours can be read.
			assert.GreaterOrEqual(t, limit, 3*60)

			return []entities.KCandleContract{
				storedContractCandle(ingestionAt(3, 0, 0), "110"),
				storedContractCandle(ingestionAt(5, 59, 0), "120"),
			}, nil
		})

	series, seriesError := underTest.service.GetKCandleContractSeries(t.Context(), dto.KCandleSeriesQueryDto{
		Symbol: "BTCUSDT", StartTime: ingestionAt(3, 0, 0), EndTime: ingestionAt(5, 59, 0), Interval: "1h",
	})

	require.NoError(t, seriesError)
	require.Len(t, series.KCandles, 2)
	assert.Equal(t, ingestionAt(3, 0, 0), series.KCandles[0].OpenTime.UTC())
	assert.Equal(t, ingestionAt(5, 0, 0), series.KCandles[1].OpenTime.UTC())
}

func TestKCandleContractServiceRefusesASeriesItCannotAnswerInItsOwnWords(t *testing.T) {
	displayable := 10
	oneMonth := ingestionAt(9, 0, 0).Add(30 * 24 * time.Hour)
	testCases := []struct {
		name            string
		queryDto        dto.KCandleSeriesQueryDto
		expectedMessage string
	}{
		{name: "兩種說法同時給", queryDto: dto.KCandleSeriesQueryDto{Symbol: "BTCUSDT", StartTime: ingestionAt(9, 0, 0),
			EndTime: ingestionAt(9, 9, 0), Interval: "5m", DisplayableCandleCount: &displayable},
			expectedMessage: "彙總刻度與可顯示根數只能挑一種說法"},
		{name: "認不得的刻度", queryDto: dto.KCandleSeriesQueryDto{Symbol: "BTCUSDT", StartTime: ingestionAt(9, 0, 0),
			EndTime: ingestionAt(9, 9, 0), Interval: "2m"}, expectedMessage: "彙總刻度只能是"},
		{name: "區間過大", queryDto: dto.KCandleSeriesQueryDto{Symbol: "BTCUSDT", StartTime: ingestionAt(9, 0, 0),
			EndTime: oneMonth, Interval: "1m"}, expectedMessage: "時間區間過大"},
		{name: "沒指定合約標的", queryDto: dto.KCandleSeriesQueryDto{StartTime: ingestionAt(9, 0, 0),
			EndTime: ingestionAt(9, 9, 0)}, expectedMessage: "必須指定交易標的"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newContractServiceUnderTest(t)

			_, seriesError := underTest.service.GetKCandleContractSeries(t.Context(), testCase.queryDto)

			assert.ErrorIs(t, seriesError, domains.ErrKCandleContractValidation)
			assert.ErrorContains(t, seriesError, testCase.expectedMessage)
		})
	}
}

func TestKCandleContractServicePassesAStorageFailureOnWhenMergingASeries(t *testing.T) {
	underTest := newContractServiceUnderTest(t)
	underTest.repository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("storage unreachable"))

	_, seriesError := underTest.service.GetKCandleContractSeries(t.Context(), dto.KCandleSeriesQueryDto{
		Symbol: "BTCUSDT", StartTime: ingestionAt(9, 0, 0), EndTime: ingestionAt(9, 9, 0), Interval: "5m",
	})

	assert.ErrorContains(t, seriesError, "storage unreachable")
	assert.NotErrorIs(t, seriesError, domains.ErrKCandleContractValidation)
}

func TestKCandleContractServiceReadsAContractsNewestCandle(t *testing.T) {
	testCases := []struct {
		name          string
		storedCandles []entities.KCandleContract
		expectedFound bool
		expectedClose string
	}{
		{name: "the newest one there is",
			storedCandles: []entities.KCandleContract{storedContractCandle(ingestionAt(9, 6, 0), "64000.5")},
			expectedFound: true, expectedClose: "64000.5"},
		{name: "nothing stored is an answer, not a failure", expectedFound: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newContractServiceUnderTest(t)
			underTest.repository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).
				Return(testCase.storedCandles, nil)

			latestCandle, found, readError := underTest.service.GetLatestKCandleContract(t.Context(), "BTCUSDT")

			require.NoError(t, readError)
			assert.Equal(t, testCase.expectedFound, found)
			if testCase.expectedFound {
				assert.Equal(t, testCase.expectedClose, latestCandle.Close.String())
			}
		})
	}
}

func TestKCandleContractServiceRefusesTheNewestCandleOfNoSymbol(t *testing.T) {
	underTest := newContractServiceUnderTest(t)

	_, _, readError := underTest.service.GetLatestKCandleContract(t.Context(), "")

	assert.ErrorIs(t, readError, domains.ErrKCandleContractValidation)
}

func TestKCandleContractServiceReportsANewestCandleItCouldNotRead(t *testing.T) {
	underTest := newContractServiceUnderTest(t)
	underTest.repository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).
		Return(nil, errors.New("the database went away"))

	_, _, readError := underTest.service.GetLatestKCandleContract(t.Context(), "BTCUSDT")

	assert.EqualError(t, readError, "the database went away")
}
