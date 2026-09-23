package service_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
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
		service:    service.NewKCandleContractService(repository, clockProxy, contractQueryMaxResults),
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
