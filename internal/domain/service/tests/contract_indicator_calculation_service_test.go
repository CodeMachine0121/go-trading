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
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var contractCalculationNow = time.Date(2026, 8, 29, 23, 0, 0, 0, time.UTC)

func onTheDay(hour int, minute int) time.Time {
	return time.Date(2026, 8, 29, hour, minute, 0, 0, time.UTC)
}

// aStoredContractCandle has ordinary figures; each test changes only what its outcome depends on.
func aStoredContractCandle(openTime time.Time) entities.KCandleContract {
	figure := decimal.RequireFromString("100")
	return entities.KCandleContract{
		Symbol: "BTCUSDT", OpenTime: openTime,
		Open: figure, High: figure, Low: figure, Close: figure,
		Volume: decimal.RequireFromString("1"), QuoteVolume: decimal.RequireFromString("100"),
		TakerBuyBaseVolume: decimal.RequireFromString("0.5"), TakerBuyQuoteVolume: decimal.RequireFromString("50"),
		TradeCount: 200,
		MarkOpen:   figure, MarkHigh: figure, MarkLow: figure, MarkClose: figure,
		IndexOpen: decimal.NewNullDecimal(figure), IndexHigh: decimal.NewNullDecimal(figure),
		IndexLow: decimal.NewNullDecimal(figure), IndexClose: decimal.NewNullDecimal(figure),
		PremiumIndexOpen:  decimal.NewNullDecimal(decimal.RequireFromString("0.0001")),
		PremiumIndexHigh:  decimal.NewNullDecimal(decimal.RequireFromString("0.0001")),
		PremiumIndexLow:   decimal.NewNullDecimal(decimal.RequireFromString("0.0001")),
		PremiumIndexClose: decimal.NewNullDecimal(decimal.RequireFromString("0.0001")),
	}
}

// oneCandleAtEachHour stores one candle per listed hour, so each hour is its own one-hour bar.
func oneCandleAtEachHour(hours ...int) []entities.KCandleContract {
	kCandleContracts := make([]entities.KCandleContract, 0, len(hours))
	for _, hour := range hours {
		kCandleContracts = append(kCandleContracts, aStoredContractCandle(onTheDay(hour, 0)))
	}

	return kCandleContracts
}

func aSettlement(settlementTime time.Time, fundingRate string) entities.ContractFundingRateSettlement {
	return entities.ContractFundingRateSettlement{
		Symbol: "BTCUSDT", SettlementTime: settlementTime, FundingRate: decimal.RequireFromString(fundingRate),
	}
}

func aStatistic(statisticTime time.Time, openInterest string) entities.ContractPositionStatistic {
	return entities.ContractPositionStatistic{
		Symbol: "BTCUSDT", StatisticTime: statisticTime, OpenInterest: decimal.RequireFromString(openInterest),
	}
}

func hourlyRequest(startHour int, endHour int) dto.IndicatorCalculationRequestDto {
	return dto.IndicatorCalculationRequestDto{
		Symbol: "BTCUSDT", AggregationInterval: "1h",
		StartTime: onTheDay(startHour, 0), EndTime: onTheDay(endHour, 0), Script: "the script",
	}
}

type contractCalculationUnderTest struct {
	contractIndicatorCalculationService     *service.ContractIndicatorCalculationService
	kCandleContractRepository               *mocks.MockIKCandleContractRepository
	contractFundingRateSettlementRepository *mocks.MockIContractFundingRateSettlementRepository
	contractPositionStatisticRepository     *mocks.MockIContractPositionStatisticRepository
	contractIndicatorScriptProxy            *mocks.MockIContractIndicatorScriptProxy
}

func newContractCalculationUnderTest(t *testing.T) contractCalculationUnderTest {
	controller := gomock.NewController(t)
	kCandleContractRepository := mocks.NewMockIKCandleContractRepository(controller)
	contractFundingRateSettlementRepository := mocks.NewMockIContractFundingRateSettlementRepository(controller)
	contractPositionStatisticRepository := mocks.NewMockIContractPositionStatisticRepository(controller)
	contractIndicatorScriptProxy := mocks.NewMockIContractIndicatorScriptProxy(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(contractCalculationNow).AnyTimes()

	return contractCalculationUnderTest{
		contractIndicatorCalculationService: service.NewContractIndicatorCalculationService(
			kCandleContractRepository, contractFundingRateSettlementRepository, contractPositionStatisticRepository,
			contractIndicatorScriptProxy, clockProxy,
			domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}), maxCandleCount),
		kCandleContractRepository:               kCandleContractRepository,
		contractFundingRateSettlementRepository: contractFundingRateSettlementRepository,
		contractPositionStatisticRepository:     contractPositionStatisticRepository,
		contractIndicatorScriptProxy:            contractIndicatorScriptProxy,
	}
}

// barsHandedToTheScript returns the bars the script received, where every alignment rule becomes visible.
func (fixture contractCalculationUnderTest) barsHandedToTheScript(
	t *testing.T,
	requestDto dto.IndicatorCalculationRequestDto,
	kCandleContracts []entities.KCandleContract,
	settlements []entities.ContractFundingRateSettlement,
	statistics []entities.ContractPositionStatistic,
) []vo.ContractKCandleVo {
	t.Helper()
	fixture.kCandleContractRepository.EXPECT().
		FindLatestBefore(gomock.Any(), "BTCUSDT", gomock.Any(), gomock.Any()).Return(kCandleContracts, nil)
	fixture.holdSettlements(settlements)
	fixture.contractPositionStatisticRepository.EXPECT().
		FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).Return(statistics, nil)
	handedBars := []vo.ContractKCandleVo{}
	fixture.contractIndicatorScriptProxy.EXPECT().
		Execute(gomock.Any(), "the script", gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ string, _ domains.IndicatorResultTypeDomain,
			contractKCandleVos []vo.ContractKCandleVo, _ domains.StrategyScriptParametersDomain,
		) (map[string]vo.IndicatorValueVo, error) {
			handedBars = contractKCandleVos
			return map[string]vo.IndicatorValueVo{}, nil
		})

	_, err := fixture.contractIndicatorCalculationService.CalculateContractIndicator(t.Context(), requestDto)
	require.NoError(t, err)

	return handedBars
}

// holdSettlements stubs the range read with the settlements inside the stretch and the lead-in read with the latest one before the cut-off.
func (fixture contractCalculationUnderTest) holdSettlements(settlements []entities.ContractFundingRateSettlement) {
	fixture.contractFundingRateSettlementRepository.EXPECT().
		FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, query domains.KCandleQueryDomain, _ int) ([]entities.ContractFundingRateSettlement, error) {
			inside := []entities.ContractFundingRateSettlement{}
			for _, settlement := range settlements {
				if !settlement.SettlementTime.Before(query.StartTime()) && !settlement.SettlementTime.After(query.EndTime()) {
					inside = append(inside, settlement)
				}
			}
			return inside, nil
		})
	fixture.contractFundingRateSettlementRepository.EXPECT().
		FindLatestBefore(gomock.Any(), "BTCUSDT", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, cutoffTime time.Time) (entities.ContractFundingRateSettlement, bool, error) {
			latest, found := entities.ContractFundingRateSettlement{}, false
			for _, settlement := range settlements {
				if settlement.SettlementTime.Before(cutoffTime) && (!found || settlement.SettlementTime.After(latest.SettlementTime)) {
					latest, found = settlement, true
				}
			}
			return latest, found, nil
		})
}

func barOpeningAt(t *testing.T, bars []vo.ContractKCandleVo, openTime time.Time) vo.ContractKCandleVo {
	t.Helper()
	for _, bar := range bars {
		if bar.OpenTimeUnixSeconds == openTime.Unix() {
			return bar
		}
	}
	require.Failf(t, "no such bar", "the script saw no bar opening at %s", openTime)

	return vo.ContractKCandleVo{}
}

func TestContractCalculationMergesABucketOfContractCandlesIntoOneBar(t *testing.T) {
	sixtyMinutes := make([]entities.KCandleContract, 0, 60)
	for minute := range 60 {
		kCandleContract := aStoredContractCandle(onTheDay(9, minute))
		sixtyMinutes = append(sixtyMinutes, kCandleContract)
	}
	sixtyMinutes[0].Open = decimal.RequireFromString("100")
	sixtyMinutes[59].Close = decimal.RequireFromString("105")
	sixtyMinutes[30].MarkHigh = decimal.RequireFromString("106")
	sixtyMinutes[45].PremiumIndexLow = decimal.NewNullDecimal(decimal.RequireFromString("-0.0009"))
	sixtyMinutes[0].IndexOpen = decimal.NewNullDecimal(decimal.RequireFromString("99"))
	sixtyMinutes[20].IndexHigh = decimal.NewNullDecimal(decimal.RequireFromString("107"))
	sixtyMinutes[40].IndexLow = decimal.NewNullDecimal(decimal.RequireFromString("95"))
	sixtyMinutes[59].IndexClose = decimal.NewNullDecimal(decimal.RequireFromString("104"))

	bars := newContractCalculationUnderTest(t).barsHandedToTheScript(
		t, hourlyRequest(9, 10), sixtyMinutes, nil, nil)

	require.Len(t, bars, 1)
	bar := bars[0]
	assert.Equal(t, 100.0, bar.Open)
	assert.Equal(t, 105.0, bar.Close)
	assert.Equal(t, int64(12000), bar.TradeCount)
	assert.Equal(t, 60.0, bar.Volume)
	assert.Equal(t, 6000.0, bar.QuoteVolume)
	assert.Equal(t, 30.0, bar.TakerBuyBaseVolume)
	assert.Equal(t, 3000.0, bar.TakerBuyQuoteVolume)
	assert.Equal(t, 106.0, bar.Mark.High)
	assert.Equal(t, -0.0009, bar.PremiumIndex.Low)
	assert.Equal(t, vo.PriceLineVo{Open: 99, High: 107, Low: 95, Close: 104}, bar.Index)
	assert.Equal(t, onTheDay(9, 0).Unix(), bar.OpenTimeUnixSeconds)
	assert.Equal(t, "BTCUSDT", bar.Symbol)
}

func TestContractCalculationGivesZeroForLinesAnOldCandleNeverHad(t *testing.T) {
	sixtyMinutes := make([]entities.KCandleContract, 0, 60)
	for minute := range 60 {
		sixtyMinutes = append(sixtyMinutes, aStoredContractCandle(onTheDay(9, minute)))
	}
	sixtyMinutes[10].IndexOpen = decimal.NullDecimal{}
	sixtyMinutes[10].IndexHigh = decimal.NullDecimal{}
	sixtyMinutes[10].IndexLow = decimal.NullDecimal{}
	sixtyMinutes[10].IndexClose = decimal.NullDecimal{}
	sixtyMinutes[10].PremiumIndexOpen = decimal.NullDecimal{}
	sixtyMinutes[10].PremiumIndexHigh = decimal.NullDecimal{}
	sixtyMinutes[10].PremiumIndexLow = decimal.NullDecimal{}
	sixtyMinutes[10].PremiumIndexClose = decimal.NullDecimal{}

	bars := newContractCalculationUnderTest(t).barsHandedToTheScript(
		t, hourlyRequest(9, 10), sixtyMinutes, nil, nil)

	require.Len(t, bars, 1)
	assert.Equal(t, vo.PriceLineVo{}, bars[0].Index)
	assert.Equal(t, vo.PriceLineVo{}, bars[0].PremiumIndex)
	assert.Equal(t, vo.PriceLineVo{Open: 100, High: 100, Low: 100, Close: 100}, bars[0].Mark)
}

func TestContractCalculationFeedsTheLatestBarsWhenMoreAreStoredThanAskedFor(t *testing.T) {
	bars := newContractCalculationUnderTest(t).barsHandedToTheScript(
		t, hourlyRequest(10, 12), oneCandleAtEachHour(11, 10, 9), nil, nil)

	require.Len(t, bars, 2)
	assert.Equal(t, onTheDay(10, 0).Unix(), bars[0].OpenTimeUnixSeconds)
	assert.Equal(t, onTheDay(11, 0).Unix(), bars[1].OpenTimeUnixSeconds)
}

func TestContractCalculationHasNoBarWhereNoContractCandleWasStored(t *testing.T) {
	bars := newContractCalculationUnderTest(t).barsHandedToTheScript(
		t, hourlyRequest(8, 11), oneCandleAtEachHour(8, 10), nil,
		[]entities.ContractPositionStatistic{aStatistic(onTheDay(9, 30), "5000")})

	require.Len(t, bars, 2)
	assert.Equal(t, onTheDay(8, 0).Unix(), bars[0].OpenTimeUnixSeconds)
	assert.Equal(t, onTheDay(10, 0).Unix(), bars[1].OpenTimeUnixSeconds)
}

func TestContractCalculationCarriesTheFundingRateInForceAtEachClose(t *testing.T) {
	testCases := []struct {
		name                string
		request             dto.IndicatorCalculationRequestDto
		kCandleContracts    []entities.KCandleContract
		settlements         []entities.ContractFundingRateSettlement
		barOpen             time.Time
		expectedFundingRate float64
		expectedSettled     bool
	}{
		{
			name:    "the bar a settlement falls in carries its rate and says it settled",
			request: hourlyRequest(7, 17), kCandleContracts: oneCandleAtEachHour(7, 8, 9, 10, 11, 12, 13, 14, 15, 16),
			settlements: []entities.ContractFundingRateSettlement{
				aSettlement(onTheDay(8, 0), "0.0001"), aSettlement(onTheDay(16, 0), "0.0005")},
			barOpen: onTheDay(8, 0), expectedFundingRate: 0.0001, expectedSettled: true,
		},
		{
			name:    "a bar between two settlements inherits the earlier rate without settling",
			request: hourlyRequest(7, 17), kCandleContracts: oneCandleAtEachHour(7, 8, 9, 10, 11, 12, 13, 14, 15, 16),
			settlements: []entities.ContractFundingRateSettlement{
				aSettlement(onTheDay(8, 0), "0.0001"), aSettlement(onTheDay(16, 0), "0.0005")},
			barOpen: onTheDay(15, 0), expectedFundingRate: 0.0001, expectedSettled: false,
		},
		{
			name:    "a settlement a millisecond past the hour is not in the bar before it",
			request: hourlyRequest(7, 9), kCandleContracts: oneCandleAtEachHour(7, 8),
			settlements: []entities.ContractFundingRateSettlement{
				aSettlement(onTheDay(0, 0), "0.0001"), aSettlement(onTheDay(8, 0).Add(time.Millisecond), "0.0002")},
			barOpen: onTheDay(7, 0), expectedFundingRate: 0.0001, expectedSettled: false,
		},
		{
			name:    "a settlement a millisecond past the hour is in the bar it falls in",
			request: hourlyRequest(7, 9), kCandleContracts: oneCandleAtEachHour(7, 8),
			settlements: []entities.ContractFundingRateSettlement{
				aSettlement(onTheDay(0, 0), "0.0001"), aSettlement(onTheDay(8, 0).Add(time.Millisecond), "0.0002")},
			barOpen: onTheDay(8, 0), expectedFundingRate: 0.0002, expectedSettled: true,
		},
		{
			name:    "a negative rate is carried as it is",
			request: hourlyRequest(8, 9), kCandleContracts: oneCandleAtEachHour(8),
			settlements: []entities.ContractFundingRateSettlement{aSettlement(onTheDay(8, 0), "-0.00003")},
			barOpen:     onTheDay(8, 0), expectedFundingRate: -0.00003, expectedSettled: true,
		},
		{
			name:    "a bar before any settlement carries zero and did not settle",
			request: hourlyRequest(8, 9), kCandleContracts: oneCandleAtEachHour(8),
			barOpen: onTheDay(8, 0), expectedFundingRate: 0, expectedSettled: false,
		},
		{
			name:    "the last settlement before the window carries into the first bar",
			request: hourlyRequest(10, 11), kCandleContracts: oneCandleAtEachHour(10),
			settlements: []entities.ContractFundingRateSettlement{aSettlement(onTheDay(8, 0), "0.0001")},
			barOpen:     onTheDay(10, 0), expectedFundingRate: 0.0001, expectedSettled: false,
		},
		{
			name:    "settlements arriving out of order are still read latest-last",
			request: hourlyRequest(8, 9), kCandleContracts: oneCandleAtEachHour(8),
			settlements: []entities.ContractFundingRateSettlement{
				aSettlement(onTheDay(8, 0), "0.0003"), aSettlement(onTheDay(0, 0), "0.0001")},
			barOpen: onTheDay(8, 0), expectedFundingRate: 0.0003, expectedSettled: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			bars := newContractCalculationUnderTest(t).barsHandedToTheScript(
				t, testCase.request, testCase.kCandleContracts, testCase.settlements, nil)

			bar := barOpeningAt(t, bars, testCase.barOpen)
			assert.Equal(t, testCase.expectedFundingRate, bar.FundingRate)
			assert.Equal(t, testCase.expectedSettled, bar.FundingSettledInBar)
		})
	}
}

func TestContractCalculationTakesTheLastOfSeveralSettlementsInsideOneBar(t *testing.T) {
	dailyRequest := dto.IndicatorCalculationRequestDto{
		Symbol: "BTCUSDT", AggregationInterval: "1d",
		StartTime: time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC), EndTime: onTheDay(0, 0), Script: "the script",
	}
	theDayBefore := []entities.KCandleContract{aStoredContractCandle(time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC))}

	bars := newContractCalculationUnderTest(t).barsHandedToTheScript(t, dailyRequest, theDayBefore,
		[]entities.ContractFundingRateSettlement{
			aSettlement(time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC), "0.0001"),
			aSettlement(time.Date(2026, 8, 28, 8, 0, 0, 0, time.UTC), "-0.0002"),
			aSettlement(time.Date(2026, 8, 28, 16, 0, 0, 0, time.UTC), "0.0003"),
		}, nil)

	require.Len(t, bars, 1)
	assert.Equal(t, 0.0003, bars[0].FundingRate)
	assert.True(t, bars[0].FundingSettledInBar)
}

func TestContractCalculationCarriesTheLatestRecentEnoughPositionStatistic(t *testing.T) {
	everyFiveMinutesFromNine := make([]entities.ContractPositionStatistic, 0, 12)
	for minute := 0; minute < 60; minute += 5 {
		everyFiveMinutesFromNine = append(everyFiveMinutesFromNine,
			aStatistic(onTheDay(9, minute), decimal.NewFromInt(int64(1000+minute)).String()))
	}
	oneMinuteRequest := func(startMinute int, endMinute int) dto.IndicatorCalculationRequestDto {
		return dto.IndicatorCalculationRequestDto{
			Symbol: "BTCUSDT", AggregationInterval: "1m",
			StartTime: onTheDay(9, startMinute), EndTime: onTheDay(9, endMinute), Script: "the script",
		}
	}
	oneCandleAtEachMinute := func(minutes ...int) []entities.KCandleContract {
		kCandleContracts := make([]entities.KCandleContract, 0, len(minutes))
		for _, minute := range minutes {
			kCandleContracts = append(kCandleContracts, aStoredContractCandle(onTheDay(9, minute)))
		}
		return kCandleContracts
	}

	testCases := []struct {
		name                 string
		request              dto.IndicatorCalculationRequestDto
		kCandleContracts     []entities.KCandleContract
		statistics           []entities.ContractPositionStatistic
		barOpen              time.Time
		expectedOpenInterest float64
	}{
		{
			name:    "of the twelve inside an hour bar, the latest",
			request: hourlyRequest(9, 10), kCandleContracts: oneCandleAtEachHour(9),
			statistics: everyFiveMinutesFromNine, barOpen: onTheDay(9, 0), expectedOpenInterest: 1055,
		},
		{
			name:    "a one-minute bar carries the statistic taken within five minutes of its close",
			request: oneMinuteRequest(0, 5), kCandleContracts: oneCandleAtEachMinute(0, 1, 2, 3, 4),
			statistics: []entities.ContractPositionStatistic{aStatistic(onTheDay(9, 0), "5000")},
			barOpen:    onTheDay(9, 3), expectedOpenInterest: 5000,
		},
		{
			name:    "a one-minute bar whose latest statistic is older than five minutes carries zero",
			request: oneMinuteRequest(5, 6), kCandleContracts: oneCandleAtEachMinute(5),
			statistics: []entities.ContractPositionStatistic{aStatistic(onTheDay(9, 0), "5000")},
			barOpen:    onTheDay(9, 5), expectedOpenInterest: 0,
		},
		{
			name:    "an hour bar with no statistic inside it carries zero rather than the one before",
			request: hourlyRequest(9, 10), kCandleContracts: oneCandleAtEachHour(9),
			statistics: []entities.ContractPositionStatistic{aStatistic(onTheDay(8, 55), "5000")},
			barOpen:    onTheDay(9, 0), expectedOpenInterest: 0,
		},
		{
			name:    "a statistic taken at a bar's close belongs to the next bar, not this one",
			request: hourlyRequest(9, 11), kCandleContracts: oneCandleAtEachHour(9, 10),
			statistics: []entities.ContractPositionStatistic{
				aStatistic(onTheDay(9, 50), "5000"), aStatistic(onTheDay(10, 0), "6000")},
			barOpen: onTheDay(9, 0), expectedOpenInterest: 5000,
		},
		{
			name:    "the next bar carries the statistic taken at its open",
			request: hourlyRequest(9, 11), kCandleContracts: oneCandleAtEachHour(9, 10),
			statistics: []entities.ContractPositionStatistic{
				aStatistic(onTheDay(9, 50), "5000"), aStatistic(onTheDay(10, 0), "6000")},
			barOpen: onTheDay(10, 0), expectedOpenInterest: 6000,
		},
		{
			name:    "statistics arriving out of order are still read latest-last",
			request: hourlyRequest(9, 10), kCandleContracts: oneCandleAtEachHour(9),
			statistics: []entities.ContractPositionStatistic{
				aStatistic(onTheDay(9, 55), "5000"), aStatistic(onTheDay(9, 5), "1000")},
			barOpen: onTheDay(9, 0), expectedOpenInterest: 5000,
		},
		{
			name:    "a stretch with no statistics at all carries zero",
			request: hourlyRequest(9, 10), kCandleContracts: oneCandleAtEachHour(9),
			barOpen: onTheDay(9, 0), expectedOpenInterest: 0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			bars := newContractCalculationUnderTest(t).barsHandedToTheScript(
				t, testCase.request, testCase.kCandleContracts, nil, testCase.statistics)

			assert.Equal(t, testCase.expectedOpenInterest, barOpeningAt(t, bars, testCase.barOpen).OpenInterest)
		})
	}
}

func TestContractCalculationCarriesEveryFigureOfThePositionStatistic(t *testing.T) {
	statistic := entities.ContractPositionStatistic{
		Symbol: "BTCUSDT", StatisticTime: onTheDay(9, 55),
		OpenInterest: decimal.RequireFromString("5000"), OpenInterestValue: decimal.RequireFromString("450000000"),
		AccountLongShare: decimal.RequireFromString("0.47"), AccountShortShare: decimal.RequireFromString("0.53"),
		AccountLongShortRatio:      decimal.RequireFromString("0.89"),
		TopTraderPositionLongShare: decimal.RequireFromString("0.6"), TopTraderPositionShortShare: decimal.RequireFromString("0.4"),
		TopTraderPositionLongShortRatio: decimal.RequireFromString("1.5"),
	}

	bars := newContractCalculationUnderTest(t).barsHandedToTheScript(
		t, hourlyRequest(9, 10), oneCandleAtEachHour(9), nil, []entities.ContractPositionStatistic{statistic})

	require.Len(t, bars, 1)
	assert.Equal(t, 5000.0, bars[0].OpenInterest)
	assert.Equal(t, 450000000.0, bars[0].OpenInterestValue)
	assert.Equal(t, 0.47, bars[0].AccountLongShare)
	assert.Equal(t, 0.53, bars[0].AccountShortShare)
	assert.Equal(t, 0.89, bars[0].AccountLongShortRatio)
	assert.Equal(t, 0.6, bars[0].TopTraderPositionLongShare)
	assert.Equal(t, 0.4, bars[0].TopTraderPositionShortShare)
	assert.Equal(t, 1.5, bars[0].TopTraderPositionLongShortRatio)
}

func TestContractCalculationReadsFundingAndPositioningOverTheStretchTheBarsCover(t *testing.T) {
	fixture := newContractCalculationUnderTest(t)
	fixture.kCandleContractRepository.EXPECT().
		FindLatestBefore(gomock.Any(), "BTCUSDT", onTheDay(12, 0), gomock.Any()).
		Return(oneCandleAtEachHour(10, 11), nil)
	fixture.contractFundingRateSettlementRepository.EXPECT().
		FindInRange(gomock.Any(), gomock.Any(), 3).
		DoAndReturn(func(_ context.Context, query domains.KCandleQueryDomain, _ int) ([]entities.ContractFundingRateSettlement, error) {
			// From the first bar's open to the last bar's close; the rate already in force is read separately.
			assert.Equal(t, onTheDay(10, 0), query.StartTime())
			assert.Equal(t, onTheDay(12, 0), query.EndTime())
			assert.Equal(t, "BTCUSDT", query.Symbol())
			return nil, nil
		})
	fixture.contractFundingRateSettlementRepository.EXPECT().
		FindLatestBefore(gomock.Any(), "BTCUSDT", onTheDay(10, 0)).
		Return(entities.ContractFundingRateSettlement{}, false, nil)
	fixture.contractPositionStatisticRepository.EXPECT().
		FindInRange(gomock.Any(), gomock.Any(), 26).
		DoAndReturn(func(_ context.Context, query domains.KCandleQueryDomain, _ int) ([]entities.ContractPositionStatistic, error) {
			assert.Equal(t, onTheDay(9, 55), query.StartTime())
			assert.Equal(t, onTheDay(12, 0), query.EndTime())
			return nil, nil
		})
	fixture.contractIndicatorScriptProxy.EXPECT().
		Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(map[string]vo.IndicatorValueVo{}, nil)

	_, err := fixture.contractIndicatorCalculationService.CalculateContractIndicator(t.Context(), hourlyRequest(10, 12))

	require.NoError(t, err)
}

func TestContractCalculationNeverReadsTheBucketStillRunning(t *testing.T) {
	// At 09:30 the 09:00 hour is unfinished, so the read stops there and the last bar is 08:00.
	fixture := newContractCalculationUnderTest(t)
	fixture.kCandleContractRepository.EXPECT().
		FindLatestBefore(gomock.Any(), "BTCUSDT", onTheDay(9, 0), gomock.Any()).
		Return(oneCandleAtEachHour(8, 7), nil)
	fixture.holdSettlements(nil)
	fixture.contractPositionStatisticRepository.EXPECT().
		FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
	fixture.contractIndicatorScriptProxy.EXPECT().
		Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ string, _ domains.IndicatorResultTypeDomain,
			contractKCandleVos []vo.ContractKCandleVo, _ domains.StrategyScriptParametersDomain,
		) (map[string]vo.IndicatorValueVo, error) {
			assert.Equal(t, onTheDay(8, 0).Unix(), contractKCandleVos[len(contractKCandleVos)-1].OpenTimeUnixSeconds)
			return map[string]vo.IndicatorValueVo{}, nil
		})
	requestDto := hourlyRequest(7, 9)
	requestDto.EndTime = onTheDay(9, 30)

	_, err := fixture.contractIndicatorCalculationService.CalculateContractIndicator(t.Context(), requestDto)

	require.NoError(t, err)
}

func TestContractCalculationRefusesAStretchTooThinForOneValue(t *testing.T) {
	testCases := []struct {
		name              string
		kCandleContracts  []entities.KCandleContract
		parameters        []dto.StrategyScriptParameterWriteDto
		expectedAvailable int
		expectedMinimum   int
	}{
		{
			name:             "twelve bars for a look-back of twenty",
			kCandleContracts: oneCandleAtEachHour(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11),
			parameters: []dto.StrategyScriptParameterWriteDto{
				{Name: "期數", Kind: "lookbackCount", DefaultValue: 20}},
			expectedAvailable: 12, expectedMinimum: 20,
		},
		{
			name:              "a symbol with no contract candles stored at all",
			kCandleContracts:  nil,
			expectedAvailable: 0, expectedMinimum: 1,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newContractCalculationUnderTest(t)
			fixture.kCandleContractRepository.EXPECT().
				FindLatestBefore(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(testCase.kCandleContracts, nil)
			requestDto := hourlyRequest(0, 12)
			requestDto.Parameters = testCase.parameters

			_, err := fixture.contractIndicatorCalculationService.CalculateContractIndicator(t.Context(), requestDto)

			availableCandleCount, minimumCandleCount, isTooThin := domains.CandleCoverageShortfall(err)
			require.True(t, isTooThin)
			assert.Equal(t, testCase.expectedAvailable, availableCandleCount)
			assert.Equal(t, testCase.expectedMinimum, minimumCandleCount)
		})
	}
}

func TestContractCalculationAnswersInTheSpotCalculationsShape(t *testing.T) {
	t.Run("a signal comes back as the result itself, with both counts", func(t *testing.T) {
		fixture := newContractCalculationUnderTest(t)
		fixture.kCandleContractRepository.EXPECT().
			FindLatestBefore(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(oneCandleAtEachHour(9, 10), nil)
		fixture.holdSettlements(nil)
		fixture.contractPositionStatisticRepository.EXPECT().
			FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		fixture.contractIndicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{vo.SignalIndicatorKey: {Signal: vo.SignalBuy}}, nil)
		requestDto := hourlyRequest(8, 11)
		requestDto.ResultType = "signal"

		resultDto, err := fixture.contractIndicatorCalculationService.CalculateContractIndicator(t.Context(), requestDto)

		require.NoError(t, err)
		assert.Equal(t, "buy", resultDto.Signal)
		assert.Equal(t, "signal", resultDto.ResultType)
		assert.Equal(t, 3, resultDto.RequiredCandleCount)
		assert.Equal(t, 2, resultDto.UsedCandleCount)
		assert.Equal(t, []time.Time{onTheDay(9, 0), onTheDay(10, 0)}, resultDto.OpenTimes)
		assert.Equal(t, "1h", resultDto.Interval)
		assert.Equal(t, "BTCUSDT", resultDto.Symbol)
	})

	t.Run("an interval left out is one minute", func(t *testing.T) {
		fixture := newContractCalculationUnderTest(t)
		fixture.kCandleContractRepository.EXPECT().
			FindLatestBefore(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandleContract{aStoredContractCandle(onTheDay(9, 5))}, nil)
		fixture.holdSettlements(nil)
		fixture.contractPositionStatisticRepository.EXPECT().
			FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		fixture.contractIndicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{"ma": {Numbers: []float64{100}}}, nil)
		requestDto := dto.IndicatorCalculationRequestDto{
			Symbol: "BTCUSDT", StartTime: onTheDay(9, 0), EndTime: onTheDay(9, 10), Script: "the script",
		}

		resultDto, err := fixture.contractIndicatorCalculationService.CalculateContractIndicator(t.Context(), requestDto)

		require.NoError(t, err)
		assert.Equal(t, "1m", resultDto.Interval)
		assert.Equal(t, []float64{100}, resultDto.Values["ma"].Numbers)
	})
}

func TestContractCalculationPassesOnWhatWentWrong(t *testing.T) {
	storageDown := errors.New("storage will not answer")

	t.Run("a request that breaks a rule is refused before anything is read", func(t *testing.T) {
		fixture := newContractCalculationUnderTest(t)
		requestDto := hourlyRequest(8, 11)
		requestDto.AggregationInterval = "2h"

		_, err := fixture.contractIndicatorCalculationService.CalculateContractIndicator(t.Context(), requestDto)

		require.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
	})

	t.Run("candles that cannot be read", func(t *testing.T) {
		fixture := newContractCalculationUnderTest(t)
		fixture.kCandleContractRepository.EXPECT().
			FindLatestBefore(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, storageDown)

		_, err := fixture.contractIndicatorCalculationService.CalculateContractIndicator(t.Context(), hourlyRequest(8, 11))

		require.ErrorIs(t, err, storageDown)
	})

	t.Run("settlements that cannot be read", func(t *testing.T) {
		fixture := newContractCalculationUnderTest(t)
		fixture.kCandleContractRepository.EXPECT().
			FindLatestBefore(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(oneCandleAtEachHour(9), nil)
		fixture.contractFundingRateSettlementRepository.EXPECT().
			FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, storageDown)

		_, err := fixture.contractIndicatorCalculationService.CalculateContractIndicator(t.Context(), hourlyRequest(8, 11))

		require.ErrorIs(t, err, storageDown)
	})

	t.Run("the settlement in force before the stretch that cannot be read", func(t *testing.T) {
		fixture := newContractCalculationUnderTest(t)
		fixture.kCandleContractRepository.EXPECT().
			FindLatestBefore(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(oneCandleAtEachHour(9), nil)
		fixture.contractFundingRateSettlementRepository.EXPECT().
			FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		fixture.contractFundingRateSettlementRepository.EXPECT().
			FindLatestBefore(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entities.ContractFundingRateSettlement{}, false, storageDown)

		_, err := fixture.contractIndicatorCalculationService.CalculateContractIndicator(t.Context(), hourlyRequest(8, 11))

		require.ErrorIs(t, err, storageDown)
	})

	t.Run("statistics that cannot be read", func(t *testing.T) {
		fixture := newContractCalculationUnderTest(t)
		fixture.kCandleContractRepository.EXPECT().
			FindLatestBefore(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(oneCandleAtEachHour(9), nil)
		fixture.holdSettlements(nil)
		fixture.contractPositionStatisticRepository.EXPECT().
			FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, storageDown)

		_, err := fixture.contractIndicatorCalculationService.CalculateContractIndicator(t.Context(), hourlyRequest(8, 11))

		require.ErrorIs(t, err, storageDown)
	})

	t.Run("a script that fails", func(t *testing.T) {
		fixture := newContractCalculationUnderTest(t)
		fixture.kCandleContractRepository.EXPECT().
			FindLatestBefore(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(oneCandleAtEachHour(9), nil)
		fixture.holdSettlements(nil)
		fixture.contractPositionStatisticRepository.EXPECT().
			FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		fixture.contractIndicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, domains.ErrIndicatorScriptFailed)

		_, err := fixture.contractIndicatorCalculationService.CalculateContractIndicator(t.Context(), hourlyRequest(8, 11))

		require.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
	})
}

func TestContractCalculationCarriesTheRateInForceHoweverLongAgoItWasSettled(t *testing.T) {
	// Settlement fetching stopped for two days; the last stored one is still the rate in force until the next arrives.
	twoDaysEarlier := onTheDay(8, 0).Add(-48 * time.Hour)

	bars := newContractCalculationUnderTest(t).barsHandedToTheScript(
		t, hourlyRequest(10, 12), oneCandleAtEachHour(10, 11),
		[]entities.ContractFundingRateSettlement{aSettlement(twoDaysEarlier, "0.0004")}, nil)

	require.Len(t, bars, 2)
	for _, bar := range bars {
		assert.Equal(t, 0.0004, bar.FundingRate)
		assert.False(t, bar.FundingSettledInBar)
	}
}

func TestContractCalculationCarriesTheEarlierRateOnEveryBarBetweenTwoSettlements(t *testing.T) {
	bars := newContractCalculationUnderTest(t).barsHandedToTheScript(
		t, hourlyRequest(8, 17), oneCandleAtEachHour(8, 9, 10, 11, 12, 13, 14, 15, 16),
		[]entities.ContractFundingRateSettlement{
			aSettlement(onTheDay(8, 0), "0.0001"), aSettlement(onTheDay(16, 0), "0.0005")}, nil)

	for hour := 9; hour <= 15; hour++ {
		bar := barOpeningAt(t, bars, onTheDay(hour, 0))
		assert.Equal(t, 0.0001, bar.FundingRate, "the %02d:00 bar", hour)
		assert.False(t, bar.FundingSettledInBar, "the %02d:00 bar", hour)
	}
}

func TestContractCalculationLeavesASettlementAtTheCloseToTheNextBar(t *testing.T) {
	bars := newContractCalculationUnderTest(t).barsHandedToTheScript(
		t, hourlyRequest(7, 9), oneCandleAtEachHour(7, 8),
		[]entities.ContractFundingRateSettlement{
			aSettlement(onTheDay(0, 0), "0.0001"), aSettlement(onTheDay(8, 0), "0.0002")}, nil)

	sevenOClock := barOpeningAt(t, bars, onTheDay(7, 0))
	assert.Equal(t, 0.0001, sevenOClock.FundingRate)
	assert.False(t, sevenOClock.FundingSettledInBar)
	eightOClock := barOpeningAt(t, bars, onTheDay(8, 0))
	assert.Equal(t, 0.0002, eightOClock.FundingRate)
	assert.True(t, eightOClock.FundingSettledInBar)
}

// everyStatisticFigure has all eight figures non-zero, so a bar carrying zeros was handed none of them.
func everyStatisticFigure(statisticTime time.Time) entities.ContractPositionStatistic {
	return entities.ContractPositionStatistic{
		Symbol: "BTCUSDT", StatisticTime: statisticTime,
		OpenInterest: decimal.RequireFromString("5000"), OpenInterestValue: decimal.RequireFromString("450000000"),
		AccountLongShare: decimal.RequireFromString("0.47"), AccountShortShare: decimal.RequireFromString("0.53"),
		AccountLongShortRatio:      decimal.RequireFromString("0.89"),
		TopTraderPositionLongShare: decimal.RequireFromString("0.6"), TopTraderPositionShortShare: decimal.RequireFromString("0.4"),
		TopTraderPositionLongShortRatio: decimal.RequireFromString("1.5"),
	}
}

func assertCarriesNoPositionStatistic(t *testing.T, bar vo.ContractKCandleVo) {
	t.Helper()
	assert.Zero(t, bar.OpenInterest)
	assert.Zero(t, bar.OpenInterestValue)
	assert.Zero(t, bar.AccountLongShare)
	assert.Zero(t, bar.AccountShortShare)
	assert.Zero(t, bar.AccountLongShortRatio)
	assert.Zero(t, bar.TopTraderPositionLongShare)
	assert.Zero(t, bar.TopTraderPositionShortShare)
	assert.Zero(t, bar.TopTraderPositionLongShortRatio)
}

func TestContractCalculationCarriesAllZerosForAStatisticThatIsNotRecentEnough(t *testing.T) {
	t.Run("a one-minute bar whose latest statistic is older than five minutes", func(t *testing.T) {
		requestDto := dto.IndicatorCalculationRequestDto{
			Symbol: "BTCUSDT", AggregationInterval: "1m",
			StartTime: onTheDay(9, 5), EndTime: onTheDay(9, 6), Script: "the script",
		}

		bars := newContractCalculationUnderTest(t).barsHandedToTheScript(
			t, requestDto, []entities.KCandleContract{aStoredContractCandle(onTheDay(9, 5))},
			nil, []entities.ContractPositionStatistic{everyStatisticFigure(onTheDay(9, 0))})

		assertCarriesNoPositionStatistic(t, barOpeningAt(t, bars, onTheDay(9, 5)))
	})

	t.Run("an hour bar whose latest statistic is from before it", func(t *testing.T) {
		bars := newContractCalculationUnderTest(t).barsHandedToTheScript(
			t, hourlyRequest(9, 10), oneCandleAtEachHour(9),
			nil, []entities.ContractPositionStatistic{everyStatisticFigure(onTheDay(8, 55))})

		assertCarriesNoPositionStatistic(t, barOpeningAt(t, bars, onTheDay(9, 0)))
	})
}

func TestContractCalculationAnswersAStretchBeforeStatisticsWereRecordedWithFundingAndPricesIntact(t *testing.T) {
	bars := newContractCalculationUnderTest(t).barsHandedToTheScript(
		t, hourlyRequest(9, 10), oneCandleAtEachHour(9),
		[]entities.ContractFundingRateSettlement{aSettlement(onTheDay(8, 0), "0.0001")}, nil)

	require.Len(t, bars, 1)
	assertCarriesNoPositionStatistic(t, bars[0])
	assert.Equal(t, 0.0001, bars[0].FundingRate)
	assert.Equal(t, 100.0, bars[0].Close)
	assert.Equal(t, 1.0, bars[0].Volume)
}

func TestContractCalculationMergesPricesAndVolumesAsUsualBesideAnOldCandle(t *testing.T) {
	sixtyMinutes := make([]entities.KCandleContract, 0, 60)
	for minute := range 60 {
		sixtyMinutes = append(sixtyMinutes, aStoredContractCandle(onTheDay(9, minute)))
	}
	sixtyMinutes[0].Open = decimal.RequireFromString("98")
	sixtyMinutes[59].Close = decimal.RequireFromString("103")
	sixtyMinutes[10].IndexOpen = decimal.NullDecimal{}
	sixtyMinutes[10].PremiumIndexClose = decimal.NullDecimal{}

	bars := newContractCalculationUnderTest(t).barsHandedToTheScript(
		t, hourlyRequest(9, 10), sixtyMinutes, nil, nil)

	require.Len(t, bars, 1)
	assert.Equal(t, 98.0, bars[0].Open)
	assert.Equal(t, 103.0, bars[0].Close)
	assert.Equal(t, 60.0, bars[0].Volume)
	assert.Equal(t, 6000.0, bars[0].QuoteVolume)
	assert.Equal(t, int64(12000), bars[0].TradeCount)
	assert.Equal(t, vo.PriceLineVo{}, bars[0].Index)
	assert.Equal(t, vo.PriceLineVo{}, bars[0].PremiumIndex)
}
