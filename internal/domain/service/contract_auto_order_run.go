package service

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// orderRun is one replica's hold on one order while it carries it out: the order, what it reads to decide, and what it has learnt along the way. It holds no business rule; those stay on ContractAutoOrderDomain.
type orderRun struct {
	order         domains.ContractAutoOrderDomain
	position      vo.AutoOrderPositionVo
	bot           entities.StrategyBot
	credential    vo.TradingKeyCredentialVo
	hasCredential bool
	// unsealable is a stored key this system could not open, which no retry fixes.
	unsealable       bool
	tickSize         decimal.Decimal
	accountChecked   bool
	botRunning       bool
	autoOrderEnabled bool
}
