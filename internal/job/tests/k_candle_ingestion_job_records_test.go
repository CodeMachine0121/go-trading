package job_test

import (
	"errors"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/CodeMachine0121/go-trading/internal/job"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// recordedLines carries whatever the job wrote down out of its goroutine.
type recordedLines struct {
	lines chan string
}

func (recorded recordedLines) Write(line []byte) (int, error) {
	recorded.lines <- string(line)

	return len(line), nil
}

func captureRecords(t *testing.T) recordedLines {
	t.Helper()

	recorded := recordedLines{lines: make(chan string, 64)}
	previousOutput := log.Writer()
	log.SetOutput(recorded)
	t.Cleanup(func() { log.SetOutput(previousOutput) })

	return recorded
}

func (recorded recordedLines) waitFor(t *testing.T, wanted string) string {
	t.Helper()

	deadline := time.After(2 * time.Second)
	for {
		select {
		case line := <-recorded.lines:
			if strings.Contains(line, wanted) {
				return line
			}
		case <-deadline:
			t.Fatalf("nothing was recorded containing %q", wanted)
			return ""
		}
	}
}

func brokenKCandle(openTime time.Time) vo.MarketKCandleVo {
	return vo.MarketKCandleVo{
		Symbol:              "BTCUSDT",
		OpenTime:            openTime,
		Open:                decimal.RequireFromString("100"),
		High:                decimal.RequireFromString("90"),
		Low:                 decimal.RequireFromString("100"),
		Close:               decimal.RequireFromString("110"),
		Volume:              decimal.RequireFromString("11"),
		QuoteVolume:         decimal.RequireFromString("1200"),
		TakerBuyBaseVolume:  decimal.RequireFromString("5"),
		TakerBuyQuoteVolume: decimal.RequireFromString("600"),
	}
}

// startJobWith wires the real ingestion path around the given source behaviour and
// starts the job, which begins with its backfill.
func startJobWith(
	t *testing.T,
	roundCandleCount int,
	fetch func(window vo.KCandleFetchWindowVo) ([]vo.MarketKCandleVo, error),
) {
	t.Helper()

	mockController := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)
	marketDataProxy := mocks.NewMockIMarketDataProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(currentTime).AnyTimes()
	kCandleRepository.EXPECT().FindLatest(gomock.Any(), 1).
		Return([]entities.KCandle{}, nil).AnyTimes()
	kCandleRepository.EXPECT().Save(gomock.Any()).Return(entities.KCandle{}, nil).AnyTimes()
	marketDataProxy.EXPECT().FetchKCandles(gomock.Any()).DoAndReturn(fetch).AnyTimes()

	ingestionJob := job.NewKCandleIngestionJob(
		application.NewKCandleIngestionApplication(
			service.NewKCandleIngestionService(
				kCandleRepository, marketDataProxy, clockProxy, roundCandleCount, lookback)),
		[]string{"BTCUSDT"}, time.Hour)
	t.Cleanup(ingestionJob.Stop)
	ingestionJob.Start()
}

func TestTheJobRecordsWhichSymbolTheSourceWouldNotAnswerFor(t *testing.T) {
	recorded := captureRecords(t)

	startJobWith(t, roundCandleCount,
		func(window vo.KCandleFetchWindowVo) ([]vo.MarketKCandleVo, error) {
			return nil, errors.New("market source unreachable")
		})

	line := recorded.waitFor(t, "got no answer")
	assert.Contains(t, line, "BTCUSDT")
	assert.Contains(t, line, "market source unreachable")
}

func TestTheJobRecordsWhichCandleBrokeWhichRule(t *testing.T) {
	recorded := captureRecords(t)

	startJobWith(t, roundCandleCount,
		func(window vo.KCandleFetchWindowVo) ([]vo.MarketKCandleVo, error) {
			return []vo.MarketKCandleVo{brokenKCandle(scheduledWindowStart)}, nil
		})

	line := recorded.waitFor(t, "skipped")
	assert.Contains(t, line, "BTCUSDT")
	assert.Contains(t, line, scheduledWindowStart.Format(time.RFC3339))
	assert.Contains(t, line, "最高價不得低於最低價")
}

func TestTheJobRecordsARunThatCouldNotHappenAtAll(t *testing.T) {
	recorded := captureRecords(t)

	startJobWith(t, 0, func(window vo.KCandleFetchWindowVo) ([]vo.MarketKCandleVo, error) {
		return []vo.MarketKCandleVo{}, nil
	})

	assert.Contains(t, recorded.waitFor(t, "did not run"), "startup backfill")
}
