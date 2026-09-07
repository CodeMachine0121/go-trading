package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_symbol_lookup_proxy.go -destination=mocks/mock_i_symbol_lookup_proxy.go -package=mocks

// ISymbolLookupProxy asks a market about one symbol. It is named for the capability,
// not the provider, so a different venue is a new implementation rather than a new
// contract.
//
// It exists so that a mistyped code is refused while somebody is still looking at
// it. Without it, the cost of a typo is not one bad row — it is a line that fails
// quietly once every round, forever, in a log nobody reads.
//
// It hands back what the market said rather than a yes or no, because the same
// answer that proves a code real also carries what the venue calls it — and a code
// on its own is a poor thing to read a watchlist by. Splitting that into two
// questions would mean asking the same endpoint twice.
//
// "Does not exist" comes back as a listing that is not listed, rather than an error,
// because it is an answer about the symbol. An error means the market could not be
// asked at all, which is a different thing to tell somebody: one says fix what you
// typed, the other says try again later.
type ISymbolLookupProxy interface {
	LookUpSymbol(
		executionContext context.Context, market vo.MarketVo, symbol string,
	) (vo.SymbolListingVo, error)
}
