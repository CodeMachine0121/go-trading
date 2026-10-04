package exchange

// binancePositionModeResponse is the account's position mode; true holds long and short separately.
type binancePositionModeResponse struct {
	DualSidePosition bool `json:"dualSidePosition"`
}

// binancePositionRiskResponse is one row of the account's position on a contract; PositionAmt is signed in one-way mode.
type binancePositionRiskResponse struct {
	PositionAmt  string `json:"positionAmt"`
	PositionSide string `json:"positionSide"`
}

// binanceContractOrderResponse keeps only how much of an order was executed and at what price.
type binanceContractOrderResponse struct {
	Status      string `json:"status"`
	ExecutedQty string `json:"executedQty"`
	AvgPrice    string `json:"avgPrice"`
}

// contractOrderFailureCodes sorts the venue's refusals by what the owner has to do about them; codes not listed pass the venue's own words on unchanged.
var (
	insufficientBalanceCodes = map[int]bool{-2018: true, -2019: true}
	venueRefusedCodes        = map[int]bool{
		-1013: true, -1111: true, -2020: true, -4003: true, -4005: true, -4131: true, -4164: true,
	}
	accountSettingRefusedCodes = map[int]bool{-4028: true, -4047: true, -4048: true, -4061: true, -4161: true}
	orderNotFoundCodes         = map[int]bool{-2011: true, -2013: true}
	// unknownOutcomeCodes say the venue did not finish answering: the request may or may not have been carried out.
	unknownOutcomeCodes = map[int]bool{-1001: true, -1003: true, -1007: true, -1021: true}
)

// marginTypeAlreadySetCode answers a request to set the margin type the contract already has.
const marginTypeAlreadySetCode = -4046
