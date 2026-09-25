package assistantqueries

import (
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// tradingStrategyConditionAssistantArgument is a condition tree node: a comparison against one
// signal source when Operator is empty, otherwise a group of nested conditions.
type tradingStrategyConditionAssistantArgument struct {
	Operator    string                                      `json:"operator"`
	Conditions  []tradingStrategyConditionAssistantArgument `json:"conditions"`
	SourceLabel string                                      `json:"sourceLabel"`
	Signal      string                                      `json:"signal"`
}

func (tradingStrategyConditionAssistantArgument tradingStrategyConditionAssistantArgument) ToDto() dto.TradingStrategyConditionDto {
	conditionDtos := make(
		[]dto.TradingStrategyConditionDto, 0, len(tradingStrategyConditionAssistantArgument.Conditions))
	for _, condition := range tradingStrategyConditionAssistantArgument.Conditions {
		conditionDtos = append(conditionDtos, condition.ToDto())
	}

	return dto.TradingStrategyConditionDto{
		Operator:    tradingStrategyConditionAssistantArgument.Operator,
		Conditions:  conditionDtos,
		SourceLabel: tradingStrategyConditionAssistantArgument.SourceLabel,
		Signal:      tradingStrategyConditionAssistantArgument.Signal,
	}
}

type tradingStrategyParameterValueAssistantArgument struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

type tradingStrategySignalSourceAssistantArgument struct {
	Label               string                                           `json:"label"`
	StrategyScriptID    uint                                             `json:"strategyScriptId"`
	AggregationInterval string                                           `json:"aggregationInterval"`
	ParameterValues     []tradingStrategyParameterValueAssistantArgument `json:"parameterValues"`
}

// tradingStrategyWriteAssistantArguments serves both build and rewrite, since a rewrite replaces
// every field; a zero identifier means the strategy is new.
type tradingStrategyWriteAssistantArguments struct {
	TradingStrategyID uint                                           `json:"tradingStrategyId"`
	Name              string                                         `json:"name"`
	SignalSources     []tradingStrategySignalSourceAssistantArgument `json:"signalSources"`
	BuyCondition      tradingStrategyConditionAssistantArgument      `json:"buyCondition"`
	SellCondition     tradingStrategyConditionAssistantArgument      `json:"sellCondition"`
}

// ToWriteDto takes the identity from the calling capability and has no owner field at all, so the
// application always sets the owner from whoever the assistant acts for.
func (tradingStrategyWriteAssistantArguments tradingStrategyWriteAssistantArguments) ToWriteDto(id uint) dto.TradingStrategyWriteDto {
	signalSourceWriteDtos := make(
		[]dto.TradingStrategySignalSourceWriteDto, 0, len(tradingStrategyWriteAssistantArguments.SignalSources))

	for _, signalSource := range tradingStrategyWriteAssistantArguments.SignalSources {
		parameterValues := make(
			[]dto.StrategyScriptParameterValueDto, 0, len(signalSource.ParameterValues))
		for _, parameterValue := range signalSource.ParameterValues {
			parameterValues = append(parameterValues, dto.StrategyScriptParameterValueDto{
				Name:  parameterValue.Name,
				Value: parameterValue.Value,
			})
		}

		signalSourceWriteDtos = append(signalSourceWriteDtos, dto.TradingStrategySignalSourceWriteDto{
			Label:               signalSource.Label,
			StrategyScriptID:    signalSource.StrategyScriptID,
			AggregationInterval: signalSource.AggregationInterval,
			ParameterValues:     parameterValues,
		})
	}

	return dto.TradingStrategyWriteDto{
		ID:            id,
		Name:          tradingStrategyWriteAssistantArguments.Name,
		SignalSources: signalSourceWriteDtos,
		BuyCondition:  tradingStrategyWriteAssistantArguments.BuyCondition.ToDto(),
		SellCondition: tradingStrategyWriteAssistantArguments.SellCondition.ToDto(),
	}
}

// tradingStrategyConditionArgumentSchema describes nesting in prose because the schema handed over
// keeps only top-level properties; the domain remains the real validator of the tree.
const tradingStrategyConditionArgumentSchema = `{"type":"object","description":` +
	`"一棵條件樹。每個節點二選一：【比較】{\"sourceLabel\":\"A\",\"signal\":\"buy\"}——` +
	`那個來源必須說出這個信號；【群組】{\"operator\":\"and\",\"conditions\":[子節點,子節點]}——` +
	`至少兩個子條件，子節點同樣是這兩種之一，可再往下巢狀。` +
	`signal 三選一：buy／sell／hold；沒有「不等於」，「不是買」寫成「賣 或 持平」。` +
	`深度上限 5、單棵樹節點上限 32、不得為空。",` +
	`"properties":{` +
	`"operator":{"type":"string","enum":["and","or"],"description":"群組節點才有；比較節點不給"},` +
	`"conditions":{"type":"array","items":{"type":"object"},"description":"群組節點的子條件，至少兩個"},` +
	`"sourceLabel":{"type":"string","description":"比較節點才有：要看哪一個來源代號"},` +
	`"signal":{"type":"string","enum":["buy","sell","hold"],"description":"比較節點才有：那個來源要說出的信號"}` +
	`}}`

// tradingStrategyWriteArgumentSchema is shared by both writing capabilities, which differ only in
// whether the identifier is required.
const tradingStrategyWriteArgumentSchema = `` +
	`"name":{"type":"string","description":"交易策略名稱，不得空白、不得與自己既有的交易策略重複，上限 128 字"},` +
	`"signalSources":{"type":"array","description":"這份交易策略聽哪幾支策略腳本說話，上限 10 個。` +
	`每個來源的彙總刻度必須相同，否則回測與上線都會被拒絕",` +
	`"items":{"type":"object","properties":{` +
	`"label":{"type":"string","description":"這個來源在條件裡叫什麼（A、B、C…），不得空白、同一份內不得重複"},` +
	`"strategyScriptId":{"type":"integer","description":"指名哪一支策略腳本；必須是自己的或市集上的"},` +
	`"aggregationInterval":{"type":"string","enum":["1m","5m","15m","1h","4h","1d"],` +
	`"description":"這個來源看多粗的 K 線。同一份交易策略的每個來源必須一樣"},` +
	`"parameterValues":{"type":"array","description":"這個來源把那支腳本的參數設成多少；` +
	`只能給那支腳本宣告過的參數名稱","items":{"type":"object","properties":{` +
	`"name":{"type":"string"},"value":{"type":"number"}},"required":["name","value"],"additionalProperties":false}}` +
	`},"required":["label","strategyScriptId","aggregationInterval"],"additionalProperties":false}},` +
	`"buyCondition":` + tradingStrategyConditionArgumentSchema + `,` +
	`"sellCondition":` + tradingStrategyConditionArgumentSchema

// renderedTradingStrategy is the one shape building, rewriting and reading all hand back.
func renderedTradingStrategy(tradingStrategyDto dto.TradingStrategyDto) (string, error) {
	payload, marshalError := json.Marshal(tradingStrategyDto)
	if marshalError != nil {
		return "", fmt.Errorf("render trading strategy: %w", marshalError)
	}

	return string(payload), nil
}
