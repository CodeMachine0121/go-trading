package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// strategyNameMaxLength is how long a strategy name may be, counted after the blanks
// around it are dropped. It is the single place this limit is written down.
const strategyNameMaxLength = 128

// StrategyDomain holds one strategy and guarantees its own invariants. An instance
// only exists when every rule passed, so there is no half-valid strategy.
//
// Two of the five things a strategy remembers — the aggregation interval and the
// indicator result type — already have models that know what they may be. This one
// borrows them rather than listing their values a second time, which is why offering
// one more interval costs nothing here. What it adds is the rest of the rules and a
// single kind of refusal, so that a caller never has to recognise a K candle's
// sentinel to find out its strategy was rejected.
//
// It deliberately does not look at the script beyond "there is one". A script that
// cannot run is still worth saving: an algorithm usually takes several sittings to
// get right, and a strategy that refused to be saved half-finished could not be
// picked up again tomorrow.
type StrategyDomain struct {
	id                  uint
	name                string
	script              string
	resultType          IndicatorResultTypeDomain
	aggregationInterval AggregationIntervalDomain
	candleCount         int
}

// NewStrategyDomain validates the strategy against every rule that applies to it.
// The rules are identical whether the strategy is being created or rewritten,
// because both arrive here as the same shape.
func NewStrategyDomain(
	writeDto dto.StrategyWriteDto, maxCandleCount int,
) (StrategyDomain, error) {
	name := strings.TrimSpace(writeDto.Name)
	if name == "" {
		return StrategyDomain{}, fmt.Errorf("%w: 必須給策略取一個名稱", ErrStrategyValidation)
	}

	if len([]rune(name)) > strategyNameMaxLength {
		return StrategyDomain{}, fmt.Errorf(
			"%w: 策略名稱長度上限為 %d 個字", ErrStrategyValidation, strategyNameMaxLength)
	}

	if strings.TrimSpace(writeDto.Script) == "" {
		return StrategyDomain{}, fmt.Errorf("%w: 策略必須帶一段指標算式", ErrStrategyValidation)
	}

	resultType, resultTypeError := NewIndicatorResultTypeDomain(writeDto.ResultType)
	if resultTypeError != nil {
		return StrategyDomain{}, fmt.Errorf("%w: %w", ErrStrategyValidation, resultTypeError)
	}

	aggregationInterval, intervalError := NewAggregationIntervalDomain(writeDto.AggregationInterval)
	if intervalError != nil {
		return StrategyDomain{}, fmt.Errorf("%w: %w", ErrStrategyValidation, intervalError)
	}

	if writeDto.CandleCount <= 0 {
		return StrategyDomain{}, fmt.Errorf("%w: 計算根數必須大於零", ErrStrategyValidation)
	}

	if writeDto.CandleCount > maxCandleCount {
		return StrategyDomain{}, fmt.Errorf(
			"%w: 超過單次可用的最大根數（最多 %d 根）", ErrStrategyValidation, maxCandleCount)
	}

	return StrategyDomain{
		id:                  writeDto.ID,
		name:                name,
		script:              writeDto.Script,
		resultType:          resultType,
		aggregationInterval: aggregationInterval,
		candleCount:         writeDto.CandleCount,
	}, nil
}

// AggregationInterval is how coarse the K candles this strategy reads are, already
// read and accepted. It is handed out as the interval itself rather than as its
// spelling, because the interval already knows how to align a bucket, count buckets
// and say how many stored K candles they hold — everything running this strategy
// will need. Handing out the spelling would invite that knowledge to be worked out a
// second time.
func (strategyDomain StrategyDomain) AggregationInterval() AggregationIntervalDomain {
	return strategyDomain.aggregationInterval
}

// ResultType is the kind of value this strategy produces, already read and accepted,
// for the same reason.
func (strategyDomain StrategyDomain) ResultType() IndicatorResultTypeDomain {
	return strategyDomain.resultType
}

// ToEntity is the strategy as it is stored. The times are left alone: when a
// strategy was first saved and when it was last touched are recorded where the
// saving happens, not claimed here.
func (strategyDomain StrategyDomain) ToEntity() entities.Strategy {
	return entities.Strategy{
		ID:                  strategyDomain.id,
		Name:                strategyDomain.name,
		Script:              strategyDomain.script,
		ResultType:          string(strategyDomain.resultType.Value()),
		AggregationInterval: string(strategyDomain.aggregationInterval.Value()),
		CandleCount:         strategyDomain.candleCount,
	}
}
