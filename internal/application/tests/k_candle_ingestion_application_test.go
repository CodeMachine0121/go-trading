package application_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	ingestionRoundCandleCount = 5
	ingestionLookback         = 24 * time.Hour
)

func ingestionAt(hour int, minute int) time.Time {
	return time.Date(2026, 8, 30, hour, minute, 0, 0, time.UTC)
}

func reportedKCandleAt(openTime time.Time) vo.MarketKCandleVo {
	return vo.MarketKCandleVo{
		Symbol:              "BTCUSDT",
		OpenTime:            openTime,
		Open:                decimal.RequireFromString("100"),
		High:                decimal.RequireFromString("120"),
		Low:                 decimal.RequireFromString("90"),
		Close:               decimal.RequireFromString("110"),
		Volume:              decimal.RequireFromString("11"),
		QuoteVolume:         decimal.NewNullDecimal(decimal.RequireFromString("1200")),
		TakerBuyBaseVolume:  decimal.NewNullDecimal(decimal.RequireFromString("5")),
		TakerBuyQuoteVolume: decimal.NewNullDecimal(decimal.RequireFromString("600")),
	}
}

type ingestionApplicationUnderTest struct {
	application             *application.KCandleIngestionApplication
	kCandleRepository       *mocks.MockIKCandleRepository
	tradingSymbolRepository *mocks.MockITradingSymbolRepository
	marketDataProxy         *mocks.MockIMarketDataProxy
}

func newIngestionApplicationUnderTest(t *testing.T) ingestionApplicationUnderTest {
	t.Helper()

	mockController := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	marketDataProxy := mocks.NewMockIMarketDataProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(ingestionAt(9, 7)).AnyTimes()

	return ingestionApplicationUnderTest{
		application: application.NewKCandleIngestionApplication(
			service.NewKCandleIngestionService(
				kCandleRepository, tradingSymbolRepository, marketDataProxy, clockProxy,
				ingestionMarketCatalog(), ingestionRoundCandleCount, ingestionLookback)),
		kCandleRepository:       kCandleRepository,
		tradingSymbolRepository: tradingSymbolRepository,
		marketDataProxy:         marketDataProxy,
	}
}

// ingestionMarketCatalog is the markets these tests are written against.
func ingestionMarketCatalog() domains.MarketCatalogDomain {
	return domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
		vo.MarketCrypto: {},
	})
}

// watchingCrypto is the watchlist a run reads at its top.
func watchingCrypto(
	tradingSymbolRepository *mocks.MockITradingSymbolRepository, symbols ...string,
) {
	watchedSymbols := make([]entities.TradingSymbol, 0, len(symbols))
	for _, symbol := range symbols {
		watchedSymbols = append(watchedSymbols, entities.TradingSymbol{
			Symbol: symbol, Market: string(vo.MarketCrypto), IsWatched: true,
		})
	}

	tradingSymbolRepository.EXPECT().
		FindWatched(gomock.Any()).Return(watchedSymbols, nil).AnyTimes()
}

func TestKCandleIngestionApplicationRunsAScheduledRound(t *testing.T) {
	underTest := newIngestionApplicationUnderTest(t)
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).Return([]vo.MarketKCandleVo{
		reportedKCandleAt(ingestionAt(8, 50)),
		reportedKCandleAt(ingestionAt(8, 55)),
		reportedKCandleAt(ingestionAt(9, 0)),
	}, nil)
	underTest.kCandleRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		Return(entities.KCandle{}, nil).Times(3)

	watchingCrypto(underTest.tradingSymbolRepository, "BTCUSDT")

	report, runError := underTest.application.RunScheduledRound(t.Context())

	require.NoError(t, runError)
	require.Len(t, report.SymbolReports, 1)
	assert.Equal(t, "BTCUSDT", report.SymbolReports[0].Symbol)
	assert.Equal(t, 3, report.SymbolReports[0].StoredCount)
}

func TestKCandleIngestionApplicationRunsTheBackfill(t *testing.T) {
	underTest := newIngestionApplicationUnderTest(t)
	underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).
		Return([]entities.KCandle{{Symbol: "BTCUSDT", OpenTime: ingestionAt(8, 30)}}, nil)
	underTest.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), vo.NewKCandleFetchWindowVo("BTCUSDT", vo.MarketCrypto, ingestionAt(8, 31), ingestionAt(9, 6))).
		Return([]vo.MarketKCandleVo{
			reportedKCandleAt(ingestionAt(8, 35)),
			reportedKCandleAt(ingestionAt(8, 40)),
		}, nil)
	underTest.kCandleRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		Return(entities.KCandle{}, nil).Times(2)

	watchingCrypto(underTest.tradingSymbolRepository, "BTCUSDT")

	report, runError := underTest.application.RunBackfill(t.Context())

	require.NoError(t, runError)
	require.Len(t, report.SymbolReports, 1)
	assert.Equal(t, 2, report.SymbolReports[0].StoredCount)
}
