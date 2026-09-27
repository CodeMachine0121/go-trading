package domains

import (
	"errors"
	"fmt"
)

// ErrContractTradeValidation is the single sentinel for every trade rule violation; the wrapped message names the rule.
var ErrContractTradeValidation = errors.New("contract trade validation failed")

// ErrContractTradeNotFound also covers trades owned by someone else so identifiers can't be probed for existence.
var ErrContractTradeNotFound = errors.New("contract trade not found")

func ContractTradeNotFound(id uint) error {
	return fmt.Errorf("%w: 找不到這筆交易（識別碼 %d）", ErrContractTradeNotFound, id)
}

// ErrContractTradeOpenPositionExists keeps one open trade per symbol and direction, as the venue does.
var ErrContractTradeOpenPositionExists = errors.New("contract trade open position exists")

// ContractTradeOpenPositionExistsError carries the open trade so a caller can send the person straight to it.
type ContractTradeOpenPositionExistsError struct {
	OpenTradeID uint
	message     string
}

func (openPositionError ContractTradeOpenPositionExistsError) Error() string {
	return openPositionError.message
}

func (openPositionError ContractTradeOpenPositionExistsError) Unwrap() error {
	return ErrContractTradeOpenPositionExists
}

// ContractTradeOpenPositionExists leaves OpenTradeID zero when the database refused a racing second trade before its identifier was known.
func ContractTradeOpenPositionExists(symbol string, directionInWords string, openTradeID uint) error {
	if openTradeID == 0 {
		return ContractTradeOpenPositionExistsError{message: fmt.Sprintf("%s: %s %s 已有一筆持倉中的交易，請在那一筆加倉",
			ErrContractTradeOpenPositionExists, symbol, directionInWords)}
	}

	return ContractTradeOpenPositionExistsError{OpenTradeID: openTradeID, message: fmt.Sprintf(
		"%s: %s %s 已有持倉中的 #%d，請在那一筆加倉",
		ErrContractTradeOpenPositionExists, symbol, directionInWords, openTradeID)}
}

// ErrContractTradeLocked refuses changes to a closed trade's plan and positions so it keeps showing what the person meant then.
var ErrContractTradeLocked = errors.New("contract trade locked")

// ErrJournalLinkNotFound covers rounds the bot has forgotten and rounds of somebody else's bot alike.
var ErrJournalLinkNotFound = errors.New("journal link not found")
