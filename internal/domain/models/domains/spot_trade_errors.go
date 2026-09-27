package domains

import (
	"errors"
	"fmt"
)

// ErrSpotTradeValidation is the single sentinel for every spot trade rule violation; the wrapped message names the rule.
var ErrSpotTradeValidation = errors.New("spot trade validation failed")

// ErrSpotTradeNotFound also covers trades owned by someone else so identifiers can't be probed for existence.
var ErrSpotTradeNotFound = errors.New("spot trade not found")

func SpotTradeNotFound(id uint) error {
	return fmt.Errorf("%w: 找不到這筆交易（識別碼 %d）", ErrSpotTradeNotFound, id)
}

// ErrSpotTradeOpenHoldingExists keeps one holding per symbol, so buying more is added to it.
var ErrSpotTradeOpenHoldingExists = errors.New("spot trade open holding exists")

// SpotTradeOpenHoldingExistsError carries the open trade so a caller can send the person straight to it.
type SpotTradeOpenHoldingExistsError struct {
	OpenTradeID uint
	message     string
}

func (openHoldingError SpotTradeOpenHoldingExistsError) Error() string {
	return openHoldingError.message
}

func (openHoldingError SpotTradeOpenHoldingExistsError) Unwrap() error {
	return ErrSpotTradeOpenHoldingExists
}

// SpotTradeOpenHoldingExists leaves OpenTradeID zero when the database refused a racing second trade before its identifier was known.
func SpotTradeOpenHoldingExists(symbol string, openTradeID uint) error {
	if openTradeID == 0 {
		return SpotTradeOpenHoldingExistsError{message: fmt.Sprintf("%s: %s 已有一筆持有中的交易，請在那一筆加買進",
			ErrSpotTradeOpenHoldingExists, symbol)}
	}

	return SpotTradeOpenHoldingExistsError{OpenTradeID: openTradeID, message: fmt.Sprintf(
		"%s: %s 已有持有中的 #%d，請在那一筆加買進", ErrSpotTradeOpenHoldingExists, symbol, openTradeID)}
}

// ErrSpotTradeLocked refuses changes to a closed trade's plan and fills so it keeps showing what the person meant then.
var ErrSpotTradeLocked = errors.New("spot trade locked")
