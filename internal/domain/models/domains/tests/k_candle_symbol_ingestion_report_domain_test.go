package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func skippedAt(minute int) dto.SkippedKCandleDto {
	return dto.SkippedKCandleDto{
		OpenTime: time.Date(2026, 9, 22, 9, minute, 0, 0, time.UTC),
		Reason:   "標記價格不得留白",
	}
}

func TestIngestionReportStartsWithAListRatherThanNothing(t *testing.T) {
	// Every way out of a fetch answers with a list. Left to appear only on the path
	// that reaches the source, it would be absent on exactly the paths a reader
	// inspects it on — a shut market, a source that would not answer.
	reportDomain := domains.NewKCandleSymbolIngestionReportDomain("BTCUSDT", vo.MarketCrypto)

	report := reportDomain.ToDto()

	assert.Equal(t, "BTCUSDT", report.Symbol)
	assert.Equal(t, "crypto", report.Market)
	assert.NotNil(t, report.SkippedKCandles)
	assert.Empty(t, report.SkippedKCandles)
	assert.False(t, report.WasAsked)
	assert.Zero(t, report.StoredCount)
	assert.Zero(t, report.SkippedCount)
	assert.False(t, report.SkippedKCandlesTruncated)
	assert.Empty(t, report.FetchFailureReason)
}

func TestIngestionReportCollectsWhatOneFetchDid(t *testing.T) {
	reportDomain := domains.NewKCandleSymbolIngestionReportDomain("BTCUSDT", vo.MarketCrypto)

	reportDomain.NoteAsked()
	reportDomain.NoteStored(3)
	reportDomain.NoteStored(1)
	reportDomain.NoteSkipped(skippedAt(1), skippedAt(2))

	report := reportDomain.ToDto()
	assert.True(t, report.WasAsked)
	assert.Equal(t, 4, report.StoredCount)
	assert.Equal(t, 2, report.SkippedCount)
	require.Len(t, report.SkippedKCandles, 2)
	assert.Equal(t, skippedAt(1).OpenTime, report.SkippedKCandles[0].OpenTime)
	assert.False(t, report.SkippedKCandlesTruncated)
}

func TestIngestionReportStopsNamingSkippedCandlesButKeepsCounting(t *testing.T) {
	// The count answers "how bad is it" without the list having to, so a source
	// answering with rubbish for four years cannot make the report unopenable.
	reportDomain := domains.NewKCandleSymbolIngestionReportDomain("BTCUSDT", vo.MarketCrypto)

	for minute := range 250 {
		reportDomain.NoteSkipped(skippedAt(minute))
	}

	report := reportDomain.ToDto()
	assert.Equal(t, 250, report.SkippedCount)
	assert.Len(t, report.SkippedKCandles, 200)
	assert.True(t, report.SkippedKCandlesTruncated)
}

func TestIngestionReportNamesExactlyTheLimitWithoutClaimingItWasTruncated(t *testing.T) {
	reportDomain := domains.NewKCandleSymbolIngestionReportDomain("BTCUSDT", vo.MarketCrypto)

	for minute := range 200 {
		reportDomain.NoteSkipped(skippedAt(minute))
	}

	report := reportDomain.ToDto()
	assert.Equal(t, 200, report.SkippedCount)
	assert.Len(t, report.SkippedKCandles, 200)
	assert.False(t, report.SkippedKCandlesTruncated)
}

func TestIngestionReportCarriesWhatTheSourceRefusedWith(t *testing.T) {
	reportDomain := domains.NewKCandleSymbolIngestionReportDomain("2330", vo.MarketTaiwanStock)

	reportDomain.NoteFetchFailure("market source unreachable")

	report := reportDomain.ToDto()
	assert.Equal(t, "taiwanStock", report.Market)
	assert.Equal(t, "market source unreachable", report.FetchFailureReason)
	assert.False(t, report.WasAsked)
}
