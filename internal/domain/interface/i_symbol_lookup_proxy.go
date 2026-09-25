package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_symbol_lookup_proxy.go -destination=mocks/mock_i_symbol_lookup_proxy.go -package=mocks

// ISymbolLookupProxy validates a symbol against the market so typos are refused up front, returning the venue's listing (including its display name).
// An unknown symbol is an unlisted result, not an error; an error means the market could not be reached.
type ISymbolLookupProxy interface {
	LookUpSymbol(
		executionContext context.Context, market vo.MarketVo, symbol string,
	) (vo.SymbolListingVo, error)
}
