package domains

import (
	"errors"
	"fmt"
)

// ErrContractTradingSymbolNotWatched marks a contract the system knows of but is not
// following on the contract watchlist, asked for where only followed contracts are
// served.
//
// It is a state rather than an absence, so it is told apart from "not registered": the
// contract exists, and adding it to the contract watchlist is the whole cure.
var ErrContractTradingSymbolNotWatched = errors.New("contract trading symbol not watched")

// ContractTradingSymbolNotWatched is that refusal for one contract, in words that say
// what to do about it.
func ContractTradingSymbolNotWatched(symbol string) error {
	return fmt.Errorf("%w: %s 不在合約追蹤名單上——請先把它加進合約追蹤名單再看即時更新",
		ErrContractTradingSymbolNotWatched, symbol)
}
