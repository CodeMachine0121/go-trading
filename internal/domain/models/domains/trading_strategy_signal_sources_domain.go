package domains

import (
	"fmt"
	"slices"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
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

		// A condition compares a source against buy, sell or hold. A strategy script
		// that hands back numbers or true/false answers has none of those to give, so
		// every condition naming this source would be a sentence with nothing on the
		// other side of it — and nothing would say so until the bot woke up at three in
		// the morning and stopped on a script failure.
		//
		// Refused here rather than filtered out of the picker, because the picker is
		// one of several ways a source is named: the screen, the assistant, and a
		// request written by hand all arrive at this model.
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

	// Every source having its own coarseness would let somebody save a trading
	// strategy that cannot do anything: replaying it is refused, and once it is
	// running, two sources on two different clocks have their opinions read as two
	// sentences about the same candle. Asked here, the refusal arrives while they are
	// still looking at the form.
	//
	// It is asked after the loop rather than inside it because the question is about
	// the set. Asked one at a time, the first source would be judged against nothing.
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

// Labels are what the conditions may name, in the order they were declared.
// RequireMarketDataKind refuses a source whose strategy script eats a different kind
// of market from the trading strategy, naming it. The two kinds' bars are not the
// same bars, so a condition tree cannot read one source's opinion beside another's.
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
