package assistantqueries

import (
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// tradingStrategyConditionAssistantArgument is one node of a condition tree as the
// assistant declares it: either a comparison against one signal source, or a group of
// conditions joined by an operator.
//
// One shape carries both, exactly as it does for a person: the operator being empty
// is what says this one is a comparison. It recurses through itself, so a condition
// nested five deep needs no more code than one nested once.
type tradingStrategyConditionAssistantArgument struct {
	Operator    string                                      `json:"operator"`
	Conditions  []tradingStrategyConditionAssistantArgument `json:"conditions"`
	SourceLabel string                                      `json:"sourceLabel"`
	Signal      string                                      `json:"signal"`
}

// ToDto turns this node and everything under it into the domain's shape.
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

// tradingStrategyParameterValueAssistantArgument is what one knob is worth in one
// source.
type tradingStrategyParameterValueAssistantArgument struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// tradingStrategySignalSourceAssistantArgument is one strategy script as it is to run
// inside this trading strategy.
type tradingStrategySignalSourceAssistantArgument struct {
	Label               string                                           `json:"label"`
	StrategyScriptID    uint                                             `json:"strategyScriptId"`
	AggregationInterval string                                           `json:"aggregationInterval"`
	ParameterValues     []tradingStrategyParameterValueAssistantArgument `json:"parameterValues"`
}

// tradingStrategyWriteAssistantArguments is what the assistant sends to build or
// rewrite a trading strategy.
//
// One shape serves both, exactly as it does for a person: a rewrite replaces
// everything a trading strategy remembers, so there is no field the assistant may set
// on one path and not the other. Which trading strategy is meant is the identifier,
// and a zero one means none yet.
type tradingStrategyWriteAssistantArguments struct {
	TradingStrategyID uint   `json:"tradingStrategyId"`
	Name              string `json:"name"`
	// TradingMode is which set of rules these are written for. It is left out rather
	// than guessed when the person did not say: the assistant knows what the two modes
	// mean, but which account the person actually holds is not something it can work
	// out from the conditions it was asked to write.
	TradingMode   string                                         `json:"tradingMode"`
	SignalSources []tradingStrategySignalSourceAssistantArgument `json:"signalSources"`
	BuyCondition  tradingStrategyConditionAssistantArgument      `json:"buyCondition"`
	SellCondition tradingStrategyConditionAssistantArgument      `json:"sellCondition"`
}

// ToWriteDto turns what the assistant declared into the shape the domain judges,
// taking the identity from the argument so that the capability calling this decides
// whether a trading strategy is being built or rewritten.
//
// The owner is not taken here at all — there is no field for one. It is settled by
// the application from whoever the assistant is acting for, which is what makes
// "the assistant never names an owner" true of the shape rather than of a check
// somebody keeps having to make.
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
		TradingMode:   tradingStrategyWriteAssistantArguments.TradingMode,
		SignalSources: signalSourceWriteDtos,
		BuyCondition:  tradingStrategyWriteAssistantArguments.BuyCondition.ToDto(),
		SellCondition: tradingStrategyWriteAssistantArguments.SellCondition.ToDto(),
	}
}

// tradingStrategyConditionArgumentSchema is one node of a condition tree as the
// assistant is told to send it.
//
// The nesting is described in words rather than declared, because the schema handed
// over carries only the top level's properties — a reference back to itself would
// arrive stripped and leave the assistant with no description at all. Nothing is
// lost by saying it in prose: the tree's real gatekeeper is the domain, which checks
// depth, node count, operators and labels and hands any refusal straight back for
// the assistant to correct. A schema that tried to enforce the same rules would be a
// second, weaker copy of them.
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

// tradingStrategyWriteArgumentSchema is the arguments both writing capabilities take.
// It is written once because they take the same ones — the only difference is whether
// the identifier is required, and each says that for itself.
const tradingStrategyWriteArgumentSchema = `` +
	`"name":{"type":"string","description":"交易策略名稱，不得空白、不得與自己既有的交易策略重複，上限 128 字"},` +
	`"tradingMode":{"type":"string","enum":["longShort","spot"],` +
	`"description":"這份規則是寫給哪一種帳戶的：longShort 做得了空（賣出＝平多並反手做空）、` +
	`spot 只做多（賣出＝平倉把錢收回來，空手時賣出不動作）。不給即 longShort。` +
	`使用者說他的帳戶不能放空（台股現貨、ETF、多數券商帳戶）時給 spot——` +
	`它決定了重演這份規則時用哪一套算法，也決定了機器人的訊息要寫「買入／賣出」還是「做多／做空」"},` +
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

// renderedTradingStrategy is one trading strategy as the assistant reads it. Building
// one, rewriting one and reading one all hand back the same shape, so the assistant
// never has to learn two ways of looking at the same thing.
func renderedTradingStrategy(tradingStrategyDto dto.TradingStrategyDto) (string, error) {
	payload, marshalError := json.Marshal(tradingStrategyDto)
	if marshalError != nil {
		return "", fmt.Errorf("render trading strategy: %w", marshalError)
	}

	return string(payload), nil
}
