package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// kCandleSeriesAssistantArguments is what the assistant sends to ask for a stretch of
// market at a chosen coarseness.
type kCandleSeriesAssistantArguments struct {
	Symbol      string `json:"symbol"`
	StartTime   string `json:"startTime"`
	EndTime     string `json:"endTime"`
	Interval    string `json:"interval"`
	CandleCount int    `json:"candleCount"`
}

// KCandleSeriesAssistantQuery lets the assistant read a stretch of market at a chosen
// coarseness.
//
// This is the capability most likely to be expensive, so it is the one the candle
// ceiling exists for. The assistant is handed the most recent candles of the stretch
// it asked for, never more than the ceiling, and is told plainly when it is being
// shown less than it asked for — an assistant shown two hundred of five hundred
// candles without being told will describe a trend that is not there.
type KCandleSeriesAssistantQuery struct {
	kCandleApplication *KCandleApplication
	candleLimit        int
}

func NewKCandleSeriesAssistantQuery(
	kCandleApplication *KCandleApplication, candleLimit int,
) *KCandleSeriesAssistantQuery {
	return &KCandleSeriesAssistantQuery{
		kCandleApplication: kCandleApplication,
		candleLimit:        candleLimit,
	}
}

func (kCandleSeriesAssistantQuery *KCandleSeriesAssistantQuery) Name() string {
	return "get_k_candle_series"
}

func (kCandleSeriesAssistantQuery *KCandleSeriesAssistantQuery) Description() string {
	return "查一段時間內的彙總 K 線序列。彙總刻度只接受 5m、15m、1h、4h、1d，未給視為 5m。" +
		"時間一律用 RFC3339（例如 2026-09-04T00:00:00Z）。" +
		fmt.Sprintf("candleCount 是你想看幾根，上限 %d 根，未給即視為上限；", kCandleSeriesAssistantQuery.candleLimit) +
		"回傳的是那一段裡最新的幾根。問「最近走勢」時優先用這個，不要逐根拉原始 K 線。"
}

func (kCandleSeriesAssistantQuery *KCandleSeriesAssistantQuery) ArgumentSchema() string {
	return `{"type":"object","properties":{` +
		`"symbol":{"type":"string","description":"交易標的代號，例如 BTCUSDT"},` +
		`"startTime":{"type":"string","description":"起始時間，RFC3339"},` +
		`"endTime":{"type":"string","description":"結束時間，RFC3339"},` +
		`"interval":{"type":"string","enum":["5m","15m","1h","4h","1d"],"description":"彙總刻度"},` +
		`"candleCount":{"type":"integer","description":"想看幾根，未給即視為上限"}` +
		`},"required":["symbol","startTime","endTime"],"additionalProperties":false}`
}

// Run reads the stretch and hands over at most the ceiling's worth of it, most recent
// last. Every rule the underlying query obeys is obeyed here unrelaxed: an
// unrecognised coarseness, a range that ends before it starts or a stretch too long
// to answer all come back as the reason they were refused, which the assistant reads
// and may act on.
func (kCandleSeriesAssistantQuery *KCandleSeriesAssistantQuery) Run(
	executionContext context.Context, arguments string,
) (string, error) {
	seriesArguments := kCandleSeriesAssistantArguments{}
	if unmarshalError := json.Unmarshal([]byte(arguments), &seriesArguments); unmarshalError != nil {
		return "", fmt.Errorf("%w: 參數不是合法的 JSON: %s", domains.ErrAssistantQueryArgument, unmarshalError)
	}

	candleLimit, limitError := domains.NewAssistantCandleLimitDomain(
		kCandleSeriesAssistantQuery.candleLimit, seriesArguments.CandleCount)
	if limitError != nil {
		return "", limitError
	}

	startTime, startTimeError := assistantMomentOf(seriesArguments.StartTime, "startTime")
	if startTimeError != nil {
		return "", startTimeError
	}

	endTime, endTimeError := assistantMomentOf(seriesArguments.EndTime, "endTime")
	if endTimeError != nil {
		return "", endTimeError
	}

	seriesDto, seriesError := kCandleSeriesAssistantQuery.kCandleApplication.GetKCandleSeries(
		executionContext, dto.KCandleSeriesQueryDto{
			Symbol:    seriesArguments.Symbol,
			StartTime: startTime,
			EndTime:   endTime,
			Interval:  seriesArguments.Interval,
		})
	if seriesError != nil {
		return "", seriesError
	}

	shownCandles, truncated := mostRecentCandles(seriesDto.KCandles, candleLimit)

	payload, marshalError := json.Marshal(struct {
		Symbol   string           `json:"symbol"`
		Interval string           `json:"interval"`
		Count    int              `json:"count"`
		KCandles []dto.KCandleDto `json:"kCandles"`
		Note     string           `json:"note,omitempty"`
	}{
		Symbol:   seriesDto.Symbol,
		Interval: seriesDto.Interval,
		Count:    len(shownCandles),
		KCandles: shownCandles,
		Note:     assistantCandleNoteFor(len(seriesDto.KCandles), len(shownCandles), truncated),
	})
	if marshalError != nil {
		return "", fmt.Errorf("render k candle series: %w", marshalError)
	}

	return string(payload), nil
}
