package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

const (
	symbolScreeningRecentDays = 30
	symbolScreeningDay        = 24 * time.Hour
)

// SymbolScreeningInitialCapital is the same for every segment so returns compare across candidates.
var SymbolScreeningInitialCapital = decimal.NewFromInt(10_000)

// SymbolScreeningPeriodDomain is the stretch a run screens: it ends at the last finished UTC symbolScreeningDay and is cut into equal segments on bar boundaries.
type SymbolScreeningPeriodDomain struct {
	snapshot       dto.SymbolScreeningSnapshotDto
	startTime      time.Time
	endTime        time.Time
	step           time.Duration
	warmupBarCount int
}

func NewSymbolScreeningPeriodDomain(
	snapshot dto.SymbolScreeningSnapshotDto, now time.Time, defaultWarmupBarCount int,
) SymbolScreeningPeriodDomain {
	endTime := now.UTC().Truncate(symbolScreeningDay)
	step, isScreenable := symbolScreeningIntervalSteps[snapshot.AggregationInterval]
	if !isScreenable {
		step = symbolScreeningDay
	}

	warmupBarCount := defaultWarmupBarCount
	if parameters, parametersError := NewStrategyScriptParametersDomain(snapshot.Parameters); parametersError == nil {
		if applied, applyError := parameters.Applying(snapshot.ParameterValues); applyError == nil &&
			applied.MaximumLookbackCount() > 0 {
			warmupBarCount = applied.MaximumLookbackCount()
		}
	}

	return SymbolScreeningPeriodDomain{
		snapshot:       snapshot,
		startTime:      endTime.Add(-time.Duration(snapshot.HistoryDays) * symbolScreeningDay),
		endTime:        endTime,
		step:           step,
		warmupBarCount: max(warmupBarCount, 0),
	}
}

func (periodDomain SymbolScreeningPeriodDomain) StartTime() time.Time {
	return periodDomain.startTime
}

func (periodDomain SymbolScreeningPeriodDomain) EndTime() time.Time {
	return periodDomain.endTime
}

// FetchStartTime reaches back far enough for the first segment to warm up.
func (periodDomain SymbolScreeningPeriodDomain) FetchStartTime() time.Time {
	return periodDomain.startTime.Add(-time.Duration(periodDomain.warmupBarCount) * periodDomain.step)
}

func (periodDomain SymbolScreeningPeriodDomain) FetchWindow(symbol string) vo.AggregatedKCandleFetchWindowVo {
	return vo.AggregatedKCandleFetchWindowVo{
		Symbol:    symbol,
		Interval:  periodDomain.snapshot.AggregationInterval,
		Step:      periodDomain.step,
		StartTime: periodDomain.FetchStartTime(),
		EndTime:   periodDomain.endTime.Add(-time.Millisecond),
	}
}

func (periodDomain SymbolScreeningPeriodDomain) Segments() []vo.SymbolScreeningSegmentVo {
	segmentCount := max(periodDomain.snapshot.SegmentCount, 1)
	totalSteps := int64(periodDomain.endTime.Sub(periodDomain.startTime) / periodDomain.step)
	segments := make([]vo.SymbolScreeningSegmentVo, 0, segmentCount)
	for index := range segmentCount {
		startStep := totalSteps * int64(index) / int64(segmentCount)
		endStep := totalSteps * int64(index+1) / int64(segmentCount)
		segments = append(segments, vo.SymbolScreeningSegmentVo{
			StartTime: periodDomain.startTime.Add(time.Duration(startStep) * periodDomain.step),
			EndTime:   periodDomain.startTime.Add(time.Duration(endStep) * periodDomain.step),
		})
	}

	return segments
}

// ClosedTradesOf gathers the trades of one replay per segment, a position still held at a segment's end counted as closed there so holding time is not lost at the edge.
func (periodDomain SymbolScreeningPeriodDomain) ClosedTradesOf(
	segmentResults []dto.ContractBacktestResultDto,
) []dto.ContractClosedTradeDto {
	segments := periodDomain.Segments()
	trades := []dto.ContractClosedTradeDto{}
	for segmentIndex, result := range segmentResults {
		trades = append(trades, result.ClosedTrades...)
		if result.OpenPositionEntryTime != nil && segmentIndex < len(segments) {
			trades = append(trades, dto.ContractClosedTradeDto{
				EntryTime: *result.OpenPositionEntryTime, ExitTime: segments[segmentIndex].EndTime,
			})
		}
	}

	return trades
}

// ReplayRequests replays each segment from the same capital, filling on the next open and staking everything.
func (periodDomain SymbolScreeningPeriodDomain) ReplayRequests(symbol string) []dto.ContractBacktestRequestDto {
	snapshot := periodDomain.snapshot
	segments := periodDomain.Segments()
	requests := make([]dto.ContractBacktestRequestDto, 0, len(segments))
	for _, segment := range segments {
		requests = append(requests, dto.ContractBacktestRequestDto{
			Symbol:                symbol,
			AggregationInterval:   snapshot.AggregationInterval,
			StartTime:             segment.StartTime,
			EndTime:               segment.EndTime.Add(-time.Second),
			Script:                snapshot.Script,
			Parameters:            snapshot.Parameters,
			ParameterValues:       snapshot.ParameterValues,
			InitialCapital:        SymbolScreeningInitialCapital,
			PositionSizingMode:    string(vo.PositionSizingModeAllIn),
			EntryCostPercentage:   snapshot.EntryCostPercentage,
			ExitCostPercentage:    snapshot.EntryCostPercentage,
			FillTiming:            string(vo.FillTimingNextOpen),
			Leverage:              snapshot.Leverage,
			TradingMode:           snapshot.TradingMode,
			SlippagePercentage:    snapshot.SlippagePercentage,
			DefaultWarmupBarCount: periodDomain.warmupBarCount,
		})
	}

	return requests
}
