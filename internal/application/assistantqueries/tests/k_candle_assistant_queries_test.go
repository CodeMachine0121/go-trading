package assistantqueries_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/application/assistantqueries"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// assistantCandleLimit is the ceiling the two K candle capabilities run under in these
// tests. It is small so that a test can cross it without building two hundred candles.
const assistantCandleLimit = 3

type kCandleAssistantQueriesUnderTest struct {
	seriesAssistantQuery *assistantqueries.KCandleSeriesAssistantQuery
	rangeAssistantQuery  *assistantqueries.KCandleRangeAssistantQuery
	kCandleRepository    *mocks.MockIKCandleRepository
}

// newKCandleAssistantQueriesUnderTest wires the real domain service and real domain
// models, mocking only storage and the clock — so the assistant's request goes
// through every rule a person's own request goes through.
func newKCandleAssistantQueriesUnderTest(t *testing.T) kCandleAssistantQueriesUnderTest {
	controller := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(at(23, 55)).AnyTimes()

	kCandleApplication := application.NewKCandleApplication(
		service.NewKCandleService(kCandleRepository, clockProxy, queryMaxResults))

	return kCandleAssistantQueriesUnderTest{
		seriesAssistantQuery: assistantqueries.NewKCandleSeriesAssistantQuery(
			kCandleApplication, assistantCandleLimit),
		rangeAssistantQuery: assistantqueries.NewKCandleRangeAssistantQuery(
			kCandleApplication, assistantCandleLimit),
		kCandleRepository: kCandleRepository,
	}
}

// storedCandles are candles five minutes apart, so that each one falls in its own
// five-minute bucket and the count the assistant sees is the count stored.
func storedCandles(count int) []entities.KCandle {
	kCandles := make([]entities.KCandle, 0, count)
	for candleNumber := range count {
		kCandles = append(kCandles,
			kCandleAt(at(9, candleNumber*5), fmt.Sprintf("%d", 100+candleNumber)))
	}

	return kCandles
}

// assistantCandlePayload is what the assistant reads back, only as far as these tests
// look into it.
type assistantCandlePayload struct {
	Symbol   string `json:"symbol"`
	Interval string `json:"interval"`
	Count    int    `json:"count"`
	Note     string `json:"note"`
	KCandles []struct {
		Close string `json:"close"`
	} `json:"kCandles"`
}

func readAssistantCandlePayload(t *testing.T, outcome string) assistantCandlePayload {
	payload := assistantCandlePayload{}
	require.NoError(t, json.Unmarshal([]byte(outcome), &payload))

	return payload
}

const seriesArguments = `{"symbol":"BTCUSDT","startTime":"2026-08-29T09:00:00Z",` +
	`"endTime":"2026-08-29T23:00:00Z","interval":"1h"}`

func TestKCandleSeriesAssistantQueryReadsTheStretchAtTheCoarsenessAsked(t *testing.T) {
	fixture := newKCandleAssistantQueriesUnderTest(t)
	fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(storedCandles(2), nil)

	outcome, runError := fixture.seriesAssistantQuery.Run(t.Context(), seriesArguments)

	require.NoError(t, runError)
	payload := readAssistantCandlePayload(t, outcome)
	assert.Equal(t, "BTCUSDT", payload.Symbol)
	assert.Equal(t, "1h", payload.Interval)
	// Two candles five minutes apart fall in one hour-long bucket.
	assert.Equal(t, 1, payload.Count)
	assert.Empty(t, payload.Note)
}

func TestKCandleSeriesAssistantQueryTellsTheAssistantWhenItIsSeeingLessThanTheStretchHolds(t *testing.T) {
	// Silence here is the one failure a cost ceiling could cause on its own: an
	// assistant shown a slice as the whole will describe a trend that is not there.
	fixture := newKCandleAssistantQueriesUnderTest(t)
	fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(storedCandles(5), nil)

	outcome, runError := fixture.seriesAssistantQuery.Run(t.Context(),
		`{"symbol":"BTCUSDT","startTime":"2026-08-29T09:00:00Z","endTime":"2026-08-29T23:00:00Z"}`)

	require.NoError(t, runError)
	payload := readAssistantCandlePayload(t, outcome)
	assert.Equal(t, assistantCandleLimit, payload.Count)
	assert.Contains(t, payload.Note, "已截斷")
	assert.Contains(t, payload.Note, "共有 5 根")
	// The newest are what every question about a market is about.
	assert.Equal(t, "104", payload.KCandles[len(payload.KCandles)-1].Close)
}

func TestKCandleSeriesAssistantQueryTreatsNamingNoCountAsTheCeiling(t *testing.T) {
	// There is deliberately no way to ask for everything, and not saying how many is
	// not asking for everything — so nothing was withheld and nothing is reported.
	fixture := newKCandleAssistantQueriesUnderTest(t)
	fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(storedCandles(3), nil)

	outcome, runError := fixture.seriesAssistantQuery.Run(t.Context(),
		`{"symbol":"BTCUSDT","startTime":"2026-08-29T09:00:00Z","endTime":"2026-08-29T23:00:00Z"}`)

	require.NoError(t, runError)
	payload := readAssistantCandlePayload(t, outcome)
	assert.Equal(t, 3, payload.Count)
	assert.Empty(t, payload.Note)
}

func TestKCandleSeriesAssistantQuerySaysWhenTheStretchHeldNothing(t *testing.T) {
	// Nothing there is an answer the assistant relays, not a refusal it works around.
	fixture := newKCandleAssistantQueriesUnderTest(t)
	fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entities.KCandle{}, nil)

	outcome, runError := fixture.seriesAssistantQuery.Run(t.Context(), seriesArguments)

	require.NoError(t, runError)
	payload := readAssistantCandlePayload(t, outcome)
	assert.Equal(t, 0, payload.Count)
	assert.Contains(t, payload.Note, "沒有任何 K 線資料")
}

func TestKCandleSeriesAssistantQueryObeysTheRulesTheUnderlyingQueryAlreadyHas(t *testing.T) {
	testCases := []struct {
		name            string
		arguments       string
		expectedMessage string
	}{
		{
			// Not one rule is relaxed for the assistant, and the reason it was
			// refused is what it reads — so it can ask again correctly.
			name: "a coarseness the system does not recognise",
			arguments: `{"symbol":"BTCUSDT","startTime":"2026-08-29T09:00:00Z",` +
				`"endTime":"2026-08-29T23:00:00Z","interval":"7m"}`,
			expectedMessage: "彙總刻度",
		},
		{
			name: "a stretch that ends before it starts",
			arguments: `{"symbol":"BTCUSDT","startTime":"2026-08-29T23:00:00Z",` +
				`"endTime":"2026-08-29T09:00:00Z"}`,
			expectedMessage: "結束時間",
		},
		{
			name: "no market named",
			arguments: `{"symbol":"","startTime":"2026-08-29T09:00:00Z",` +
				`"endTime":"2026-08-29T23:00:00Z"}`,
			expectedMessage: "交易標的",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newKCandleAssistantQueriesUnderTest(t)

			_, runError := fixture.seriesAssistantQuery.Run(t.Context(), testCase.arguments)

			require.Error(t, runError)
			assert.Contains(t, runError.Error(), testCase.expectedMessage)
		})
	}
}

func TestKCandleSeriesAssistantQueryRefusesArgumentsItCannotRead(t *testing.T) {
	testCases := []struct {
		name            string
		arguments       string
		expectedMessage string
	}{
		{name: "not JSON at all", arguments: `not json`, expectedMessage: "不是合法的 JSON"},
		{
			name: "a count below zero",
			arguments: `{"symbol":"BTCUSDT","startTime":"2026-08-29T09:00:00Z",` +
				`"endTime":"2026-08-29T23:00:00Z","candleCount":-1}`,
			expectedMessage: "必須大於零",
		},
		{
			name:            "a moment that is not a moment",
			arguments:       `{"symbol":"BTCUSDT","startTime":"昨天","endTime":"2026-08-29T23:00:00Z"}`,
			expectedMessage: "RFC3339",
		},
		{
			name:            "an end that is not a moment",
			arguments:       `{"symbol":"BTCUSDT","startTime":"2026-08-29T09:00:00Z","endTime":"明天"}`,
			expectedMessage: "RFC3339",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newKCandleAssistantQueriesUnderTest(t)

			_, runError := fixture.seriesAssistantQuery.Run(t.Context(), testCase.arguments)

			require.ErrorIs(t, runError, domains.ErrAssistantQueryArgument)
			assert.Contains(t, runError.Error(), testCase.expectedMessage)
		})
	}
}

func TestKCandleRangeAssistantQueryObeysTheSameCeiling(t *testing.T) {
	fixture := newKCandleAssistantQueriesUnderTest(t)
	fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(storedCandles(5), nil)

	outcome, runError := fixture.rangeAssistantQuery.Run(t.Context(),
		`{"symbol":"BTCUSDT","startTime":"2026-08-29T09:00:00Z","endTime":"2026-08-29T23:00:00Z",`+
			`"candleCount":500}`)

	require.NoError(t, runError)
	payload := readAssistantCandlePayload(t, outcome)
	assert.Equal(t, assistantCandleLimit, payload.Count)
	assert.Contains(t, payload.Note, "已截斷")
	assert.Equal(t, "104", payload.KCandles[len(payload.KCandles)-1].Close)
}

func TestKCandleRangeAssistantQueryHandsOverWhatFitsWithinTheCeiling(t *testing.T) {
	fixture := newKCandleAssistantQueriesUnderTest(t)
	fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(storedCandles(2), nil)

	outcome, runError := fixture.rangeAssistantQuery.Run(t.Context(),
		`{"symbol":"BTCUSDT","startTime":"2026-08-29T09:00:00Z","endTime":"2026-08-29T23:00:00Z",`+
			`"candleCount":2}`)

	require.NoError(t, runError)
	payload := readAssistantCandlePayload(t, outcome)
	assert.Equal(t, 2, payload.Count)
	assert.Empty(t, payload.Note)
}

func TestKCandleRangeAssistantQueryRefusesArgumentsItCannotRead(t *testing.T) {
	testCases := []struct {
		name            string
		arguments       string
		expectedMessage string
	}{
		{name: "not JSON at all", arguments: `nope`, expectedMessage: "不是合法的 JSON"},
		{
			name: "a count below zero",
			arguments: `{"symbol":"BTCUSDT","startTime":"2026-08-29T09:00:00Z",` +
				`"endTime":"2026-08-29T23:00:00Z","candleCount":-5}`,
			expectedMessage: "必須大於零",
		},
		{
			name:            "a start that is not a moment",
			arguments:       `{"symbol":"BTCUSDT","startTime":"昨天","endTime":"2026-08-29T23:00:00Z"}`,
			expectedMessage: "RFC3339",
		},
		{
			name:            "an end that is not a moment",
			arguments:       `{"symbol":"BTCUSDT","startTime":"2026-08-29T09:00:00Z","endTime":"明天"}`,
			expectedMessage: "RFC3339",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newKCandleAssistantQueriesUnderTest(t)

			_, runError := fixture.rangeAssistantQuery.Run(t.Context(), testCase.arguments)

			require.ErrorIs(t, runError, domains.ErrAssistantQueryArgument)
			assert.Contains(t, runError.Error(), testCase.expectedMessage)
		})
	}
}

func TestKCandleRangeAssistantQueryObeysTheRulesTheUnderlyingQueryAlreadyHas(t *testing.T) {
	fixture := newKCandleAssistantQueriesUnderTest(t)

	_, runError := fixture.rangeAssistantQuery.Run(t.Context(),
		`{"symbol":"","startTime":"2026-08-29T09:00:00Z","endTime":"2026-08-29T23:00:00Z"}`)

	require.Error(t, runError)
	assert.Contains(t, runError.Error(), "交易標的")
}
