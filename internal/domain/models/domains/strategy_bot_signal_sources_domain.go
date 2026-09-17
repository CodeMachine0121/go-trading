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

// StrategyBotSignalSourcesDomain is everything one bot's signal sources have to be
// true of together: distinct labels, a bounded number of them, and values that the
// strategy scripts they name actually declared.
//
// It is a model of the set rather than of one source, because every rule here is
// about the set. A label is unique *among these*; the count is a count *of these*;
// and the one rule that is about a single source — does this strategy script declare this
// knob — is checked here too, because that is where the labels are already being
// walked.
type StrategyBotSignalSourcesDomain struct {
	sources []dto.StrategyBotSignalSourceWriteDto
	labels  []string
}

// NewStrategyBotSignalSourcesDomain validates the sources against every rule that
// applies to them.
func NewStrategyBotSignalSourcesDomain(
	sources []dto.StrategyBotSignalSourceWriteDto,
) (StrategyBotSignalSourcesDomain, error) {
	if len(sources) == 0 {
		return StrategyBotSignalSourcesDomain{}, fmt.Errorf(
			"%w: 一台機器人至少要有一個信號來源", ErrStrategyBotValidation)
	}

	if len(sources) > strategyBotSignalSourceMaxCount {
		return StrategyBotSignalSourcesDomain{}, fmt.Errorf(
			"%w: 一台機器人的信號來源上限是 %d 個",
			ErrStrategyBotValidation, strategyBotSignalSourceMaxCount)
	}

	settledSources := make([]dto.StrategyBotSignalSourceWriteDto, 0, len(sources))
	labels := make([]string, 0, len(sources))

	for _, source := range sources {
		label := strings.TrimSpace(source.Label)
		if label == "" {
			return StrategyBotSignalSourcesDomain{}, fmt.Errorf(
				"%w: 每一個信號來源都要有一個代號", ErrStrategyBotValidation)
		}

		if len([]rune(label)) > strategyBotSignalSourceLabelMaxLength {
			return StrategyBotSignalSourcesDomain{}, fmt.Errorf(
				"%w: 信號來源代號長度上限為 %d 個字",
				ErrStrategyBotValidation, strategyBotSignalSourceLabelMaxLength)
		}

		// Two sources answering to one label would make every condition naming it
		// mean two things at once, and no reading of that is the one somebody
		// intended.
		if slices.Contains(labels, label) {
			return StrategyBotSignalSourcesDomain{}, fmt.Errorf(
				"%w: 信號來源代號 %q 重複了，同一台機器人內的代號必須各不相同",
				ErrStrategyBotValidation, label)
		}

		if source.StrategyScriptID == 0 {
			return StrategyBotSignalSourcesDomain{}, fmt.Errorf(
				"%w: 信號來源 %q 必須指名一支策略腳本", ErrStrategyBotValidation, label)
		}

		aggregationInterval, intervalError := NewAggregationIntervalDomain(source.AggregationInterval)
		if intervalError != nil {
			return StrategyBotSignalSourcesDomain{}, fmt.Errorf(
				"%w: 信號來源 %q 的彙總刻度不對：%w", ErrStrategyBotValidation, label, intervalError)
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
				return StrategyBotSignalSourcesDomain{}, fmt.Errorf(
					"%w: 信號來源 %q 給了參數 %q 的值，但它指名的那支策略腳本沒有宣告這個名字",
					ErrStrategyBotValidation, label, parameterValue.Name)
			}
		}

		source.Label = label
		source.AggregationInterval = string(aggregationInterval.Value())
		settledSources = append(settledSources, source)
		labels = append(labels, label)
	}

	return StrategyBotSignalSourcesDomain{sources: settledSources, labels: labels}, nil
}

// Labels are what the conditions may name, in the order they were declared.
func (strategyBotSignalSourcesDomain StrategyBotSignalSourcesDomain) Labels() []string {
	return strategyBotSignalSourcesDomain.labels
}

// ToEntities flattens the sources into the rows they are stored as. Identifiers are
// left unset: they belong to the store.
func (strategyBotSignalSourcesDomain StrategyBotSignalSourcesDomain) ToEntities() []entities.StrategyBotSignalSource {
	signalSources := make(
		[]entities.StrategyBotSignalSource, 0, len(strategyBotSignalSourcesDomain.sources))

	for _, source := range strategyBotSignalSourcesDomain.sources {
		parameterValues := make(
			[]entities.StrategyBotSignalSourceParameterValue, 0, len(source.ParameterValues))
		for _, parameterValue := range source.ParameterValues {
			parameterValues = append(parameterValues, entities.StrategyBotSignalSourceParameterValue{
				Name:  strings.TrimSpace(parameterValue.Name),
				Value: parameterValue.Value,
			})
		}

		signalSources = append(signalSources, entities.StrategyBotSignalSource{
			Label:               source.Label,
			StrategyScriptID:    source.StrategyScriptID,
			AggregationInterval: source.AggregationInterval,
			ParameterValues:     parameterValues,
		})
	}

	return signalSources
}
