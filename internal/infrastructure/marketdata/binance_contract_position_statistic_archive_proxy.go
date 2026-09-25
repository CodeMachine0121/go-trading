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
	archiveDayLayout = "2006-01-02"
	// archiveStatisticTimeLayout is UTC, though the file does not say so.
	archiveStatisticTimeLayout = "2006-01-02 15:04:05"
	// archiveDayFileSizeCeiling guards against reading a non-archive response into memory; real files are tens of kilobytes.
	archiveDayFileSizeCeiling = 16 << 20
)

// Archive columns read by header name; the account-count ratio and taker buy-sell ratio columns are not stored.
const (
	archiveStatisticTimeColumn          = "create_time"
	archiveOpenInterestColumn           = "sum_open_interest"
	archiveOpenInterestValueColumn      = "sum_open_interest_value"
	archiveAccountLongShortRatioColumn  = "count_long_short_ratio"
	archiveTopTraderPositionRatioColumn = "sum_toptrader_long_short_ratio"
)

// BinanceContractPositionStatisticArchiveProxy reads the public archive's daily zipped CSVs of five-minute statistics, which go back years unlike the live endpoint's thirty days; columns are located by header, and it has its own pacer because the archive is a separate host.
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

// FetchDailyPositionStatistics returns one UTC day's statistics in file order; a day with no file yet (or before listing) returns found = false.
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

	unreadableDay := fmt.Sprintf("read position statistic archive for %s on %s", symbol, dayName)
	zippedDay, readError := io.ReadAll(io.LimitReader(response.Body, archiveDayFileSizeCeiling))
	if readError != nil {
		return nil, false, fmt.Errorf("%s: %w", unreadableDay, readError)
	}

	zipReader, zipError := zip.NewReader(bytes.NewReader(zippedDay), int64(len(zippedDay)))
	if zipError != nil {
		return nil, false, fmt.Errorf("%s: open day file: %w", unreadableDay, zipError)
	}
	if len(zipReader.File) == 0 {
		return nil, false, fmt.Errorf("%s: day file holds nothing", unreadableDay)
	}

	csvFile, openError := zipReader.File[0].Open()
	if openError != nil {
		return nil, false, fmt.Errorf("%s: open day file: %w", unreadableDay, openError)
	}
	defer func() { _ = csvFile.Close() }()

	rows, csvError := csv.NewReader(csvFile).ReadAll()
	if csvError != nil {
		return nil, false, fmt.Errorf("%s: read day file: %w", unreadableDay, csvError)
	}
	if len(rows) == 0 {
		return nil, false, fmt.Errorf("%s: day file has no header", unreadableDay)
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
			return nil, false, fmt.Errorf("%s: day file has no %s column", unreadableDay, requiredColumn)
		}
	}

	statistics := make([]vo.ContractPositionStatisticArchiveVo, 0, len(rows)-1)
	for _, row := range rows[1:] {
		statisticTime, timeError := time.ParseInLocation(archiveStatisticTimeLayout,
			strings.TrimSpace(row[columnIndexes[archiveStatisticTimeColumn]]), time.UTC)
		if timeError != nil {
			return nil, false, fmt.Errorf("%s: read statistic time: %w", unreadableDay, timeError)
		}

		statistic := vo.ContractPositionStatisticArchiveVo{Symbol: symbol, StatisticTime: statisticTime}
		// Checked in column order so a row with two bad cells always reports the same one.
		for _, figureColumn := range []struct {
			name        string
			destination *decimal.NullDecimal
		}{
			{archiveOpenInterestColumn, &statistic.OpenInterest},
			{archiveOpenInterestValueColumn, &statistic.OpenInterestValue},
			{archiveTopTraderPositionRatioColumn, &statistic.TopTraderPositionLongShortRatio},
			{archiveAccountLongShortRatioColumn, &statistic.AccountLongShortRatio},
		} {
			// A blank cell stays absent for the domain to judge.
			cell := strings.TrimSpace(row[columnIndexes[figureColumn.name]])
			if cell == "" {
				continue
			}

			figure, figureError := decimal.NewFromString(cell)
			if figureError != nil {
				return nil, false, fmt.Errorf("%s: read %s at %s: %w", unreadableDay,
					figureColumn.name, statisticTime.Format(archiveStatisticTimeLayout), figureError)
			}
			*figureColumn.destination = decimal.NewNullDecimal(figure)
		}

		statistics = append(statistics, statistic)
	}

	return statistics, true, nil
}
