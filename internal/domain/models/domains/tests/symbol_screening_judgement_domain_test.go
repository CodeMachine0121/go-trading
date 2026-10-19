package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var screeningNow = time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)

var screeningPeriodEnd = time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)

func aScreeningSnapshot(historyDays int, segmentCount int) dto.SymbolScreeningSnapshotDto {
	return dto.SymbolScreeningSnapshotDto{
		AggregationInterval: "1d", HistoryDays: historyDays, SegmentCount: segmentCount, MinimumTradeCount: 6,
		MinimumAverageDailyQuoteVolume: decimal.NewFromInt(20_000_000),
	}
}

func aScreeningPeriod(historyDays int, segmentCount int) domains.SymbolScreeningPeriodDomain {
	return domains.NewSymbolScreeningPeriodDomain(aScreeningSnapshot(historyDays, segmentCount), screeningNow, 0)
}

func dailyCandlesSince(firstOpen time.Time, quoteVolume decimal.Decimal) []entities.KCandleContract {
	kCandles := []entities.KCandleContract{}
	for openTime := firstOpen; openTime.Before(screeningPeriodEnd); openTime = openTime.Add(24 * time.Hour) {
		kCandles = append(kCandles, entities.KCandleContract{OpenTime: openTime, QuoteVolume: quoteVolume})
	}

	return kCandles
}

var tradingListing = vo.ContractSymbolListingVo{IsListed: true, IsKnownToVenue: true}

func TestSymbolScreeningBasicConditionInOrder(t *testing.T) {
	fourYearsAgo := screeningPeriodEnd.AddDate(-4, 0, 0)
	plentiful := decimal.NewFromInt(300_000_000)
	testCases := []struct {
		name            string
		symbol          string
		listing         vo.ContractSymbolListingVo
		kCandles        []entities.KCandleContract
		expectedPassed  bool
		expectedMessage string
	}{
		{name: "everything holds", symbol: "LINKUSDT", listing: tradingListing,
			kCandles: dailyCandlesSince(fourYearsAgo, plentiful), expectedPassed: true},
		{name: "not on the venue", symbol: "FOOUSDT", listing: vo.ContractSymbolListingVo{},
			expectedMessage: "幣安沒有這個 USDT 永續合約"},
		{name: "no longer trading", symbol: "LINKUSDT", listing: vo.ContractSymbolListingVo{IsKnownToVenue: true},
			expectedMessage: "目前沒有在交易"},
		{name: "one day short of history", symbol: "LINKUSDT", listing: tradingListing,
			kCandles:        dailyCandlesSince(screeningPeriodEnd.Add(-729*24*time.Hour), plentiful),
			expectedMessage: "歷史只有 729 天，需要 730 天"},
		{name: "history exactly long enough", symbol: "LINKUSDT", listing: tradingListing,
			kCandles:       dailyCandlesSince(screeningPeriodEnd.Add(-730*24*time.Hour), plentiful),
			expectedPassed: true},
		{name: "turnover just short", symbol: "LINKUSDT", listing: tradingListing,
			kCandles:        dailyCandlesSince(fourYearsAgo, decimal.NewFromInt(19_990_000)),
			expectedMessage: "低於門檻"},
		{name: "turnover exactly at the threshold", symbol: "LINKUSDT", listing: tradingListing,
			kCandles:       dailyCandlesSince(fourYearsAgo, decimal.NewFromInt(20_000_000)),
			expectedPassed: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			passed, reason, _ := domains.NewSymbolScreeningBasicConditionDomain(
				testCase.symbol, testCase.listing, testCase.kCandles, aScreeningPeriod(730, 3),
				decimal.NewFromInt(20_000_000)).Judge()

			assert.Equal(t, testCase.expectedPassed, passed)
			if !testCase.expectedPassed {
				assert.Contains(t, reason, testCase.expectedMessage)
			}
		})
	}
}

func segmentResults(returnRates []float64, tradesPerSegment []int) []dto.ContractBacktestResultDto {
	results := []dto.ContractBacktestResultDto{}
	for index, returnRate := range returnRates {
		closedTrades := make([]dto.ContractClosedTradeDto, tradesPerSegment[index])
		results = append(results, dto.ContractBacktestResultDto{
			Summary: dto.ContractBacktestSummaryDto{TotalReturnRate: returnRate}, ClosedTrades: closedTrades,
		})
	}

	return results
}

func TestSymbolScreeningPortabilityJudgesEverySegment(t *testing.T) {
	testCases := []struct {
		name                    string
		returnRates             []float64
		tradesPerSegment        []int
		expectedPassed          bool
		expectedMessage         string
		expectedWithoutBest     float64
		checksReturnWithoutBest bool
	}{
		{name: "most win and still win without the best", returnRates: []float64{0.20, 0.05, 0.12},
			tradesPerSegment: []int{3, 3, 3}, expectedPassed: true, expectedWithoutBest: 0.176, checksReturnWithoutBest: true},
		{name: "most lose", returnRates: []float64{0.40, -0.10, -0.05}, tradesPerSegment: []int{3, 3, 3},
			expectedMessage: "多數區段虧損"},
		{name: "only one big win", returnRates: []float64{3.00, 0.01, -0.08}, tradesPerSegment: []int{3, 3, 3},
			expectedMessage: "去掉最好的一段就虧損", expectedWithoutBest: -0.0708, checksReturnWithoutBest: true},
		{name: "too few trades", returnRates: []float64{0.10, 0.10, 0.10}, tradesPerSegment: []int{2, 2, 1},
			expectedMessage: "交易太少，無法判斷"},
		{name: "trades exactly enough", returnRates: []float64{0.10, 0.10, 0.10}, tradesPerSegment: []int{2, 2, 2},
			expectedPassed: true},
		{name: "half winning is not most", returnRates: []float64{0.10, 0.10, -0.05, -0.05},
			tradesPerSegment: []int{3, 3, 3, 3}, expectedMessage: "多數區段虧損"},
		{name: "a flat segment is not a win", returnRates: []float64{0.10, 0, 0.05}, tradesPerSegment: []int{3, 3, 3},
			expectedPassed: true},
		{name: "a flat segment beside a losing one leaves most losing", returnRates: []float64{0.10, 0, -0.05},
			tradesPerSegment: []int{3, 3, 3}, expectedMessage: "多數區段虧損"},
		{name: "slightly losing without the best", returnRates: []float64{0.50, 0.02, -0.02},
			tradesPerSegment: []int{3, 3, 3}, expectedMessage: "去掉最好的一段就虧損",
			expectedWithoutBest: -0.0004, checksReturnWithoutBest: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			period := aScreeningPeriod(730, len(testCase.returnRates))

			passed, reason, findings := domains.NewSymbolScreeningPortabilityDomain(
				period.Segments(), segmentResults(testCase.returnRates, testCase.tradesPerSegment), 6,
			).Judge(dto.SymbolScreeningFindingsDto{})

			assert.Equal(t, testCase.expectedPassed, passed)
			if !testCase.expectedPassed {
				assert.Contains(t, reason, testCase.expectedMessage)
			}
			if testCase.checksReturnWithoutBest {
				require.NotNil(t, findings.ReturnWithoutBestSegment)
				assert.InDelta(t, testCase.expectedWithoutBest, *findings.ReturnWithoutBestSegment, 0.00001)
			}
			assert.Len(t, findings.Segments, len(testCase.returnRates))
		})
	}
}

func TestSymbolScreeningPeriodEndsAtTheLastFinishedDayAndCutsEqualSegments(t *testing.T) {
	period := aScreeningPeriod(730, 3)

	segments := period.Segments()

	assert.Equal(t, time.Date(2024, 10, 19, 0, 0, 0, 0, time.UTC), period.StartTime())
	assert.Equal(t, screeningPeriodEnd, period.EndTime())
	require.Len(t, segments, 3)
	assert.Equal(t, period.StartTime(), segments[0].StartTime)
	assert.Equal(t, segments[0].EndTime, segments[1].StartTime)
	assert.Equal(t, segments[1].EndTime, segments[2].StartTime)
	assert.Equal(t, screeningPeriodEnd, segments[2].EndTime)
	for _, request := range period.ReplayRequests("LINKUSDT") {
		assert.True(t, domains.SymbolScreeningInitialCapital.Equal(request.InitialCapital))
		assert.Equal(t, "nextOpen", request.FillTiming)
		assert.Equal(t, "allIn", request.PositionSizingMode)
	}
}

func holdingsFrom(spans ...[2]int) []dto.ContractClosedTradeDto {
	trades := []dto.ContractClosedTradeDto{}
	for _, span := range spans {
		trades = append(trades, dto.ContractClosedTradeDto{
			EntryTime: screeningPeriodEnd.Add(-time.Duration(span[0]) * 24 * time.Hour),
			ExitTime:  screeningPeriodEnd.Add(-time.Duration(span[1]) * 24 * time.Hour),
		})
	}

	return trades
}

func TestSymbolScreeningContributionMeasuresTheShortenedGap(t *testing.T) {
	periodStart := screeningPeriodEnd.Add(-200 * 24 * time.Hour)
	// The baseline holds from day 200 to day 105 and from day 10 on, so it sits out 95 days.
	baseline := domains.NewSymbolScreeningHoldingTimelineDomain(
		holdingsFrom([2]int{200, 105}, [2]int{10, 0}), periodStart, screeningPeriodEnd)
	// The candidate holds from day 105 to day 70, leaving the longest flat stretch at 60 days.
	candidate := domains.NewSymbolScreeningHoldingTimelineDomain(
		holdingsFrom([2]int{105, 70}), periodStart, screeningPeriodEnd)

	contribution := candidate.ContributionAgainst(baseline)

	assert.Equal(t, 95, contribution.BaselineLongestGapDays)
	assert.Equal(t, 60, contribution.CombinedLongestGapDays)
	assert.Equal(t, 35, contribution.GapReductionDays)
	assert.InDelta(t, 35.0/200.0, contribution.InMarketShare, 0.0001)
	assert.InDelta(t, 0.0, contribution.OverlapShare, 0.0001)
}

func aRankedCandidate(position int, status string, gapReduction *int, returnWithoutBest float64) entities.SymbolScreeningCandidate {
	findings := dto.SymbolScreeningFindingsDto{ReturnWithoutBestSegment: &returnWithoutBest}
	if gapReduction != nil {
		findings.Contribution = &dto.SymbolScreeningContributionDto{GapReductionDays: *gapReduction}
	}

	return entities.SymbolScreeningCandidate{
		Position: position, Symbol: string(rune('A' + position - 1)), Status: status, Findings: findings,
	}
}

func rankedSymbols(candidates []entities.SymbolScreeningCandidate) []string {
	ranked := domains.NewSymbolScreeningRecommendationDomain(candidates).Ranked()
	symbols := []string{}
	for _, candidate := range ranked {
		symbols = append(symbols, candidate.Symbol)
	}

	return symbols
}

func TestSymbolScreeningRecommendationOrder(t *testing.T) {
	thirtyFive, ten, twenty := 35, 10, 20
	testCases := []struct {
		name            string
		candidates      []entities.SymbolScreeningCandidate
		expectedSymbols []string
	}{
		{name: "the larger gap reduction first", candidates: []entities.SymbolScreeningCandidate{
			aRankedCandidate(1, "passed", &ten, 0.5), aRankedCandidate(2, "passed", &thirtyFive, 0.1),
		}, expectedSymbols: []string{"B", "A"}},
		{name: "equal reductions by return without the best segment", candidates: []entities.SymbolScreeningCandidate{
			aRankedCandidate(1, "passed", &twenty, 0.05), aRankedCandidate(2, "passed", &twenty, 0.12),
		}, expectedSymbols: []string{"B", "A"}},
		{name: "no baseline ranks by return without the best segment", candidates: []entities.SymbolScreeningCandidate{
			aRankedCandidate(1, "passed", nil, 0.05), aRankedCandidate(2, "passed", nil, 0.12),
		}, expectedSymbols: []string{"B", "A"}},
		{name: "passed first, the rest in pasted order", candidates: []entities.SymbolScreeningCandidate{
			aRankedCandidate(1, "rejected", nil, 0), aRankedCandidate(2, "passed", nil, 0.1),
			aRankedCandidate(3, "rejected", nil, 0),
		}, expectedSymbols: []string{"B", "A", "C"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expectedSymbols, rankedSymbols(testCase.candidates))
		})
	}
}

func aScreeningRunIn(status string) domains.SymbolScreeningRunDomain {
	return domains.NewSymbolScreeningRunDomain(entities.SymbolScreeningRun{ID: 3, Status: status, CandidateCount: 4})
}

func TestSymbolScreeningRunCountsProgressInCandidates(t *testing.T) {
	assert.Equal(t, 50, aScreeningRunIn("running").Advanced(2).ToDto().ProgressPercent)
	assert.Equal(t, 100, aScreeningRunIn("running").Succeeded("", screeningNow).ToDto().ProgressPercent)
}

func TestSymbolScreeningRunCancelsOnlyWhileActive(t *testing.T) {
	_, runningError := aScreeningRunIn("running").Cancelled(screeningNow)
	_, endedError := aScreeningRunIn("succeeded").Cancelled(screeningNow)

	assert.NoError(t, runningError)
	assert.ErrorIs(t, endedError, domains.ErrSymbolScreeningRunAlreadyEnded)
}

func TestSymbolScreeningRunWordsWhyItFailed(t *testing.T) {
	testCases := []struct {
		name           string
		failure        error
		interrupted    bool
		expectedReason string
	}{
		{name: "time allowance spent", failure: domains.ErrBacktestTimeAllowanceSpent, expectedReason: "超過時間上限"},
		{name: "restart", interrupted: true, expectedReason: "被重新啟動打斷"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			failed := aScreeningRunIn("running").FailedFor(testCase.failure, testCase.interrupted, screeningNow)

			assert.Equal(t, "failed", failed.ToDto().Status)
			assert.Equal(t, testCase.expectedReason, failed.ToDto().FailureReason)
		})
	}
}

func TestSymbolScreeningAdmissionAllowsOneActiveRunPerAdministrator(t *testing.T) {
	assert.NoError(t, domains.NewSymbolScreeningAdmissionDomain().Admit(0))
	assert.ErrorIs(t, domains.NewSymbolScreeningAdmissionDomain().Admit(1), domains.ErrSymbolScreeningRunAlreadyActive)
}
