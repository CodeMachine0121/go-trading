package vo

import "github.com/shopspring/decimal"

// TradeResultVo is what tallying wins needs from a closed trade of either journal.
type TradeResultVo struct {
	NetProfit               decimal.Decimal
	RMultiple               *float64
	ReturnRate              *float64
	EntrySlippagePercentage *float64
	Direction               PositionDirectionVo
	FollowsTradingStrategy  bool
}
