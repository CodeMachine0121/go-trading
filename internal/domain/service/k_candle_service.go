package service

import (
	"context"
	"fmt"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

type KCandleService struct {
	kCandleRepository       domaininterface.IKCandleRepository
	tradingSymbolRepository domaininterface.ITradingSymbolRepository
	clockProxy              domaininterface.IClockProxy
	marketCatalogDomain     domains.MarketCatalogDomain
	queryMaxResults         int
}

func NewKCandleService(
	kCandleRepository domaininterface.IKCandleRepository,
	tradingSymbolRepository domaininterface.ITradingSymbolRepository,
	clockProxy domaininterface.IClockProxy,
	marketCatalogDomain domains.MarketCatalogDomain,
	queryMaxResults int,
) *KCandleService {
	return &KCandleService{
		kCandleRepository:       kCandleRepository,
		tradingSymbolRepository: tradingSymbolRepository,
		clockProxy:              clockProxy,
		marketCatalogDomain:     marketCatalogDomain,
		queryMaxResults:         queryMaxResults,
	}
}

// SaveKCandle upserts one K candle by trading symbol and open time.
func (kCandleService *KCandleService) SaveKCandle(
	executionContext context.Context, writeDto dto.KCandleWriteDto,
) (dto.KCandleDto, error) {
	kCandleDomain, validationError := domains.NewKCandleDomain(writeDto, kCandleService.clockProxy.Now())
	if validationError != nil {
		return dto.KCandleDto{}, validationError
	}

	savedKCandle, saveError := kCandleService.kCandleRepository.Save(executionContext, kCandleDomain.ToEntity())
	if saveError != nil {
		return dto.KCandleDto{}, saveError
	}

	return savedKCandle.ToDto(), nil
}

// GetKCandlesInRange returns candles in the range, earliest first, refusing ranges over the configured maximum.
func (kCandleService *KCandleService) GetKCandlesInRange(
	executionContext context.Context, queryDto dto.KCandleQueryDto,
) ([]dto.KCandleDto, error) {
	queryDomain, validationError := domains.NewKCandleQueryDomain(queryDto)
	if validationError != nil {
		return nil, validationError
	}

	kCandles, findError := kCandleService.kCandleRepository.FindInRange(
		executionContext, queryDomain, kCandleService.queryMaxResults+1)
	if findError != nil {
		return nil, findError
	}

	if len(kCandles) > kCandleService.queryMaxResults {
		return nil, fmt.Errorf(
			"%w: 時間區間過大，請縮小區間（單次最多 %d 根）",
			domains.ErrKCandleValidation, kCandleService.queryMaxResults)
	}

	kCandleDtos := make([]dto.KCandleDto, 0, len(kCandles))
	for _, kCandle := range kCandles {
		kCandleDtos = append(kCandleDtos, kCandle.ToDto())
	}

	return kCandleDtos, nil
}

// GetKCandleSeries merges the range into one candle per bucket, earliest first, at the requested or finest displayable interval, refusing too many buckets before reading.
func (kCandleService *KCandleService) GetKCandleSeries(
	executionContext context.Context, seriesQueryDto dto.KCandleSeriesQueryDto,
) (dto.KCandleSeriesDto, error) {
	// The symbol's market sizes the range; an unregistered symbol reads as the round-the-clock market rather than being refused.
	registeredSymbol, _, findSymbolError := kCandleService.tradingSymbolRepository.
		FindBySymbol(executionContext, seriesQueryDto.Symbol)
	if findSymbolError != nil {
		return dto.KCandleSeriesDto{}, findSymbolError
	}

	seriesQueryDomain, validationError := domains.NewKCandleSeriesQueryDomain(
		seriesQueryDto,
		kCandleService.marketCatalogDomain.MarketOf(registeredSymbol.Market),
		kCandleService.queryMaxResults,
	)
	if validationError != nil {
		return dto.KCandleSeriesDto{}, validationError
	}

	kCandles, findError := kCandleService.kCandleRepository.FindInRange(
		executionContext, seriesQueryDomain.RangeQuery(), seriesQueryDomain.SourceCandleLimit())
	if findError != nil {
		return dto.KCandleSeriesDto{}, findError
	}

	return seriesQueryDomain.SeriesOf(kCandles).ToDto(), nil
}

func (kCandleService *KCandleService) GetKCandle(
	executionContext context.Context, symbol string, openTime time.Time,
) (dto.KCandleDto, error) {
	// No domain model validates the symbol on this path, so it is checked here to report a bad request instead of a server error.
	tradingSymbol, symbolError := domains.NewTradingSymbolDomain(symbol)
	if symbolError != nil {
		return dto.KCandleDto{}, fmt.Errorf("%w: %w", domains.ErrKCandleValidation, symbolError)
	}

	kCandle, findError := kCandleService.kCandleRepository.FindOne(
		executionContext, tradingSymbol.Value(), openTime.UTC())
	if findError != nil {
		return dto.KCandleDto{}, findError
	}

	return kCandle.ToDto(), nil
}

// GetLatestKCandle returns the newest stored candle for the symbol; nothing stored is reported as false rather than an error.
func (kCandleService *KCandleService) GetLatestKCandle(
	executionContext context.Context, symbol string,
) (dto.KCandleDto, bool, error) {
	tradingSymbol, symbolError := domains.NewTradingSymbolDomain(symbol)
	if symbolError != nil {
		return dto.KCandleDto{}, false, fmt.Errorf(
			"%w: %w", domains.ErrKCandleValidation, symbolError)
	}

	kCandles, findError := kCandleService.kCandleRepository.FindLatest(
		executionContext, tradingSymbol.Value(), 1)
	if findError != nil {
		return dto.KCandleDto{}, false, findError
	}

	if len(kCandles) == 0 {
		return dto.KCandleDto{}, false, nil
	}

	return kCandles[0].ToDto(), true, nil
}

// UpdateKCandle replaces the figures of the candle named by the input's symbol and open time.
func (kCandleService *KCandleService) UpdateKCandle(
	executionContext context.Context, writeDto dto.KCandleWriteDto,
) (dto.KCandleDto, error) {
	kCandleDomain, validationError := domains.NewKCandleDomain(writeDto, kCandleService.clockProxy.Now())
	if validationError != nil {
		return dto.KCandleDto{}, validationError
	}

	updatedKCandle, updateError := kCandleService.kCandleRepository.Update(executionContext, kCandleDomain.ToEntity())
	if updateError != nil {
		return dto.KCandleDto{}, updateError
	}

	return updatedKCandle.ToDto(), nil
}

func (kCandleService *KCandleService) DeleteKCandle(
	executionContext context.Context, symbol string, openTime time.Time,
) error {
	tradingSymbol, symbolError := domains.NewTradingSymbolDomain(symbol)
	if symbolError != nil {
		return fmt.Errorf("%w: %w", domains.ErrKCandleValidation, symbolError)
	}

	return kCandleService.kCandleRepository.Delete(
		executionContext, tradingSymbol.Value(), openTime.UTC())
}
