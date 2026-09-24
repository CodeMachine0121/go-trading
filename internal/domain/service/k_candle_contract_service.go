package service

import (
	"context"
	"fmt"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// KCandleContractService is the application layer's only entry point for perpetual
// contract K candles. Its public use-case methods never call one another.
//
// Perpetual contracts never close, so the one thing it takes from the market catalogue
// is the round-the-clock calendar, and only to size a merged series by the same
// arithmetic the spot series uses.
type KCandleContractService struct {
	kCandleContractRepository domaininterface.IKCandleContractRepository
	clockProxy                domaininterface.IClockProxy
	// roundTheClockMarket is the calendar perpetual contracts keep, borrowed from the
	// catalogue so that "how many buckets does this stretch hold" is answered by the
	// same arithmetic the spot series uses.
	roundTheClockMarket domains.MarketDomain
	queryMaxResults     int
}

func NewKCandleContractService(
	kCandleContractRepository domaininterface.IKCandleContractRepository,
	clockProxy domaininterface.IClockProxy,
	marketCatalogDomain domains.MarketCatalogDomain,
	queryMaxResults int,
) *KCandleContractService {
	return &KCandleContractService{
		kCandleContractRepository: kCandleContractRepository,
		clockProxy:                clockProxy,
		roundTheClockMarket:       marketCatalogDomain.MarketOf(string(vo.MarketCrypto)),
		queryMaxResults:           queryMaxResults,
	}
}

// GetKCandleContractSeries merges the contract K candles of one stretch into one
// candle per interval bucket, earliest first, saying which interval it used. How the
// interval is chosen and how large a stretch may be asked for are the spot series'
// rules, word for word.
func (kCandleContractService *KCandleContractService) GetKCandleContractSeries(
	executionContext context.Context, seriesQueryDto dto.KCandleSeriesQueryDto,
) (dto.KCandleContractSeriesDto, error) {
	seriesQueryDomain, validationError := domains.NewKCandleSeriesQueryDomain(
		seriesQueryDto, kCandleContractService.roundTheClockMarket, kCandleContractService.queryMaxResults)
	if validationError != nil {
		// Re-badged before it leaves, for the reason the range query gives.
		return dto.KCandleContractSeriesDto{}, fmt.Errorf(
			"%w: %w", domains.ErrKCandleContractValidation, validationError)
	}

	kCandleContracts, findError := kCandleContractService.kCandleContractRepository.FindInRange(
		executionContext, seriesQueryDomain.RangeQuery(), seriesQueryDomain.SourceCandleLimit())
	if findError != nil {
		return dto.KCandleContractSeriesDto{}, findError
	}

	return seriesQueryDomain.ContractSeriesOf(kCandleContracts).ToDto(), nil
}

// SaveKCandleContract stores one contract K candle, replacing any candle already held
// for the same trading symbol and open time.
//
// It does not ask whether the symbol is being followed. A candle put here by hand is
// the caller's own figure for a minute, and requiring the symbol to be on the
// watchlist first would make test data impossible to place without also arranging for
// it to be fetched forever.
func (kCandleContractService *KCandleContractService) SaveKCandleContract(
	executionContext context.Context, writeDto dto.KCandleContractWriteDto,
) (dto.KCandleContractDto, error) {
	contractDomain, validationError := domains.NewKCandleContractDomain(
		writeDto, kCandleContractService.clockProxy.Now())
	if validationError != nil {
		return dto.KCandleContractDto{}, validationError
	}

	savedCandle, saveError := kCandleContractService.kCandleContractRepository.Save(
		executionContext, contractDomain.ToEntity())
	if saveError != nil {
		return dto.KCandleContractDto{}, saveError
	}

	return savedCandle.ToDto(), nil
}

// GetKCandleContractsInRange returns the contract K candles whose open time falls
// inside the range, earliest first. A range holding more than the configured maximum
// is refused.
func (kCandleContractService *KCandleContractService) GetKCandleContractsInRange(
	executionContext context.Context, queryDto dto.KCandleQueryDto,
) ([]dto.KCandleContractDto, error) {
	queryDomain, validationError := domains.NewKCandleQueryDomain(queryDto)
	if validationError != nil {
		// Re-badged before it leaves. The query model is shared with the spot side and
		// answers in the spot sentinel, and a caller of this path recognises this
		// path's — told otherwise, it reports a bad request as a broken server.
		return nil, fmt.Errorf("%w: %w", domains.ErrKCandleContractValidation, validationError)
	}

	// One more than the maximum is asked for, so that "too many" is something the
	// answer shows rather than something a second count has to establish.
	contractCandles, findError := kCandleContractService.kCandleContractRepository.FindInRange(
		executionContext, queryDomain, kCandleContractService.queryMaxResults+1)
	if findError != nil {
		return nil, findError
	}

	if len(contractCandles) > kCandleContractService.queryMaxResults {
		return nil, fmt.Errorf(
			"%w: 時間區間過大，請縮小區間（單次最多 %d 根）",
			domains.ErrKCandleContractValidation, kCandleContractService.queryMaxResults)
	}

	contractCandleDtos := make([]dto.KCandleContractDto, 0, len(contractCandles))
	for _, contractCandle := range contractCandles {
		contractCandleDtos = append(contractCandleDtos, contractCandle.ToDto())
	}

	return contractCandleDtos, nil
}

// GetKCandleContract returns the single contract K candle named by trading symbol and
// open time.
func (kCandleContractService *KCandleContractService) GetKCandleContract(
	executionContext context.Context, symbol string, openTime time.Time,
) (dto.KCandleContractDto, error) {
	tradingSymbol, symbolError := domains.NewTradingSymbolDomain(symbol)
	if symbolError != nil {
		return dto.KCandleContractDto{}, fmt.Errorf(
			"%w: %w", domains.ErrKCandleContractValidation, symbolError)
	}

	contractCandle, findError := kCandleContractService.kCandleContractRepository.FindOne(
		executionContext, tradingSymbol.Value(), openTime.UTC())
	if findError != nil {
		return dto.KCandleContractDto{}, findError
	}

	return contractCandle.ToDto(), nil
}

// GetLatestKCandleContract returns the newest contract K candle stored for this
// symbol, and whether there was one at all — the contract twin of the spot question.
//
// Nothing stored is an answer rather than a failure: a contract nobody has ingested
// yet is an ordinary state, and a caller told it was a failure would have to work out
// which failures are really nothing.
func (kCandleContractService *KCandleContractService) GetLatestKCandleContract(
	executionContext context.Context, symbol string,
) (dto.KCandleContractDto, bool, error) {
	tradingSymbol, symbolError := domains.NewTradingSymbolDomain(symbol)
	if symbolError != nil {
		return dto.KCandleContractDto{}, false, fmt.Errorf(
			"%w: %w", domains.ErrKCandleContractValidation, symbolError)
	}

	contractCandles, findError := kCandleContractService.kCandleContractRepository.FindLatest(
		executionContext, tradingSymbol.Value(), 1)
	if findError != nil {
		return dto.KCandleContractDto{}, false, findError
	}

	if len(contractCandles) == 0 {
		return dto.KCandleContractDto{}, false, nil
	}

	return contractCandles[0].ToDto(), true, nil
}

// UpdateKCandleContract replaces the figures of an existing contract K candle. The
// candle it acts on is the one named by the trading symbol and open time carried in
// the input.
func (kCandleContractService *KCandleContractService) UpdateKCandleContract(
	executionContext context.Context, writeDto dto.KCandleContractWriteDto,
) (dto.KCandleContractDto, error) {
	contractDomain, validationError := domains.NewKCandleContractDomain(
		writeDto, kCandleContractService.clockProxy.Now())
	if validationError != nil {
		return dto.KCandleContractDto{}, validationError
	}

	updatedCandle, updateError := kCandleContractService.kCandleContractRepository.Update(
		executionContext, contractDomain.ToEntity())
	if updateError != nil {
		return dto.KCandleContractDto{}, updateError
	}

	return updatedCandle.ToDto(), nil
}

// DeleteKCandleContract removes the single contract K candle named by trading symbol
// and open time. The spot candle of the same name and minute is a different record
// and is not touched.
func (kCandleContractService *KCandleContractService) DeleteKCandleContract(
	executionContext context.Context, symbol string, openTime time.Time,
) error {
	tradingSymbol, symbolError := domains.NewTradingSymbolDomain(symbol)
	if symbolError != nil {
		return fmt.Errorf("%w: %w", domains.ErrKCandleContractValidation, symbolError)
	}

	return kCandleContractService.kCandleContractRepository.Delete(
		executionContext, tradingSymbol.Value(), openTime.UTC())
}
