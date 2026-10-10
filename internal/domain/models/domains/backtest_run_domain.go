package domains

import (
	"errors"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

const (
	// BacktestRunInterruptedReason is written on a run whose replica is gone; the request is kept, so it can simply be submitted again.
	BacktestRunInterruptedReason   = "被重新啟動打斷"
	backtestRunBrokeDownReason     = "回測服務壞掉了，這筆沒有跑完"
	backtestRunOverAllowanceReason = "超過時間上限"
)

// BacktestRunDomain is one run's life: queued, claimed by a replica, advancing, then succeeded, failed or cancelled; final statuses never change.
type BacktestRunDomain struct {
	backtestRun entities.BacktestRun
	status      vo.BacktestRunStatusVo
	kind        vo.BacktestRunKindVo
}

// NewBacktestRunDomain reads a stored run, settling an unrecognised status to failed and an unrecognised kind to none.
func NewBacktestRunDomain(backtestRun entities.BacktestRun) BacktestRunDomain {
	return BacktestRunDomain{
		backtestRun: backtestRun,
		status:      vo.NewBacktestRunStatusVo(backtestRun.Status),
		kind:        vo.NewBacktestRunKindVo(backtestRun.Kind),
	}
}

// NewSubmittedBacktestRunDomain describes a freshly accepted request from whichever of the four requests the submission carries.
func NewSubmittedBacktestRunDomain(
	ownerID uint, submission dto.BacktestRunSubmissionDto, submittedAt time.Time,
) (BacktestRunDomain, error) {
	backtestRun := entities.BacktestRun{
		OwnerID:           ownerID,
		Kind:              submission.Kind,
		StrategyScriptID:  submission.StrategyScriptID,
		TradingStrategyID: submission.TradingStrategyID,
		SubjectName:       submission.SubjectName,
		Request:           submission.Request,
		Status:            string(vo.BacktestRunQueued),
		SubmittedAt:       submittedAt.UTC(),
	}

	switch vo.NewBacktestRunKindVo(submission.Kind) {
	case vo.BacktestRunSpotScript:
		request := submission.Request.SpotScript
		if request == nil {
			return BacktestRunDomain{}, fmt.Errorf("%w: 現貨腳本回測沒有帶請求", ErrBacktestValidation)
		}
		backtestRun.Symbol, backtestRun.Interval = request.Symbol, request.AggregationInterval
		backtestRun.StartTime, backtestRun.EndTime = request.StartTime.UTC(), request.EndTime.UTC()
	case vo.BacktestRunContractScript:
		request := submission.Request.ContractScript
		if request == nil {
			return BacktestRunDomain{}, fmt.Errorf("%w: 合約腳本回測沒有帶請求", ErrBacktestValidation)
		}
		backtestRun.Symbol, backtestRun.Interval = request.Symbol, request.AggregationInterval
		backtestRun.StartTime, backtestRun.EndTime = request.StartTime.UTC(), request.EndTime.UTC()
		backtestRun.Leverage = decimal.NewNullDecimal(request.Leverage)
		if request.Leverage.IsZero() {
			backtestRun.Leverage = decimal.NewNullDecimal(oneWhole)
		}
		tradingMode, tradingModeError := NewContractTradingModeDomain(request.TradingMode)
		if tradingModeError != nil {
			return BacktestRunDomain{}, BacktestValidationFailure(BacktestTradingModeField, tradingModeError.Error())
		}
		backtestRun.TradingMode = string(tradingMode.Value())
	case vo.BacktestRunSpotTradingStrategy:
		request := submission.Request.SpotTradingStrategy
		if request == nil || len(request.SignalSources) == 0 {
			return BacktestRunDomain{}, fmt.Errorf("%w: 現貨交易策略回測沒有帶請求", ErrBacktestValidation)
		}
		backtestRun.Symbol, backtestRun.Interval = request.Symbol, request.SignalSources[0].AggregationInterval
		backtestRun.StartTime, backtestRun.EndTime = request.StartTime.UTC(), request.EndTime.UTC()
	case vo.BacktestRunContractTradingStrategy:
		request := submission.Request.ContractTradingStrategy
		if request == nil || len(request.SignalSources) == 0 {
			return BacktestRunDomain{}, fmt.Errorf("%w: 合約交易策略回測沒有帶請求", ErrBacktestValidation)
		}
		backtestRun.Symbol, backtestRun.Interval = request.Symbol, request.SignalSources[0].AggregationInterval
		backtestRun.StartTime, backtestRun.EndTime = request.StartTime.UTC(), request.EndTime.UTC()
		backtestRun.Leverage = decimal.NewNullDecimal(request.Leverage)
		if request.Leverage.IsZero() {
			backtestRun.Leverage = decimal.NewNullDecimal(oneWhole)
		}
		tradingMode, tradingModeError := NewContractTradingModeDomain(request.TradingStrategyTradingMode)
		if tradingModeError != nil {
			return BacktestRunDomain{}, BacktestValidationFailure(BacktestTradingModeField, tradingModeError.Error())
		}
		backtestRun.TradingMode = string(tradingMode.Value())
	default:
		return BacktestRunDomain{}, fmt.Errorf("%w: 認不得的回測種類 %q", ErrBacktestValidation, submission.Kind)
	}

	return NewBacktestRunDomain(backtestRun), nil
}

func (backtestRunDomain BacktestRunDomain) Kind() vo.BacktestRunKindVo {
	return backtestRunDomain.kind
}

func (backtestRunDomain BacktestRunDomain) Request() dto.BacktestRunRequestDto {
	return backtestRunDomain.backtestRun.Request
}

// IsActive is true while the run still has work ahead: queued or running.
func (backtestRunDomain BacktestRunDomain) IsActive() bool {
	return backtestRunDomain.status == vo.BacktestRunQueued || backtestRunDomain.status == vo.BacktestRunRunning
}

// ProgressPercent is whole bars done out of the total, floored so 100 is only reported when every bar is done.
func (backtestRunDomain BacktestRunDomain) ProgressPercent() int {
	if backtestRunDomain.status == vo.BacktestRunSucceeded {
		return 100
	}
	if backtestRunDomain.backtestRun.TotalBarCount <= 0 {
		return 0
	}

	return min(100, backtestRunDomain.backtestRun.CompletedBarCount*100/backtestRunDomain.backtestRun.TotalBarCount)
}

// Claimed is the run as a replica takes it off the queue.
func (backtestRunDomain BacktestRunDomain) Claimed(replicaName string, startedAt time.Time) BacktestRunDomain {
	startedAtUtc := startedAt.UTC()
	backtestRunDomain.backtestRun.Status = string(vo.BacktestRunRunning)
	backtestRunDomain.backtestRun.ReplicaName = replicaName
	backtestRunDomain.backtestRun.StartedAt = &startedAtUtc
	backtestRunDomain.status = vo.BacktestRunRunning

	return backtestRunDomain
}

// Advanced records how far the replay has got; the total is given each time because a strategy with several sources counts every source's bars.
func (backtestRunDomain BacktestRunDomain) Advanced(completedBarCount int, totalBarCount int) BacktestRunDomain {
	backtestRunDomain.backtestRun.CompletedBarCount = completedBarCount
	backtestRunDomain.backtestRun.TotalBarCount = totalBarCount

	return backtestRunDomain
}

// Succeeded closes the run with the headline figures a list shows; the report itself is stored beside the run.
func (backtestRunDomain BacktestRunDomain) Succeeded(
	summary dto.BacktestRunSummaryDto, finishedAt time.Time,
) BacktestRunDomain {
	finishedAtUtc := finishedAt.UTC()
	backtestRunDomain.backtestRun.Status = string(vo.BacktestRunSucceeded)
	backtestRunDomain.backtestRun.TotalReturnRate = &summary.TotalReturnRate
	backtestRunDomain.backtestRun.MaximumDrawdown = &summary.MaximumDrawdown
	backtestRunDomain.backtestRun.TradeCount = &summary.TradeCount
	backtestRunDomain.backtestRun.CompletedBarCount = backtestRunDomain.backtestRun.TotalBarCount
	backtestRunDomain.backtestRun.FinishedAt = &finishedAtUtc
	backtestRunDomain.status = vo.BacktestRunSucceeded

	return backtestRunDomain
}

// FailedFor closes the run with why it did not finish, in the words the person reads; a stop for a restart says so whatever the replay reported.
func (backtestRunDomain BacktestRunDomain) FailedFor(
	replayError error, interrupted bool, finishedAt time.Time,
) BacktestRunDomain {
	reason := backtestRunBrokeDownReason
	switch {
	case interrupted:
		reason = BacktestRunInterruptedReason
	case errors.Is(replayError, ErrBacktestTimeAllowanceSpent):
		reason = backtestRunOverAllowanceReason
	case replayError != nil && !errors.Is(replayError, ErrBacktestRunBrokeDown):
		reason = replayError.Error()
	}

	finishedAtUtc := finishedAt.UTC()
	backtestRunDomain.backtestRun.Status = string(vo.BacktestRunFailed)
	backtestRunDomain.backtestRun.FailureReason = reason
	backtestRunDomain.backtestRun.FinishedAt = &finishedAtUtc
	backtestRunDomain.status = vo.BacktestRunFailed

	return backtestRunDomain
}

// Cancelled ends a queued or running run; a run already ended is refused, since there is nothing left to stop.
func (backtestRunDomain BacktestRunDomain) Cancelled(cancelledAt time.Time) (BacktestRunDomain, error) {
	if !backtestRunDomain.IsActive() {
		return BacktestRunDomain{}, ErrBacktestRunAlreadyEnded
	}

	cancelledAtUtc := cancelledAt.UTC()
	backtestRunDomain.backtestRun.Status = string(vo.BacktestRunCancelled)
	backtestRunDomain.backtestRun.FinishedAt = &cancelledAtUtc
	backtestRunDomain.status = vo.BacktestRunCancelled

	return backtestRunDomain, nil
}

func (backtestRunDomain BacktestRunDomain) ToEntity() entities.BacktestRun {
	return backtestRunDomain.backtestRun
}

func (backtestRunDomain BacktestRunDomain) ToDto() dto.BacktestRunDto {
	backtestRun := backtestRunDomain.backtestRun

	leverage := (*decimal.Decimal)(nil)
	tradingMode := (*string)(nil)
	if backtestRun.Leverage.Valid {
		leverageValue := backtestRun.Leverage.Decimal
		leverage = &leverageValue
		tradingModeValue := backtestRun.TradingMode
		tradingMode = &tradingModeValue
	}
	startedAt := (*time.Time)(nil)
	if backtestRun.StartedAt != nil {
		startedAtUtc := backtestRun.StartedAt.UTC()
		startedAt = &startedAtUtc
	}
	finishedAt := (*time.Time)(nil)
	if backtestRun.FinishedAt != nil {
		finishedAtUtc := backtestRun.FinishedAt.UTC()
		finishedAt = &finishedAtUtc
	}

	return dto.BacktestRunDto{
		ID:                backtestRun.ID,
		Kind:              string(backtestRunDomain.kind),
		Status:            string(backtestRunDomain.status),
		ProgressPercent:   backtestRunDomain.ProgressPercent(),
		Symbol:            backtestRun.Symbol,
		Interval:          backtestRun.Interval,
		StartTime:         backtestRun.StartTime.UTC(),
		EndTime:           backtestRun.EndTime.UTC(),
		Leverage:          leverage,
		TradingMode:       tradingMode,
		StrategyScriptID:  backtestRun.StrategyScriptID,
		TradingStrategyID: backtestRun.TradingStrategyID,
		SubjectName:       backtestRun.SubjectName,
		SubmittedAt:       backtestRun.SubmittedAt.UTC(),
		StartedAt:         startedAt,
		FinishedAt:        finishedAt,
		TotalReturnRate:   backtestRun.TotalReturnRate,
		MaximumDrawdown:   backtestRun.MaximumDrawdown,
		TradeCount:        backtestRun.TradeCount,
		FailureReason:     backtestRun.FailureReason,
	}
}

// ToDetailDto pairs the run with its report; a nil result is right for every status but succeeded.
func (backtestRunDomain BacktestRunDomain) ToDetailDto(result *dto.BacktestRunResultDto) dto.BacktestRunDetailDto {
	return dto.BacktestRunDetailDto{BacktestRunDto: backtestRunDomain.ToDto(), Result: result}
}
