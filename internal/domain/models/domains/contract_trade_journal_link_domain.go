package domains

import (
	"net/url"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// ContractTradeJournalLinkDomain decides whether a round's message invites the person to record the trade it suggests.
type ContractTradeJournalLinkDomain struct {
	round dto.StrategyBotRoundDto
}

func NewContractTradeJournalLinkDomain(round dto.StrategyBotRoundDto) ContractTradeJournalLinkDomain {
	return ContractTradeJournalLinkDomain{round: round}
}

// Offered only for a contract round that suggests opening a position the venue would take, since only then is there a trade to prefill.
func (linkDomain ContractTradeJournalLinkDomain) Offered() bool {
	positionPlan := linkDomain.round.PositionPlan
	direction := vo.PositionDirectionVo(positionPlan.Direction)

	return linkDomain.round.MarketDataKind == string(vo.MarketDataKindContractKCandle) &&
		linkDomain.round.HasPositionPlan && positionPlan.Affordable && !positionPlan.HasVenueRefusal &&
		(direction == vo.PositionDirectionLong || direction == vo.PositionDirectionShort)
}

func (linkDomain ContractTradeJournalLinkDomain) UrlFor(frontendBaseUrl string, identifier string) string {
	return frontendBaseUrl + "/contract-trade-journal/new?journalLink=" + url.QueryEscape(identifier)
}
