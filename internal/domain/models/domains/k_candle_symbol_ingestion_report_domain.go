package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// maxNamedSkippedKCandles is generous on purpose; it only stops a broken source from producing an unreadable report.
const maxNamedSkippedKCandles = 200

// KCandleSymbolIngestionReportDomain accumulates one symbol's fetch outcome under shared spot/contract rules; it is mutated in place, hence the pointer.
type KCandleSymbolIngestionReportDomain struct {
	symbol                   string
	market                   vo.MarketVo
	wasAsked                 bool
	storedCount              int
	skippedCount             int
	skippedKCandles          []dto.SkippedKCandleDto
	skippedKCandlesTruncated bool
	fetchFailureReason       string
}

// NewKCandleSymbolIngestionReportDomain starts the skipped list empty rather than nil so every exit path answers with a list.
func NewKCandleSymbolIngestionReportDomain(
	symbol string, market vo.MarketVo,
) *KCandleSymbolIngestionReportDomain {
	return &KCandleSymbolIngestionReportDomain{
		symbol:          symbol,
		market:          market,
		skippedKCandles: make([]dto.SkippedKCandleDto, 0),
	}
}

// NoteAsked distinguishes "the source said nothing" from "never asked"; only the former lets a market be presumed shut.
func (reportDomain *KCandleSymbolIngestionReportDomain) NoteAsked() {
	reportDomain.wasAsked = true
}

func (reportDomain *KCandleSymbolIngestionReportDomain) NoteStored(storedCount int) {
	reportDomain.storedCount += storedCount
}

// NoteSkipped counts every skip and names them up to maxNamedSkippedKCandles.
func (reportDomain *KCandleSymbolIngestionReportDomain) NoteSkipped(
	skippedKCandles ...dto.SkippedKCandleDto,
) {
	for _, skippedKCandle := range skippedKCandles {
		reportDomain.skippedCount++

		if len(reportDomain.skippedKCandles) >= maxNamedSkippedKCandles {
			reportDomain.skippedKCandlesTruncated = true

			continue
		}

		reportDomain.skippedKCandles = append(reportDomain.skippedKCandles, skippedKCandle)
	}
}

// NoteFetchFailure records a source refusal, which is not this system's fault.
func (reportDomain *KCandleSymbolIngestionReportDomain) NoteFetchFailure(reason string) {
	reportDomain.fetchFailureReason = reason
}

func (reportDomain *KCandleSymbolIngestionReportDomain) ToDto() dto.KCandleSymbolIngestionReportDto {
	return dto.KCandleSymbolIngestionReportDto{
		Symbol:                   reportDomain.symbol,
		Market:                   string(reportDomain.market),
		WasAsked:                 reportDomain.wasAsked,
		StoredCount:              reportDomain.storedCount,
		SkippedCount:             reportDomain.skippedCount,
		SkippedKCandles:          reportDomain.skippedKCandles,
		SkippedKCandlesTruncated: reportDomain.skippedKCandlesTruncated,
		FetchFailureReason:       reportDomain.fetchFailureReason,
	}
}
