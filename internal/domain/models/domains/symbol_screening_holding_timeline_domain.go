package domains

import (
	"slices"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

type symbolScreeningHolding struct {
	startTime time.Time
	endTime   time.Time
}

// SymbolScreeningHoldingTimelineDomain is when a set of symbols held a position during the screened period, merged so overlaps count once.
type SymbolScreeningHoldingTimelineDomain struct {
	holdings  []symbolScreeningHolding
	startTime time.Time
	endTime   time.Time
}

func NewSymbolScreeningHoldingTimelineDomain(
	closedTrades []dto.ContractClosedTradeDto, startTime time.Time, endTime time.Time,
) SymbolScreeningHoldingTimelineDomain {
	holdings := make([]symbolScreeningHolding, 0, len(closedTrades))
	for _, closedTrade := range closedTrades {
		holdingStart := closedTrade.EntryTime.UTC()
		if holdingStart.Before(startTime) {
			holdingStart = startTime
		}
		holdingEnd := closedTrade.ExitTime.UTC()
		if holdingEnd.After(endTime) {
			holdingEnd = endTime
		}
		if holdingEnd.After(holdingStart) {
			holdings = append(holdings, symbolScreeningHolding{startTime: holdingStart, endTime: holdingEnd})
		}
	}
	slices.SortFunc(holdings, func(earlier, later symbolScreeningHolding) int {
		return earlier.startTime.Compare(later.startTime)
	})

	merged := make([]symbolScreeningHolding, 0, len(holdings))
	for _, holding := range holdings {
		if last := len(merged) - 1; last >= 0 && !holding.startTime.After(merged[last].endTime) {
			if holding.endTime.After(merged[last].endTime) {
				merged[last].endTime = holding.endTime
			}
			continue
		}
		merged = append(merged, holding)
	}

	return SymbolScreeningHoldingTimelineDomain{holdings: merged, startTime: startTime, endTime: endTime}
}

// ContributionAgainst compares this candidate with the baseline it would join.
func (timelineDomain SymbolScreeningHoldingTimelineDomain) ContributionAgainst(
	baseline SymbolScreeningHoldingTimelineDomain,
) dto.SymbolScreeningContributionDto {
	combinedTrades := []dto.ContractClosedTradeDto{}
	overlap := time.Duration(0)
	for _, holding := range slices.Concat(timelineDomain.holdings, baseline.holdings) {
		combinedTrades = append(combinedTrades, dto.ContractClosedTradeDto{EntryTime: holding.startTime, ExitTime: holding.endTime})
	}
	for _, own := range timelineDomain.holdings {
		for _, other := range baseline.holdings {
			overlapStart := own.startTime
			if other.startTime.After(overlapStart) {
				overlapStart = other.startTime
			}
			overlapEnd := own.endTime
			if other.endTime.Before(overlapEnd) {
				overlapEnd = other.endTime
			}
			if overlapEnd.After(overlapStart) {
				overlap += overlapEnd.Sub(overlapStart)
			}
		}
	}
	combined := NewSymbolScreeningHoldingTimelineDomain(combinedTrades, timelineDomain.startTime, timelineDomain.endTime)
	longestGapDaysOf := func(timeline SymbolScreeningHoldingTimelineDomain) int {
		longestGap := time.Duration(0)
		cursor := timeline.startTime
		for _, holding := range timeline.holdings {
			longestGap = max(longestGap, holding.startTime.Sub(cursor))
			cursor = holding.endTime
		}

		return int(max(longestGap, timeline.endTime.Sub(cursor)) / symbolScreeningDay)
	}

	held := time.Duration(0)
	for _, holding := range timelineDomain.holdings {
		held += holding.endTime.Sub(holding.startTime)
	}
	overlapShare := 0.0
	if held > 0 {
		overlapShare = float64(overlap) / float64(held)
	}
	inMarketShare := 0.0
	if total := timelineDomain.endTime.Sub(timelineDomain.startTime); total > 0 {
		inMarketShare = float64(held) / float64(total)
	}
	baselineLongestGapDays := longestGapDaysOf(baseline)
	combinedLongestGapDays := longestGapDaysOf(combined)

	return dto.SymbolScreeningContributionDto{
		InMarketShare:          inMarketShare,
		OverlapShare:           overlapShare,
		BaselineLongestGapDays: baselineLongestGapDays,
		CombinedLongestGapDays: combinedLongestGapDays,
		GapReductionDays:       baselineLongestGapDays - combinedLongestGapDays,
	}
}
