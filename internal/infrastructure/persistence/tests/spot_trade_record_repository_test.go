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
	"gorm.io/gorm/clause"
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
	assert.NotEmpty(t, openTrade.Fills, "the open holding carries its fills so a sell link can offer the whole holding")
	_, hasClosedAsOpen, _ := repository.FindOpenByOwnerSymbol(t.Context(), owner.ID, "2330")
	assert.False(t, hasClosedAsOpen)

	_, missingError := repository.FindOne(t.Context(), 99999)
	require.ErrorIs(t, missingError, domains.ErrSpotTradeNotFound)
}

func aClosedSpotTrade(ownerID uint, tradingStrategyID uint, closedAt time.Time) entities.SpotTradeRecord {
	trade := anOpenSpotTrade(ownerID, "2330", "taiwanStock", closedAt.Add(-time.Hour))
	trade.Status = string(vo.SpotTradeStatusClosed)
	trade.TradingStrategyID = &tradingStrategyID
	trade.ClosedAt = &closedAt

	return trade
}

func TestSpotTradeRecordRepositoryKeepsADeletedTradeOnRecord(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	repository := persistence.NewSpotTradeRecordRepository(database)
	trade := anOpenSpotTrade(owner.ID, "2330", "taiwanStock", journalOpenedAt)
	trade.Notes = []entities.SpotTradeNote{{Content: "盤前掛單", CreatedAt: journalOpenedAt}}
	created, createError := repository.Create(t.Context(), trade)
	require.NoError(t, createError)
	assert.False(t, created.IsDeleted)

	require.NoError(t, repository.MarkDeleted(t.Context(), created.ID, journalDeletedAt))
	secondDeleteError := repository.MarkDeleted(t.Context(), created.ID, journalDeletedAt.Add(time.Hour))

	require.ErrorIs(t, secondDeleteError, domains.ErrSpotTradeNotFound)
	kept := entities.SpotTradeRecord{}
	require.NoError(t, database.Where(clause.Eq{Column: "id", Value: created.ID}).First(&kept).Error)
	assert.True(t, kept.IsDeleted)
	require.NotNil(t, kept.DeletedAt)
	assert.True(t, journalDeletedAt.Equal(*kept.DeletedAt))
	keptFills, keptNotes := int64(0), int64(0)
	require.NoError(t, database.Model(&entities.SpotTradeFill{}).
		Where(clause.Eq{Column: "spot_trade_record_id", Value: created.ID}).Count(&keptFills).Error)
	require.NoError(t, database.Model(&entities.SpotTradeNote{}).
		Where(clause.Eq{Column: "spot_trade_record_id", Value: created.ID}).Count(&keptNotes).Error)
	assert.Equal(t, int64(1), keptFills)
	assert.Equal(t, int64(1), keptNotes)
}

func TestSpotTradeRecordRepositoryReadsOnlyTradesNotDeleted(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	repository := persistence.NewSpotTradeRecordRepository(database)
	tradingStrategyID := uint(12)
	deletedOpen, deletedOpenError := repository.Create(t.Context(), anOpenSpotTrade(owner.ID, "2330", "taiwanStock", journalOpenedAt))
	require.NoError(t, deletedOpenError)
	deletedClosed, deletedClosedError := repository.Create(t.Context(),
		aClosedSpotTrade(owner.ID, tradingStrategyID, journalOpenedAt.AddDate(0, 0, 1)))
	require.NoError(t, deletedClosedError)
	keptClosed, keptClosedError := repository.Create(t.Context(),
		aClosedSpotTrade(owner.ID, tradingStrategyID, journalOpenedAt.AddDate(0, 0, 2)))
	require.NoError(t, keptClosedError)
	require.NoError(t, repository.MarkDeleted(t.Context(), deletedOpen.ID, journalDeletedAt))
	require.NoError(t, repository.MarkDeleted(t.Context(), deletedClosed.ID, journalDeletedAt))

	_, findError := repository.FindOne(t.Context(), deletedOpen.ID)
	page, totalCount, pageError := repository.FindPageByOwner(t.Context(), owner.ID, vo.TradeListFilterVo{Limit: 20})
	openPage, openCount, openPageError := repository.FindPageByOwner(t.Context(), owner.ID,
		vo.TradeListFilterVo{Status: string(vo.SpotTradeStatusOpen), Limit: 20})
	allClosed, closedError := repository.FindClosedByOwner(t.Context(), owner.ID, nil)
	strategyClosed, strategyError := repository.FindClosedByOwnerAndTradingStrategy(t.Context(), owner.ID, tradingStrategyID)
	_, hasOpenTrade, openError := repository.FindOpenByOwnerSymbol(t.Context(), owner.ID, "2330")

	require.ErrorIs(t, findError, domains.ErrSpotTradeNotFound)
	require.NoError(t, pageError)
	require.Len(t, page, 1)
	assert.Equal(t, keptClosed.ID, page[0].ID)
	assert.Equal(t, int64(1), totalCount)
	require.NoError(t, openPageError)
	assert.Empty(t, openPage)
	assert.Zero(t, openCount)
	require.NoError(t, closedError)
	require.Len(t, allClosed, 1)
	assert.Equal(t, keptClosed.ID, allClosed[0].ID)
	require.NoError(t, strategyError)
	require.Len(t, strategyClosed, 1)
	assert.Equal(t, keptClosed.ID, strategyClosed[0].ID)
	require.NoError(t, openError)
	assert.False(t, hasOpenTrade)
}

func TestSpotTradeRecordRepositoryFreesTheHoldingOfADeletedTrade(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	repository := persistence.NewSpotTradeRecordRepository(database)
	deleted, deletedError := repository.Create(t.Context(), anOpenSpotTrade(owner.ID, "2330", "taiwanStock", journalOpenedAt))
	require.NoError(t, deletedError)
	require.NoError(t, repository.MarkDeleted(t.Context(), deleted.ID, journalDeletedAt))

	recorded, recordError := repository.Create(t.Context(), anOpenSpotTrade(owner.ID, "2330", "taiwanStock", journalOpenedAt))
	_, blockedError := repository.Create(t.Context(), anOpenSpotTrade(owner.ID, "2330", "taiwanStock", journalOpenedAt))

	require.NoError(t, recordError)
	assert.NotEqual(t, deleted.ID, recorded.ID)
	require.ErrorIs(t, blockedError, domains.ErrSpotTradeOpenHoldingExists)
}

func TestSpotTradeRecordRepositoryRefusesToSaveADeletedTrade(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	repository := persistence.NewSpotTradeRecordRepository(database)
	created, createError := repository.Create(t.Context(), anOpenSpotTrade(owner.ID, "2330", "taiwanStock", journalOpenedAt))
	require.NoError(t, createError)
	require.NoError(t, repository.MarkDeleted(t.Context(), created.ID, journalDeletedAt))

	amended := created
	amended.Notes = append(amended.Notes, entities.SpotTradeNote{Content: "事後補記", CreatedAt: journalDeletedAt})
	_, saveError := repository.Save(t.Context(), amended)

	require.ErrorIs(t, saveError, domains.ErrSpotTradeNotFound)
	keptNotes := int64(-1)
	require.NoError(t, database.Model(&entities.SpotTradeNote{}).
		Where(clause.Eq{Column: "spot_trade_record_id", Value: created.ID}).Count(&keptNotes).Error)
	assert.Zero(t, keptNotes)
}

func TestSpotTradeRecordRepositoryCountsOnlyTradesNotDeletedCarryingATag(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	mistakeTag, tagError := persistence.NewTradeTagRepository(database).Create(t.Context(),
		entities.TradeTag{OwnerID: owner.ID, Kind: string(vo.TradeTagKindMistake), Name: "追價進場"})
	require.NoError(t, tagError)
	repository := persistence.NewSpotTradeRecordRepository(database)
	deletedTrade := anOpenSpotTrade(owner.ID, "2330", "taiwanStock", journalOpenedAt)
	deletedTrade.Tags = []entities.TradeTag{mistakeTag}
	deleted, deletedError := repository.Create(t.Context(), deletedTrade)
	require.NoError(t, deletedError)
	untaggedCount, untaggedError := repository.CountByTag(t.Context(), mistakeTag.ID+1)
	require.NoError(t, repository.MarkDeleted(t.Context(), deleted.ID, journalDeletedAt))
	countWithOnlyDeleted, onlyDeletedError := repository.CountByTag(t.Context(), mistakeTag.ID)
	keptTrade := anOpenSpotTrade(owner.ID, "2317", "taiwanStock", journalOpenedAt)
	keptTrade.Tags = []entities.TradeTag{mistakeTag}
	_, keptError := repository.Create(t.Context(), keptTrade)
	require.NoError(t, keptError)

	countWithOneKept, oneKeptError := repository.CountByTag(t.Context(), mistakeTag.ID)

	require.NoError(t, untaggedError)
	assert.Zero(t, untaggedCount)
	require.NoError(t, onlyDeletedError)
	assert.Zero(t, countWithOnlyDeleted)
	require.NoError(t, oneKeptError)
	assert.Equal(t, int64(1), countWithOneKept)
}

func TestSchemaMigratorStopsDeletedSpotTradesHoldingTheOpenHolding(t *testing.T) {
	// A database synced before deletion kept trades still carries the index that counted them as held.
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	require.NoError(t, database.Exec(`DROP INDEX IF EXISTS ?`,
		clause.Column{Name: persistence.SpotTradeOneOpenPerSymbolIndex}).Error)
	require.NoError(t, database.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS "idx_spot_trade_records_one_open_per_symbol"
		ON "SpotTradeRecords" (owner_id, symbol) WHERE status = 'open'`).Error)

	_, migrateError := persistence.NewSchemaMigrator(database).Migrate()

	require.NoError(t, migrateError)
	repository := persistence.NewSpotTradeRecordRepository(database)
	deleted, deletedError := repository.Create(t.Context(), anOpenSpotTrade(owner.ID, "2330", "taiwanStock", journalOpenedAt))
	require.NoError(t, deletedError)
	require.NoError(t, repository.MarkDeleted(t.Context(), deleted.ID, journalDeletedAt))
	_, recordError := repository.Create(t.Context(), anOpenSpotTrade(owner.ID, "2330", "taiwanStock", journalOpenedAt))
	require.NoError(t, recordError)
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
	deleteError := repository.MarkDeleted(t.Context(), 1, journalDeletedAt)
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
