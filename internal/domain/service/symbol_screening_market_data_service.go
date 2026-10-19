package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// symbolScreeningSettlementLeadIn reaches back past the first bar so it opens with the funding rate then in force.
const symbolScreeningSettlementLeadIn = 24 * time.Hour

// symbolScreeningSettlementFreshness skips asking for settlements when the last one kept is this close to the period's end.
const symbolScreeningSettlementFreshness = 8 * time.Hour

// SymbolScreeningMarketDataService gets screening data straight from the venue and keeps it apart from the followed minute candles, reusing what is already kept.
type SymbolScreeningMarketDataService struct {
	kCandleRepository          domaininterface.ISymbolScreeningKCandleRepository
	settlementRepository       domaininterface.ISymbolScreeningFundingRateSettlementRepository
	contractMarketDataProxy    domaininterface.IContractMarketDataProxy
	contractSymbolLookupProxy  domaininterface.IContractSymbolLookupProxy
	contractFundingRateProxy   domaininterface.IContractFundingRateProxy
	maintenanceMarginTierProxy domaininterface.IContractMaintenanceMarginTierProxy
	clockProxy                 domaininterface.IClockProxy
}

func NewSymbolScreeningMarketDataService(
	kCandleRepository domaininterface.ISymbolScreeningKCandleRepository,
	settlementRepository domaininterface.ISymbolScreeningFundingRateSettlementRepository,
	contractMarketDataProxy domaininterface.IContractMarketDataProxy,
	contractSymbolLookupProxy domaininterface.IContractSymbolLookupProxy,
	contractFundingRateProxy domaininterface.IContractFundingRateProxy,
	maintenanceMarginTierProxy domaininterface.IContractMaintenanceMarginTierProxy,
	clockProxy domaininterface.IClockProxy,
) *SymbolScreeningMarketDataService {
	return &SymbolScreeningMarketDataService{
		kCandleRepository:          kCandleRepository,
		settlementRepository:       settlementRepository,
		contractMarketDataProxy:    contractMarketDataProxy,
		contractSymbolLookupProxy:  contractSymbolLookupProxy,
		contractFundingRateProxy:   contractFundingRateProxy,
		maintenanceMarginTierProxy: maintenanceMarginTierProxy,
		clockProxy:                 clockProxy,
	}
}

// MarginTiersBySymbol reads every contract's ladder once per run; without account credentials it is empty, and replays fall back to the base rate.
func (marketDataService *SymbolScreeningMarketDataService) MarginTiersBySymbol(
	executionContext context.Context,
) (map[string][]vo.ContractMaintenanceMarginTierVo, error) {
	ladders, fetchError := marketDataService.maintenanceMarginTierProxy.FetchMaintenanceMarginLadders(executionContext)
	if errors.Is(fetchError, domains.ErrContractAccountCredentialsMissing) {
		return map[string][]vo.ContractMaintenanceMarginTierVo{}, nil
	}
	if fetchError != nil {
		return nil, fetchError
	}

	tiersBySymbol := make(map[string][]vo.ContractMaintenanceMarginTierVo, len(ladders))
	for _, ladder := range ladders {
		tiersBySymbol[ladder.Symbol] = ladder.Tiers
	}

	return tiersBySymbol, nil
}

// Prepare returns the venue's listing and, for a trading contract, its candles and a replay input covering the period plus warm-up.
func (marketDataService *SymbolScreeningMarketDataService) Prepare(
	executionContext context.Context,
	symbol string,
	period domains.SymbolScreeningPeriodDomain,
	marginTiers []vo.ContractMaintenanceMarginTierVo,
) (vo.ContractSymbolListingVo, []entities.KCandleContract, domains.ContractReplayInputDomain, error) {
	listing, lookUpError := marketDataService.contractSymbolLookupProxy.LookUpSymbol(executionContext, symbol)
	if lookUpError != nil {
		return vo.ContractSymbolListingVo{}, nil, domains.ContractReplayInputDomain{}, fmt.Errorf("行情取不到：%w", lookUpError)
	}
	if !listing.IsListed {
		return listing, nil, domains.ContractReplayInputDomain{}, nil
	}
	if listing.Specification.Symbol == "" {
		return listing, nil, domains.ContractReplayInputDomain{}, errors.New("幣安沒有提供這個合約的交易規格")
	}

	now := marketDataService.clockProxy.Now()
	window := period.FetchWindow(symbol)
	stored, findError := marketDataService.kCandleRepository.FindInRange(
		executionContext, symbol, window.Interval, window.StartTime, period.EndTime())
	if findError != nil {
		return listing, nil, domains.ContractReplayInputDomain{}, findError
	}

	isContinuous := len(stored) > 0 && !stored[0].OpenTime.After(window.StartTime) &&
		stored[len(stored)-1].OpenTime.Sub(stored[0].OpenTime) == time.Duration(len(stored)-1)*window.Step
	missingFrom := window.StartTime
	if isContinuous {
		missingFrom = stored[len(stored)-1].OpenTime.Add(window.Step)
	}
	if missingFrom.Before(period.EndTime()) {
		window.StartTime = missingFrom
		fetched, fetchError := marketDataService.contractMarketDataProxy.FetchAggregatedKCandles(executionContext, window)
		if fetchError != nil {
			return listing, nil, domains.ContractReplayInputDomain{}, fmt.Errorf("行情取不到：%w", fetchError)
		}
		kept := make([]entities.SymbolScreeningKCandle, 0, len(fetched))
		for _, kCandle := range fetched {
			// A bar the venue reports no mark price for is marked at its traded prices rather than dropped, so the series stays continuous.
			markOpen, markHigh, markLow, markClose := kCandle.Open, kCandle.High, kCandle.Low, kCandle.Close
			if kCandle.MarkClose.Valid {
				markOpen, markHigh = kCandle.MarkOpen.Decimal, kCandle.MarkHigh.Decimal
				markLow, markClose = kCandle.MarkLow.Decimal, kCandle.MarkClose.Decimal
			}
			kept = append(kept, entities.SymbolScreeningKCandle{
				Symbol: symbol, Interval: window.Interval, OpenTime: kCandle.OpenTime.UTC(),
				Open: kCandle.Open, High: kCandle.High, Low: kCandle.Low, Close: kCandle.Close,
				Volume: kCandle.Volume, QuoteVolume: kCandle.QuoteVolume,
				TakerBuyBaseVolume: kCandle.TakerBuyBaseVolume, TakerBuyQuoteVolume: kCandle.TakerBuyQuoteVolume,
				TradeCount: kCandle.TradeCount,
				MarkOpen:   markOpen, MarkHigh: markHigh, MarkLow: markLow, MarkClose: markClose,
				FetchedAt: now,
			})
		}
		if saveError := marketDataService.kCandleRepository.SaveAll(executionContext, kept); saveError != nil {
			return listing, nil, domains.ContractReplayInputDomain{}, saveError
		}
		if !isContinuous {
			stored = nil
		}
		stored = append(stored, kept...)
	}

	settlementStart := period.FetchStartTime().Add(-symbolScreeningSettlementLeadIn)
	storedSettlements, findSettlementsError := marketDataService.settlementRepository.FindInRange(
		executionContext, symbol, settlementStart, period.EndTime())
	if findSettlementsError != nil {
		return listing, nil, domains.ContractReplayInputDomain{}, findSettlementsError
	}
	// Kept settlements are reused only when they reach back to the period's lead-in; otherwise all are asked for again.
	reachesBack := len(storedSettlements) > 0 &&
		!storedSettlements[0].SettlementTime.After(settlementStart.Add(symbolScreeningSettlementFreshness))
	settlementsAfter := settlementStart
	if reachesBack {
		settlementsAfter = storedSettlements[len(storedSettlements)-1].SettlementTime
	}
	if !reachesBack || settlementsAfter.Before(period.EndTime().Add(-symbolScreeningSettlementFreshness)) {
		fetchedSettlements, fetchError := marketDataService.contractFundingRateProxy.FetchFundingRateSettlements(
			executionContext, symbol, settlementsAfter)
		if fetchError != nil {
			return listing, nil, domains.ContractReplayInputDomain{}, fmt.Errorf("資金費率取不到：%w", fetchError)
		}
		keptSettlements := make([]entities.SymbolScreeningFundingRateSettlement, 0, len(fetchedSettlements))
		for _, settlement := range fetchedSettlements {
			if !settlement.SettlementTime.Before(period.EndTime()) {
				continue
			}
			keptSettlements = append(keptSettlements, entities.SymbolScreeningFundingRateSettlement{
				Symbol: symbol, SettlementTime: settlement.SettlementTime.UTC(), FundingRate: settlement.FundingRate,
				MarkPrice: settlement.MarkPrice, FetchedAt: now,
			})
		}
		if saveError := marketDataService.settlementRepository.SaveAll(executionContext, keptSettlements); saveError != nil {
			return listing, nil, domains.ContractReplayInputDomain{}, saveError
		}
		if !reachesBack {
			storedSettlements = nil
		}
		storedSettlements = append(storedSettlements, keptSettlements...)
	}

	kCandleContracts := make([]entities.KCandleContract, 0, len(stored))
	for _, kCandle := range stored {
		kCandleContracts = append(kCandleContracts, kCandle.ToKCandleContract())
	}
	settlements := make([]entities.ContractFundingRateSettlement, 0, len(storedSettlements))
	for _, settlement := range storedSettlements {
		settlements = append(settlements, settlement.ToContractFundingRateSettlement())
	}

	return listing, kCandleContracts, domains.NewContractReplayInputDomain(
		symbol, listing.Specification, marginTiers, kCandleContracts, settlements, now), nil
}

// DeleteExpired drops data fetched longer ago than the retention; the next screening fetches it again.
func (marketDataService *SymbolScreeningMarketDataService) DeleteExpired(
	executionContext context.Context, retention time.Duration,
) (int, error) {
	cutoff := marketDataService.clockProxy.Now().Add(-retention)
	deletedKCandles, deleteKCandlesError := marketDataService.kCandleRepository.DeleteFetchedBefore(executionContext, cutoff)
	if deleteKCandlesError != nil {
		return 0, deleteKCandlesError
	}
	deletedSettlements, deleteSettlementsError := marketDataService.settlementRepository.DeleteFetchedBefore(
		executionContext, cutoff)

	return deletedKCandles + deletedSettlements, deleteSettlementsError
}
