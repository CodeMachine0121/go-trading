package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_contract_symbol_lookup_proxy.go -destination=mocks/mock_i_contract_symbol_lookup_proxy.go -package=mocks

// IContractSymbolLookupProxy takes no market because it serves exactly one venue.
type IContractSymbolLookupProxy interface {
	LookUpSymbol(executionContext context.Context, symbol string) (vo.ContractSymbolListingVo, error)
	// FetchTradingSpecifications omits contracts the venue no longer lists as followable.
	FetchTradingSpecifications(executionContext context.Context) ([]vo.ContractTradingSpecificationVo, error)
}
