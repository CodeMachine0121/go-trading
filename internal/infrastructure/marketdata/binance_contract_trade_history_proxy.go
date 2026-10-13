package marketdata

import (
	"bufio"
	"compress/flate"
	"context"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

const (
	// zipLocalFileHeaderSignature opens every entry of a zip file; the archive's single CSV is read from it directly.
	zipLocalFileHeaderSignature = 0x04034b50
	zipLocalFileHeaderLength    = 30
	zipMethodStored             = 0
	zipMethodDeflated           = 8
	// zipFlagSizesTrail means the sizes come after the data, so a stored entry's length is unknown up front.
	zipFlagSizesTrail = 0x0008
	// liveTradePageLength is the venue's largest page of trades.
	liveTradePageLength = 1000
	// liveTradeWindow is the longest span the venue accepts when both ends are given.
	liveTradeWindow = time.Hour
	// microsecondTimestampFloor tells microsecond timestamps from millisecond ones; milliseconds stay below it until the year 5138.
	microsecondTimestampFloor = 100_000_000_000_000
)

// Archive columns, in order; older files carry no header row.
const (
	archiveTradePriceColumn = iota + 1
	archiveTradeQuantityColumn
	archiveTradeFirstIdColumn
	archiveTradeLastIdColumn
	archiveTradeTimeColumn
	archiveTradeBuyerIsMakerColumn
	archiveTradeColumnCount
)

// BinanceContractTradeHistoryProxy reads aggregated trades from the public daily archive (streamed out of the zip as it downloads, never held whole nor written to disk) and from the live endpoint (an hour window at a time, then paging by trade identifier).
type BinanceContractTradeHistoryProxy struct {
	archiveBaseUrl    string
	liveTradesUrl     string
	archiveHttpClient *http.Client
	liveHttpClient    *http.Client
	archivePacer      RequestPacer
	livePacer         RequestPacer
	// archiveDownloads holds one token per download allowed at once.
	archiveDownloads chan struct{}
}

func NewBinanceContractTradeHistoryProxy(
	archiveBaseUrl string,
	liveTradesUrl string,
	archiveTimeout time.Duration,
	liveTimeout time.Duration,
	archivePacer RequestPacer,
	livePacer RequestPacer,
	concurrentArchiveDownloads int,
) *BinanceContractTradeHistoryProxy {
	return &BinanceContractTradeHistoryProxy{
		archiveBaseUrl:    strings.TrimRight(archiveBaseUrl, "/"),
		liveTradesUrl:     liveTradesUrl,
		archiveHttpClient: &http.Client{Timeout: archiveTimeout},
		liveHttpClient:    &http.Client{Timeout: liveTimeout},
		archivePacer:      archivePacer,
		livePacer:         livePacer,
		archiveDownloads:  make(chan struct{}, max(concurrentArchiveDownloads, 1)),
	}
}

func (tradeHistoryProxy *BinanceContractTradeHistoryProxy) VisitDailyTrades(
	executionContext context.Context, symbol string, day time.Time, visit func(vo.ContractAggregatedTradeVo),
) (bool, error) {
	select {
	case tradeHistoryProxy.archiveDownloads <- struct{}{}:
	case <-executionContext.Done():
		return false, executionContext.Err()
	}
	defer func() { <-tradeHistoryProxy.archiveDownloads }()

	if waitError := tradeHistoryProxy.archivePacer.WaitForTurn(executionContext); waitError != nil {
		return false, waitError
	}

	dayName := day.UTC().Format(archiveDayLayout)
	fileUrl := fmt.Sprintf("%s/%s/%s-aggTrades-%s.zip", tradeHistoryProxy.archiveBaseUrl, symbol, symbol, dayName)
	unreadableDay := fmt.Sprintf("read trade archive for %s on %s", symbol, dayName)

	request, buildError := http.NewRequestWithContext(executionContext, http.MethodGet, fileUrl, nil)
	if buildError != nil {
		return false, fmt.Errorf("%s: %w", unreadableDay, buildError)
	}
	response, requestError := tradeHistoryProxy.archiveHttpClient.Do(request)
	if requestError != nil {
		return false, fmt.Errorf("%s: %w", unreadableDay, requestError)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if response.StatusCode != http.StatusOK {
		return false, fmt.Errorf("%s: archive answered %d", unreadableDay, response.StatusCode)
	}

	zippedDay := bufio.NewReaderSize(response.Body, 1<<16)
	header := make([]byte, zipLocalFileHeaderLength)
	if _, headerError := io.ReadFull(zippedDay, header); headerError != nil {
		return false, fmt.Errorf("%s: read zip header: %w", unreadableDay, headerError)
	}
	if binary.LittleEndian.Uint32(header[0:4]) != zipLocalFileHeaderSignature {
		return false, fmt.Errorf("%s: not a zip file", unreadableDay)
	}
	flags := binary.LittleEndian.Uint16(header[6:8])
	method := binary.LittleEndian.Uint16(header[8:10])
	storedLength := int64(binary.LittleEndian.Uint32(header[18:22]))
	nameAndExtraLength := int64(binary.LittleEndian.Uint16(header[26:28])) +
		int64(binary.LittleEndian.Uint16(header[28:30]))
	if _, skipError := zippedDay.Discard(int(nameAndExtraLength)); skipError != nil {
		return false, fmt.Errorf("%s: read zip header: %w", unreadableDay, skipError)
	}

	csvContent := io.Reader(nil)
	switch {
	case method == zipMethodDeflated:
		inflater := flate.NewReader(zippedDay)
		defer func() { _ = inflater.Close() }()
		csvContent = inflater
	case method == zipMethodStored && flags&zipFlagSizesTrail == 0:
		csvContent = io.LimitReader(zippedDay, storedLength)
	default:
		return false, fmt.Errorf("%s: unsupported zip compression %d", unreadableDay, method)
	}

	rows := csv.NewReader(csvContent)
	rows.ReuseRecord = true
	rows.FieldsPerRecord = -1
	for rowNumber := 1; ; rowNumber++ {
		row, rowError := rows.Read()
		if errors.Is(rowError, io.EOF) {
			return true, nil
		}
		if rowError != nil {
			return false, fmt.Errorf("%s: row %d: %w", unreadableDay, rowNumber, rowError)
		}
		if len(row) < archiveTradeColumnCount {
			return false, fmt.Errorf("%s: row %d has %d columns", unreadableDay, rowNumber, len(row))
		}
		// Newer files open with a header row; its first cell is a name, not a number.
		if _, identifierError := strconv.ParseInt(row[0], 10, 64); rowNumber == 1 && identifierError != nil {
			continue
		}

		trade, tradeError := parseAggregatedTrade(
			row[archiveTradePriceColumn], row[archiveTradeQuantityColumn],
			row[archiveTradeFirstIdColumn], row[archiveTradeLastIdColumn],
			row[archiveTradeTimeColumn], strings.EqualFold(strings.TrimSpace(row[archiveTradeBuyerIsMakerColumn]), "true"))
		if tradeError != nil {
			return false, fmt.Errorf("%s: row %d: %w", unreadableDay, rowNumber, tradeError)
		}
		visit(trade)
	}
}

// liveAggregatedTrade is one trade as the live endpoint spells it.
type liveAggregatedTrade struct {
	AggregateTradeID int64  `json:"a"`
	Price            string `json:"p"`
	Quantity         string `json:"q"`
	FirstTradeID     int64  `json:"f"`
	LastTradeID      int64  `json:"l"`
	TradeTime        int64  `json:"T"`
	BuyerIsMaker     bool   `json:"m"`
}

func (tradeHistoryProxy *BinanceContractTradeHistoryProxy) VisitTradesBetween(
	executionContext context.Context, symbol string, start time.Time, end time.Time,
	visit func(vo.ContractAggregatedTradeVo),
) error {
	for windowStart := start.UTC(); windowStart.Before(end); {
		windowEnd := windowStart.Add(liveTradeWindow)
		if windowEnd.After(end) {
			windowEnd = end
		}

		// The venue's end time is inclusive, so the window stops a millisecond short of its end.
		page, pageError := tradeHistoryProxy.fetchLivePage(executionContext, symbol, url.Values{
			"startTime": {strconv.FormatInt(windowStart.UnixMilli(), 10)},
			"endTime":   {strconv.FormatInt(windowEnd.UnixMilli()-1, 10)},
		})
		if pageError != nil {
			return pageError
		}

		// A full page may leave trades behind in this window; the rest are read on by identifier, which also carries past the window.
		pagedOnByIdentifier := false
		for {
			for _, liveTrade := range page {
				trade, tradeError := parseAggregatedTrade(liveTrade.Price, liveTrade.Quantity,
					strconv.FormatInt(liveTrade.FirstTradeID, 10), strconv.FormatInt(liveTrade.LastTradeID, 10),
					strconv.FormatInt(liveTrade.TradeTime, 10), liveTrade.BuyerIsMaker)
				if tradeError != nil {
					return fmt.Errorf("read live trades for %s: %w", symbol, tradeError)
				}
				if !trade.TradeTime.Before(end) {
					return nil
				}
				visit(trade)
			}
			// A short page read by identifier has reached the newest trade there is; one read by time has only finished its window.
			if len(page) < liveTradePageLength {
				if pagedOnByIdentifier {
					return nil
				}
				break
			}

			page, pageError = tradeHistoryProxy.fetchLivePage(executionContext, symbol, url.Values{
				"fromId": {strconv.FormatInt(page[len(page)-1].AggregateTradeID+1, 10)},
			})
			if pageError != nil {
				return pageError
			}
			pagedOnByIdentifier = true
		}

		windowStart = windowEnd
	}

	return nil
}

// fetchLivePage asks for one page of trades; extracted because the first read of a window and the reads paging on from it would otherwise repeat the whole request.
func (tradeHistoryProxy *BinanceContractTradeHistoryProxy) fetchLivePage(
	executionContext context.Context, symbol string, query url.Values,
) ([]liveAggregatedTrade, error) {
	if waitError := tradeHistoryProxy.livePacer.WaitForTurn(executionContext); waitError != nil {
		return nil, waitError
	}

	query.Set("symbol", symbol)
	query.Set("limit", strconv.Itoa(liveTradePageLength))
	request, buildError := http.NewRequestWithContext(
		executionContext, http.MethodGet, tradeHistoryProxy.liveTradesUrl+"?"+query.Encode(), nil)
	if buildError != nil {
		return nil, fmt.Errorf("reach live trades for %s: %w", symbol, buildError)
	}
	response, requestError := tradeHistoryProxy.liveHttpClient.Do(request)
	if requestError != nil {
		return nil, fmt.Errorf("reach live trades for %s: %w", symbol, requestError)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("live trades answered %d for %s", response.StatusCode, symbol)
	}

	page := []liveAggregatedTrade{}
	if decodeError := json.NewDecoder(response.Body).Decode(&page); decodeError != nil {
		return nil, fmt.Errorf("read live trades for %s: %w", symbol, decodeError)
	}

	return page, nil
}

// parseAggregatedTrade normalizes either source's spelling of one trade; the buyer being the maker means a seller initiated it.
func parseAggregatedTrade(
	price string, quantity string, firstTradeID string, lastTradeID string, tradeTime string, buyerIsMaker bool,
) (vo.ContractAggregatedTradeVo, error) {
	tradePrice, priceError := decimal.NewFromString(strings.TrimSpace(price))
	if priceError != nil {
		return vo.ContractAggregatedTradeVo{}, fmt.Errorf("price %q: %w", price, priceError)
	}
	tradeQuantity, quantityError := strconv.ParseFloat(strings.TrimSpace(quantity), 64)
	if quantityError != nil {
		return vo.ContractAggregatedTradeVo{}, fmt.Errorf("quantity %q: %w", quantity, quantityError)
	}
	first, firstError := strconv.ParseInt(strings.TrimSpace(firstTradeID), 10, 64)
	last, lastError := strconv.ParseInt(strings.TrimSpace(lastTradeID), 10, 64)
	if firstError != nil || lastError != nil {
		return vo.ContractAggregatedTradeVo{}, fmt.Errorf("trade identifiers %q to %q", firstTradeID, lastTradeID)
	}
	stamp, stampError := strconv.ParseInt(strings.TrimSpace(tradeTime), 10, 64)
	if stampError != nil {
		return vo.ContractAggregatedTradeVo{}, fmt.Errorf("trade time %q: %w", tradeTime, stampError)
	}

	timestamp := time.UnixMilli(stamp)
	if stamp >= microsecondTimestampFloor {
		timestamp = time.UnixMicro(stamp)
	}

	return vo.ContractAggregatedTradeVo{
		Price:        tradePrice,
		Quantity:     tradeQuantity,
		TradeCount:   max(last-first+1, 1),
		TradeTime:    timestamp.UTC(),
		BuyerIsTaker: !buyerIsMaker,
	}, nil
}
