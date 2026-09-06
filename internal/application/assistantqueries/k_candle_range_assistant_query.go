package assistantqueries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// kCandleRangeAssistantArguments is what the assistant sends to ask for the raw
// candles of a stretch.
type kCandleRangeAssistantArguments struct {
	Symbol      string `json:"symbol"`
	StartTime   string `json:"startTime"`
	EndTime     string `json:"endTime"`
	CandleCount int    `json:"candleCount"`
}

// KCandleRangeAssistantQuery lets the assistant read the raw five-minute candles of a
// stretch.
//
// It is offered alongside the aggregated series, not instead of it, because some
// questions really are about the five-minute detail. It obeys the same ceiling, and
// the description points the assistant at the series first: raw candles are the
// expensive way to look at anything longer than a few hours.
type KCandleRangeAssistantQuery struct {
	kCandleApplication *application.KCandleApplication
	candleLimit        int
}

func NewKCandleRangeAssistantQuery(
	kCandleApplication *application.KCandleApplication, candleLimit int,
) *KCandleRangeAssistantQuery {
	return &KCandleRangeAssistantQuery{
		kCandleApplication: kCandleApplication,
		candleLimit:        candleLimit,
	}
}

func (kCandleRangeAssistantQuery *KCandleRangeAssistantQuery) Name() string {
	return "get_k_candles"
}

func (kCandleRangeAssistantQuery *KCandleRangeAssistantQuery) Description() string {
	return "查一段時間內的原始五分鐘 K 線。時間一律用 RFC3339。" +
		fmt.Sprintf("candleCount 上限 %d 根，未給即視為上限，回傳那一段裡最新的幾根。", kCandleRangeAssistantQuery.candleLimit) +
		"只有真的需要五分鐘級細節時才用；問走勢或形狀請改用 get_k_candle_series。"
}

func (kCandleRangeAssistantQuery *KCandleRangeAssistantQuery) ArgumentSchema() string {
	return `{"type":"object","properties":{` +
		`"symbol":{"type":"string","description":"交易標的代號，例如 BTCUSDT"},` +
		`"startTime":{"type":"string","description":"起始時間，RFC3339"},` +
		`"endTime":{"type":"string","description":"結束時間，RFC3339"},` +
		`"candleCount":{"type":"integer","description":"想看幾根，未給即視為上限"}` +
		`},"required":["symbol","startTime","endTime"],"additionalProperties":false}`
}

// Run reads the stretch and hands over at most the ceiling's worth of it, most recent
// last. Every rule the underlying query obeys is obeyed here unrelaxed.
func (kCandleRangeAssistantQuery *KCandleRangeAssistantQuery) Run(
	executionContext context.Context, arguments string,
) (string, error) {
	rangeArguments := kCandleRangeAssistantArguments{}
	if unmarshalError := json.Unmarshal([]byte(arguments), &rangeArguments); unmarshalError != nil {
		return "", fmt.Errorf("%w: 參數不是合法的 JSON: %s", domains.ErrAssistantQueryArgument, unmarshalError)
	}

	candleLimit, limitError := domains.NewAssistantCandleLimitDomain(
		kCandleRangeAssistantQuery.candleLimit, rangeArguments.CandleCount)
	if limitError != nil {
		return "", limitError
	}

	startTime, startTimeError := assistantMomentOf(rangeArguments.StartTime, "startTime")
	if startTimeError != nil {
		return "", startTimeError
	}

	endTime, endTimeError := assistantMomentOf(rangeArguments.EndTime, "endTime")
	if endTimeError != nil {
		return "", endTimeError
	}

	kCandleDtos, findError := kCandleRangeAssistantQuery.kCandleApplication.GetKCandlesInRange(
		executionContext, dto.KCandleQueryDto{
			Symbol:    rangeArguments.Symbol,
			StartTime: startTime,
			EndTime:   endTime,
		})
	if findError != nil {
		return "", findError
	}

	shownCandles, truncated := mostRecentCandles(kCandleDtos, candleLimit)

	payload, marshalError := json.Marshal(struct {
		Symbol   string           `json:"symbol"`
		Count    int              `json:"count"`
		KCandles []dto.KCandleDto `json:"kCandles"`
		Note     string           `json:"note,omitempty"`
	}{
		Symbol:   rangeArguments.Symbol,
		Count:    len(shownCandles),
		KCandles: shownCandles,
		Note:     assistantCandleNoteFor(len(kCandleDtos), len(shownCandles), truncated),
	})
	if marshalError != nil {
		return "", fmt.Errorf("render k candles: %w", marshalError)
	}

	return string(payload), nil
}
