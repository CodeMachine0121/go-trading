package domains

import "errors"

// ErrWatchlistEntryValidation covers a blank code or unrecognised market; the wrapped
// message names which.
var ErrWatchlistEntryValidation = errors.New("watchlist entry validation failed")

// ErrTradingSymbolNotInMarket is the only one of these the caller can fix by retyping the code.
var ErrTradingSymbolNotInMarket = errors.New("trading symbol not found in market")

// ErrMarketDataSourceUnavailable means the request is fine and worth retrying later.
var ErrMarketDataSourceUnavailable = errors.New("market data source unavailable")

// ErrTradingSymbolNamed is about what was typed (blank or invalid characters), unlike
// ErrTradingSymbolNotRegistered, which is about the system's records.
var ErrTradingSymbolNamed = errors.New("trading symbol not named")

var ErrTradingSymbolNotRegistered = errors.New("trading symbol not registered")
