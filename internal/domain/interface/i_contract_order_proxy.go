package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_contract_order_proxy.go -destination=mocks/mock_i_contract_order_proxy.go -package=mocks

// IContractOrderProxy trades perpetual contracts on an exchange with a person's own trading key; Binance is currently the only implementation.
// Every answer, including the venue's refusals and failures on this side, comes back as a call outcome rather than an error, and none carries either key string.
type IContractOrderProxy interface {
	// ReadPositionMode reports whether the account holds long and short separately (hedge mode) rather than as one net position.
	ReadPositionMode(
		executionContext context.Context, credential vo.TradingKeyCredentialVo,
	) (bool, vo.ContractOrderCallVo)

	ReadPosition(
		executionContext context.Context, credential vo.TradingKeyCredentialVo, symbol string,
	) (vo.ContractExchangePositionVo, vo.ContractOrderCallVo)

	// PrepareIsolatedLeverage sets the contract to isolated margin and the given leverage; already being so counts as done.
	PrepareIsolatedLeverage(
		executionContext context.Context, credential vo.TradingKeyCredentialVo, symbol string, leverage int,
	) vo.ContractOrderCallVo

	PlaceMarketOrder(
		executionContext context.Context, credential vo.TradingKeyCredentialVo, order vo.ContractMarketOrderVo,
	) (vo.ContractOrderFillVo, vo.ContractOrderCallVo)

	// FindMarketOrder looks an order up by its client order id; a venue with no record of it answers not found.
	FindMarketOrder(
		executionContext context.Context, credential vo.TradingKeyCredentialVo, symbol string, clientOrderID string,
	) (vo.ContractOrderFillVo, vo.ContractOrderCallVo)

	// PlaceLimitOrder rests a limit order until it fills or is cancelled; the answer is the order as it stood once accepted.
	PlaceLimitOrder(
		executionContext context.Context, credential vo.TradingKeyCredentialVo, order vo.ContractLimitOrderVo,
	) (vo.ContractOrderStateVo, vo.ContractOrderCallVo)

	// FindOrder reports where an order stands by its client order id; a venue with no record of it answers not found.
	FindOrder(
		executionContext context.Context, credential vo.TradingKeyCredentialVo, symbol string, clientOrderID string,
	) (vo.ContractOrderStateVo, vo.ContractOrderCallVo)

	// CancelOrder answers the order as it ended; an order no longer open (filled already, or unknown) answers not found.
	CancelOrder(
		executionContext context.Context, credential vo.TradingKeyCredentialVo, symbol string, clientOrderID string,
	) (vo.ContractOrderStateVo, vo.ContractOrderCallVo)

	PlaceProtectiveOrder(
		executionContext context.Context, credential vo.TradingKeyCredentialVo, order vo.ContractProtectiveOrderVo,
	) vo.ContractOrderCallVo

	// FindProtectiveOrder answers not found for an order the venue has no record of.
	FindProtectiveOrder(
		executionContext context.Context, credential vo.TradingKeyCredentialVo, symbol string, clientOrderID string,
	) vo.ContractOrderCallVo

	// CancelProtectiveOrder treats an order already gone (triggered, cancelled or never placed) as cancelled.
	CancelProtectiveOrder(
		executionContext context.Context, credential vo.TradingKeyCredentialVo, symbol string, clientOrderID string,
	) vo.ContractOrderCallVo
}
