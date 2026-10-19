package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"sync"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

const symbolScreeningWriteTimeout = 10 * time.Second

// errSymbolScreeningRunEndedElsewhere stops a runner whose run was cancelled or swept while it worked.
var errSymbolScreeningRunEndedElsewhere = errors.New("symbol screening run ended elsewhere")

type symbolScreeningRunner struct {
	runService       *SymbolScreeningRunService
	runID            uint
	screeningRun     domains.SymbolScreeningRunDomain
	screeningContext context.Context
	stopScreening    func()
	mutex            sync.Mutex
	interrupted      bool
	cancelled        bool
	ended            bool
}

func newSymbolScreeningRunner(
	runService *SymbolScreeningRunService, run domains.SymbolScreeningRunDomain,
) *symbolScreeningRunner {
	baseContext, cancelBase := context.WithCancel(context.Background())
	screeningContext, cancelTimed := baseContext, context.CancelFunc(func() {})
	if runService.timeAllowance > 0 {
		screeningContext, cancelTimed = context.WithTimeoutCause(
			baseContext, runService.timeAllowance, domains.ErrBacktestTimeAllowanceSpent)
	}

	return &symbolScreeningRunner{
		runService:       runService,
		runID:            run.ToEntity().ID,
		screeningRun:     run,
		screeningContext: screeningContext,
		stopScreening: func() {
			cancelTimed()
			cancelBase()
		},
	}
}

// run screens each candidate in the pasted order, then compares the passed ones with the baseline and ranks them all.
func (runner *symbolScreeningRunner) run() {
	service := runner.runService
	defer service.runnerFinished()
	defer runner.stopScreening()
	defer func() {
		if panicValue := recover(); panicValue != nil {
			log.Printf("symbol screening run %d panicked: %v\n%s", runner.runID, panicValue, debug.Stack())
			runner.end(domains.ErrSymbolScreeningBrokeDown, nil, "")
		}
	}()

	screeningContext := runner.screeningContext
	runStopped := func() error {
		if screeningContext.Err() == nil {
			return nil
		}
		if cause := context.Cause(screeningContext); errors.Is(cause, domains.ErrBacktestTimeAllowanceSpent) {
			return cause
		}

		return errSymbolScreeningRunEndedElsewhere
	}
	snapshot := runner.screeningRun.Snapshot()
	period := domains.NewSymbolScreeningPeriodDomain(snapshot, service.clockProxy.Now(), service.defaultWarmupBarCount)
	marginTiersBySymbol, tiersError := service.marketDataService.MarginTiersBySymbol(screeningContext)
	if tiersError != nil {
		log.Printf("symbol screening run %d replays without margin ladders: %v", runner.runID, tiersError)
	}

	// replayedTrades replays every segment of one symbol; a replay error that is not the run stopping is that symbol's own failure.
	replayedTrades := func(symbol string, replayInput domains.ContractReplayInputDomain) ([]dto.ContractBacktestResultDto, error) {
		results := []dto.ContractBacktestResultDto{}
		for _, request := range period.ReplayRequests(symbol) {
			result, replayError := service.contractBacktestService.RunContractBacktestOn(
				screeningContext, request, replayInput, nil)
			if replayError != nil {
				return nil, replayError
			}
			results = append(results, result)
		}

		return results, nil
	}

	candidates, findError := service.candidateRepository.FindByRun(screeningContext, runner.runID)
	if findError != nil {
		runner.end(findError, nil, "")
		return
	}

	closedTradesBySymbol := map[string][]dto.ContractClosedTradeDto{}
	for index, storedCandidate := range candidates {
		candidate := domains.NewSymbolScreeningCandidateDomain(storedCandidate).Processing()
		symbol := storedCandidate.Symbol
		if saveError := service.candidateRepository.Save(screeningContext, candidate.ToEntity()); saveError != nil {
			runner.end(errors.Join(runStopped(), saveError), nil, "")
			return
		}

		listing, kCandles, replayInput, prepareError := service.marketDataService.Prepare(
			screeningContext, symbol, period, marginTiersBySymbol[symbol])
		if stopped := runStopped(); stopped != nil {
			runner.end(stopped, nil, "")
			return
		}
		switch {
		case prepareError != nil:
			candidate = candidate.Unfinished(prepareError.Error(), dto.SymbolScreeningFindingsDto{})
		default:
			passed, reason, findings := domains.NewSymbolScreeningBasicConditionDomain(
				symbol, listing, kCandles, period, snapshot.MinimumAverageDailyQuoteVolume).Judge()
			if !passed {
				candidate = candidate.Rejected(vo.SymbolScreeningStageBasicCondition, reason, findings)
				break
			}

			results, replayError := replayedTrades(symbol, replayInput)
			if stopped := runStopped(); stopped != nil {
				runner.end(stopped, nil, "")
				return
			}
			if replayError != nil {
				candidate = candidate.Unfinished(replayError.Error(), findings)
				break
			}

			passed, reason, findings = domains.NewSymbolScreeningPortabilityDomain(
				period.Segments(), results, snapshot.MinimumTradeCount).Judge(findings)
			candidate = candidate.Passed(findings)
			if !passed {
				candidate = candidate.Rejected(vo.SymbolScreeningStagePortability, reason, findings)
			}
			for index, result := range results {
				closedTradesBySymbol[symbol] = append(closedTradesBySymbol[symbol],
					result.TradesClosedBy(period.Segments()[index].EndTime)...)
			}
		}

		if saveError := service.candidateRepository.Save(screeningContext, candidate.ToEntity()); saveError != nil {
			runner.end(errors.Join(runStopped(), saveError), nil, "")
			return
		}
		candidates[index] = candidate.ToEntity()
		if advanceError := runner.advance(index + 1); advanceError != nil {
			runner.end(advanceError, nil, "")
			return
		}
	}

	contributionFailureReason := ""
	hasPassed := false
	for _, candidate := range candidates {
		hasPassed = hasPassed || candidate.Status == string(vo.SymbolScreeningCandidatePassed)
	}
	if hasPassed && len(snapshot.BaselineSymbols) > 0 {
		baselineTrades := []dto.ContractClosedTradeDto{}
		for _, baselineSymbol := range snapshot.BaselineSymbols {
			listing, _, replayInput, prepareError := service.marketDataService.Prepare(
				screeningContext, baselineSymbol, period, marginTiersBySymbol[baselineSymbol])
			results, replayError := []dto.ContractBacktestResultDto{}, error(nil)
			switch {
			case prepareError != nil:
				replayError = prepareError
			case !listing.IsListed:
				replayError = errors.New("目前沒有在交易")
			default:
				results, replayError = replayedTrades(baselineSymbol, replayInput)
			}
			if stopped := runStopped(); stopped != nil {
				runner.end(stopped, nil, "")
				return
			}
			if replayError != nil {
				contributionFailureReason = fmt.Sprintf("現有組合標的 %s 無法比較：%s", baselineSymbol, replayError.Error())
				break
			}
			for index, result := range results {
				baselineTrades = append(baselineTrades, result.TradesClosedBy(period.Segments()[index].EndTime)...)
			}
		}

		if contributionFailureReason == "" {
			baseline := domains.NewSymbolScreeningHoldingTimelineDomain(baselineTrades, period.StartTime(), period.EndTime())
			for index, candidate := range candidates {
				if candidate.Status != string(vo.SymbolScreeningCandidatePassed) {
					continue
				}
				contribution := domains.NewSymbolScreeningHoldingTimelineDomain(
					closedTradesBySymbol[candidate.Symbol], period.StartTime(), period.EndTime()).ContributionAgainst(baseline)
				candidates[index].Findings.Contribution = &contribution
			}
		}
	}

	runner.end(nil, domains.NewSymbolScreeningRecommendationDomain(candidates).Ranked(), contributionFailureReason)
}

// advance lands the progress; a refused write means the run was cancelled or swept meanwhile.
func (runner *symbolScreeningRunner) advance(processedCandidateCount int) error {
	runner.mutex.Lock()
	defer runner.mutex.Unlock()

	runner.screeningRun = runner.screeningRun.Advanced(processedCandidateCount)
	writeContext, endWrite := context.WithTimeout(context.Background(), symbolScreeningWriteTimeout)
	defer endWrite()
	stillRunning, writeError := runner.runService.runRepository.UpdateIfRunning(writeContext, runner.screeningRun.ToEntity())
	if writeError != nil {
		return writeError
	}
	if !stillRunning {
		runner.cancelled = true
		return errSymbolScreeningRunEndedElsewhere
	}

	return nil
}

func (runner *symbolScreeningRunner) cancel() {
	runner.mutex.Lock()
	runner.cancelled = true
	runner.mutex.Unlock()

	runner.stopScreening()
}

func (runner *symbolScreeningRunner) interrupt() {
	runner.mutex.Lock()
	runner.interrupted = true
	runner.mutex.Unlock()

	runner.stopScreening()
}

// end writes the ending once; a run cancelled meanwhile is left as the cancel wrote it.
func (runner *symbolScreeningRunner) end(
	screeningError error, rankedCandidates []entities.SymbolScreeningCandidate, contributionFailureReason string,
) {
	runner.mutex.Lock()
	defer runner.mutex.Unlock()

	if runner.ended {
		return
	}
	runner.ended = true
	if runner.cancelled || (errors.Is(screeningError, errSymbolScreeningRunEndedElsewhere) && !runner.interrupted) {
		return
	}

	service := runner.runService
	now := service.clockProxy.Now()
	if screeningError == nil {
		runner.screeningRun = runner.screeningRun.Succeeded(contributionFailureReason, now)
	} else {
		runner.screeningRun = runner.screeningRun.FailedFor(screeningError, runner.interrupted, now)
	}

	writeContext, endWrite := context.WithTimeout(context.Background(), symbolScreeningWriteTimeout)
	defer endWrite()
	writeError := service.transactionRepository.Atomically(writeContext,
		func(transactionContext context.Context) error {
			for _, candidate := range rankedCandidates {
				if saveError := service.candidateRepository.Save(transactionContext, candidate); saveError != nil {
					return saveError
				}
			}
			stillRunning, updateError := service.runRepository.UpdateIfRunning(transactionContext, runner.screeningRun.ToEntity())
			if updateError != nil {
				return updateError
			}
			if !stillRunning {
				return errSymbolScreeningRunEndedElsewhere
			}
			if screeningError != nil {
				return service.candidateRepository.CancelUnfinished(transactionContext, runner.runID)
			}

			return nil
		})
	if writeError != nil && !errors.Is(writeError, errSymbolScreeningRunEndedElsewhere) {
		log.Printf("symbol screening run %d ending could not be written: %v", runner.runID, writeError)
	}
}
