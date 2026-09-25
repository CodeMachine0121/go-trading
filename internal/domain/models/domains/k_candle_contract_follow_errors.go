package domains

import (
	"errors"
	"fmt"
)

// ErrContractTradingSymbolNotWatched marks a known contract that is off the contract watchlist, distinct from not registered because adding it to the watchlist is the cure.
var ErrContractTradingSymbolNotWatched = errors.New("contract trading symbol not watched")

func ContractTradingSymbolNotWatched(symbol string) error {
	return fmt.Errorf("%w: %s 不在合約追蹤名單上——請先把它加進合約追蹤名單再看即時更新",
		ErrContractTradingSymbolNotWatched, symbol)
}
