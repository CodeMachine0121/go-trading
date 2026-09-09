package marketdata

import (
	"context"
	"errors"
	"net/http"
	"time"

	_interface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// FubonSymbolLookupProxy answers whether Fubon lists Taiwan index futures under a
// standing code.
//
// It asks which contracts the venue currently lists rather than asking about the
// standing code itself, because the standing code is not a code this venue trades —
// "the futures" is a thing a person watches, and the venue only knows the contract it
// stands for this month. A code with a listed contract is real; a code with none is
// not.
//
// The name comes from the same answer, and it is the contract's own name, because
// that is the only name this venue publishes. It changes every month, which is honest:
// it says which contract is being watched right now.
type FubonSymbolLookupProxy struct {
	listedContracts fubonListedContracts
}

func NewFubonSymbolLookupProxy(
	productsUrl string,
	apiKey string,
	clockProxy _interface.IClockProxy,
	requestTimeout time.Duration,
) *FubonSymbolLookupProxy {
	return &FubonSymbolLookupProxy{
		listedContracts: fubonListedContracts{
			productsUrl: productsUrl,
			apiKey:      apiKey,
			clockProxy:  clockProxy,
			httpClient:  &http.Client{Timeout: requestTimeout},
		},
	}
}

// LookUpSymbol reports whether this source lists a contract for the standing code,
// and what it calls the one it lists.
//
// A source that lists nothing for the code answers "not listed" rather than failing:
// that is an answer about the code, and it is the typo this check exists to catch. A
// source that could not be reached fails, because that says nothing about the code
// and the two want telling apart — one says fix what you typed, the other says try
// again later.
//
// The market is accepted and ignored: this proxy is only ever reached for the one
// market it serves, and taking the argument is what lets it satisfy the same contract
// every other source does.
func (fubonSymbolLookupProxy *FubonSymbolLookupProxy) LookUpSymbol(
	executionContext context.Context, market vo.MarketVo, symbol string,
) (vo.SymbolListingVo, error) {
	nearestContract, resolveError := fubonSymbolLookupProxy.listedContracts.nearestTo(
		executionContext, symbol)
	if errors.Is(resolveError, errNoListedContract) {
		return vo.SymbolListingVo{}, nil
	}
	if resolveError != nil {
		return vo.SymbolListingVo{}, resolveError
	}

	return vo.SymbolListingVo{IsListed: true, DisplayName: nearestContract.Name}, nil
}
