package persistence_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

// The old reads are rebuilt here from the domain's own rules, so the summarized reads are checked against them on the same stored market.

var readsNow = time.Date(2026, 10, 21, 10, 20, 0, 0, time.UTC)

const readsMaxCandleCount = 1000

// readsSeedBatchSize keeps each insert under PostgreSQL's parameter limit.
const readsSeedBatchSize = 2000

func readsMarketCatalog() domains.MarketCatalogDomain {
	return domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
		vo.MarketCrypto: {},
		vo.MarketTaiwanStock: {
			TradingSession: vo.TradingSessionVo{
				Location:   time.FixedZone("Asia/Taipei", 8*60*60),
				DailyStart: 9 * time.Hour,
				DailyEnd:   13*time.Hour + 30*time.Minute,
				Weekdays:   []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday},
			},
		},
	})
}

func readsFigure(random *rand.Rand, base int64) decimal.Decimal {
	return decimal.New(base*1000+random.Int64N(1000), -2)
}

func readsOptionalFigure(random *rand.Rand, base int64) decimal.NullDecimal {
	if random.IntN(20) == 0 {
		return decimal.NullDecimal{}
	}

	return decimal.NewNullDecimal(readsFigure(random, base))
}

// seededMinutes lists every minute from start up to end that isTraded keeps, minus a few dropped at random.
func seededMinutes(start time.Time, end time.Time, isTraded func(time.Time) bool, random *rand.Rand) []time.Time {
	minutes := make([]time.Time, 0)
	for minute := start; minute.Before(end); minute = minute.Add(time.Minute) {
		if isTraded(minute) && random.IntN(30) != 0 {
			minutes = append(minutes, minute)
		}
	}

	return minutes
}

func seedContractCandles(t testing.TB, database *gorm.DB, symbol string, minutes []time.Time, random *rand.Rand) {
	t.Helper()
	repository := persistence.NewKCandleContractRepository(database)
	batch := make([]entities.KCandleContract, 0, readsSeedBatchSize)
	flush := func() {
		_, saveError := repository.SaveAllIfAbsent(t.Context(), batch)
		require.NoError(t, saveError)
		batch = batch[:0]
	}
	for _, minute := range minutes {
		low := readsFigure(random, 90)
		high := readsFigure(random, 110)
		batch = append(batch, entities.KCandleContract{
			Symbol: symbol, OpenTime: minute,
			Open: readsFigure(random, 100), High: high, Low: low, Close: readsFigure(random, 100),
			Volume: readsFigure(random, 5), QuoteVolume: readsFigure(random, 500),
			TakerBuyBaseVolume: readsFigure(random, 2), TakerBuyQuoteVolume: readsFigure(random, 200),
			TradeCount: random.Int64N(500),
			MarkOpen:   readsFigure(random, 100), MarkHigh: high, MarkLow: low, MarkClose: readsFigure(random, 100),
			IndexOpen: readsOptionalFigure(random, 100), IndexHigh: readsOptionalFigure(random, 110),
			IndexLow: readsOptionalFigure(random, 90), IndexClose: readsOptionalFigure(random, 100),
			PremiumIndexOpen: readsOptionalFigure(random, 0), PremiumIndexHigh: readsOptionalFigure(random, 1),
			PremiumIndexLow: readsOptionalFigure(random, 0), PremiumIndexClose: readsOptionalFigure(random, 0),
		})
		if len(batch) == readsSeedBatchSize {
			flush()
		}
	}
	if len(batch) > 0 {
		flush()
	}
}

func seedSpotCandles(t testing.TB, database *gorm.DB, symbol string, minutes []time.Time, random *rand.Rand, reportsVolumes bool) {
	t.Helper()
	repository := persistence.NewKCandleRepository(database)
	batch := make([]entities.KCandle, 0, readsSeedBatchSize)
	flush := func() {
		_, saveError := repository.SaveAllIfAbsent(t.Context(), batch)
		require.NoError(t, saveError)
		batch = batch[:0]
	}
	for _, minute := range minutes {
		kCandle := entities.KCandle{
			Symbol: symbol, OpenTime: minute,
			Open: readsFigure(random, 100), High: readsFigure(random, 110), Low: readsFigure(random, 90),
			Close: readsFigure(random, 100), Volume: readsFigure(random, 5),
		}
		if reportsVolumes {
			kCandle.QuoteVolume = readsOptionalFigure(random, 500)
			kCandle.TakerBuyBaseVolume = readsOptionalFigure(random, 2)
			kCandle.TakerBuyQuoteVolume = readsOptionalFigure(random, 200)
		}
		batch = append(batch, kCandle)
		if len(batch) == readsSeedBatchSize {
			flush()
		}
	}
	if len(batch) > 0 {
		flush()
	}
}

func seedPositioningAndFunding(
	t testing.TB, database *gorm.DB, symbol string, start time.Time, end time.Time,
	isRecorded func(time.Time) bool, random *rand.Rand,
) {
	t.Helper()
	statistics := make([]entities.ContractPositionStatistic, 0)
	for moment := start; moment.Before(end); moment = moment.Add(domains.ContractPositionStatisticInterval) {
		if !isRecorded(moment) || random.IntN(15) == 0 {
			continue
		}
		statistics = append(statistics, entities.ContractPositionStatistic{
			Symbol: symbol, StatisticTime: moment,
			OpenInterest: readsFigure(random, 5000), OpenInterestValue: readsFigure(random, 500000),
			AccountLongShare: readsFigure(random, 0), AccountShortShare: readsFigure(random, 0),
			AccountLongShortRatio: readsFigure(random, 1), TopTraderPositionLongShare: readsFigure(random, 0),
			TopTraderPositionShortShare: readsFigure(random, 0), TopTraderPositionLongShortRatio: readsFigure(random, 1),
		})
	}
	_, statisticsError := persistence.NewContractPositionStatisticRepository(database).SaveAllIfAbsent(t.Context(), statistics)
	require.NoError(t, statisticsError)

	settlements := make([]entities.ContractFundingRateSettlement, 0)
	for moment := start.Add(-8 * time.Hour); moment.Before(end); moment = moment.Add(8 * time.Hour) {
		settlements = append(settlements, entities.ContractFundingRateSettlement{
			Symbol: symbol, SettlementTime: moment.Add(time.Millisecond), FundingRate: decimal.New(random.Int64N(20)-10, -5),
		})
	}
	_, settlementsError := persistence.NewContractFundingRateSettlementRepository(database).
		SaveAllIfAbsent(t.Context(), settlements)
	require.NoError(t, settlementsError)
}

type contractReadsUnderTest struct {
	database           *gorm.DB
	calculationService *service.ContractIndicatorCalculationService
	handedBars         *[]vo.ContractKCandleVo
}

func newContractReadsUnderTest(t testing.TB, database *gorm.DB, now time.Time) contractReadsUnderTest {
	controller := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(now).AnyTimes()
	handedBars := &[]vo.ContractKCandleVo{}
	scriptProxy := mocks.NewMockIContractIndicatorScriptProxy(controller)
	scriptProxy.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ string, _ domains.IndicatorResultTypeDomain,
			bars []vo.ContractKCandleVo, _ domains.StrategyScriptParametersDomain,
		) (map[string]vo.IndicatorValueVo, error) {
			*handedBars = bars
			return map[string]vo.IndicatorValueVo{}, nil
		}).AnyTimes()

	return contractReadsUnderTest{
		database: database,
		calculationService: service.NewContractIndicatorCalculationService(
			persistence.NewKCandleContractRepository(database),
			persistence.NewKCandleContractSummaryDayRepository(database),
			persistence.NewContractFundingRateSettlementRepository(database),
			persistence.NewContractPositionStatisticRepository(database),
			scriptProxy,
			service.NewContractOrderFlowService(mocks.NewMockIContractTradeHistoryProxy(controller), clockProxy,
				service.NewContractOrderFlowMemory(1<<20)),
			clockProxy, readsMarketCatalog(), readsMaxCandleCount),
		handedBars: handedBars,
	}
}

// barsFromSummaries is the calculation as it now reads.
func (underTest contractReadsUnderTest) barsFromSummaries(
	executionContext context.Context, requestDto dto.IndicatorCalculationRequestDto,
) ([]vo.ContractKCandleVo, error) {
	*underTest.handedBars = nil
	_, calculationError := underTest.calculationService.CalculateContractIndicator(executionContext, requestDto)

	return *underTest.handedBars, calculationError
}

// barsFromMinutes is the calculation as it read before: the latest minutes by count and every statistic over the stretch, one read after another.
func (underTest contractReadsUnderTest) barsFromMinutes(
	executionContext context.Context, requestDto dto.IndicatorCalculationRequestDto, now time.Time,
) ([]vo.ContractKCandleVo, error) {
	calculationDomain, validationError := domains.NewIndicatorCalculationDomain(
		requestDto, readsMarketCatalog().MarketOf(string(vo.MarketCrypto)), readsMaxCandleCount, now)
	if validationError != nil {
		return nil, validationError
	}
	newestFirst, findError := persistence.NewKCandleContractRepository(underTest.database).FindLatestBefore(
		executionContext, calculationDomain.Symbol(), calculationDomain.ReadCutoff(), calculationDomain.SourceCandleLimit())
	if findError != nil {
		return nil, findError
	}
	alignment, selectionError := calculationDomain.SelectContractInput(newestFirst)
	if selectionError != nil {
		return nil, selectionError
	}
	settlementRepository := persistence.NewContractFundingRateSettlementRepository(underTest.database)
	settlements, settlementsError := settlementRepository.FindInRange(
		executionContext, alignment.SettlementQuery(), alignment.SettlementReadLimit())
	if settlementsError != nil {
		return nil, settlementsError
	}
	leadIn, hasLeadIn, leadInError := settlementRepository.FindLatestBefore(
		executionContext, calculationDomain.Symbol(), alignment.SettlementLeadInCutoff())
	if leadInError != nil {
		return nil, leadInError
	}
	if hasLeadIn {
		settlements = append(settlements, leadIn)
	}
	statistics, statisticsError := persistence.NewContractPositionStatisticRepository(underTest.database).FindInRange(
		executionContext, alignment.StatisticQuery(), alignment.StatisticReadLimit())
	if statisticsError != nil {
		return nil, statisticsError
	}

	return alignment.Aligning(settlements, statistics), nil
}

type readsCase struct {
	name          string
	symbol        string
	interval      string
	startTime     time.Time
	endTime       time.Time
	lookbackCount float64
}

func (readsCase readsCase) request() dto.IndicatorCalculationRequestDto {
	requestDto := dto.IndicatorCalculationRequestDto{
		Symbol: readsCase.symbol, AggregationInterval: readsCase.interval,
		StartTime: readsCase.startTime, EndTime: readsCase.endTime, Script: "the script",
	}
	if readsCase.lookbackCount > 0 {
		requestDto.Parameters = []dto.StrategyScriptParameterWriteDto{
			{Name: "期數", Kind: "lookbackCount", DefaultValue: readsCase.lookbackCount}}
	}

	return requestDto
}

func onDay(month time.Month, day int, hour int, minute int) time.Time {
	return time.Date(2026, month, day, hour, minute, 0, 0, time.UTC)
}

func TestContractIndicatorCalculationReadsTheSameBarsFromSummariesAsFromMinutes(t *testing.T) {
	database := newTestDatabase(t)
	random := rand.New(rand.NewPCG(7, 11))
	historyStart := onDay(9, 28, 13, 37)
	haltStart, haltEnd := onDay(10, 5, 0, 0), onDay(10, 9, 6, 0)
	isTraded := func(minute time.Time) bool { return minute.Before(haltStart) || !minute.Before(haltEnd) }
	seedContractCandles(t, database, "BTCUSDT", seededMinutes(historyStart, readsNow, isTraded, random), random)
	statisticsStop, statisticsResume := onDay(10, 15, 12, 2), onDay(10, 16, 0, 0)
	seedPositioningAndFunding(t, database, "BTCUSDT", onDay(10, 1, 0, 0), readsNow, func(moment time.Time) bool {
		return isTraded(moment) && (moment.Before(statisticsStop) || !moment.Before(statisticsResume))
	}, random)
	underTest := newContractReadsUnderTest(t, database, readsNow)

	readsCases := []readsCase{
		{name: "ten days up to now", symbol: "BTCUSDT", interval: "1d", startTime: onDay(10, 11, 0, 0), endTime: readsNow, lookbackCount: 3},
		{name: "a week ending on a day's edge", symbol: "BTCUSDT", interval: "1d", startTime: onDay(10, 13, 0, 0), endTime: onDay(10, 20, 0, 0)},
		{name: "more days than there is history", symbol: "BTCUSDT", interval: "1d", startTime: onDay(8, 1, 0, 0), endTime: readsNow},
		{name: "reaching back over the halt", symbol: "BTCUSDT", interval: "1d", startTime: onDay(10, 6, 0, 0), endTime: onDay(10, 12, 0, 0), lookbackCount: 5},
		{name: "a look-back history cannot fill", symbol: "BTCUSDT", interval: "1d", startTime: onDay(10, 19, 0, 0), endTime: readsNow, lookbackCount: 40},
		{name: "four hours ending mid-bucket in the past", symbol: "BTCUSDT", interval: "4h", startTime: onDay(10, 10, 0, 0), endTime: onDay(10, 12, 13, 0)},
		{name: "four hours up to now", symbol: "BTCUSDT", interval: "4h", startTime: onDay(10, 18, 0, 0), endTime: readsNow, lookbackCount: 6},
		{name: "hours across the halt", symbol: "BTCUSDT", interval: "1h", startTime: onDay(10, 9, 0, 0), endTime: onDay(10, 9, 12, 30), lookbackCount: 20},
		{name: "hours over the day statistics stopped", symbol: "BTCUSDT", interval: "1h", startTime: onDay(10, 15, 8, 0), endTime: onDay(10, 16, 2, 0)},
		{name: "hours up to now", symbol: "BTCUSDT", interval: "1h", startTime: onDay(10, 20, 20, 0), endTime: readsNow},
		{name: "fifteen minutes keep reading minutes", symbol: "BTCUSDT", interval: "15m", startTime: onDay(10, 21, 6, 0), endTime: readsNow},
		{name: "a symbol never stored", symbol: "ETHUSDT", interval: "1d", startTime: onDay(10, 11, 0, 0), endTime: readsNow},
	}

	for _, readsCase := range readsCases {
		t.Run(readsCase.name, func(t *testing.T) {
			expectedBars, expectedError := underTest.barsFromMinutes(t.Context(), readsCase.request(), readsNow)

			for _, pass := range []string{"summarizing on the way", "from stored summaries"} {
				bars, readError := underTest.barsFromSummaries(t.Context(), readsCase.request())

				if expectedError != nil {
					require.Error(t, readError, pass)
					assert.Equal(t, expectedError.Error(), readError.Error(), pass)
					continue
				}
				require.NoError(t, readError, pass)
				require.NotEmpty(t, expectedBars)
				assert.Equal(t, expectedBars, bars, pass)
			}
		})
	}
}

type spotReadsUnderTest struct {
	database           *gorm.DB
	calculationService *service.IndicatorCalculationService
	handedCandles      *[]vo.KCandleVo
}

func newSpotReadsUnderTest(t testing.TB, database *gorm.DB, now time.Time) spotReadsUnderTest {
	controller := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(now).AnyTimes()
	handedCandles := &[]vo.KCandleVo{}
	scriptProxy := mocks.NewMockIIndicatorScriptProxy(controller)
	scriptProxy.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ string, _ domains.IndicatorResultTypeDomain,
			kCandleVos []vo.KCandleVo, _ domains.StrategyScriptParametersDomain,
		) (map[string]vo.IndicatorValueVo, error) {
			*handedCandles = kCandleVos
			return map[string]vo.IndicatorValueVo{}, nil
		}).AnyTimes()

	return spotReadsUnderTest{
		database: database,
		calculationService: service.NewIndicatorCalculationService(
			persistence.NewKCandleRepository(database), persistence.NewKCandleSummaryDayRepository(database),
			persistence.NewTradingSymbolRepository(database), scriptProxy, clockProxy,
			readsMarketCatalog(), readsMaxCandleCount),
		handedCandles: handedCandles,
	}
}

func (underTest spotReadsUnderTest) candlesFromSummaries(
	executionContext context.Context, requestDto dto.IndicatorCalculationRequestDto,
) ([]vo.KCandleVo, error) {
	*underTest.handedCandles = nil
	_, calculationError := underTest.calculationService.CalculateIndicator(executionContext, requestDto)

	return *underTest.handedCandles, calculationError
}

func (underTest spotReadsUnderTest) candlesFromMinutes(
	executionContext context.Context, requestDto dto.IndicatorCalculationRequestDto, market vo.MarketVo, now time.Time,
) ([]vo.KCandleVo, error) {
	calculationDomain, validationError := domains.NewIndicatorCalculationDomain(
		requestDto, readsMarketCatalog().MarketOf(string(market)), readsMaxCandleCount, now)
	if validationError != nil {
		return nil, validationError
	}
	newestFirst, findError := persistence.NewKCandleRepository(underTest.database).FindLatestBefore(
		executionContext, calculationDomain.Symbol(), calculationDomain.ReadCutoff(), calculationDomain.SourceCandleLimit())
	if findError != nil {
		return nil, findError
	}

	return calculationDomain.SelectInputCandles(newestFirst)
}

func TestSpotIndicatorCalculationReadsTheSameCandlesFromSummariesAsFromMinutes(t *testing.T) {
	database := newTestDatabase(t)
	random := rand.New(rand.NewPCG(3, 5))
	taiwanSession := readsMarketCatalog().MarketOf(string(vo.MarketTaiwanStock))
	seedSpotCandles(t, database, "2330",
		seededMinutes(onDay(8, 3, 1, 0), readsNow, taiwanSession.IsOpen, random), random, false)
	seedSpotCandles(t, database, "BTCUSDT",
		seededMinutes(onDay(10, 2, 7, 45), readsNow, func(time.Time) bool { return true }, random), random, true)
	require.NoError(t, persistence.NewTradingSymbolRepository(database).Save(t.Context(),
		entities.TradingSymbol{Symbol: "2330", Market: string(vo.MarketTaiwanStock)}))
	underTest := newSpotReadsUnderTest(t, database, readsNow)

	readsCases := []struct {
		readsCase
		market vo.MarketVo
	}{
		{readsCase{name: "trading days of a market that closes", symbol: "2330", interval: "1d",
			startTime: onDay(9, 21, 0, 0), endTime: readsNow, lookbackCount: 5}, vo.MarketTaiwanStock},
		{readsCase{name: "hours of a market that closes", symbol: "2330", interval: "1h",
			startTime: onDay(10, 13, 0, 0), endTime: readsNow, lookbackCount: 10}, vo.MarketTaiwanStock},
		{readsCase{name: "four hours of a market that closes", symbol: "2330", interval: "4h",
			startTime: onDay(10, 1, 0, 0), endTime: onDay(10, 17, 3, 0)}, vo.MarketTaiwanStock},
		{readsCase{name: "more trading days than there is history", symbol: "2330", interval: "1d",
			startTime: onDay(1, 5, 0, 0), endTime: readsNow}, vo.MarketTaiwanStock},
		{readsCase{name: "days of a market that never closes", symbol: "BTCUSDT", interval: "1d",
			startTime: onDay(10, 6, 0, 0), endTime: readsNow, lookbackCount: 3}, vo.MarketCrypto},
		{readsCase{name: "days starting with history's partial first day", symbol: "BTCUSDT", interval: "1d",
			startTime: onDay(9, 1, 0, 0), endTime: onDay(10, 20, 0, 0)}, vo.MarketCrypto},
		{readsCase{name: "four hours of a market that never closes", symbol: "BTCUSDT", interval: "4h",
			startTime: onDay(10, 19, 0, 0), endTime: readsNow}, vo.MarketCrypto},
	}

	for _, readsCase := range readsCases {
		t.Run(readsCase.name, func(t *testing.T) {
			expectedCandles, expectedError := underTest.candlesFromMinutes(
				t.Context(), readsCase.request(), readsCase.market, readsNow)

			for _, pass := range []string{"summarizing on the way", "from stored summaries"} {
				kCandleVos, readError := underTest.candlesFromSummaries(t.Context(), readsCase.request())

				if expectedError != nil {
					require.Error(t, readError, pass)
					assert.Equal(t, expectedError.Error(), readError.Error(), pass)
					continue
				}
				require.NoError(t, readError, pass)
				require.NotEmpty(t, expectedCandles)
				assert.Equal(t, expectedCandles, kCandleVos, pass)
			}
		})
	}
}

// countingReadRows adds every row a query brings back to the counter.
func countingReadRows(t testing.TB, database *gorm.DB, readRows *int64) {
	t.Helper()
	require.NoError(t, database.Callback().Query().After("gorm:query").
		Register(fmt.Sprintf("count_read_rows_%p", readRows), func(statement *gorm.DB) {
			*readRows += statement.Statement.RowsAffected
		}))
}

// BenchmarkContractIndicatorCalculationReads measures a daily contract calculation over about three months, as a chart asks for it.
func BenchmarkContractIndicatorCalculationReads(b *testing.B) {
	database := newTestDatabase(b)
	random := rand.New(rand.NewPCG(13, 17))
	historyStart := readsNow.Truncate(24*time.Hour).AddDate(0, 0, -100)
	seedContractCandles(b, database, "BTCUSDT",
		seededMinutes(historyStart, readsNow, func(time.Time) bool { return true }, random), random)
	seedPositioningAndFunding(b, database, "BTCUSDT", historyStart, readsNow, func(time.Time) bool { return true }, random)
	underTest := newContractReadsUnderTest(b, database, readsNow)
	readRows := int64(0)
	countingReadRows(b, database, &readRows)
	requestDto := readsCase{symbol: "BTCUSDT", interval: "1d",
		startTime: readsNow.Truncate(24*time.Hour).AddDate(0, 0, -95), endTime: readsNow}.request()

	coldStart := time.Now()
	_, coldError := underTest.barsFromSummaries(b.Context(), requestDto)
	require.NoError(b, coldError)
	b.Logf("summarizing every day on the way, once: %s", time.Since(coldStart))

	b.Run("minutes", func(b *testing.B) {
		readRows = 0
		for b.Loop() {
			_, readError := underTest.barsFromMinutes(b.Context(), requestDto, readsNow)
			require.NoError(b, readError)
		}
		b.ReportMetric(float64(readRows)/float64(b.N), "rows/op")
	})
	b.Run("summaries", func(b *testing.B) {
		readRows = 0
		for b.Loop() {
			_, readError := underTest.barsFromSummaries(b.Context(), requestDto)
			require.NoError(b, readError)
		}
		b.ReportMetric(float64(readRows)/float64(b.N), "rows/op")
	})
}
