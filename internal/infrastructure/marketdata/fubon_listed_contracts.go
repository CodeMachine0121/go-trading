package marketdata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	_interface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
)

// fubonListedContracts asks this source which futures contracts it currently lists,
// and answers which of them expires next.
//
// It is the whole of rolling to a new contract, and it is here rather than in the
// domain because it is a question about this venue's product list — nothing above
// infrastructure has to learn what a futures contract is, or that the code it watches
// stands for a different one every month.
//
// It is asked fresh every time and remembers nothing. A remembered answer would keep
// this system on a contract the venue had already stopped listing, and the roll would
// then be late by however long the memory lasted — which is the one thing this must
// never be.
type fubonListedContracts struct {
	productsUrl string
	apiKey      string
	clockProxy  _interface.IClockProxy
	httpClient  *http.Client
}

// errNoListedContract is a source that answered perfectly well and lists no contract
// for the code it was asked about.
//
// It is told apart from a source that could not be answered by, because the two want
// different things done about them: this one means the code is not one this venue
// trades, which is an answer somebody typing it wants to see, while an unreachable
// source says nothing about the code at all.
var errNoListedContract = errors.New("market source lists no contract")

// nearestTo is the listed contract of this standing code that expires soonest.
//
// A source that lists none comes back as errNoListedContract, which a caller may read
// either way: fetching candles treats it as a failure, because carrying on as though
// the market were merely quiet would store nothing and report nothing wrong, while
// looking a code up treats it as "no such code".
func (fubonListedContracts fubonListedContracts) nearestTo(
	executionContext context.Context, standingSymbol string,
) (fubonProduct, error) {
	queryValues := url.Values{}
	queryValues.Set("type", "FUTURE")
	queryValues.Set("exchange", "TAIFEX")

	request, buildError := http.NewRequestWithContext(executionContext, http.MethodGet,
		fubonListedContracts.productsUrl+"?"+queryValues.Encode(), nil)
	if buildError != nil {
		return fubonProduct{}, fmt.Errorf(
			"reach market source for %s: %w", standingSymbol, buildError)
	}
	request.Header.Set(fugleApiKeyHeader, fubonListedContracts.apiKey)

	response, requestError := fubonListedContracts.httpClient.Do(request)
	if requestError != nil {
		return fubonProduct{}, fmt.Errorf(
			"reach market source for %s: %w", standingSymbol, requestError)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return fubonProduct{}, fmt.Errorf(
			"market source answered %d for %s", response.StatusCode, standingSymbol)
	}

	var productsAnswer fubonProductsAnswer
	if decodeError := json.NewDecoder(response.Body).Decode(&productsAnswer); decodeError != nil {
		return fubonProduct{}, fmt.Errorf(
			"read listed contracts for %s: %w", standingSymbol, decodeError)
	}

	referenceYear := fubonListedContracts.clockProxy.Now().Year()
	nearestContract := fubonProduct{}
	nearestMonths := 0
	for _, listedProduct := range productsAnswer.Data {
		monthsUntilDelivery, isContractOfCode := listedProduct.monthsUntilDelivery(
			standingSymbol, referenceYear)
		if !isContractOfCode {
			continue
		}

		if nearestContract.Symbol == "" || monthsUntilDelivery < nearestMonths {
			nearestContract, nearestMonths = listedProduct, monthsUntilDelivery
		}
	}

	if nearestContract.Symbol == "" {
		return fubonProduct{}, fmt.Errorf("%w for %s", errNoListedContract, standingSymbol)
	}

	return nearestContract, nil
}
