package domains

import (
	"fmt"
	"slices"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// strategyBotSignalSourceMaxCount is how many strategy scripts one bot may run.
//
// Every source is a whole indicator calculation of its own — a script interpreted
// over freshly read candles — and they all happen again every trigger interval, for
// every running bot. Ten is where one round stops being cheap.
const strategyBotSignalSourceMaxCount = 10

// strategyBotSignalSourceLabelMaxLength is how long a label may be. Labels are
// meant to be A, B, C: the thing a condition says out loud. The limit is generous
// enough for a short word and short enough that a condition stays readable.
const strategyBotSignalSourceLabelMaxLength = 32

// TradingStrategySignalSourcesDomain is everything one bot's signal sources have to be
// true of together: distinct labels, a bounded number of them, and values that the
// strategy scripts they name actually declared.
//
// It is a model of the set rather than of one source, because every rule here is
// about the set. A label is unique *among these*; the count is a count *of these*;
// and the one rule that is about a single source — does this strategy script declare this
// knob — is checked here too, because that is where the labels are already being
// walked.
type TradingStrategySignalSourcesDomain struct {
	sources []dto.TradingStrategySignalSourceWriteDto
	labels  []string
}

// NewTradingStrategySignalSourcesDomain validates the sources against every rule that
// applies to them.
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

		// Two sources answering to one label would make every condition naming it
		// mean two things at once, and no reading of that is the one somebody
		// intended.
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

		// Setting a knob the strategy script never declared is caught now rather than at
		// three in the morning, when the same mistake would come back as a script
		// failure and stop the bot.
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

	return TradingStrategySignalSourcesDomain{sources: settledSources, labels: labels}, nil
}

// Labels are what the conditions may name, in the order they were declared.
func (tradingStrategySignalSourcesDomain TradingStrategySignalSourcesDomain) Labels() []string {
	return tradingStrategySignalSourcesDomain.labels
}

// ToEntities flattens the sources into the rows they are stored as. Identifiers are
// left unset: they belong to the store.
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
