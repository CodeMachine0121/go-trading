package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContractSeriesSymbolReportDomainCountsEverySkipButNamesOnlyTheFirstFewHundred(t *testing.T) {
	testCases := []struct {
		name              string
		skippedCount      int
		expectedNamed     int
		expectedTruncated bool
	}{
		{name: "上限以內全部點名", skippedCount: 200, expectedNamed: 200, expectedTruncated: false},
		{name: "超過上限只點名前兩百筆", skippedCount: 201, expectedNamed: 200, expectedTruncated: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			report := domains.NewContractSeriesSymbolReportDomain("BTCUSDT")
			firstRecordTime := time.Date(2026, 9, 23, 8, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))

			for index := range testCase.skippedCount {
				report.NoteSkipped(firstRecordTime.Add(time.Duration(index)*5*time.Minute), "佔比必須介於零與一之間")
			}
			report.NoteStored(3)

			reportDto := report.ToDto()
			assert.Equal(t, "BTCUSDT", reportDto.Symbol)
			assert.Equal(t, 3, reportDto.StoredCount)
			assert.Equal(t, testCase.skippedCount, reportDto.SkippedCount)
			require.Len(t, reportDto.SkippedRecords, testCase.expectedNamed)
			assert.Equal(t, testCase.expectedTruncated, reportDto.SkippedRecordsTruncated)
			assert.Equal(t, time.UTC, reportDto.SkippedRecords[0].RecordTime.Location())
			assert.True(t, firstRecordTime.Equal(reportDto.SkippedRecords[0].RecordTime))
		})
	}
}
