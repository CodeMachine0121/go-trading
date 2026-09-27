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

// ContractTradeOpenPositionExists names the open trade so the person can add the fill there instead.
func ContractTradeOpenPositionExists(symbol string, directionInWords string, openTradeID uint) error {
	if openTradeID == 0 {
		return fmt.Errorf("%w: %s %s 已有一筆持倉中的交易，請在那一筆加成交",
			ErrContractTradeOpenPositionExists, symbol, directionInWords)
	}

	return fmt.Errorf("%w: %s %s 已有持倉中的 #%d，請在那一筆加成交",
		ErrContractTradeOpenPositionExists, symbol, directionInWords, openTradeID)
}

// ErrContractTradeLocked refuses changes to a closed trade's plan and fills so it keeps showing what the person meant then.
var ErrContractTradeLocked = errors.New("contract trade locked")

// ErrJournalLinkNotFound covers rounds the bot has forgotten and rounds of somebody else's bot alike.
var ErrJournalLinkNotFound = errors.New("journal link not found")
