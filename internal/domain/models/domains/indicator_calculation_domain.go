package domains

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// excludedNewestCandleCount is how many of the newest K candles every calculation
// leaves out, because the newest one has not finished the five minutes it covers.
// It is the single place this rule is written down.
const excludedNewestCandleCount = 1

// IndicatorCalculationDomain holds one calculation request and guarantees its own
// invariants. It also owns the two rules that decide which K candles the script sees:
// exclude the newest, and refuse when what remains is not enough.
type IndicatorCalculationDomain struct {
	symbol      string
	candleCount int
	resultType  IndicatorResultTypeDomain
}

// NewIndicatorCalculationDomain validates the request against every request rule.
func NewIndicatorCalculationDomain(
	requestDto dto.IndicatorCalculationRequestDto, maxCandleCount int,
) (IndicatorCalculationDomain, error) {
	if requestDto.Symbol == "" {
		return IndicatorCalculationDomain{},
			fmt.Errorf("%w: 必須指定交易標的", ErrIndicatorCalculationValidation)
	}

	if requestDto.CandleCount <= 0 {
		return IndicatorCalculationDomain{},
			fmt.Errorf("%w: 計算根數必須大於零", ErrIndicatorCalculationValidation)
	}

	if requestDto.CandleCount > maxCandleCount {
		return IndicatorCalculationDomain{}, fmt.Errorf(
			"%w: 超過單次可用的最大根數（最多 %d 根）",
			ErrIndicatorCalculationValidation, maxCandleCount)
	}

	resultType, resultTypeError := NewIndicatorResultTypeDomain(requestDto.ResultType)
	if resultTypeError != nil {
		return IndicatorCalculationDomain{}, fmt.Errorf(
			"%w: %w", ErrIndicatorCalculationValidation, resultTypeError)
	}

	return IndicatorCalculationDomain{
		symbol:      requestDto.Symbol,
		candleCount: requestDto.CandleCount,
		resultType:  resultType,
	}, nil
}

func (indicatorCalculationDomain IndicatorCalculationDomain) Symbol() string {
	return indicatorCalculationDomain.symbol
}

// ResultType is the indicator value kind this request declared, already read and
// accepted. Everything downstream takes the kind from here rather than from the raw
// declaration, so a declaration is only ever interpreted once.
func (indicatorCalculationDomain IndicatorCalculationDomain) ResultType() IndicatorResultTypeDomain {
	return indicatorCalculationDomain.resultType
}

// CandleFetchCount is how many K candles must be read to satisfy this request:
// one more than asked for, because the newest one is thrown away.
func (indicatorCalculationDomain IndicatorCalculationDomain) CandleFetchCount() int {
	return indicatorCalculationDomain.candleCount + excludedNewestCandleCount
}

// SelectInputCandles takes the K candles as read — newest first — throws away the
// newest, and hands back what the script sees: oldest first, exactly as many as
// asked for. Too few remaining is refused, naming how many are actually usable.
func (indicatorCalculationDomain IndicatorCalculationDomain) SelectInputCandles(
	newestFirstKCandles []entities.KCandle,
) ([]vo.KCandleVo, error) {
	usableCount := max(0, len(newestFirstKCandles)-excludedNewestCandleCount)

	if usableCount < indicatorCalculationDomain.candleCount {
		return nil, fmt.Errorf(
			"%w: K 線不足，排除最新一根後目前可用 %d 根，但要求 %d 根",
			ErrIndicatorCalculationValidation, usableCount, indicatorCalculationDomain.candleCount)
	}

	oldestFirstKCandleVos := make([]vo.KCandleVo, 0, indicatorCalculationDomain.candleCount)
	for index := indicatorCalculationDomain.candleCount; index >= excludedNewestCandleCount; index-- {
		oldestFirstKCandleVos = append(oldestFirstKCandleVos, newestFirstKCandles[index].ToVo())
	}

	return oldestFirstKCandleVos, nil
}
