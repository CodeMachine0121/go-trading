package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

const (
	// positionStatisticPeriod is how the venue spells the five-minute statistic.
	positionStatisticPeriod = "5m"
	// positionStatisticPageLimit is the most statistics the venue hands back at once.
	positionStatisticPageLimit = 500
	// positionStatisticStep is the grid the venue takes statistics on.
	positionStatisticStep = 5 * time.Minute
)

// The three answers one position statistic is assembled from, relative to the
// statistics address.
const (
	openInterestPath           = "/openInterestHist"
	accountLongShortPath       = "/globalLongShortAccountRatio"
	topTraderPositionRatioPath = "/topLongShortPositionRatio"
)

// binanceOpenInterest is one open interest reading as the venue spells it.
type binanceOpenInterest struct {
	SumOpenInterest      string `json:"sumOpenInterest"`
	SumOpenInterestValue string `json:"sumOpenInterestValue"`
	Timestamp            int64  `json:"timestamp"`
}

// binanceLongShortSplit is one long-short split as the venue spells it. Both splits
// share the spelling, though the largest accounts' one counts positions rather than
// accounts.
type binanceLongShortSplit struct {
	LongAccount    string `json:"longAccount"`
	ShortAccount   string `json:"shortAccount"`
	LongShortRatio string `json:"longShortRatio"`
	Timestamp      int64  `json:"timestamp"`
}

// longShortSplitFigures is one split, read.
type longShortSplitFigures struct {
	longShare      decimal.Decimal
	shortShare     decimal.Decimal
	longShortRatio decimal.Decimal
}

// BinanceContractPositionStatisticProxy fetches five-minute position statistics from
// Binance's perpetual contract venue.
//
// **Every question names both ends, and no question covers more than one page.** The
// venue ignores a start given on its own and answers with the latest statistics
// instead; and asked about a stretch longer than a page, it answers with the latest
// page of that stretch rather than the first. Both would pass for an answer. So the
// window is walked a page-sized stretch at a time, each with its own start and end.
//
// It spends an allowance of its own: the venue counts these statistics apart from
// the K candles.
type BinanceContractPositionStatisticProxy struct {
	statisticsBaseUrl string
	httpClient        *http.Client
	pacer             RequestPacer
}

func NewBinanceContractPositionStatisticProxy(
	statisticsBaseUrl string, requestTimeout time.Duration, pacer RequestPacer,
) *BinanceContractPositionStatisticProxy {
	return &BinanceContractPositionStatisticProxy{
		statisticsBaseUrl: statisticsBaseUrl,
		httpClient:        &http.Client{Timeout: requestTimeout},
		pacer:             pacer,
	}
}

// FetchPositionStatistics returns every statistic in the window, oldest first.
//
// The open interest decides which moments exist; a split for a moment with no open
// interest is a reading of nothing and is dropped. Any one question failing fails the
// whole call.
func (positionStatisticProxy *BinanceContractPositionStatisticProxy) FetchPositionStatistics(
	executionContext context.Context, symbol string, startTime time.Time, endTime time.Time,
) ([]vo.ContractPositionStatisticVo, error) {
	statistics := make([]vo.ContractPositionStatisticVo, 0)

	for chunkStart := startTime.UTC(); !chunkStart.After(endTime); {
		chunkEnd := chunkStart.Add((positionStatisticPageLimit - 1) * positionStatisticStep)
		if chunkEnd.After(endTime) {
			chunkEnd = endTime.UTC()
		}

		chunkStatistics, chunkError := positionStatisticProxy.fetchChunk(
			executionContext, symbol, chunkStart, chunkEnd)
		if chunkError != nil {
			return nil, chunkError
		}
		statistics = append(statistics, chunkStatistics...)

		chunkStart = chunkEnd.Add(positionStatisticStep)
	}

	return statistics, nil
}

// fetchChunk asks the three questions about one page-sized stretch and aligns the
// answers.
func (positionStatisticProxy *BinanceContractPositionStatisticProxy) fetchChunk(
	executionContext context.Context, symbol string, startTime time.Time, endTime time.Time,
) ([]vo.ContractPositionStatisticVo, error) {
	openInterestAnswer, askError := positionStatisticProxy.ask(
		executionContext, openInterestPath, symbol, startTime, endTime)
	if askError != nil {
		return nil, askError
	}

	var openInterests []binanceOpenInterest
	if decodeError := json.Unmarshal(openInterestAnswer, &openInterests); decodeError != nil {
		return nil, fmt.Errorf("read contract position statistic answer for %s: %w", symbol, decodeError)
	}

	if len(openInterests) == 0 {
		// Nothing was open to split. Asking anyway would spend the allowance on two
		// answers with no home.
		return []vo.ContractPositionStatisticVo{}, nil
	}

	accountSplits, accountError := positionStatisticProxy.askForSplits(
		executionContext, accountLongShortPath, symbol, startTime, endTime)
	if accountError != nil {
		return nil, accountError
	}

	topTraderSplits, topTraderError := positionStatisticProxy.askForSplits(
		executionContext, topTraderPositionRatioPath, symbol, startTime, endTime)
	if topTraderError != nil {
		return nil, topTraderError
	}

	statistics := make([]vo.ContractPositionStatisticVo, 0, len(openInterests))
	for _, openInterest := range openInterests {
		statisticTime := time.UnixMilli(openInterest.Timestamp).UTC()
		if statisticTime.Before(startTime) || statisticTime.After(endTime) {
			continue
		}

		openInterestAmount, amountError := decimal.NewFromString(openInterest.SumOpenInterest)
		if amountError != nil {
			return nil, fmt.Errorf("read open interest for %s: %w", symbol, amountError)
		}
		openInterestValue, valueError := decimal.NewFromString(openInterest.SumOpenInterestValue)
		if valueError != nil {
			return nil, fmt.Errorf("read open interest value for %s: %w", symbol, valueError)
		}

		statistic := vo.ContractPositionStatisticVo{
			Symbol:            symbol,
			StatisticTime:     statisticTime,
			OpenInterest:      openInterestAmount,
			OpenInterestValue: openInterestValue,
		}
		if accountSplit, hasAccountSplit := accountSplits[openInterest.Timestamp]; hasAccountSplit {
			statistic.AccountLongShare = decimal.NewNullDecimal(accountSplit.longShare)
			statistic.AccountShortShare = decimal.NewNullDecimal(accountSplit.shortShare)
			statistic.AccountLongShortRatio = decimal.NewNullDecimal(accountSplit.longShortRatio)
		}
		if topTraderSplit, hasTopTraderSplit := topTraderSplits[openInterest.Timestamp]; hasTopTraderSplit {
			statistic.TopTraderPositionLongShare = decimal.NewNullDecimal(topTraderSplit.longShare)
			statistic.TopTraderPositionShortShare = decimal.NewNullDecimal(topTraderSplit.shortShare)
			statistic.TopTraderPositionLongShortRatio = decimal.NewNullDecimal(topTraderSplit.longShortRatio)
		}
		statistics = append(statistics, statistic)
	}

	return statistics, nil
}

// askForSplits asks one of the two split questions and keys the answers by the
// moment they describe.
func (positionStatisticProxy *BinanceContractPositionStatisticProxy) askForSplits(
	executionContext context.Context, path string, symbol string, startTime time.Time, endTime time.Time,
) (map[int64]longShortSplitFigures, error) {
	splitAnswer, askError := positionStatisticProxy.ask(executionContext, path, symbol, startTime, endTime)
	if askError != nil {
		return nil, askError
	}

	var reportedSplits []binanceLongShortSplit
	if decodeError := json.Unmarshal(splitAnswer, &reportedSplits); decodeError != nil {
		return nil, fmt.Errorf("read contract position statistic answer for %s: %w", symbol, decodeError)
	}

	splitsByTimestamp := make(map[int64]longShortSplitFigures, len(reportedSplits))
	for _, reportedSplit := range reportedSplits {
		figures := make([]decimal.Decimal, 0, 3)
		for _, quotedFigure := range []string{
			reportedSplit.LongAccount, reportedSplit.ShortAccount, reportedSplit.LongShortRatio,
		} {
			figure, parseError := decimal.NewFromString(quotedFigure)
			if parseError != nil {
				return nil, fmt.Errorf("read long-short split for %s: %w", symbol, parseError)
			}
			figures = append(figures, figure)
		}

		splitsByTimestamp[reportedSplit.Timestamp] = longShortSplitFigures{
			longShare: figures[0], shortShare: figures[1], longShortRatio: figures[2],
		}
	}

	return splitsByTimestamp, nil
}

// ask puts one question to the venue and hands back the answer as it arrived. The
// three answers are spelled differently, so reading one is left to whoever asked it.
//
// **It stays a method of its own because of what it encloses: one answer's body, from
// the moment it arrives to the moment it is let go.** Every page-sized stretch asks
// three of these, and a thirty-day catch-up walks eighteen stretches.
func (positionStatisticProxy *BinanceContractPositionStatisticProxy) ask(
	executionContext context.Context,
	path string,
	symbol string,
	startTime time.Time,
	endTime time.Time,
) ([]byte, error) {
	if waitError := positionStatisticProxy.pacer.WaitForTurn(executionContext); waitError != nil {
		return nil, waitError
	}

	queryValues := url.Values{}
	queryValues.Set("symbol", symbol)
	queryValues.Set("period", positionStatisticPeriod)
	queryValues.Set("limit", strconv.Itoa(positionStatisticPageLimit))
	queryValues.Set("startTime", strconv.FormatInt(startTime.UnixMilli(), 10))
	queryValues.Set("endTime", strconv.FormatInt(endTime.UnixMilli(), 10))

	request, buildError := http.NewRequestWithContext(executionContext, http.MethodGet,
		positionStatisticProxy.statisticsBaseUrl+path+"?"+queryValues.Encode(), nil)
	if buildError != nil {
		return nil, fmt.Errorf("reach contract position statistic source for %s: %w", symbol, buildError)
	}

	response, requestError := positionStatisticProxy.httpClient.Do(request)
	if requestError != nil {
		return nil, fmt.Errorf("reach contract position statistic source for %s: %w", symbol, requestError)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("contract position statistic source answered %d for %s at %s",
			response.StatusCode, symbol, path)
	}

	answer, readError := io.ReadAll(response.Body)
	if readError != nil {
		return nil, fmt.Errorf("read contract position statistic answer for %s: %w", symbol, readError)
	}

	return answer, nil
}
