package domains

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

const (
	// orderFlowMaximumDays bounds how much trade history one run may fetch.
	orderFlowMaximumDays = 31
	// orderFlowMaximumLevelsPerBar keeps a bar readable; more means the price step is too fine.
	orderFlowMaximumLevelsPerBar = 1000
	// orderFlowMaximumLevelsPerRun keeps one run inside the script compartment's memory cap.
	orderFlowMaximumLevelsPerRun = 2_000_000
)

// OrderFlowReachDomain is the order flow one run needs: the UTC days its bars span and the price step to tally at; it turns gathered days into each bar's footprint.
type OrderFlowReachDomain struct {
	symbol    string
	priceStep OrderFlowPriceStepDomain
	interval  time.Duration
	// start is the first bar's open; end is the last bar's close.
	start time.Time
	end   time.Time
}

// Days are the UTC day starts the bars touch, oldest first.
func (reachDomain OrderFlowReachDomain) Days() []time.Time {
	firstDay := reachDomain.start.Truncate(orderFlowDayLength)
	lastDay := reachDomain.end.Add(-time.Nanosecond).Truncate(orderFlowDayLength)

	days := make([]time.Time, 0, orderFlowMaximumDays)
	for day := firstDay; !day.After(lastDay); day = day.Add(orderFlowDayLength) {
		days = append(days, day)
	}

	return days
}

// requireWithinDays refuses a stretch spanning more UTC days than one run may hold; shared by the bars a run reads and the stretch a replay asks for before anything is read.
func (reachDomain OrderFlowReachDomain) requireWithinDays() error {
	if dayCount := len(reachDomain.Days()); dayCount > orderFlowMaximumDays {
		return fmt.Errorf(
			"%w: %w: 訂單流一次至多涵蓋 %d 天，這一次要 %d 天，請縮短時間或改用較細的彙總刻度",
			ErrContractOrderFlowRefused, ErrContractOrderFlowReachTooLong, orderFlowMaximumDays, dayCount)
	}

	return nil
}

// End is the last bar's close; nothing at or after it is needed.
func (reachDomain OrderFlowReachDomain) End() time.Time {
	return reachDomain.end
}

func (reachDomain OrderFlowReachDomain) Symbol() string {
	return reachDomain.symbol
}

func (reachDomain OrderFlowReachDomain) PriceStep() OrderFlowPriceStepDomain {
	return reachDomain.priceStep
}

// Footprinting gives each bar the trades of its own minutes (open included, close excluded), the same level in different minutes added together; the days must cover every bar.
func (reachDomain OrderFlowReachDomain) Footprinting(
	bars []vo.ContractKCandleVo, days []ContractOrderFlowDayDomain,
) ([]vo.ContractKCandleVo, error) {
	earliestFirstDays := slices.Clone(days)
	slices.SortFunc(earliestFirstDays, func(former, latter ContractOrderFlowDayDomain) int {
		return former.dayStart.Compare(latter.dayStart)
	})
	minutes := make([]orderFlowMinute, 0)
	for _, day := range earliestFirstDays {
		minutes = append(minutes, day.minutes...)
	}

	intervalSeconds := int64(reachDomain.interval / time.Second)
	footprintedBars := make([]vo.ContractKCandleVo, 0, len(bars))
	levelCount := 0
	nextMinute := 0
	for _, bar := range bars {
		barClose := bar.OpenTimeUnixSeconds + intervalSeconds
		for nextMinute < len(minutes) && minutes[nextMinute].openUnix < bar.OpenTimeUnixSeconds {
			nextMinute++
		}

		tallies := map[int64]*orderFlowTally{}
		for ; nextMinute < len(minutes) && minutes[nextMinute].openUnix < barClose; nextMinute++ {
			for _, minuteLevel := range minutes[nextMinute].levels {
				tally, hasLevel := tallies[minuteLevel.level]
				if !hasLevel {
					tally = &orderFlowTally{}
					tallies[minuteLevel.level] = tally
				}
				tally.buyVolume += minuteLevel.buyVolume
				tally.sellVolume += minuteLevel.sellVolume
				tally.tradeCount += minuteLevel.tradeCount
			}
		}

		if len(tallies) > orderFlowMaximumLevelsPerBar {
			return nil, fmt.Errorf(
				"%w: 訂單流價位寬度太細，一格至多 %d 個價位（%s 那一格有 %d 個），請放寬價位寬度",
				ErrContractOrderFlowRefused, orderFlowMaximumLevelsPerBar,
				time.Unix(bar.OpenTimeUnixSeconds, 0).UTC().Format(time.RFC3339), len(tallies))
		}
		levelCount += len(tallies)
		if levelCount > orderFlowMaximumLevelsPerRun {
			return nil, fmt.Errorf(
				"%w: 訂單流價位寬度太細或時間太長，一次至多 %d 個價位，請放寬價位寬度或縮短時間",
				ErrContractOrderFlowRefused, orderFlowMaximumLevelsPerRun)
		}

		// Levels are listed lowest first, and the busiest is kept on the lower level when two tie.
		levels := make([]int64, 0, len(tallies))
		for level := range tallies {
			levels = append(levels, level)
		}
		slices.SortFunc(levels, cmp.Compare[int64])

		footprint := vo.OrderFlowFootprintVo{
			PriceStep: reachDomain.priceStep.Value(),
			Levels:    make([]vo.OrderFlowPriceLevelVo, 0, len(levels)),
		}
		busiestVolume := 0.0
		for _, level := range levels {
			tally := tallies[level]
			price := reachDomain.priceStep.PriceOf(level)
			footprint.Levels = append(footprint.Levels, vo.OrderFlowPriceLevelVo{
				Price:      price,
				BuyVolume:  tally.buyVolume,
				SellVolume: tally.sellVolume,
				TradeCount: tally.tradeCount,
			})
			footprint.Delta += tally.buyVolume - tally.sellVolume
			if levelVolume := tally.buyVolume + tally.sellVolume; levelVolume > busiestVolume {
				busiestVolume = levelVolume
				footprint.PointOfControl = price
			}
		}

		footprintedBar := bar
		footprintedBar.Footprint = footprint
		footprintedBars = append(footprintedBars, footprintedBar)
	}

	return footprintedBars, nil
}
