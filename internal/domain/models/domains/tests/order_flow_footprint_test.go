package domains_test

import (
	"fmt"
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

// orderFlowNow is long after every bar here, so every bar has finished.
var orderFlowNow = time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)

var orderFlowMorning = time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)

func orderFlowStep(t *testing.T, width float64) domains.OrderFlowPriceStepDomain {
	t.Helper()

	priceStep, stepError := domains.NewOrderFlowPriceStepDomain(width)
	require.NoError(t, stepError)

	return priceStep
}

var orderFlowIntervals = map[string]time.Duration{"1m": time.Minute, "1h": time.Hour, "1d": 24 * time.Hour}

// orderFlowAlignment lines up one stored contract K candle per minute from first to last (inclusive) at the given interval.
func orderFlowAlignment(
	t *testing.T, interval string, first time.Time, last time.Time,
) domains.ContractKCandleAlignmentDomain {
	t.Helper()

	newestFirst := make([]entities.KCandleContract, 0)
	for openTime := last; !openTime.Before(first); openTime = openTime.Add(-time.Minute) {
		newestFirst = append(newestFirst, entities.KCandleContract{
			Symbol: "BTCUSDT", OpenTime: openTime,
			Open: decimal.NewFromInt(100), High: decimal.NewFromInt(100),
			Low: decimal.NewFromInt(100), Close: decimal.NewFromInt(100),
		})
	}
	calculationDomain, validationError := domains.NewIndicatorCalculationDomain(dto.IndicatorCalculationRequestDto{
		Symbol:              "BTCUSDT",
		StartTime:           first,
		EndTime:             last.Add(orderFlowIntervals[interval]),
		AggregationInterval: interval,
	}, cryptoMarket(), 1_000_000, orderFlowNow)
	require.NoError(t, validationError)

	alignment, selectionError := calculationDomain.SelectContractInput(newestFirst)
	require.NoError(t, selectionError)

	return alignment
}

type orderFlowTrade struct {
	at       time.Time
	price    string
	quantity float64
	buyer    bool
}

// gatheredDay tallies the trades into one whole UTC day, the way a day's archive arrives.
func gatheredDay(
	t *testing.T, priceStep domains.OrderFlowPriceStepDomain, day time.Time, trades ...orderFlowTrade,
) domains.ContractOrderFlowDayDomain {
	t.Helper()

	ledger := domains.NewOrderFlowMinuteLedgerDomain(priceStep)
	for _, trade := range trades {
		ledger.Record(vo.ContractAggregatedTradeVo{
			Price: decimal.RequireFromString(trade.price), Quantity: trade.quantity,
			TradeCount: 1, TradeTime: trade.at, BuyerIsTaker: trade.buyer,
		})
	}

	dayStart := day.Truncate(24 * time.Hour)

	return domains.NewContractOrderFlowDayDomain(priceStep, dayStart).Extending(ledger, dayStart.Add(24*time.Hour))
}

func footprintsOf(
	t *testing.T, alignment domains.ContractKCandleAlignmentDomain, priceStep domains.OrderFlowPriceStepDomain,
	days ...domains.ContractOrderFlowDayDomain,
) []vo.OrderFlowFootprintVo {
	t.Helper()

	reach, reachError := alignment.OrderFlowReach(priceStep)
	require.NoError(t, reachError)
	bars, footprintError := reach.Footprinting(alignment.Aligning(nil, nil), days)
	require.NoError(t, footprintError)

	footprints := make([]vo.OrderFlowFootprintVo, 0, len(bars))
	for _, bar := range bars {
		footprints = append(footprints, bar.Footprint)
	}

	return footprints
}

func TestAMinuteIsTalliedByPriceLevel(t *testing.T) {
	priceStep := orderFlowStep(t, 10)
	alignment := orderFlowAlignment(t, "1m", orderFlowMorning, orderFlowMorning)
	day := gatheredDay(t, priceStep, orderFlowMorning,
		orderFlowTrade{at: orderFlowMorning.Add(5 * time.Second), price: "84157", quantity: 2, buyer: true},
		orderFlowTrade{at: orderFlowMorning.Add(10 * time.Second), price: "84151", quantity: 1, buyer: false},
		orderFlowTrade{at: orderFlowMorning.Add(20 * time.Second), price: "84163", quantity: 3, buyer: true},
	)

	footprints := footprintsOf(t, alignment, priceStep, day)

	assert.Equal(t, []vo.OrderFlowFootprintVo{{
		PriceStep: 10,
		Levels: []vo.OrderFlowPriceLevelVo{
			{Price: 84150, BuyVolume: 2, SellVolume: 1, TradeCount: 2},
			{Price: 84160, BuyVolume: 3, SellVolume: 0, TradeCount: 1},
		},
		Delta:          4,
		PointOfControl: 84150,
	}}, footprints)
}

func TestATradeOnTheCloseBelongsToTheNextBar(t *testing.T) {
	priceStep := orderFlowStep(t, 10)
	alignment := orderFlowAlignment(t, "1m", orderFlowMorning, orderFlowMorning.Add(time.Minute))
	day := gatheredDay(t, priceStep, orderFlowMorning,
		orderFlowTrade{at: orderFlowMorning.Add(time.Minute), price: "84157", quantity: 2, buyer: true},
	)

	footprints := footprintsOf(t, alignment, priceStep, day)

	require.Len(t, footprints, 2)
	assert.Empty(t, footprints[0].Levels, "09:00 那一格沒有這筆成交")
	assert.Equal(t, []vo.OrderFlowPriceLevelVo{{Price: 84150, BuyVolume: 2, TradeCount: 1}}, footprints[1].Levels)
}

func TestACoarserBarAddsTheSameLevelAcrossItsMinutes(t *testing.T) {
	priceStep := orderFlowStep(t, 10)
	alignment := orderFlowAlignment(t, "1h", orderFlowMorning, orderFlowMorning.Add(59*time.Minute))
	day := gatheredDay(t, priceStep, orderFlowMorning,
		orderFlowTrade{at: orderFlowMorning, price: "84150", quantity: 2, buyer: true},
		orderFlowTrade{at: orderFlowMorning.Add(30 * time.Minute), price: "84155", quantity: 2, buyer: true},
	)

	footprints := footprintsOf(t, alignment, priceStep, day)

	require.Len(t, footprints, 1)
	assert.Equal(t, []vo.OrderFlowPriceLevelVo{{Price: 84150, BuyVolume: 4, TradeCount: 2}}, footprints[0].Levels)
}

func TestABarWithNoTradesHasAnEmptyFootprintAtThisRunsStep(t *testing.T) {
	priceStep := orderFlowStep(t, 25)
	alignment := orderFlowAlignment(t, "1m", orderFlowMorning, orderFlowMorning)

	footprints := footprintsOf(t, alignment, priceStep, gatheredDay(t, priceStep, orderFlowMorning))

	require.Len(t, footprints, 1)
	assert.Empty(t, footprints[0].Levels)
	assert.Equal(t, 0.0, footprints[0].Delta)
	assert.Equal(t, 0.0, footprints[0].PointOfControl)
	assert.Equal(t, 25.0, footprints[0].PriceStep)
}

func TestThePointOfControlIsTheBusiestLevel(t *testing.T) {
	priceStep := orderFlowStep(t, 10)
	alignment := orderFlowAlignment(t, "1m", orderFlowMorning, orderFlowMorning)
	day := gatheredDay(t, priceStep, orderFlowMorning,
		orderFlowTrade{at: orderFlowMorning, price: "100", quantity: 1, buyer: true},
		orderFlowTrade{at: orderFlowMorning, price: "110", quantity: 5, buyer: false},
		orderFlowTrade{at: orderFlowMorning, price: "120", quantity: 2, buyer: true},
	)

	footprints := footprintsOf(t, alignment, priceStep, day)

	assert.Equal(t, 110.0, footprints[0].PointOfControl)
	assert.Equal(t, -2.0, footprints[0].Delta)
}

func TestDaysMayBeGivenInAnyOrder(t *testing.T) {
	priceStep := orderFlowStep(t, 10)
	lateNight := time.Date(2026, 10, 12, 23, 59, 0, 0, time.UTC)
	alignment := orderFlowAlignment(t, "1m", lateNight, lateNight.Add(time.Minute))
	firstDay := gatheredDay(t, priceStep, lateNight, orderFlowTrade{at: lateNight, price: "100", quantity: 1, buyer: true})
	secondDay := gatheredDay(t, priceStep, lateNight.Add(time.Minute),
		orderFlowTrade{at: lateNight.Add(time.Minute), price: "200", quantity: 1, buyer: true})

	footprints := footprintsOf(t, alignment, priceStep, secondDay, firstDay)

	require.Len(t, footprints, 2)
	assert.Equal(t, 100.0, footprints[0].Levels[0].Price)
	assert.Equal(t, 200.0, footprints[1].Levels[0].Price)
}

// tradesAtLevels puts one trade on each of that many consecutive levels within one minute.
func tradesAtLevels(minute time.Time, levelCount int) []orderFlowTrade {
	trades := make([]orderFlowTrade, 0, levelCount)
	for level := range levelCount {
		trades = append(trades, orderFlowTrade{at: minute, price: fmt.Sprint(level), quantity: 1, buyer: true})
	}

	return trades
}

func TestABarMayHoldAThousandLevelsButNoMore(t *testing.T) {
	priceStep := orderFlowStep(t, 1)
	alignment := orderFlowAlignment(t, "1m", orderFlowMorning, orderFlowMorning)
	reach, reachError := alignment.OrderFlowReach(priceStep)
	require.NoError(t, reachError)

	_, thousandError := reach.Footprinting(alignment.Aligning(nil, nil),
		[]domains.ContractOrderFlowDayDomain{gatheredDay(t, priceStep, orderFlowMorning, tradesAtLevels(orderFlowMorning, 1000)...)})
	_, overError := reach.Footprinting(alignment.Aligning(nil, nil),
		[]domains.ContractOrderFlowDayDomain{gatheredDay(t, priceStep, orderFlowMorning, tradesAtLevels(orderFlowMorning, 1001)...)})

	assert.NoError(t, thousandError)
	require.ErrorIs(t, overError, domains.ErrContractOrderFlowRefused)
	assert.Contains(t, overError.Error(), "訂單流價位寬度太細，一格至多 1000 個價位")
}

func TestARunMayHoldTwoMillionLevelsButNoMore(t *testing.T) {
	priceStep := orderFlowStep(t, 1)
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	// Two thousand full minutes make exactly two million levels; one more trade in the next minute is one too many.
	trades := make([]orderFlowTrade, 0, 2_000_001)
	for minute := range 2000 {
		trades = append(trades, tradesAtLevels(start.Add(time.Duration(minute)*time.Minute), 1000)...)
	}
	exactly := gatheredDays(t, priceStep, start, trades...)
	oneMore := gatheredDays(t, priceStep, start,
		append(trades, orderFlowTrade{at: start.Add(2000 * time.Minute), price: "1", quantity: 1, buyer: true})...)

	exactAlignment := orderFlowAlignment(t, "1m", start, start.Add(1999*time.Minute))
	exactReach, _ := exactAlignment.OrderFlowReach(priceStep)
	_, exactError := exactReach.Footprinting(exactAlignment.Aligning(nil, nil), exactly)
	overAlignment := orderFlowAlignment(t, "1m", start, start.Add(2000*time.Minute))
	overReach, _ := overAlignment.OrderFlowReach(priceStep)
	_, overError := overReach.Footprinting(overAlignment.Aligning(nil, nil), oneMore)

	assert.NoError(t, exactError)
	require.ErrorIs(t, overError, domains.ErrContractOrderFlowRefused)
	assert.Contains(t, overError.Error(), "一次至多 2000000 個價位")
}

// gatheredDays tallies trades spanning several days into one whole day each.
func gatheredDays(
	t *testing.T, priceStep domains.OrderFlowPriceStepDomain, start time.Time, trades ...orderFlowTrade,
) []domains.ContractOrderFlowDayDomain {
	t.Helper()

	ledger := domains.NewOrderFlowMinuteLedgerDomain(priceStep)
	for _, trade := range trades {
		ledger.Record(vo.ContractAggregatedTradeVo{
			Price: decimal.RequireFromString(trade.price), Quantity: trade.quantity,
			TradeCount: 1, TradeTime: trade.at, BuyerIsTaker: trade.buyer,
		})
	}

	days := make([]domains.ContractOrderFlowDayDomain, 0, 3)
	for day := start.Truncate(24 * time.Hour); day.Before(start.Add(72 * time.Hour)); day = day.Add(24 * time.Hour) {
		days = append(days, domains.NewContractOrderFlowDayDomain(priceStep, day).Extending(ledger, day.Add(24*time.Hour)))
	}

	return days
}

func TestOrderFlowReachesThirtyOneCalendarDaysButNoMore(t *testing.T) {
	priceStep := orderFlowStep(t, 10)
	// From 00:00 on the 1st to the close of the last bar at 24:00 on the 31st is thirty-one days.
	first := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	thirtyOne := orderFlowAlignment(t, "1d", first, time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC))
	thirtyTwo := orderFlowAlignment(t, "1d", first, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))

	reach, thirtyOneError := thirtyOne.OrderFlowReach(priceStep)
	_, thirtyTwoError := thirtyTwo.OrderFlowReach(priceStep)

	require.NoError(t, thirtyOneError)
	assert.Len(t, reach.Days(), 31)
	require.ErrorIs(t, thirtyTwoError, domains.ErrContractOrderFlowRefused)
	assert.Contains(t, thirtyTwoError.Error(), "訂單流一次至多涵蓋 31 天")
}

func TestAReachSpansTheDaysItsBarsTouch(t *testing.T) {
	lateNight := time.Date(2026, 10, 12, 23, 30, 0, 0, time.UTC)
	alignment := orderFlowAlignment(t, "1h", lateNight.Add(-30*time.Minute), lateNight.Add(89*time.Minute))

	reach, reachError := alignment.OrderFlowReach(orderFlowStep(t, 10))

	require.NoError(t, reachError)
	assert.Equal(t, []time.Time{
		time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC),
	}, reach.Days())
	assert.Equal(t, time.Date(2026, 10, 13, 1, 0, 0, 0, time.UTC), reach.End())
}

func TestADayGrowsOnlyByTheMinutesItDidNotHave(t *testing.T) {
	priceStep := orderFlowStep(t, 10)
	firstLedger := domains.NewOrderFlowMinuteLedgerDomain(priceStep)
	firstLedger.Record(vo.ContractAggregatedTradeVo{Price: decimal.NewFromInt(100), Quantity: 1, TradeCount: 1, TradeTime: orderFlowMorning, BuyerIsTaker: true})
	day := domains.NewContractOrderFlowDayDomain(priceStep, orderFlowMorning).Extending(firstLedger, orderFlowMorning.Add(time.Minute))

	secondLedger := domains.NewOrderFlowMinuteLedgerDomain(priceStep)
	// The first minute is already held, so a second account of it is ignored.
	secondLedger.Record(vo.ContractAggregatedTradeVo{Price: decimal.NewFromInt(100), Quantity: 9, TradeCount: 1, TradeTime: orderFlowMorning, BuyerIsTaker: true})
	secondLedger.Record(vo.ContractAggregatedTradeVo{Price: decimal.NewFromInt(100), Quantity: 1, TradeCount: 1, TradeTime: orderFlowMorning.Add(time.Minute), BuyerIsTaker: false})
	extended := day.Extending(secondLedger, orderFlowMorning.Add(2*time.Minute))

	alignment := orderFlowAlignment(t, "1m", orderFlowMorning, orderFlowMorning.Add(time.Minute))
	footprints := footprintsOf(t, alignment, priceStep, extended)

	assert.False(t, day.Covers(orderFlowMorning.Add(2*time.Minute)))
	assert.True(t, extended.Covers(orderFlowMorning.Add(2*time.Minute)))
	assert.Equal(t, 1.0, footprints[0].Levels[0].BuyVolume)
	assert.Equal(t, 1.0, footprints[1].Levels[0].SellVolume)
}

func TestAWholeDayCoversAnyMomentAfterIt(t *testing.T) {
	priceStep := orderFlowStep(t, 10)
	day := gatheredDay(t, priceStep, orderFlowMorning)

	assert.True(t, day.Covers(orderFlowMorning.Add(72*time.Hour)))
	assert.Equal(t, time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC), day.CoveredUntil())
	assert.Equal(t, time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC), day.DayStart())
}

func TestAnInterruptedFetchTrustsOnlyTheMinutesBeforeItsLatestTrade(t *testing.T) {
	ledger := domains.NewOrderFlowMinuteLedgerDomain(orderFlowStep(t, 10))
	ledger.Record(vo.ContractAggregatedTradeVo{Price: decimal.NewFromInt(100), Quantity: 1, TradeTime: orderFlowMorning.Add(3*time.Minute + 59*time.Second)})

	assert.Equal(t, orderFlowMorning.Add(3*time.Minute), ledger.CompleteUntil(orderFlowMorning))
	assert.Equal(t, orderFlowMorning.Add(5*time.Minute), ledger.CompleteUntil(orderFlowMorning.Add(5*time.Minute)),
		"沒有比起點更新的成交時，可信到起點為止")
}

func TestARememberedDayGrowsInSizeWithWhatItHolds(t *testing.T) {
	priceStep := orderFlowStep(t, 1)
	empty := gatheredDay(t, priceStep, orderFlowMorning)
	busy := gatheredDay(t, priceStep, orderFlowMorning, tradesAtLevels(orderFlowMorning, 100)...)

	assert.Greater(t, busy.EstimatedBytes(), empty.EstimatedBytes()+100*16)
}

func TestARecordFoldingSeveralFillsCountsThemAll(t *testing.T) {
	priceStep := orderFlowStep(t, 10)
	ledger := domains.NewOrderFlowMinuteLedgerDomain(priceStep)
	ledger.Record(vo.ContractAggregatedTradeVo{Price: decimal.NewFromInt(100), Quantity: 1, TradeCount: 3, TradeTime: orderFlowMorning, BuyerIsTaker: true})
	day := domains.NewContractOrderFlowDayDomain(priceStep, orderFlowMorning).Extending(ledger, orderFlowMorning.Add(time.Minute))

	footprints := footprintsOf(t, orderFlowAlignment(t, "1m", orderFlowMorning, orderFlowMorning), priceStep, day)

	assert.Equal(t, int64(3), footprints[0].Levels[0].TradeCount)
}
