package domains_test

import (
	"errors"
	"strings"
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

func TestSymbolScreeningProfileRefusesTheRestOfWhatItCannotUse(t *testing.T) {
	halfLeverage := decimal.RequireFromString("0.5")
	tooCostly := decimal.NewFromInt(101)
	testCases := []struct {
		name            string
		change          func(writeDto *dto.SymbolScreeningProfileWriteDto)
		expectedMessage string
	}{
		{name: "no script", change: func(writeDto *dto.SymbolScreeningProfileWriteDto) {
			writeDto.StrategyScriptID = 0
		}, expectedMessage: "請指定策略腳本"},
		{name: "an unknown trading mode", change: func(writeDto *dto.SymbolScreeningProfileWriteDto) {
			writeDto.TradingMode = "sideways"
		}, expectedMessage: "合約的交易模式只能是"},
		{name: "leverage under one", change: func(writeDto *dto.SymbolScreeningProfileWriteDto) {
			writeDto.Leverage = &halfLeverage
		}, expectedMessage: "槓桿倍數超出可接受的範圍"},
		{name: "a fee above a hundred percent", change: func(writeDto *dto.SymbolScreeningProfileWriteDto) {
			writeDto.EntryCostPercentage = &tooCostly
		}, expectedMessage: "手續費百分比超出可接受的範圍"},
		{name: "an unreadable baseline symbol", change: func(writeDto *dto.SymbolScreeningProfileWriteDto) {
			writeDto.BaselineSymbols = []string{"BTC-USDT"}
		}, expectedMessage: "BTC-USDT 只能有英文字母與數字"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writeDto := aScreeningProfileWrite()
			testCase.change(&writeDto)

			_, profileError := domains.NewSymbolScreeningProfileDomain(7, writeDto)

			require.Error(t, profileError)
			assert.Contains(t, profileError.Error(), testCase.expectedMessage)
		})
	}
}

func TestSymbolScreeningProfileRefusesKnobsTheScriptDoesNotHave(t *testing.T) {
	writeDto := aScreeningProfileWrite()
	writeDto.ParameterValues = []dto.StrategyScriptParameterValueDto{{Name: "沒有這個旋鈕", Value: 3}}
	profile, profileError := domains.NewSymbolScreeningProfileDomain(7, writeDto)
	require.NoError(t, profileError)
	badlyDeclared := aContractSignalScript()
	badlyDeclared.Parameters = []dto.StrategyScriptParameterWriteDto{{Name: "x", Kind: "nonsense", DefaultValue: 1}}

	assert.ErrorIs(t, profile.AcceptScript(aContractSignalScript()), domains.ErrSymbolScreeningValidation)
	assert.ErrorIs(t, profile.AcceptScript(badlyDeclared), domains.ErrSymbolScreeningValidation)
}

func TestSymbolScreeningRunReadsAndWordsEveryState(t *testing.T) {
	assert.Equal(t, "failed", domains.NewSymbolScreeningRunDomain(entities.SymbolScreeningRun{Status: "exploded"}).ToDto().Status)
	assert.Equal(t, 0, domains.NewSymbolScreeningRunDomain(entities.SymbolScreeningRun{Status: "running"}).ProgressPercent())
	assert.Equal(t, "第 3 行除以零",
		aScreeningRunIn("running").FailedFor(errors.New("第 3 行除以零"), false, screeningNow).ToDto().FailureReason)
	assert.Equal(t, "篩選服務壞掉了，這筆沒有跑完",
		aScreeningRunIn("running").FailedFor(domains.ErrSymbolScreeningBrokeDown, false, screeningNow).ToDto().FailureReason)

	finished := aScreeningRunIn("queued").Claimed("replica-a", screeningNow).Succeeded("", screeningNow.Add(time.Hour)).ToDto()
	require.NotNil(t, finished.StartedAt)
	require.NotNil(t, finished.FinishedAt)
	assert.Equal(t, screeningNow, *finished.StartedAt)
	assert.Equal(t, screeningNow.Add(time.Hour), *finished.FinishedAt)
}

func TestSymbolScreeningCandidatesShowInPastedOrderUntilRanked(t *testing.T) {
	candidates := []entities.SymbolScreeningCandidate{
		{Position: 2, Symbol: "B", Status: "waiting"}, {Position: 1, Symbol: "A", Status: "passed"},
	}

	detail := aScreeningRunIn("running").ToDetailDto(candidates)

	assert.Equal(t, "A", detail.Candidates[0].Symbol)
	assert.Equal(t, "B", detail.Candidates[1].Symbol)
}

func TestSymbolScreeningRecommendationPutsAPassedCandidateFirstWhicheverSideItIsOn(t *testing.T) {
	assert.Equal(t, []string{"B", "A"}, rankedSymbols([]entities.SymbolScreeningCandidate{
		aRankedCandidate(1, "unfinished", nil, 0), aRankedCandidate(2, "passed", nil, 0.1),
	}))
	assert.Equal(t, []string{"A", "B"}, rankedSymbols([]entities.SymbolScreeningCandidate{
		aRankedCandidate(1, "passed", nil, 0.1), aRankedCandidate(2, "rejected", nil, 0),
	}))
}

func TestSymbolScreeningPortabilityFindsTheBestSegmentWhereverItIs(t *testing.T) {
	_, _, findings := domains.NewSymbolScreeningPortabilityDomain(
		aScreeningPeriod(730, 3).Segments(), segmentResults([]float64{0.01, 3.00, -0.08}, []int{3, 3, 3}), 6,
	).Judge(dto.SymbolScreeningFindingsDto{})

	require.NotNil(t, findings.ReturnWithoutBestSegment)
	assert.InDelta(t, -0.0708, *findings.ReturnWithoutBestSegment, 0.00001)
}

func TestSymbolScreeningTimelineKeepsOnlyHoldingsInsideThePeriod(t *testing.T) {
	periodStart := screeningPeriodEnd.Add(-100 * 24 * time.Hour)
	// Held from before the period to day 90, and from day 5 to after the period; one trade lies wholly before it.
	candidate := domains.NewSymbolScreeningHoldingTimelineDomain(append(holdingsFrom([2]int{120, 90}, [2]int{300, 200}),
		dto.ContractClosedTradeDto{
			EntryTime: screeningPeriodEnd.Add(-5 * 24 * time.Hour), ExitTime: screeningPeriodEnd.Add(24 * time.Hour),
		}), periodStart, screeningPeriodEnd)
	baseline := domains.NewSymbolScreeningHoldingTimelineDomain(holdingsFrom([2]int{100, 95}), periodStart, screeningPeriodEnd)

	contribution := candidate.ContributionAgainst(baseline)

	assert.InDelta(t, 15.0/100.0, contribution.InMarketShare, 0.0001)
	assert.InDelta(t, 5.0/15.0, contribution.OverlapShare, 0.0001)
	assert.Equal(t, 85, contribution.CombinedLongestGapDays)
}

func TestSymbolScreeningPeriodFallsBackWhenTheSnapshotCannotSay(t *testing.T) {
	snapshot := aScreeningSnapshot(180, 2)
	snapshot.AggregationInterval = "15m"
	snapshot.Parameters = []dto.StrategyScriptParameterWriteDto{{Name: "x", Kind: "nonsense", DefaultValue: 1}}

	period := domains.NewSymbolScreeningPeriodDomain(snapshot, screeningNow, 30)

	assert.Equal(t, period.StartTime().Add(-30*24*time.Hour), period.FetchStartTime())
	assert.Equal(t, "15m", period.FetchWindow("LINKUSDT").Interval)
}

func TestSymbolScreeningPeriodWarmsUpByTheScriptsOwnLookBack(t *testing.T) {
	snapshot := aScreeningSnapshot(180, 2)
	snapshot.Parameters = []dto.StrategyScriptParameterWriteDto{{Name: "讀取根數", Kind: "lookbackCount", DefaultValue: 50}}
	snapshot.ParameterValues = []dto.StrategyScriptParameterValueDto{{Name: "讀取根數", Value: 60}}

	period := domains.NewSymbolScreeningPeriodDomain(snapshot, screeningNow, 200)

	assert.Equal(t, period.StartTime().Add(-60*24*time.Hour), period.FetchStartTime())
}

func TestContractReplayInputFiltersWhatAReplayReads(t *testing.T) {
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	kCandles := []entities.KCandleContract{{OpenTime: start}, {OpenTime: start.Add(4 * time.Hour)}, {OpenTime: start.Add(8 * time.Hour)}}
	settlements := []entities.ContractFundingRateSettlement{
		{SettlementTime: start.Add(-8 * time.Hour)}, {SettlementTime: start}, {SettlementTime: start.Add(8 * time.Hour)},
	}
	replayInput := domains.NewContractReplayInputDomain(
		"LINKUSDT", screeningSpecificationFor("LINKUSDT"), nil, kCandles, settlements, screeningNow)
	query, queryError := domains.NewKCandleQueryDomain(dto.KCandleQueryDto{
		Symbol: "LINKUSDT", StartTime: start.Add(time.Hour), EndTime: start.Add(8 * time.Hour),
	})
	require.NoError(t, queryError)

	assert.Len(t, replayInput.CandlesIn(query), 2)
	assert.Len(t, replayInput.SettlementsIn(query), 1)
	latest, found := replayInput.LatestSettlementBefore(start.Add(time.Hour))
	require.True(t, found)
	assert.Equal(t, start, latest.SettlementTime)
	_, foundNone := replayInput.LatestSettlementBefore(start.Add(-9 * time.Hour))
	assert.False(t, foundNone)
}

func screeningSpecificationFor(symbol string) vo.ContractTradingSpecificationVo {
	return vo.ContractTradingSpecificationVo{
		Symbol: symbol, TickSize: decimal.RequireFromString("0.001"), QuantityStep: decimal.RequireFromString("0.01"),
		MinimumQuantity: decimal.RequireFromString("0.01"), MinimumNotional: decimal.NewFromInt(5),
		MaintenanceMarginRate: decimal.RequireFromString("0.01"), LiquidationFeeRate: decimal.RequireFromString("0.005"),
	}
}

func TestContractReplayInputTradingRules(t *testing.T) {
	brokenSpecification := screeningSpecificationFor("LINKUSDT")
	brokenSpecification.TickSize = decimal.Zero
	_, brokenError := domains.NewContractReplayInputDomain(
		"LINKUSDT", brokenSpecification, nil, nil, nil, screeningNow).TradingRules()

	ladderRules, ladderError := domains.NewContractReplayInputDomain("LINKUSDT", screeningSpecificationFor("LINKUSDT"),
		[]vo.ContractMaintenanceMarginTierVo{{Tier: 1, NotionalCap: decimal.NewFromInt(50_000),
			MaintenanceMarginRate: decimal.RequireFromString("0.01"), MaximumLeverage: 20}},
		nil, nil, screeningNow).TradingRules()
	unreadableLadderRules, unreadableLadderError := domains.NewContractReplayInputDomain("LINKUSDT",
		screeningSpecificationFor("LINKUSDT"), []vo.ContractMaintenanceMarginTierVo{{Tier: 0}}, nil, nil, screeningNow).
		TradingRules()

	assert.ErrorIs(t, brokenError, domains.ErrContractTradingSpecificationValidation)
	require.NoError(t, ladderError)
	maximumLeverage, isLimited := ladderRules.MaximumLeverage()
	assert.True(t, isLimited)
	assert.Equal(t, 20, maximumLeverage)
	require.NoError(t, unreadableLadderError)
	_, unreadableIsLimited := unreadableLadderRules.MaximumLeverage()
	assert.False(t, unreadableIsLimited)
	assert.Equal(t, "smallestTier", unreadableLadderRules.MaintenanceMarginBasisDto().Kind)
	assert.Equal(t, "tiers", ladderRules.MaintenanceMarginBasisDto().Kind)
}

func TestSymbolScreeningProfileAcceptsExactlyTheEdgesOfEachRange(t *testing.T) {
	figure := func(value int) *int { return &value }
	testCases := []struct {
		name           string
		change         func(writeDto *dto.SymbolScreeningProfileWriteDto)
		expectAccepted bool
	}{
		{name: "history 180 days", change: func(w *dto.SymbolScreeningProfileWriteDto) { w.HistoryDays = figure(180) }, expectAccepted: true},
		{name: "history 179 days", change: func(w *dto.SymbolScreeningProfileWriteDto) { w.HistoryDays = figure(179) }},
		{name: "history 1825 days", change: func(w *dto.SymbolScreeningProfileWriteDto) { w.HistoryDays = figure(1825) }, expectAccepted: true},
		{name: "history 1826 days", change: func(w *dto.SymbolScreeningProfileWriteDto) { w.HistoryDays = figure(1826) }},
		{name: "six segments", change: func(w *dto.SymbolScreeningProfileWriteDto) { w.SegmentCount = figure(6) }, expectAccepted: true},
		{name: "seven segments", change: func(w *dto.SymbolScreeningProfileWriteDto) { w.SegmentCount = figure(7) }},
		{name: "one trade", change: func(w *dto.SymbolScreeningProfileWriteDto) { w.MinimumTradeCount = figure(1) }, expectAccepted: true},
		{name: "no trades", change: func(w *dto.SymbolScreeningProfileWriteDto) { w.MinimumTradeCount = figure(0) }},
		{name: "two hundred trades", change: func(w *dto.SymbolScreeningProfileWriteDto) { w.MinimumTradeCount = figure(200) }, expectAccepted: true},
		{name: "two hundred and one trades", change: func(w *dto.SymbolScreeningProfileWriteDto) { w.MinimumTradeCount = figure(201) }},
		{name: "ten baseline symbols", change: func(w *dto.SymbolScreeningProfileWriteDto) {
			w.BaselineSymbols = strings.Split(distinctSymbols(10), ",")
		}, expectAccepted: true},
		{name: "eleven baseline symbols", change: func(w *dto.SymbolScreeningProfileWriteDto) {
			w.BaselineSymbols = strings.Split(distinctSymbols(11), ",")
		}},
		{name: "a hundred and twenty-eight character name", change: func(w *dto.SymbolScreeningProfileWriteDto) {
			w.Name = strings.Repeat("篩", 128)
		}, expectAccepted: true},
		{name: "a hundred and twenty-nine character name", change: func(w *dto.SymbolScreeningProfileWriteDto) {
			w.Name = strings.Repeat("篩", 129)
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writeDto := aScreeningProfileWrite()
			testCase.change(&writeDto)

			_, profileError := domains.NewSymbolScreeningProfileDomain(7, writeDto)

			if testCase.expectAccepted {
				assert.NoError(t, profileError)
			} else {
				assert.ErrorIs(t, profileError, domains.ErrSymbolScreeningValidation)
			}
		})
	}
}

func TestAPositionStillHeldAtASegmentEndCountsAsHeldUntilThatEnd(t *testing.T) {
	period := domains.NewSymbolScreeningPeriodDomain(aScreeningSnapshot(100, 2), screeningPeriodEnd.Add(time.Hour), 0)
	firstSegmentEnd := period.Segments()[0].EndTime
	openedInFirst := firstSegmentEnd.Add(-48 * time.Hour)
	openedInSecond := screeningPeriodEnd.Add(-24 * time.Hour)

	trades := period.ClosedTradesOf([]dto.ContractBacktestResultDto{
		{ClosedTrades: holdingsFrom([2]int{90, 80}), OpenPositionEntryTime: &openedInFirst},
		{OpenPositionEntryTime: &openedInSecond},
	})

	require.Len(t, trades, 3)
	assert.Equal(t, dto.ContractClosedTradeDto{EntryTime: openedInFirst, ExitTime: firstSegmentEnd}, trades[1])
	assert.Equal(t, dto.ContractClosedTradeDto{EntryTime: openedInSecond, ExitTime: screeningPeriodEnd}, trades[2])
	assert.Len(t, period.ClosedTradesOf([]dto.ContractBacktestResultDto{{ClosedTrades: holdingsFrom([2]int{10, 8})}}), 1)
}

func TestSymbolScreeningContributionCountsAPositionHeldAtTheEndOfASegment(t *testing.T) {
	period := domains.NewSymbolScreeningPeriodDomain(aScreeningSnapshot(100, 1), screeningPeriodEnd.Add(time.Hour), 0)
	openedAt := screeningPeriodEnd.Add(-40 * 24 * time.Hour)
	baseline := domains.NewSymbolScreeningBaselineDomain(period).
		Including([]dto.ContractBacktestResultDto{{ClosedTrades: holdingsFrom([2]int{100, 60})}})

	compared := baseline.Compared(
		[]entities.SymbolScreeningCandidate{{Symbol: "LINKUSDT", Status: "passed"}},
		map[string][]dto.ContractClosedTradeDto{
			"LINKUSDT": period.ClosedTradesOf([]dto.ContractBacktestResultDto{{OpenPositionEntryTime: &openedAt}}),
		})

	require.NotNil(t, compared[0].Findings.Contribution)
	assert.Equal(t, 60, compared[0].Findings.Contribution.BaselineLongestGapDays)
	assert.Equal(t, 20, compared[0].Findings.Contribution.CombinedLongestGapDays)
}

func TestSymbolScreeningBaselineThatCannotBeReplayedComparesNothing(t *testing.T) {
	period := domains.NewSymbolScreeningPeriodDomain(aScreeningSnapshot(100, 1), screeningPeriodEnd.Add(time.Hour), 0)
	candidates := []entities.SymbolScreeningCandidate{
		{Symbol: "LINKUSDT", Status: "passed"}, {Symbol: "FOOUSDT", Status: "rejected"},
	}
	available := domains.NewSymbolScreeningBaselineDomain(period).
		Including([]dto.ContractBacktestResultDto{{ClosedTrades: holdingsFrom([2]int{100, 60})}})
	unavailable := available.Unavailable("BTCUSDT", domains.ErrSymbolScreeningSymbolNotTrading)

	comparedWhileAvailable := available.Compared(candidates, map[string][]dto.ContractClosedTradeDto{})
	comparedWhileUnavailable := unavailable.Compared(candidates, map[string][]dto.ContractClosedTradeDto{})

	assert.Empty(t, available.UnavailableReason())
	assert.NotNil(t, comparedWhileAvailable[0].Findings.Contribution)
	assert.Nil(t, comparedWhileAvailable[1].Findings.Contribution)
	assert.Nil(t, candidates[0].Findings.Contribution, "the candidates handed in are left as they were")
	assert.Equal(t, "現有組合標的 BTCUSDT 無法比較：目前沒有在交易", unavailable.UnavailableReason())
	assert.Nil(t, comparedWhileUnavailable[0].Findings.Contribution)
}
