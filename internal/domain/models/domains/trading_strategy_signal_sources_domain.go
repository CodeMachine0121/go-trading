package domains

import (
	"fmt"
	"slices"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// strategyBotSignalSourceMaxCount bounds round cost, since every source is a full script run
// per trigger for every running bot.
const strategyBotSignalSourceMaxCount = 10

const strategyBotSignalSourceLabelMaxLength = 32

// TradingStrategySignalSourcesDomain validates the source set: distinct labels, bounded
// count, and only declared parameters.
type TradingStrategySignalSourcesDomain struct {
	sources []dto.TradingStrategySignalSourceWriteDto
	labels  []string
}

func NewTradingStrategySignalSourcesDomain(
	sources []dto.TradingStrategySignalSourceWriteDto,
) (TradingStrategySignalSourcesDomain, error) {
	if len(sources) == 0 {
		return TradingStrategySignalSourcesDomain{}, fmt.Errorf(
			"%w: 一份交易策略至少要有一個信號來源", ErrTradingStrategyValidation)
	}

	if len(sources) > strategyBotSignalSourceMaxCount {
		return TradingStrategySignalSourcesDomain{}, fmt.Errorf(
			"%w: 一份交易策略的信號來源上限是 %d 個",
			ErrTradingStrategyValidation, strategyBotSignalSourceMaxCount)
	}

	settledSources := make([]dto.TradingStrategySignalSourceWriteDto, 0, len(sources))
	labels := make([]string, 0, len(sources))

	for _, source := range sources {
		label := strings.TrimSpace(source.Label)
		if label == "" {
			return TradingStrategySignalSourcesDomain{}, fmt.Errorf(
				"%w: 每一個信號來源都要有一個代號", ErrTradingStrategyValidation)
		}

		if len([]rune(label)) > strategyBotSignalSourceLabelMaxLength {
			return TradingStrategySignalSourcesDomain{}, fmt.Errorf(
				"%w: 信號來源代號長度上限為 %d 個字",
				ErrTradingStrategyValidation, strategyBotSignalSourceLabelMaxLength)
		}

		if slices.Contains(labels, label) {
			return TradingStrategySignalSourcesDomain{}, fmt.Errorf(
				"%w: 信號來源代號 %q 重複了，同一份交易策略內的代號必須各不相同",
				ErrTradingStrategyValidation, label)
		}

		if source.StrategyScriptID == 0 {
			return TradingStrategySignalSourcesDomain{}, fmt.Errorf(
				"%w: 信號來源 %q 必須指名一支策略腳本", ErrTradingStrategyValidation, label)
		}

		aggregationInterval, intervalError := NewAggregationIntervalDomain(source.AggregationInterval)
		if intervalError != nil {
			return TradingStrategySignalSourcesDomain{}, fmt.Errorf(
				"%w: 信號來源 %q 的彙總刻度不對：%w", ErrTradingStrategyValidation, label, intervalError)
		}

		// A source must produce signals to be compared in conditions; this is refused here
		// because sources can be named by the screen, the assistant or raw requests.
		declaredResultType, resultTypeError := NewIndicatorResultTypeDomain(source.DeclaredResultType)
		if resultTypeError != nil {
			return TradingStrategySignalSourcesDomain{}, fmt.Errorf(
				"%w: 信號來源 %q 指名的那支策略腳本的指標值種類不對：%w",
				ErrTradingStrategyValidation, label, resultTypeError)
		}

		if !declaredResultType.IsSignal() {
			return TradingStrategySignalSourcesDomain{}, fmt.Errorf(
				"%w: 信號來源 %q 指名的那支策略腳本不吐訊號——它的指標值種類是 %q，"+
					"而條件比對的是買入／賣出／持有，只有 %q 這一種說得出那三個值",
				ErrTradingStrategyValidation, label,
				string(declaredResultType.Value()), string(vo.IndicatorResultTypeSignal))
		}

		// Undeclared parameters are refused now rather than failing the bot at run time.
		for _, parameterValue := range source.ParameterValues {
			parameterName := strings.TrimSpace(parameterValue.Name)
			declaresIt := slices.ContainsFunc(
				source.DeclaredParameters,
				func(declaredParameter dto.StrategyScriptParameterWriteDto) bool {
					return strings.TrimSpace(declaredParameter.Name) == parameterName
				})
			if !declaresIt {
				return TradingStrategySignalSourcesDomain{}, fmt.Errorf(
					"%w: 信號來源 %q 給了參數 %q 的值，但它指名的那支策略腳本沒有宣告這個名字",
					ErrTradingStrategyValidation, label, parameterValue.Name)
			}
		}

		source.Label = label
		source.AggregationInterval = string(aggregationInterval.Value())
		settledSources = append(settledSources, source)
		labels = append(labels, label)
	}

	// Sources must share one interval, otherwise the trading strategy cannot be replayed and
	// live signals would compare different candles.
	intervals := make([]string, 0, len(settledSources))
	for _, source := range settledSources {
		intervals = append(intervals, source.AggregationInterval)
	}

	_, sharedIntervalError := NewSharedAggregationIntervalDomain(intervals).Shared()
	if sharedIntervalError != nil {
		return TradingStrategySignalSourcesDomain{}, fmt.Errorf(
			"%w: %w", ErrTradingStrategyValidation, sharedIntervalError)
	}

	return TradingStrategySignalSourcesDomain{sources: settledSources, labels: labels}, nil
}

// RequireMarketDataKind refuses, by name, a source whose script reads a different kind of
// market than the trading strategy.
func (tradingStrategySignalSourcesDomain TradingStrategySignalSourcesDomain) RequireMarketDataKind(
	marketDataKind MarketDataKindDomain,
) error {
	for _, source := range tradingStrategySignalSourcesDomain.sources {
		sourceKind, kindError := NewMarketDataKindDomain(source.DeclaredMarketDataKind)
		if kindError != nil {
			return fmt.Errorf("%w: %w", ErrTradingStrategyValidation, kindError)
		}

		if sourceKind.value != marketDataKind.value {
			return fmt.Errorf(
				"%w: 信號來源 %q 指名的那支策略腳本吃的是%s，這份交易策略吃的是%s——"+
					"一份交易策略的每一個信號來源都要吃同一種行情",
				ErrTradingStrategyValidation, source.Label, sourceKind.label(), marketDataKind.label())
		}
	}

	return nil
}

func (tradingStrategySignalSourcesDomain TradingStrategySignalSourcesDomain) Labels() []string {
	return tradingStrategySignalSourcesDomain.labels
}

// ToEntities leaves IDs unset for the store to assign.
func (tradingStrategySignalSourcesDomain TradingStrategySignalSourcesDomain) ToEntities() []entities.TradingStrategySignalSource {
	signalSources := make(
		[]entities.TradingStrategySignalSource, 0, len(tradingStrategySignalSourcesDomain.sources))

	for _, source := range tradingStrategySignalSourcesDomain.sources {
		parameterValues := make(
			[]entities.TradingStrategySignalSourceParameterValue, 0, len(source.ParameterValues))
		for _, parameterValue := range source.ParameterValues {
			parameterValues = append(parameterValues, entities.TradingStrategySignalSourceParameterValue{
				Name:  strings.TrimSpace(parameterValue.Name),
				Value: parameterValue.Value,
			})
		}

		signalSources = append(signalSources, entities.TradingStrategySignalSource{
			Label:               source.Label,
			StrategyScriptID:    source.StrategyScriptID,
			AggregationInterval: source.AggregationInterval,
			ParameterValues:     parameterValues,
		})
	}

	return signalSources
}
