package domains

import "errors"

// ErrWatchlistEntryValidation marks a request to watch something the system will not
// accept as named: a blank code, or a market it does not recognise. The wrapped
// message names which.
var ErrWatchlistEntryValidation = errors.New("watchlist entry validation failed")

// ErrTradingSymbolNotInMarket marks a code that market has never heard of. It is
// kept apart from the two below because it is the only one the person asking can fix
// themselves — they typed something that is not there.
var ErrTradingSymbolNotInMarket = errors.New("trading symbol not found in market")

// ErrMarketDataSourceUnavailable marks a market that could not be asked at all.
// Nothing is wrong with the request; it is worth making again later, which is the
// opposite advice from the error above and so must not share it.
var ErrMarketDataSourceUnavailable = errors.New("market data source unavailable")

// ErrTradingSymbolNamed marks a symbol that cannot be read as a name at all — blank,
// or carrying something no name may carry. It is separate from "not registered"
// because that one is an answer about the system's records, and this one is an answer
// about what was typed: the first invites you to register it, the second to retype it.
var ErrTradingSymbolNamed = errors.New("trading symbol not named")

// ErrTradingSymbolNotRegistered marks a symbol the system has never been told about,
// and therefore cannot say which market it belongs to or where its candles come from.
var ErrTradingSymbolNotRegistered = errors.New("trading symbol not registered")
