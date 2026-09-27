package persistence_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func aSpotFill(kind vo.SpotTradeFillKindVo, filledAt time.Time, price string, quantity string) entities.SpotTradeFill {
	return entities.SpotTradeFill{
		Kind: string(kind), FilledAt: filledAt, Price: decimal.RequireFromString(price),
		Quantity: decimal.RequireFromString(quantity), Fee: decimal.RequireFromString("1496"),
	}
}

func anOpenSpotTrade(ownerID uint, symbol string, market string, openedAt time.Time) entities.SpotTradeRecord {
	return entities.SpotTradeRecord{
		OwnerID: ownerID, Symbol: symbol, Market: market, Status: string(vo.SpotTradeStatusOpen), OpenedAt: openedAt,
		Fills: []entities.SpotTradeFill{aSpotFill(vo.SpotTradeFillKindBuy, openedAt, "1050", "1000")},
	}
}

func TestSpotTradeRecordRepositoryStoresATradeWithItsChildren(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	setupTag, tagError := persistence.NewTradeTagRepository(database).Create(t.Context(),
		entities.TradeTag{OwnerID: owner.ID, Kind: string(vo.TradeTagKindSetup), Name: "突破"})
	require.NoError(t, tagError)
	repository := persistence.NewSpotTradeRecordRepository(database)
	trade := anOpenSpotTrade(owner.ID, "2330", "taiwanStock", journalOpenedAt)
	trade.Tags = []entities.TradeTag{setupTag}
	trade.PlannedStopLossPrice = decimal.NullDecimal{Decimal: decimal.RequireFromString("1000"), Valid: true}

	created, createError := repository.Create(t.Context(), trade)

	require.NoError(t, createError)
	assert.Positive(t, created.ID)
	assert.Equal(t, "taiwanStock", created.Market)
	require.Len(t, created.Fills, 1)
	assert.Equal(t, "1050", created.Fills[0].Price.String())
	require.Len(t, created.Tags, 1)
	assert.Equal(t, "1000", created.PlannedStopLossPrice.Decimal.String())
}

func TestSpotTradeRecordRepositoryKeepsOneOpenTradePerSymbol(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	repository := persistence.NewSpotTradeRecordRepository(database)
	_, firstError := repository.Create(t.Context(), anOpenSpotTrade(owner.ID, "2330", "taiwanStock", journalOpenedAt))
	require.NoError(t, firstError)

	_, secondError := repository.Create(t.Context(), anOpenSpotTrade(owner.ID, "2330", "taiwanStock", journalOpenedAt))
	_, otherSymbolError := repository.Create(t.Context(), anOpenSpotTrade(owner.ID, "0050", "taiwanStock", journalOpenedAt))
	_, contractSameSymbolError := persistence.NewContractTradeRecordRepository(database).Create(
		t.Context(), anOpenTrade(owner.ID, "2330", "long", journalOpenedAt))

	require.ErrorIs(t, secondError, domains.ErrSpotTradeOpenHoldingExists)
	require.NoError(t, otherSymbolError)
	require.NoError(t, contractSameSymbolError)
}

func TestSpotTradeRecordRepositorySaveKeepsSurvivingFillIdentifiers(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	repository := persistence.NewSpotTradeRecordRepository(database)
	created, createError := repository.Create(t.Context(), anOpenSpotTrade(owner.ID, "2330", "taiwanStock", journalOpenedAt))
	require.NoError(t, createError)
	firstFillID := created.Fills[0].ID

	closedAt := journalOpenedAt.Add(time.Hour)
	created.Fills = append(created.Fills, aSpotFill(vo.SpotTradeFillKindSell, closedAt, "1120", "1000"))
	created.Status = string(vo.SpotTradeStatusClosed)
	created.ClosedAt = &closedAt
	created.Notes = append(created.Notes, entities.SpotTradeNote{Content: "打錯了", CreatedAt: closedAt})
	saved, saveError := repository.Save(t.Context(), created)
	require.NoError(t, saveError)
	require.Len(t, saved.Fills, 2)
	assert.Equal(t, firstFillID, saved.Fills[0].ID)
	require.Len(t, saved.Notes, 1)

	saved.Fills = saved.Fills[:1]
	saved.Status = string(vo.SpotTradeStatusOpen)
	saved.ClosedAt = nil
	resaved, resaveError := repository.Save(t.Context(), saved)
	require.NoError(t, resaveError)
	require.Len(t, resaved.Fills, 1)
	assert.Equal(t, firstFillID, resaved.Fills[0].ID)
	assert.Nil(t, resaved.ClosedAt)
}

func TestSpotTradeRecordRepositoryListsAndFindsTrades(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	stranger := aDeliveryOwner(t, database, "stranger@example.com")
	repository := persistence.NewSpotTradeRecordRepository(database)
	tradingStrategyID := uint(11)

	taiwanTrade := anOpenSpotTrade(owner.ID, "2330", "taiwanStock", journalOpenedAt)
	closedAt := journalOpenedAt.Add(48 * time.Hour)
	taiwanTrade.Fills = append(taiwanTrade.Fills, aSpotFill(vo.SpotTradeFillKindSell, closedAt, "1120", "1000"))
	taiwanTrade.Status = string(vo.SpotTradeStatusClosed)
	taiwanTrade.ClosedAt = &closedAt
	taiwanTrade.TradingStrategyID = &tradingStrategyID
	_, taiwanError := repository.Create(t.Context(), taiwanTrade)
	require.NoError(t, taiwanError)
	cryptoTrade, cryptoError := repository.Create(t.Context(),
		anOpenSpotTrade(owner.ID, "BTCUSDT", "crypto", journalOpenedAt.AddDate(0, 0, 1)))
	require.NoError(t, cryptoError)
	_, strangerError := repository.Create(t.Context(), anOpenSpotTrade(stranger.ID, "2330", "taiwanStock", journalOpenedAt))
	require.NoError(t, strangerError)

	everything, totalCount, listError := repository.FindPageByOwner(t.Context(), owner.ID, vo.TradeListFilterVo{Limit: 10})
	require.NoError(t, listError)
	assert.Equal(t, int64(2), totalCount)
	assert.Equal(t, "BTCUSDT", everything[0].Symbol)

	openedSince := journalOpenedAt.Add(12 * time.Hour)
	narrowed, narrowedCount, narrowedError := repository.FindPageByOwner(t.Context(), owner.ID, vo.TradeListFilterVo{
		Status: "closed", Symbol: "2330", Market: "taiwanStock", Limit: 10})
	require.NoError(t, narrowedError)
	assert.Equal(t, int64(1), narrowedCount)
	assert.Equal(t, "2330", narrowed[0].Symbol)
	_, sinceCount, sinceError := repository.FindPageByOwner(t.Context(), owner.ID, vo.TradeListFilterVo{
		Market: "crypto", OpenedSince: &openedSince, Limit: 10})
	require.NoError(t, sinceError)
	assert.Equal(t, int64(1), sinceCount)

	closedTrades, closedError := repository.FindClosedByOwner(t.Context(), owner.ID, nil)
	require.NoError(t, closedError)
	require.Len(t, closedTrades, 1)
	laterSince := closedAt.Add(time.Hour)
	noneClosed, noneError := repository.FindClosedByOwner(t.Context(), owner.ID, &laterSince)
	require.NoError(t, noneError)
	assert.Empty(t, noneClosed)

	followingTrades, followingError := repository.FindClosedByOwnerAndTradingStrategy(t.Context(), owner.ID, tradingStrategyID)
	require.NoError(t, followingError)
	assert.Len(t, followingTrades, 1)

	openTrade, hasOpenTrade, openError := repository.FindOpenByOwnerSymbol(t.Context(), owner.ID, "BTCUSDT")
	require.NoError(t, openError)
	assert.True(t, hasOpenTrade)
	assert.Equal(t, cryptoTrade.ID, openTrade.ID)
	_, hasClosedAsOpen, _ := repository.FindOpenByOwnerSymbol(t.Context(), owner.ID, "2330")
	assert.False(t, hasClosedAsOpen)

	_, missingError := repository.FindOne(t.Context(), 99999)
	require.ErrorIs(t, missingError, domains.ErrSpotTradeNotFound)
}

func TestSpotTradeRecordRepositoryDeletesATradeAndCountsTags(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	mistakeTag, tagError := persistence.NewTradeTagRepository(database).Create(t.Context(),
		entities.TradeTag{OwnerID: owner.ID, Kind: string(vo.TradeTagKindMistake), Name: "追價進場"})
	require.NoError(t, tagError)
	repository := persistence.NewSpotTradeRecordRepository(database)
	trade := anOpenSpotTrade(owner.ID, "2330", "taiwanStock", journalOpenedAt)
	trade.Tags = []entities.TradeTag{mistakeTag}
	trade.Notes = []entities.SpotTradeNote{{Content: "盤前掛單", CreatedAt: journalOpenedAt}}
	created, createError := repository.Create(t.Context(), trade)
	require.NoError(t, createError)

	countBefore, countBeforeError := repository.CountByTag(t.Context(), mistakeTag.ID)
	require.NoError(t, repository.Delete(t.Context(), created.ID))
	countAfter, countAfterError := repository.CountByTag(t.Context(), mistakeTag.ID)

	require.NoError(t, countBeforeError)
	require.NoError(t, countAfterError)
	assert.Equal(t, int64(1), countBefore)
	assert.Equal(t, int64(0), countAfter)
	remainingFills := int64(0)
	require.NoError(t, database.Model(&entities.SpotTradeFill{}).Count(&remainingFills).Error)
	assert.Zero(t, remainingFills)
	remainingNotes := int64(0)
	require.NoError(t, database.Model(&entities.SpotTradeNote{}).Count(&remainingNotes).Error)
	assert.Zero(t, remainingNotes)
}

func TestSpotTradeRecordRepositoryReportsStorageFailures(t *testing.T) {
	repository := persistence.NewSpotTradeRecordRepository(closedDatabase(t))

	_, createError := repository.Create(t.Context(), anOpenSpotTrade(1, "2330", "taiwanStock", journalOpenedAt))
	_, saveError := repository.Save(t.Context(), anOpenSpotTrade(1, "2330", "taiwanStock", journalOpenedAt))
	_, findError := repository.FindOne(t.Context(), 1)
	_, _, pageError := repository.FindPageByOwner(t.Context(), 1, vo.TradeListFilterVo{Limit: 1})
	_, closedError := repository.FindClosedByOwner(t.Context(), 1, nil)
	_, strategyError := repository.FindClosedByOwnerAndTradingStrategy(t.Context(), 1, 1)
	_, _, openError := repository.FindOpenByOwnerSymbol(t.Context(), 1, "2330")
	deleteError := repository.Delete(t.Context(), 1)
	_, countError := repository.CountByTag(t.Context(), 1)

	for _, storageError := range []error{
		createError, saveError, findError, pageError, closedError, strategyError, openError, deleteError, countError,
	} {
		require.Error(t, storageError)
		assert.NotErrorIs(t, storageError, domains.ErrSpotTradeNotFound)
	}
}

func TestKCandleRepositoryFindsPriceExtremes(t *testing.T) {
	database := newTestDatabase(t)
	repository := persistence.NewKCandleRepository(database)
	for minute, closePrice := range []string{"1050", "1060", "1040"} {
		candle := kCandleAt("2330", at(1, minute), closePrice)
		candle.High = decimal.RequireFromString(closePrice).Add(decimal.NewFromInt(5))
		candle.Low = decimal.RequireFromString(closePrice).Sub(decimal.NewFromInt(5))
		_, saveError := repository.Save(t.Context(), candle)
		require.NoError(t, saveError)
	}

	extremes, findError := repository.FindPriceExtremesInRange(t.Context(), "2330", at(1, 0), at(1, 1))
	nothing, nothingError := repository.FindPriceExtremesInRange(t.Context(), "2330", at(5, 0), at(6, 0))

	require.NoError(t, findError)
	assert.True(t, extremes.Has)
	assert.Equal(t, "1065", extremes.HighestPrice.String())
	assert.Equal(t, "1045", extremes.LowestPrice.String())
	require.NoError(t, nothingError)
	assert.False(t, nothing.Has)

	_, failure := persistence.NewKCandleRepository(closedDatabase(t)).FindPriceExtremesInRange(t.Context(), "2330", at(1, 0), at(1, 1))
	require.Error(t, failure)
}
