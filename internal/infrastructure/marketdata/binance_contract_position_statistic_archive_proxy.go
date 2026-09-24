package marketdata

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

const (
	// archiveDayLayout is how the archive spells a day in a file name.
	archiveDayLayout = "2006-01-02"
	// archiveStatisticTimeLayout is how the archive spells a statistic time. It is
	// UTC, though the file never says so.
	archiveStatisticTimeLayout = "2006-01-02 15:04:05"
)

// The columns of one archive day that this system reads, by the name the archive's
// header gives them. The archive keeps two more — the largest accounts' ratio counted
// by accounts rather than positions, and the taker buy-sell ratio — which this system
// does not store.
const (
	archiveStatisticTimeColumn          = "create_time"
	archiveOpenInterestColumn           = "sum_open_interest"
	archiveOpenInterestValueColumn      = "sum_open_interest_value"
	archiveAccountLongShortRatioColumn  = "count_long_short_ratio"
	archiveTopTraderPositionRatioColumn = "sum_toptrader_long_short_ratio"
)

// BinanceContractPositionStatisticArchiveProxy reads position statistics out of
// Binance's public history archive, where every perpetual contract has one zipped
// CSV file per UTC day of five-minute statistics going back years — unlike the live
// statistics, which the venue only keeps thirty days of.
//
// **Columns are found by the header, never by position.** The archive is a file
// format, not an API, and a file format that gains a column tends to gain it in the
// middle. A column this proxy needs that is not in the header is a file it cannot
// read, which it says rather than guessing.
//
// It spends an allowance of its own: the archive is a different host from every
// other address the venue serves.
type BinanceContractPositionStatisticArchiveProxy struct {
	archiveBaseUrl string
	httpClient     *http.Client
	pacer          RequestPacer
}

func NewBinanceContractPositionStatisticArchiveProxy(
	archiveBaseUrl string, requestTimeout time.Duration, pacer RequestPacer,
) *BinanceContractPositionStatisticArchiveProxy {
	return &BinanceContractPositionStatisticArchiveProxy{
		archiveBaseUrl: strings.TrimRight(archiveBaseUrl, "/"),
		httpClient:     &http.Client{Timeout: requestTimeout},
		pacer:          pacer,
	}
}

// FetchDailyPositionStatistics returns every statistic the archive holds for the
// contract on that UTC day, oldest first. A day the archive has no file for — not yet
// published, or before the contract existed — answers found = false.
func (archiveProxy *BinanceContractPositionStatisticArchiveProxy) FetchDailyPositionStatistics(
	executionContext context.Context, symbol string, day time.Time,
) ([]vo.ContractPositionStatisticArchiveVo, bool, error) {
	if waitError := archiveProxy.pacer.WaitForTurn(executionContext); waitError != nil {
		return nil, false, waitError
	}

	dayName := day.UTC().Format(archiveDayLayout)
	fileUrl := fmt.Sprintf("%s/%s/%s-metrics-%s.zip", archiveProxy.archiveBaseUrl, symbol, symbol, dayName)

	request, buildError := http.NewRequestWithContext(executionContext, http.MethodGet, fileUrl, nil)
	if buildError != nil {
		return nil, false, fmt.Errorf("reach position statistic archive for %s on %s: %w", symbol, dayName, buildError)
	}

	response, requestError := archiveProxy.httpClient.Do(request)
	if requestError != nil {
		return nil, false, fmt.Errorf("reach position statistic archive for %s on %s: %w", symbol, dayName, requestError)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}
	if response.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("position statistic archive answered %d for %s on %s",
			response.StatusCode, symbol, dayName)
	}

	zippedDay, readError := io.ReadAll(response.Body)
	if readError != nil {
		return nil, false, fmt.Errorf("read position statistic archive for %s on %s: %w", symbol, dayName, readError)
	}

	statistics, parseError := archivedDay(zippedDay).statistics(symbol)
	if parseError != nil {
		return nil, false, fmt.Errorf("read position statistic archive for %s on %s: %w", symbol, dayName, parseError)
	}

	return statistics, true, nil
}

// archivedDay is one day's file as it arrived: a zip holding a single CSV.
type archivedDay []byte

// statistics reads every row of the day, oldest first as the file keeps them.
func (zippedDay archivedDay) statistics(symbol string) ([]vo.ContractPositionStatisticArchiveVo, error) {
	zipReader, zipError := zip.NewReader(bytes.NewReader(zippedDay), int64(len(zippedDay)))
	if zipError != nil {
		return nil, fmt.Errorf("open day file: %w", zipError)
	}
	if len(zipReader.File) == 0 {
		return nil, fmt.Errorf("day file holds nothing")
	}

	csvFile, openError := zipReader.File[0].Open()
	if openError != nil {
		return nil, fmt.Errorf("open day file: %w", openError)
	}
	defer func() { _ = csvFile.Close() }()

	rows, csvError := csv.NewReader(csvFile).ReadAll()
	if csvError != nil {
		return nil, fmt.Errorf("read day file: %w", csvError)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("day file has no header")
	}

	columnIndexes := make(map[string]int, len(rows[0]))
	for index, columnName := range rows[0] {
		columnIndexes[strings.TrimSpace(columnName)] = index
	}
	for _, requiredColumn := range []string{
		archiveStatisticTimeColumn, archiveOpenInterestColumn, archiveOpenInterestValueColumn,
		archiveAccountLongShortRatioColumn, archiveTopTraderPositionRatioColumn,
	} {
		if _, hasColumn := columnIndexes[requiredColumn]; !hasColumn {
			return nil, fmt.Errorf("day file has no %s column", requiredColumn)
		}
	}

	statistics := make([]vo.ContractPositionStatisticArchiveVo, 0, len(rows)-1)
	for _, row := range rows[1:] {
		statisticTime, timeError := time.ParseInLocation(archiveStatisticTimeLayout,
			strings.TrimSpace(row[columnIndexes[archiveStatisticTimeColumn]]), time.UTC)
		if timeError != nil {
			return nil, fmt.Errorf("read statistic time: %w", timeError)
		}

		statistic := vo.ContractPositionStatisticArchiveVo{Symbol: symbol, StatisticTime: statisticTime}
		for columnName, destination := range map[string]*decimal.NullDecimal{
			archiveOpenInterestColumn:           &statistic.OpenInterest,
			archiveOpenInterestValueColumn:      &statistic.OpenInterestValue,
			archiveAccountLongShortRatioColumn:  &statistic.AccountLongShortRatio,
			archiveTopTraderPositionRatioColumn: &statistic.TopTraderPositionLongShortRatio,
		} {
			// A blank cell is a figure the archive does not have. It stays absent, and
			// the domain decides what a reading without it is worth.
			cell := strings.TrimSpace(row[columnIndexes[columnName]])
			if cell == "" {
				continue
			}

			figure, figureError := decimal.NewFromString(cell)
			if figureError != nil {
				return nil, fmt.Errorf("read %s at %s: %w", columnName, statisticTime.Format(archiveStatisticTimeLayout), figureError)
			}
			*destination = decimal.NewNullDecimal(figure)
		}

		statistics = append(statistics, statistic)
	}

	return statistics, nil
}
