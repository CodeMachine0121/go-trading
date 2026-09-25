package application_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
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

// The strategy scripts the contract calculation may run, one per kind under test.
const (
	ownSpotStrategyScriptID              = uint(20)
	ownContractStrategyScriptID          = uint(21)
	strangersUnpublishedContractScriptID = uint(22)
	unrecognisedKindStrategyScriptID     = uint(23)
)

type contractIndicatorUnderTest struct {
	indicatorCalculationApplication *application.IndicatorCalculationApplication
	kCandleRepository               *mocks.MockIKCandleRepository
	indicatorScriptProxy            *mocks.MockIIndicatorScriptProxy
	kCandleContractRepository       *mocks.MockIKCandleContractRepository
	contractIndicatorScriptProxy    *mocks.MockIContractIndicatorScriptProxy
}

// newContractIndicatorUnderTest mocks only storage, the clock and the two script runners.
func newContractIndicatorUnderTest(t *testing.T) contractIndicatorUnderTest {
	controller := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(indicatorNow).AnyTimes()
	marketCatalog := domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}})

	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(controller)
	for _, strategyScript := range []entities.StrategyScript{
		{ID: ownSpotStrategyScriptID, OwnerID: indicatorViewerID, Script: "the spot script", MarketDataKind: "kCandle"},
		{ID: ownContractStrategyScriptID, OwnerID: indicatorViewerID, Script: "the contract script",
			ResultType: "signal", MarketDataKind: "contractKCandle"},
		{ID: strangersUnpublishedContractScriptID, OwnerID: indicatorViewerID + 1, Script: "somebody else's",
			MarketDataKind: "contractKCandle"},
		{ID: unrecognisedKindStrategyScriptID, OwnerID: indicatorViewerID, Script: "a stored oddity",
			MarketDataKind: "選擇權"},
	} {
		strategyScriptRepository.EXPECT().FindOne(gomock.Any(), strategyScript.ID).Return(strategyScript, nil).AnyTimes()
	}
	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(controller)
	publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished).AnyTimes()

	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(controller)
	tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
		Return(entities.TradingSymbol{Market: string(vo.MarketCrypto)}, true, nil).AnyTimes()
	indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(controller)

	kCandleContractRepository := mocks.NewMockIKCandleContractRepository(controller)
	contractFundingRateSettlementRepository := mocks.NewMockIContractFundingRateSettlementRepository(controller)
	contractFundingRateSettlementRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).AnyTimes()
	contractFundingRateSettlementRepository.EXPECT().FindLatestBefore(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entities.ContractFundingRateSettlement{}, false, nil).AnyTimes()
	contractPositionStatisticRepository := mocks.NewMockIContractPositionStatisticRepository(controller)
	contractPositionStatisticRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).AnyTimes()
	contractIndicatorScriptProxy := mocks.NewMockIContractIndicatorScriptProxy(controller)

	return contractIndicatorUnderTest{
		indicatorCalculationApplication: application.NewIndicatorCalculationApplication(
			service.NewStrategyScriptService(strategyScriptRepository, publishedStrategyScriptRepository),
			service.NewIndicatorCalculationService(
				kCandleRepository, tradingSymbolRepository, indicatorScriptProxy, clockProxy, marketCatalog,
				queryMaxResults),
			service.NewContractIndicatorCalculationService(
				kCandleContractRepository, contractFundingRateSettlementRepository, contractPositionStatisticRepository,
				contractIndicatorScriptProxy, clockProxy, marketCatalog, queryMaxResults)),
		kCandleRepository:            kCandleRepository,
		indicatorScriptProxy:         indicatorScriptProxy,
		kCandleContractRepository:    kCandleContractRepository,
		contractIndicatorScriptProxy: contractIndicatorScriptProxy,
	}
}

// storedContractCandlesAt are complete contract K candles at those moments.
func storedContractCandlesAt(openTimes ...time.Time) []entities.KCandleContract {
	kCandleContracts := make([]entities.KCandleContract, 0, len(openTimes))
	for _, openTime := range openTimes {
		figure := decimal.RequireFromString("100")
		kCandleContracts = append(kCandleContracts, entities.KCandleContract{
			Symbol: "BTCUSDT", OpenTime: openTime, Open: figure, High: figure, Low: figure, Close: figure,
			MarkOpen: figure, MarkHigh: figure, MarkLow: figure, MarkClose: figure,
		})
	}

	return kCandleContracts
}

func TestContractIndicatorCalculationRunsANamedContractScriptOverContractBars(t *testing.T) {
	fixture := newContractIndicatorUnderTest(t)
	fixture.kCandleContractRepository.EXPECT().
		FindLatestBefore(gomock.Any(), "BTCUSDT", indicatorCutoff, gomock.Any()).
		Return(storedContractCandlesAt(at(9, 14), at(9, 13)), nil)
	fixture.contractIndicatorScriptProxy.EXPECT().
		Execute(gomock.Any(), "the contract script", gomock.Any(), gomock.Len(2), gomock.Any()).
		Return(map[string]vo.IndicatorValueVo{vo.SignalIndicatorKey: {Signal: vo.SignalSell}}, nil)

	resultDto, err := fixture.indicatorCalculationApplication.CalculateContractIndicator(
		t.Context(), indicatorViewerID, namingStrategyScript(t, ownContractStrategyScriptID), indicatorRequest(2))

	require.NoError(t, err)
	assert.Equal(t, "sell", resultDto.Signal)
	assert.Equal(t, 2, resultDto.RequiredCandleCount)
	assert.Equal(t, 2, resultDto.UsedCandleCount)
}

func TestContractIndicatorCalculationRunsTheCallersOwnAlgorithmAsAContractScript(t *testing.T) {
	fixture := newContractIndicatorUnderTest(t)
	fixture.kCandleContractRepository.EXPECT().
		FindLatestBefore(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(storedContractCandlesAt(at(9, 14)), nil)
	fixture.contractIndicatorScriptProxy.EXPECT().
		Execute(gomock.Any(), "my own contract script", gomock.Any(), gomock.Any(), gomock.Any()).
		Return(map[string]vo.IndicatorValueVo{"funding": {Numbers: []float64{0.0001}}}, nil)

	resultDto, err := fixture.indicatorCalculationApplication.CalculateContractIndicator(
		t.Context(), indicatorViewerID, carrying(t, "my own contract script"), indicatorRequest(1))

	require.NoError(t, err)
	assert.Equal(t, []float64{0.0001}, resultDto.Values["funding"].Numbers)
}

func TestIndicatorCalculationRefusesAScriptNamedForTheOtherKindOfMarket(t *testing.T) {
	// Nothing is stubbed on storage or either script runner: a refusal that still
	// read the market or ran the script would fail here.
	t.Run("a spot script named for a contract calculation", func(t *testing.T) {
		fixture := newContractIndicatorUnderTest(t)

		_, err := fixture.indicatorCalculationApplication.CalculateContractIndicator(
			t.Context(), indicatorViewerID, namingStrategyScript(t, ownSpotStrategyScriptID), indicatorRequest(2))

		require.ErrorIs(t, err, domains.ErrStrategyScriptMarketDataKindMismatch)
		assert.Contains(t, err.Error(), "這支策略腳本吃的是 K 線")
	})

	t.Run("a contract script named for a spot calculation", func(t *testing.T) {
		fixture := newContractIndicatorUnderTest(t)

		_, err := fixture.indicatorCalculationApplication.CalculateIndicator(
			t.Context(), indicatorViewerID, namingStrategyScript(t, ownContractStrategyScriptID), indicatorRequest(2))

		require.ErrorIs(t, err, domains.ErrStrategyScriptMarketDataKindMismatch)
		assert.Contains(t, err.Error(), "這支策略腳本吃的是合約行情")
	})
}

func TestIndicatorCalculationStillRunsASpotScriptOverSpotCandles(t *testing.T) {
	fixture := newContractIndicatorUnderTest(t)
	fixture.kCandleRepository.EXPECT().
		FindLatestBefore(gomock.Any(), "BTCUSDT", indicatorCutoff, gomock.Any()).
		Return([]entities.KCandle{kCandleAt(at(9, 14), "100")}, nil)
	fixture.indicatorScriptProxy.EXPECT().
		Execute(gomock.Any(), "the spot script", gomock.Any(), gomock.Any(), gomock.Any()).
		Return(map[string]vo.IndicatorValueVo{"ma": {Numbers: []float64{100}}}, nil)

	resultDto, err := fixture.indicatorCalculationApplication.CalculateIndicator(
		t.Context(), indicatorViewerID, namingStrategyScript(t, ownSpotStrategyScriptID), indicatorRequest(1))

	require.NoError(t, err)
	assert.Equal(t, []float64{100}, resultDto.Values["ma"].Numbers)
}

func TestContractIndicatorCalculationAnswersAStrangersUnpublishedScriptAsNotThere(t *testing.T) {
	fixture := newContractIndicatorUnderTest(t)

	_, err := fixture.indicatorCalculationApplication.CalculateContractIndicator(
		t.Context(), indicatorViewerID, namingStrategyScript(t, strangersUnpublishedContractScriptID),
		dto.IndicatorCalculationRequestDto{Symbol: "BTCUSDT", StartTime: indicatorNow.Add(-time.Minute)})

	require.ErrorIs(t, err, domains.ErrStrategyScriptNotFound)
	assert.NotErrorIs(t, err, domains.ErrStrategyScriptMarketDataKindMismatch)
}

func TestContractIndicatorCalculationRefusesToGuessAStoredKindOfMarketItDoesNotKnow(t *testing.T) {
	fixture := newContractIndicatorUnderTest(t)

	_, err := fixture.indicatorCalculationApplication.CalculateContractIndicator(
		t.Context(), indicatorViewerID, namingStrategyScript(t, unrecognisedKindStrategyScriptID), indicatorRequest(1))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "行情種類只能是")
}
