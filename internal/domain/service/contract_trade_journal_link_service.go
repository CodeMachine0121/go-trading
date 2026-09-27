package service

import (
	"log"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// ContractTradeJournalLinkService mints the link a bot message carries; it is its own service so running a round needs nothing from the journal's storage.
type ContractTradeJournalLinkService struct {
	opaqueIdentifierProxy domaininterface.IOpaqueIdentifierProxy
	frontendBaseUrl       string
}

func NewContractTradeJournalLinkService(
	opaqueIdentifierProxy domaininterface.IOpaqueIdentifierProxy, frontendBaseUrl string,
) *ContractTradeJournalLinkService {
	return &ContractTradeJournalLinkService{opaqueIdentifierProxy: opaqueIdentifierProxy, frontendBaseUrl: frontendBaseUrl}
}

// OfferJournalLink leaves the round unchanged when no identifier can be minted, since a missing link must never cost the person the signal.
func (linkService *ContractTradeJournalLinkService) OfferJournalLink(round dto.StrategyBotRoundDto) dto.StrategyBotRoundDto {
	linkDomain := domains.NewContractTradeJournalLinkDomain(round)
	if !linkDomain.Offered() {
		return round
	}

	identifier, mintError := linkService.opaqueIdentifierProxy.Mint()
	if mintError != nil {
		log.Printf("strategy bot round for %s: sent without a journal link: %v", round.Symbol, mintError)

		return round
	}

	round.JournalLinkIdentifier = identifier.Value
	round.JournalLinkUrl = linkDomain.UrlFor(linkService.frontendBaseUrl, identifier.Value)

	return round
}
