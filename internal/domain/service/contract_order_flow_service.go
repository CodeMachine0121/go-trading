package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"golang.org/x/sync/errgroup"
)

// contractOrderFlowDayLength is one UTC calendar day, the unit the archive is published in.
const contractOrderFlowDayLength = 24 * time.Hour

// contractOrderFlowConcurrentDays is how many days one run gathers at once; the proxy separately limits archive downloads across all runs.
const contractOrderFlowConcurrentDays = 4

// ContractOrderFlowService attaches order flow footprints to contract bars when the script declared a price step: it works out which days are needed, takes each from memory, the day's archive, or the live venue, and has the reach merge them into bars. Nothing is stored.
type ContractOrderFlowService struct {
	contractTradeHistoryProxy domaininterface.IContractTradeHistoryProxy
	clockProxy                domaininterface.IClockProxy
	memory                    *ContractOrderFlowMemory
}

func NewContractOrderFlowService(
	contractTradeHistoryProxy domaininterface.IContractTradeHistoryProxy,
	clockProxy domaininterface.IClockProxy,
	memory *ContractOrderFlowMemory,
) *ContractOrderFlowService {
	return &ContractOrderFlowService{
		contractTradeHistoryProxy: contractTradeHistoryProxy,
		clockProxy:                clockProxy,
		memory:                    memory,
	}
}

// AttachFootprints returns the bars unchanged when no order flow price step is declared; otherwise every bar carries its footprint. A positive preparation allowance bounds the gathering: running out is ErrContractOrderFlowNotReady, and what was gathered is kept for next time.
func (contractOrderFlowService *ContractOrderFlowService) AttachFootprints(
	executionContext context.Context,
	alignment domains.ContractKCandleAlignmentDomain,
	bars []vo.ContractKCandleVo,
	parameters domains.StrategyScriptParametersDomain,
	preparationAllowance time.Duration,
) ([]vo.ContractKCandleVo, error) {
	priceStep, declaresOrderFlow := parameters.OrderFlowPriceStep()
	if !declaresOrderFlow {
		return bars, nil
	}

	reach, reachError := alignment.OrderFlowReach(priceStep)
	if reachError != nil {
		return nil, reachError
	}

	gatheringContext := executionContext
	if preparationAllowance > 0 {
		boundedContext, stopGathering := context.WithTimeout(executionContext, preparationAllowance)
		defer stopGathering()
		gatheringContext = boundedContext
	}

	reachedDays := reach.Days()
	days := make([]domains.ContractOrderFlowDayDomain, len(reachedDays))
	gathering, groupContext := errgroup.WithContext(gatheringContext)
	gathering.SetLimit(contractOrderFlowConcurrentDays)
	for dayIndex, dayStart := range reachedDays {
		gathering.Go(func() error {
			day, gatherError := contractOrderFlowService.gatherDay(groupContext, reach, dayStart)
			days[dayIndex] = day

			return gatherError
		})
	}

	if gatherError := gathering.Wait(); gatherError != nil {
		switch {
		// The caller's own deadline or cancellation is the caller's to report, e.g. a replay out of time.
		case executionContext.Err() != nil:
			return nil, executionContext.Err()
		case errors.Is(gatherError, context.DeadlineExceeded) || errors.Is(gatherError, context.Canceled):
			return nil, fmt.Errorf("%w：%s 內沒能備齊", domains.ErrContractOrderFlowNotReady, preparationAllowance)
		case errors.Is(gatherError, domains.ErrContractOrderFlowUnavailable):
			return nil, gatherError
		default:
			return nil, fmt.Errorf("%w：%w", domains.ErrContractOrderFlowUnavailable, gatherError)
		}
	}

	return reach.Footprinting(bars, days)
}

// gatherDay holds the day's gathering lock from the recall that finds it missing to the keep that fills it, so two runs never fetch the same day at once; extracted so the deferred release scopes exactly that.
// A day that ended before today comes from its archive; yesterday's archive may not be out yet, so it falls back to the live venue; today always comes live, only the minutes not yet remembered.
func (contractOrderFlowService *ContractOrderFlowService) gatherDay(
	executionContext context.Context, reach domains.OrderFlowReachDomain, dayStart time.Time,
) (domains.ContractOrderFlowDayDomain, error) {
	symbol := reach.Symbol()
	priceStep := reach.PriceStep()
	neededUntil := dayStart.Add(contractOrderFlowDayLength)
	if reach.End().Before(neededUntil) {
		neededUntil = reach.End()
	}

	releaseGathering := contractOrderFlowService.memory.Gathering(symbol, dayStart, priceStep)
	defer releaseGathering()

	day, isRemembered := contractOrderFlowService.memory.Recall(symbol, dayStart, priceStep)
	if isRemembered && day.Covers(neededUntil) {
		return day, nil
	}
	if !isRemembered {
		day = domains.NewContractOrderFlowDayDomain(priceStep, dayStart)
	}

	today := contractOrderFlowService.clockProxy.Now().UTC().Truncate(contractOrderFlowDayLength)
	// Only an untouched past day is read from its archive; a day already partly read live carries on live, so its minutes never come from two sources.
	if dayStart.Before(today) && !day.CoveredUntil().After(dayStart) {
		ledger := domains.NewOrderFlowMinuteLedgerDomain(priceStep)
		isPublished, archiveError := contractOrderFlowService.contractTradeHistoryProxy.VisitDailyTrades(
			executionContext, symbol, dayStart, ledger.Record)
		if archiveError != nil {
			return domains.ContractOrderFlowDayDomain{}, archiveError
		}
		if isPublished {
			day = day.Extending(ledger, dayStart.Add(contractOrderFlowDayLength))
			contractOrderFlowService.memory.Keep(symbol, day)

			return day, nil
		}
		if dayStart.Before(today.Add(-contractOrderFlowDayLength)) {
			return domains.ContractOrderFlowDayDomain{}, fmt.Errorf(
				"%w：%s 的 %s 歷史檔不存在", domains.ErrContractOrderFlowUnavailable,
				symbol, dayStart.Format(time.DateOnly))
		}
	}

	from := day.CoveredUntil()
	ledger := domains.NewOrderFlowMinuteLedgerDomain(priceStep)
	liveError := contractOrderFlowService.contractTradeHistoryProxy.VisitTradesBetween(
		executionContext, symbol, from, neededUntil, ledger.Record)
	if liveError != nil {
		// Whole minutes read before the interruption are kept, so the next attempt only asks for the rest.
		contractOrderFlowService.memory.Keep(symbol, day.Extending(ledger, ledger.CompleteUntil(from)))

		return domains.ContractOrderFlowDayDomain{}, liveError
	}

	day = day.Extending(ledger, neededUntil)
	contractOrderFlowService.memory.Keep(symbol, day)

	return day, nil
}
