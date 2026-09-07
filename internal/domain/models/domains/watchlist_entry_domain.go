package domains

import (
	"fmt"
	"strings"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// WatchlistEntryDomain is a request to start watching a market, checked as far as it
// can be checked without asking the market itself.
//
// The two things it checks are the two nobody else can answer: a code has to be
// something, and it has to be said to belong to a market this system recognises.
// Whether that market has actually heard of the code is a question only the market
// can answer, and asking it costs a round trip — so it happens after this, and only
// for requests worth the trip.
type WatchlistEntryDomain struct {
	symbol string
	market vo.MarketVo
}

// NewWatchlistEntryDomain reads what was asked for, refusing a blank code and a
// market the catalog does not recognise.
//
// Naming a market that does not exist is refused, even though reading a stored row
// that names nothing is forgiven. The difference is that one is a request being made
// now, which can be corrected, and the other is history, which cannot.
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

// Symbol is the code, with the blanks around it dropped.
func (watchlistEntryDomain WatchlistEntryDomain) Symbol() string {
	return watchlistEntryDomain.symbol
}

// Market is the market this code belongs to.
func (watchlistEntryDomain WatchlistEntryDomain) Market() vo.MarketVo {
	return watchlistEntryDomain.market
}

// ToEntity is this entry as a registered, watched market.
//
// It keeps the registration time it was first given, so that adding back a market
// somebody removed does not send it to the back of the queue for a market's follow
// places. A symbol nobody registered before is stamped with now.
func (watchlistEntryDomain WatchlistEntryDomain) ToEntity(
	previouslyRegisteredAt time.Time, currentTime time.Time,
) entities.TradingSymbol {
	registeredAt := previouslyRegisteredAt.UTC()
	if registeredAt.IsZero() {
		registeredAt = currentTime.UTC()
	}

	return entities.TradingSymbol{
		Symbol:       watchlistEntryDomain.symbol,
		Market:       string(watchlistEntryDomain.market),
		IsWatched:    true,
		RegisteredAt: registeredAt,
	}
}
