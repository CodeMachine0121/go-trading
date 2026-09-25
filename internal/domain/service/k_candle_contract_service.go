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

type KCandleContractService struct {
	kCandleContractRepository domaininterface.IKCandleContractRepository
	clockProxy                domaininterface.IClockProxy
	// roundTheClockMarket sizes merged series with the same bucket arithmetic as spot, since contracts never close.
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

// GetKCandleContractSeries merges one stretch into a candle per interval bucket, earliest first, using the spot series' interval and size rules.
func (kCandleContractService *KCandleContractService) GetKCandleContractSeries(
	executionContext context.Context, seriesQueryDto dto.KCandleSeriesQueryDto,
) (dto.KCandleContractSeriesDto, error) {
	seriesQueryDomain, validationError := domains.NewKCandleSeriesQueryDomain(
		seriesQueryDto, kCandleContractService.roundTheClockMarket, kCandleContractService.queryMaxResults)
	if validationError != nil {
		// Re-badged so callers recognise the contract validation sentinel.
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

// SaveKCandleContract upserts a candle by symbol and open time without requiring the symbol to be watched, so hand-placed test data needs no watchlist entry.
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

// GetKCandleContractsInRange returns candles in the range, earliest first, refusing ranges over the configured maximum.
func (kCandleContractService *KCandleContractService) GetKCandleContractsInRange(
	executionContext context.Context, queryDto dto.KCandleQueryDto,
) ([]dto.KCandleContractDto, error) {
	queryDomain, validationError := domains.NewKCandleQueryDomain(queryDto)
	if validationError != nil {
		// The shared query model answers with the spot sentinel; re-badge it or callers report a bad request as a server error.
		return nil, fmt.Errorf("%w: %w", domains.ErrKCandleContractValidation, validationError)
	}

	// Fetching one extra row detects "too many" without a separate count.
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

// GetLatestKCandleContract returns the newest stored candle for the symbol; nothing stored is reported as false rather than an error.
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

// UpdateKCandleContract replaces the figures of the candle named by the input's symbol and open time.
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

// DeleteKCandleContract removes one contract candle; the spot candle of the same symbol and minute is untouched.
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
