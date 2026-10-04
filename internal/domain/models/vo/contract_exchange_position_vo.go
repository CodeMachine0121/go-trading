package vo

import "github.com/shopspring/decimal"

// ContractExchangePositionVo is what the account holds on one contract at the venue, whoever opened it.
type ContractExchangePositionVo struct {
	LongQuantity  decimal.Decimal
	ShortQuantity decimal.Decimal
}
