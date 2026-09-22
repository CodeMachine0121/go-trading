package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// maxNamedSkippedKCandles is how many skipped candles one symbol's report names
// before it stops naming them and only counts them.
//
// It is generous on purpose. Anything short of a broken source produces a handful,
// and a handful is what somebody reads a report for; the limit is only there so that
// a source answering with rubbish for four years cannot turn the report into
// something nobody can open.
const maxNamedSkippedKCandles = 200

// KCandleSymbolIngestionReportDomain is what one trading symbol's fetch collected, as
// it is being collected.
//
// It exists because the collecting has rules, and those rules were being applied by
// whoever happened to be holding the report: count every skip, name only the first
// few, say so when the naming stopped. Spelled out at each call site, the rules were
// written twice — once for the spot venue and once for the contract one — as two
// copies that merely happened to agree.
//
// It is a report being built rather than a report, so it is mutated rather than
// rebuilt. That is why it is reached through a pointer: a value copy would collect
// into something the caller then throws away.
//
// The shape it is finally read in is a DTO, which stays pure data. Nothing outside
// reaches in to change a field.
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

// NewKCandleSymbolIngestionReportDomain starts a report for one symbol.
//
// The skipped list starts empty rather than nil, so that every way out of a fetch
// answers with a list. Left to appear only on the path that reaches the source, it
// would be absent on exactly the paths a reader inspects it on — a shut market, a
// source that would not answer — and a caller counting it would break there and
// nowhere else.
func NewKCandleSymbolIngestionReportDomain(
	symbol string, market vo.MarketVo,
) *KCandleSymbolIngestionReportDomain {
	return &KCandleSymbolIngestionReportDomain{
		symbol:          symbol,
		market:          market,
		skippedKCandles: make([]dto.SkippedKCandleDto, 0),
	}
}

// NoteAsked records that the source was actually reached for this symbol.
//
// It is what tells "the source said there was nothing" apart from "the source was
// never asked", and a market is only ever presumed shut on the strength of the first.
func (reportDomain *KCandleSymbolIngestionReportDomain) NoteAsked() {
	reportDomain.wasAsked = true
}

// NoteStored adds to what this symbol's fetch put away.
func (reportDomain *KCandleSymbolIngestionReportDomain) NoteStored(storedCount int) {
	reportDomain.storedCount += storedCount
}

// NoteSkipped writes candles that did not make it into the report, counting all of
// them and naming them up to the limit.
//
// Every skip goes through here so that the count and the list mean the same thing
// wherever they are read.
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

// NoteFetchFailure records the source refusing, which ends this symbol's fetch
// without being this system's fault.
func (reportDomain *KCandleSymbolIngestionReportDomain) NoteFetchFailure(reason string) {
	reportDomain.fetchFailureReason = reason
}

// ToDto is the shape this report leaves the domain in.
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
