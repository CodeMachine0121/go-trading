package domains

import (
	"net/url"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// TradeJournalLinkDomain decides whether a round's message invites the person to record the trade, and which journal the link opens.
type TradeJournalLinkDomain struct {
	round dto.StrategyBotRoundDto
}

func NewTradeJournalLinkDomain(round dto.StrategyBotRoundDto) TradeJournalLinkDomain {
	return TradeJournalLinkDomain{round: round}
}

// Offered for a contract round that suggests a position the venue would take, and for every spot buy or exit.
func (linkDomain TradeJournalLinkDomain) Offered() bool {
	if !linkDomain.isContract() {
		verdict := vo.StrategyBotRoundResultVo(linkDomain.round.Verdict)

		return verdict == vo.StrategyBotRoundResultBuy || verdict == vo.StrategyBotRoundResultSell
	}

	positionPlan := linkDomain.round.PositionPlan
	direction := vo.PositionDirectionVo(positionPlan.Direction)

	return linkDomain.round.HasPositionPlan && positionPlan.Affordable && !positionPlan.HasVenueRefusal &&
		(direction == vo.PositionDirectionLong || direction == vo.PositionDirectionShort)
}

func (linkDomain TradeJournalLinkDomain) UrlFor(frontendBaseUrl string, identifier string) string {
	journalPath := "/spot-trade-journal/new"
	if linkDomain.isContract() {
		journalPath = "/contract-trade-journal/new"
	}

	return frontendBaseUrl + journalPath + "?journalLink=" + url.QueryEscape(identifier)
}

func (linkDomain TradeJournalLinkDomain) isContract() bool {
	return linkDomain.round.MarketDataKind == string(vo.MarketDataKindContractKCandle)
}
