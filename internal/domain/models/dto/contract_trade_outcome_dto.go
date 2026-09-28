package dto

import "github.com/shopspring/decimal"

// ContractTradeOutcomeDto pairs every figure with whether it could be worked out and why not, so a missing figure is never shown as zero.
type ContractTradeOutcomeDto struct {
	GrossProfit decimal.Decimal `json:"grossProfit"`
	TotalFee    decimal.Decimal `json:"totalFee"`
	// FeeRateMissing means at least one fee is zero only because no rate was set.
	FeeRateMissing bool                    `json:"feeRateMissing"`
	Funding        ContractTradeFundingDto `json:"funding"`
	NetProfit      decimal.Decimal         `json:"netProfit"`
	// NetProfitExcludesFunding is true when funding could not be worked out.
	NetProfitExcludesFunding bool                `json:"netProfitExcludesFunding"`
	PlannedRisk              decimal.NullDecimal `json:"plannedRisk"`
	RMultiple                *float64            `json:"rMultiple"`
	// RMultipleUnavailableReason is noStopLoss when no planned stop was given.
	RMultipleUnavailableReason string                      `json:"rMultipleUnavailableReason"`
	Excursion                  TradeExcursionDto           `json:"excursion"`
	ProfitCaptureRate          *float64                    `json:"profitCaptureRate"`
	FloatingProfit             TradeFloatingDto            `json:"floatingProfit"`
	LiquidationPrice           ContractTradeLiquidationDto `json:"liquidationPrice"`
	// EntrySlippagePercentage is positive when the fill was worse than the bot's reference price.
	EntrySlippagePercentage *float64 `json:"entrySlippagePercentage"`
	// EntryNotional is the average entry price times everything entered.
	EntryNotional decimal.Decimal `json:"entryNotional"`
	EntryMargin   decimal.Decimal `json:"entryMargin"`
	// EntryMargin is the entered value over leverage, so a trade that scaled out and back in counts both entries.
	// ReturnOnMarginPercentage is net profit over margin; 24.13 is 24.13%. It leaves funding out whenever NetProfitExcludesFunding does.
	ReturnOnMarginPercentage *float64 `json:"returnOnMarginPercentage"`
	// ReturnOnMarginUnavailableReason is notClosed while the trade is held.
	ReturnOnMarginUnavailableReason string `json:"returnOnMarginUnavailableReason"`
	// ImplausibleFeeFillIDs are fills whose fee is under 0.001% or over 0.5% of their value, in the order they happened.
	ImplausibleFeeFillIDs []uint `json:"implausibleFeeFillIds"`
}

type ContractTradeFundingDto struct {
	Available bool `json:"available"`
	// Amount is positive when received and negative when paid.
	Amount          decimal.Decimal `json:"amount"`
	SettlementCount int             `json:"settlementCount"`
	// UnavailableReason is noSettlementData.
	UnavailableReason string `json:"unavailableReason"`
}

type ContractTradeLiquidationDto struct {
	Available          bool            `json:"available"`
	Price              decimal.Decimal `json:"price"`
	CannotBeLiquidated bool            `json:"cannotBeLiquidated"`
	// UnavailableReason is notOpen, noTradingSpecification or notComputed.
	UnavailableReason string `json:"unavailableReason"`
}
