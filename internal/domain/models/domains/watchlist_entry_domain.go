package domains

import (
	"fmt"
	"strings"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// WatchlistEntryDomain checks only what needs no round trip to the market; whether the
// market knows the code is checked afterwards.
type WatchlistEntryDomain struct {
	symbol string
	market vo.MarketVo
}

// NewWatchlistEntryDomain refuses unknown markets on new requests, even though stored rows
// naming none are tolerated.
func NewWatchlistEntryDomain(
	entryDto dto.WatchlistEntryDto, marketCatalogDomain MarketCatalogDomain,
) (WatchlistEntryDomain, error) {
	tradingSymbolDomain, symbolError := NewTradingSymbolDomain(strings.TrimSpace(entryDto.Symbol))
	if symbolError != nil {
		return WatchlistEntryDomain{}, fmt.Errorf("%w: %w", ErrWatchlistEntryValidation, symbolError)
	}

	if !marketCatalogDomain.IsRecognised(entryDto.Market) {
		return WatchlistEntryDomain{}, fmt.Errorf(
			"%w: 市場只能是 %s 其中之一",
			ErrWatchlistEntryValidation,
			strings.Join(marketCatalogDomain.RecognisedMarkets(), "、"))
	}

	return WatchlistEntryDomain{
		symbol: tradingSymbolDomain.Value(),
		market: marketCatalogDomain.MarketOf(entryDto.Market).Value(),
	}, nil
}

// Symbol is the trimmed code.
func (watchlistEntryDomain WatchlistEntryDomain) Symbol() string {
	return watchlistEntryDomain.symbol
}

func (watchlistEntryDomain WatchlistEntryDomain) Market() vo.MarketVo {
	return watchlistEntryDomain.market
}

// ToEntity keeps any earlier registration time so a re-added market keeps its place in the
// follow queue; the display name comes from the venue and stays empty if it gave none.
func (watchlistEntryDomain WatchlistEntryDomain) ToEntity(
	listing vo.SymbolListingVo, previouslyRegisteredAt time.Time, currentTime time.Time,
) entities.TradingSymbol {
	registeredAt := previouslyRegisteredAt.UTC()
	if registeredAt.IsZero() {
		registeredAt = currentTime.UTC()
	}

	return entities.TradingSymbol{
		Symbol:       watchlistEntryDomain.symbol,
		Market:       string(watchlistEntryDomain.market),
		DisplayName:  strings.TrimSpace(listing.DisplayName),
		IsWatched:    true,
		RegisteredAt: registeredAt,
	}
}
