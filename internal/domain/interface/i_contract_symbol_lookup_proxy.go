package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_contract_symbol_lookup_proxy.go -destination=mocks/mock_i_contract_symbol_lookup_proxy.go -package=mocks

// IContractSymbolLookupProxy answers whether the contract venue lists a trading pair.
//
// It takes no market, which is the one way it is narrower than the spot lookup: it
// serves exactly one venue, and an argument every caller fills the same way is an
// argument that only gives them a chance to fill it wrongly.
type IContractSymbolLookupProxy interface {
	LookUpSymbol(executionContext context.Context, symbol string) (vo.SymbolListingVo, error)
}
