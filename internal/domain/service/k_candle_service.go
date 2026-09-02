package service

import (
	"fmt"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// KCandleService is the application layer's only entry point for K candles.
// Its public use-case methods never call one another.
type KCandleService struct {
	kCandleRepository domaininterface.IKCandleRepository
	clockProxy        domaininterface.IClockProxy
	queryMaxResults   int
}

func NewKCandleService(
	kCandleRepository domaininterface.IKCandleRepository,
	clockProxy domaininterface.IClockProxy,
	queryMaxResults int,
) *KCandleService {
	return &KCandleService{
		kCandleRepository: kCandleRepository,
		clockProxy:        clockProxy,
		queryMaxResults:   queryMaxResults,
	}
}

// SaveKCandle stores one K candle, replacing any candle already held for the
// same trading symbol and open time.
func (kCandleService *KCandleService) SaveKCandle(writeDto dto.KCandleWriteDto) (dto.KCandleDto, error) {
	kCandleDomain, validationError := domains.NewKCandleDomain(writeDto, kCandleService.clockProxy.Now())
	if validationError != nil {
		return dto.KCandleDto{}, validationError
	}

	savedKCandle, saveError := kCandleService.kCandleRepository.Save(kCandleDomain.ToEntity())
	if saveError != nil {
		return dto.KCandleDto{}, saveError
	}

	return savedKCandle.ToDto(), nil
}

// GetKCandlesInRange returns the K candles whose open time falls inside the range,
// earliest first. A range holding more than the configured maximum is refused.
func (kCandleService *KCandleService) GetKCandlesInRange(queryDto dto.KCandleQueryDto) ([]dto.KCandleDto, error) {
	queryDomain, validationError := domains.NewKCandleQueryDomain(queryDto)
	if validationError != nil {
		return nil, validationError
	}

	kCandles, findError := kCandleService.kCandleRepository.FindInRange(
		queryDomain, kCandleService.queryMaxResults+1)
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

// GetKCandleSeries returns the K candles inside the range merged into one candle per
// bucket of the requested aggregation interval, earliest first. A range cut into more
// buckets than the configured maximum is refused before anything is read.
func (kCandleService *KCandleService) GetKCandleSeries(
	seriesQueryDto dto.KCandleSeriesQueryDto,
) (dto.KCandleSeriesDto, error) {
	seriesQueryDomain, validationError := domains.NewKCandleSeriesQueryDomain(
		seriesQueryDto, kCandleService.queryMaxResults)
	if validationError != nil {
		return dto.KCandleSeriesDto{}, validationError
	}

	kCandles, findError := kCandleService.kCandleRepository.FindInRange(
		seriesQueryDomain.RangeQuery(), seriesQueryDomain.SourceCandleLimit())
	if findError != nil {
		return dto.KCandleSeriesDto{}, findError
	}

	return domains.NewKCandleSeriesDomain(
		seriesQueryDomain.RangeQuery().Symbol(),
		seriesQueryDomain.Interval(),
		kCandles,
	).ToDto(), nil
}

// GetKCandle returns the single K candle named by trading symbol and open time.
func (kCandleService *KCandleService) GetKCandle(symbol string, openTime time.Time) (dto.KCandleDto, error) {
	kCandle, findError := kCandleService.kCandleRepository.FindOne(symbol, openTime.UTC())
	if findError != nil {
		return dto.KCandleDto{}, findError
	}

	return kCandle.ToDto(), nil
}

// UpdateKCandle replaces the figures of an existing K candle. The candle it acts on
// is the one named by the trading symbol and open time carried in the input.
func (kCandleService *KCandleService) UpdateKCandle(writeDto dto.KCandleWriteDto) (dto.KCandleDto, error) {
	kCandleDomain, validationError := domains.NewKCandleDomain(writeDto, kCandleService.clockProxy.Now())
	if validationError != nil {
		return dto.KCandleDto{}, validationError
	}

	updatedKCandle, updateError := kCandleService.kCandleRepository.Update(kCandleDomain.ToEntity())
	if updateError != nil {
		return dto.KCandleDto{}, updateError
	}

	return updatedKCandle.ToDto(), nil
}

// DeleteKCandle removes the single K candle named by trading symbol and open time.
func (kCandleService *KCandleService) DeleteKCandle(symbol string, openTime time.Time) error {
	return kCandleService.kCandleRepository.Delete(symbol, openTime.UTC())
}
