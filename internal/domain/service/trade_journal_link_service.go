package service

import (
	"context"
	"errors"
	"fmt"
	"log"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// TradeJournalLinkService mints the link a bot message carries and reads its round back; both journals open links through it.
type TradeJournalLinkService struct {
	opaqueIdentifierProxy          domaininterface.IOpaqueIdentifierProxy
	strategyBotRunRecordRepository domaininterface.IStrategyBotRunRecordRepository
	strategyBotRepository          domaininterface.IStrategyBotRepository
	frontendBaseUrl                string
}

func NewTradeJournalLinkService(
	opaqueIdentifierProxy domaininterface.IOpaqueIdentifierProxy,
	strategyBotRunRecordRepository domaininterface.IStrategyBotRunRecordRepository,
	strategyBotRepository domaininterface.IStrategyBotRepository,
	frontendBaseUrl string,
) *TradeJournalLinkService {
	return &TradeJournalLinkService{
		opaqueIdentifierProxy:          opaqueIdentifierProxy,
		strategyBotRunRecordRepository: strategyBotRunRecordRepository,
		strategyBotRepository:          strategyBotRepository,
		frontendBaseUrl:                frontendBaseUrl,
	}
}

// OfferJournalLink leaves the round unchanged when no identifier can be minted, since a missing link must never cost the person the signal.
func (linkService *TradeJournalLinkService) OfferJournalLink(round dto.StrategyBotRoundDto) dto.StrategyBotRoundDto {
	linkDomain := domains.NewTradeJournalLinkDomain(round)
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

func (linkService *TradeJournalLinkService) FindOwnedRound(
	executionContext context.Context, viewerID uint, journalLinkIdentifier string,
) (dto.JournalLinkRoundDto, error) {
	return linkService.ownedRound(executionContext, viewerID, journalLinkIdentifier)
}

// ownedRound answers the same not-found for a forgotten round and for somebody else's bot, so links cannot be probed.
func (linkService *TradeJournalLinkService) ownedRound(
	executionContext context.Context, viewerID uint, journalLinkIdentifier string,
) (dto.JournalLinkRoundDto, error) {
	runRecord, isRemembered, findError := linkService.strategyBotRunRecordRepository.FindByJournalLinkIdentifier(
		executionContext, journalLinkIdentifier)
	if findError != nil {
		return dto.JournalLinkRoundDto{}, findError
	}
	if !isRemembered {
		return dto.JournalLinkRoundDto{}, fmt.Errorf("%w: 這一輪的建議已不在紀錄中", domains.ErrJournalLinkNotFound)
	}

	strategyBot, botError := linkService.strategyBotRepository.FindOne(executionContext, runRecord.StrategyBotID)
	if errors.Is(botError, domains.ErrStrategyBotNotFound) || (botError == nil && strategyBot.OwnerID != viewerID) {
		return dto.JournalLinkRoundDto{}, fmt.Errorf("%w: 找不到這一輪的建議", domains.ErrJournalLinkNotFound)
	}
	if botError != nil {
		return dto.JournalLinkRoundDto{}, botError
	}

	return runRecord.ToJournalLinkRoundDto(strategyBot), nil
}

// FindOwnedRoundIfRemembered is nil for a blank identifier or a round that can no longer be read, so a trade can still be recorded without its source.
func (linkService *TradeJournalLinkService) FindOwnedRoundIfRemembered(
	executionContext context.Context, viewerID uint, journalLinkIdentifier string,
) *dto.JournalLinkRoundDto {
	if journalLinkIdentifier == "" {
		return nil
	}

	linkRound, linkError := linkService.ownedRound(executionContext, viewerID, journalLinkIdentifier)
	if linkError != nil {
		return nil
	}

	return &linkRound
}
