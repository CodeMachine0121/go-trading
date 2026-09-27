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

var journalOpenedAt = time.Date(2026, 9, 25, 14, 3, 0, 0, time.UTC)

func aJournalFill(kind vo.ContractTradeFillKindVo, filledAt time.Time, price string, quantity string) entities.ContractTradeFill {
	return entities.ContractTradeFill{
		Kind: string(kind), FilledAt: filledAt, Price: decimal.RequireFromString(price),
		Quantity: decimal.RequireFromString(quantity), Liquidity: string(vo.TradeFillLiquidityTaker),
		Fee: decimal.RequireFromString("1.47"),
	}
}

func anOpenTrade(ownerID uint, symbol string, direction string, openedAt time.Time) entities.ContractTradeRecord {
	return entities.ContractTradeRecord{
		OwnerID: ownerID, Symbol: symbol, Direction: direction, Leverage: decimal.NewFromInt(10),
		Status: string(vo.ContractTradeStatusOpen), OpenedAt: openedAt,
		Fills: []entities.ContractTradeFill{aJournalFill(vo.ContractTradeFillKindEntry, openedAt, "97905", "0.030")},
	}
}

func TestContractTradeRecordRepositoryStoresATradeWithItsChildren(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	setupTag, tagError := persistence.NewTradeTagRepository(database).Create(t.Context(),
		entities.TradeTag{OwnerID: owner.ID, Kind: string(vo.TradeTagKindSetup), Name: "突破"})
	require.NoError(t, tagError)
	repository := persistence.NewContractTradeRecordRepository(database)
	trade := anOpenTrade(owner.ID, "BTCUSDT", "long", journalOpenedAt)
	trade.Tags = []entities.TradeTag{setupTag}
	trade.PlannedStopLossPrice = decimal.NullDecimal{Decimal: decimal.RequireFromString("96380"), Valid: true}

	created, createError := repository.Create(t.Context(), trade)

	require.NoError(t, createError)
	assert.Positive(t, created.ID)
	require.Len(t, created.Fills, 1)
	assert.Equal(t, "97905", created.Fills[0].Price.String())
	require.Len(t, created.Tags, 1)
	assert.Equal(t, "突破", created.Tags[0].Name)
	assert.Equal(t, "96380", created.PlannedStopLossPrice.Decimal.String())
}

func TestContractTradeRecordRepositoryKeepsOneOpenTradePerSymbolAndDirection(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	repository := persistence.NewContractTradeRecordRepository(database)
	_, firstError := repository.Create(t.Context(), anOpenTrade(owner.ID, "BTCUSDT", "long", journalOpenedAt))
	require.NoError(t, firstError)

	_, secondLongError := repository.Create(t.Context(), anOpenTrade(owner.ID, "BTCUSDT", "long", journalOpenedAt))
	_, shortError := repository.Create(t.Context(), anOpenTrade(owner.ID, "BTCUSDT", "short", journalOpenedAt))

	require.ErrorIs(t, secondLongError, domains.ErrContractTradeOpenPositionExists)
	require.NoError(t, shortError)
}

func TestContractTradeRecordRepositorySaveKeepsSurvivingFillIdentifiers(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	repository := persistence.NewContractTradeRecordRepository(database)
	created, createError := repository.Create(t.Context(), anOpenTrade(owner.ID, "BTCUSDT", "long", journalOpenedAt))
	require.NoError(t, createError)
	firstFillID := created.Fills[0].ID

	closedAt := journalOpenedAt.Add(time.Hour)
	created.Fills = append(created.Fills, aJournalFill(vo.ContractTradeFillKindExit, closedAt, "99000", "0.030"))
	created.Status = string(vo.ContractTradeStatusClosed)
	created.ClosedAt = &closedAt
	created.Notes = append(created.Notes, entities.ContractTradeNote{Content: "打錯了", CreatedAt: closedAt})
	saved, saveError := repository.Save(t.Context(), created)
	require.NoError(t, saveError)

	require.Len(t, saved.Fills, 2)
	assert.Equal(t, firstFillID, saved.Fills[0].ID)
	assert.Equal(t, string(vo.ContractTradeStatusClosed), saved.Status)
	require.Len(t, saved.Notes, 1)

	saved.Fills = saved.Fills[:1]
	saved.Status = string(vo.ContractTradeStatusOpen)
	saved.ClosedAt = nil
	resaved, resaveError := repository.Save(t.Context(), saved)
	require.NoError(t, resaveError)
	require.Len(t, resaved.Fills, 1)
	assert.Equal(t, firstFillID, resaved.Fills[0].ID)
	assert.Len(t, resaved.Notes, 1)
	assert.Nil(t, resaved.ClosedAt)
}

func TestContractTradeRecordRepositoryListsAPersonsTradesNewestFirst(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	stranger := aDeliveryOwner(t, database, "stranger@example.com")
	repository := persistence.NewContractTradeRecordRepository(database)
	for dayOffset, symbol := range []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"} {
		_, createError := repository.Create(t.Context(),
			anOpenTrade(owner.ID, symbol, "long", journalOpenedAt.AddDate(0, 0, dayOffset)))
		require.NoError(t, createError)
	}
	_, strangerError := repository.Create(t.Context(), anOpenTrade(stranger.ID, "BTCUSDT", "long", journalOpenedAt))
	require.NoError(t, strangerError)

	trades, totalCount, listError := repository.FindPageByOwner(t.Context(), owner.ID,
		vo.TradeListFilterVo{Limit: 2})

	require.NoError(t, listError)
	assert.Equal(t, int64(3), totalCount)
	require.Len(t, trades, 2)
	assert.Equal(t, "SOLUSDT", trades[0].Symbol)
	assert.Equal(t, "ETHUSDT", trades[1].Symbol)
	require.Len(t, trades[0].Fills, 1)

	openedSince := journalOpenedAt.AddDate(0, 0, 1)
	narrowed, narrowedCount, narrowError := repository.FindPageByOwner(t.Context(), owner.ID,
		vo.TradeListFilterVo{Status: "open", Symbol: "ETHUSDT", OpenedSince: &openedSince, Limit: 20})
	require.NoError(t, narrowError)
	assert.Equal(t, int64(1), narrowedCount)
	require.Len(t, narrowed, 1)
	assert.Equal(t, "ETHUSDT", narrowed[0].Symbol)
}

func TestContractTradeRecordRepositoryFindsClosedTrades(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	repository := persistence.NewContractTradeRecordRepository(database)
	tradingStrategyID := uint(12)
	for dayOffset, status := range []vo.ContractTradeStatusVo{
		vo.ContractTradeStatusClosed, vo.ContractTradeStatusReviewed, vo.ContractTradeStatusOpen,
	} {
		trade := anOpenTrade(owner.ID, "BTCUSDT", "long", journalOpenedAt.AddDate(0, 0, dayOffset))
		trade.Status = string(status)
		trade.TradingStrategyID = &tradingStrategyID
		if status != vo.ContractTradeStatusOpen {
			closedAt := journalOpenedAt.AddDate(0, 0, dayOffset).Add(time.Hour)
			trade.ClosedAt = &closedAt
		}
		_, createError := repository.Create(t.Context(), trade)
		require.NoError(t, createError)
	}

	allClosed, allError := repository.FindClosedByOwner(t.Context(), owner.ID, nil)
	closedSince := journalOpenedAt.AddDate(0, 0, 1)
	recentClosed, recentError := repository.FindClosedByOwner(t.Context(), owner.ID, &closedSince)
	strategyClosed, strategyError := repository.FindClosedByOwnerAndTradingStrategy(t.Context(), owner.ID, tradingStrategyID)

	require.NoError(t, allError)
	require.NoError(t, recentError)
	require.NoError(t, strategyError)
	assert.Len(t, allClosed, 2)
	require.Len(t, recentClosed, 1)
	assert.Equal(t, string(vo.ContractTradeStatusReviewed), recentClosed[0].Status)
	assert.Len(t, strategyClosed, 2)
}

func TestContractTradeRecordRepositoryFindsTheOpenTradeForASymbolAndDirection(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	repository := persistence.NewContractTradeRecordRepository(database)
	created, createError := repository.Create(t.Context(), anOpenTrade(owner.ID, "BTCUSDT", "long", journalOpenedAt))
	require.NoError(t, createError)

	openTrade, hasOpenTrade, findError := repository.FindOpenByOwnerSymbolDirection(t.Context(), owner.ID, "BTCUSDT", "long")
	_, hasShort, shortError := repository.FindOpenByOwnerSymbolDirection(t.Context(), owner.ID, "BTCUSDT", "short")

	require.NoError(t, findError)
	require.NoError(t, shortError)
	assert.True(t, hasOpenTrade)
	assert.Equal(t, created.ID, openTrade.ID)
	assert.False(t, hasShort)
}

var journalDeletedAt = time.Date(2026, 9, 28, 9, 30, 0, 0, time.UTC)

func aClosedTrade(ownerID uint, tradingStrategyID uint, closedAt time.Time) entities.ContractTradeRecord {
	trade := anOpenTrade(ownerID, "BTCUSDT", "long", closedAt.Add(-time.Hour))
	trade.Status = string(vo.ContractTradeStatusClosed)
	trade.TradingStrategyID = &tradingStrategyID
	trade.ClosedAt = &closedAt

	return trade
}

func TestContractTradeRecordRepositoryKeepsADeletedTradeOnRecord(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	repository := persistence.NewContractTradeRecordRepository(database)
	trade := anOpenTrade(owner.ID, "BTCUSDT", "long", journalOpenedAt)
	trade.Notes = []entities.ContractTradeNote{{Content: "加碼太急", CreatedAt: journalOpenedAt}}
	created, createError := repository.Create(t.Context(), trade)
	require.NoError(t, createError)
	assert.False(t, created.IsDeleted)

	require.NoError(t, repository.MarkDeleted(t.Context(), created.ID, journalDeletedAt))
	secondDeleteError := repository.MarkDeleted(t.Context(), created.ID, journalDeletedAt.Add(time.Hour))

	require.ErrorIs(t, secondDeleteError, domains.ErrContractTradeNotFound)
	kept := entities.ContractTradeRecord{}
	require.NoError(t, database.Where(clause.Eq{Column: "id", Value: created.ID}).First(&kept).Error)
	assert.True(t, kept.IsDeleted)
	require.NotNil(t, kept.DeletedAt)
	assert.True(t, journalDeletedAt.Equal(*kept.DeletedAt))
	keptFills, keptNotes := int64(0), int64(0)
	require.NoError(t, database.Model(&entities.ContractTradeFill{}).
		Where(clause.Eq{Column: "contract_trade_record_id", Value: created.ID}).Count(&keptFills).Error)
	require.NoError(t, database.Model(&entities.ContractTradeNote{}).
		Where(clause.Eq{Column: "contract_trade_record_id", Value: created.ID}).Count(&keptNotes).Error)
	assert.Equal(t, int64(1), keptFills)
	assert.Equal(t, int64(1), keptNotes)
}

func TestContractTradeRecordRepositoryReadsOnlyTradesNotDeleted(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	repository := persistence.NewContractTradeRecordRepository(database)
	tradingStrategyID := uint(12)
	deletedOpen, deletedOpenError := repository.Create(t.Context(), anOpenTrade(owner.ID, "BTCUSDT", "long", journalOpenedAt))
	require.NoError(t, deletedOpenError)
	deletedClosed, deletedClosedError := repository.Create(t.Context(),
		aClosedTrade(owner.ID, tradingStrategyID, journalOpenedAt.AddDate(0, 0, 1)))
	require.NoError(t, deletedClosedError)
	keptClosed, keptClosedError := repository.Create(t.Context(),
		aClosedTrade(owner.ID, tradingStrategyID, journalOpenedAt.AddDate(0, 0, 2)))
	require.NoError(t, keptClosedError)
	require.NoError(t, repository.MarkDeleted(t.Context(), deletedOpen.ID, journalDeletedAt))
	require.NoError(t, repository.MarkDeleted(t.Context(), deletedClosed.ID, journalDeletedAt))

	_, findError := repository.FindOne(t.Context(), deletedOpen.ID)
	page, totalCount, pageError := repository.FindPageByOwner(t.Context(), owner.ID, vo.TradeListFilterVo{Limit: 20})
	openPage, openCount, openPageError := repository.FindPageByOwner(t.Context(), owner.ID,
		vo.TradeListFilterVo{Status: string(vo.ContractTradeStatusOpen), Limit: 20})
	allClosed, closedError := repository.FindClosedByOwner(t.Context(), owner.ID, nil)
	strategyClosed, strategyError := repository.FindClosedByOwnerAndTradingStrategy(t.Context(), owner.ID, tradingStrategyID)
	_, hasOpenTrade, openError := repository.FindOpenByOwnerSymbolDirection(t.Context(), owner.ID, "BTCUSDT", "long")

	require.ErrorIs(t, findError, domains.ErrContractTradeNotFound)
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

func TestContractTradeRecordRepositoryListsNothingWhenEveryTradeIsDeleted(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	repository := persistence.NewContractTradeRecordRepository(database)
	created, createError := repository.Create(t.Context(), anOpenTrade(owner.ID, "BTCUSDT", "long", journalOpenedAt))
	require.NoError(t, createError)
	require.NoError(t, repository.MarkDeleted(t.Context(), created.ID, journalDeletedAt))

	page, totalCount, pageError := repository.FindPageByOwner(t.Context(), owner.ID, vo.TradeListFilterVo{Limit: 20})

	require.NoError(t, pageError)
	assert.Empty(t, page)
	assert.Zero(t, totalCount)
}

func TestContractTradeRecordRepositoryFreesTheOpenPositionOfADeletedTrade(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	repository := persistence.NewContractTradeRecordRepository(database)
	deleted, deletedError := repository.Create(t.Context(), anOpenTrade(owner.ID, "BTCUSDT", "long", journalOpenedAt))
	require.NoError(t, deletedError)
	require.NoError(t, repository.MarkDeleted(t.Context(), deleted.ID, journalDeletedAt))

	recorded, recordError := repository.Create(t.Context(), anOpenTrade(owner.ID, "BTCUSDT", "long", journalOpenedAt))
	_, blockedError := repository.Create(t.Context(), anOpenTrade(owner.ID, "BTCUSDT", "long", journalOpenedAt))

	require.NoError(t, recordError)
	assert.NotEqual(t, deleted.ID, recorded.ID)
	require.ErrorIs(t, blockedError, domains.ErrContractTradeOpenPositionExists)
}

func TestContractTradeRecordRepositoryRefusesToSaveADeletedTrade(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	repository := persistence.NewContractTradeRecordRepository(database)
	created, createError := repository.Create(t.Context(), anOpenTrade(owner.ID, "BTCUSDT", "long", journalOpenedAt))
	require.NoError(t, createError)
	require.NoError(t, repository.MarkDeleted(t.Context(), created.ID, journalDeletedAt))

	amended := created
	amended.Notes = append(amended.Notes, entities.ContractTradeNote{Content: "事後補記", CreatedAt: journalDeletedAt})
	_, saveError := repository.Save(t.Context(), amended)

	require.ErrorIs(t, saveError, domains.ErrContractTradeNotFound)
	keptNotes := int64(-1)
	require.NoError(t, database.Model(&entities.ContractTradeNote{}).
		Where(clause.Eq{Column: "contract_trade_record_id", Value: created.ID}).Count(&keptNotes).Error)
	assert.Zero(t, keptNotes)
}

func TestContractTradeRecordRepositoryCountsOnlyTradesNotDeletedCarryingATag(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	mistakeTag, tagError := persistence.NewTradeTagRepository(database).Create(t.Context(),
		entities.TradeTag{OwnerID: owner.ID, Kind: string(vo.TradeTagKindMistake), Name: "追價進場"})
	require.NoError(t, tagError)
	repository := persistence.NewContractTradeRecordRepository(database)
	deletedTrade := anOpenTrade(owner.ID, "BTCUSDT", "long", journalOpenedAt)
	deletedTrade.Tags = []entities.TradeTag{mistakeTag}
	deleted, deletedError := repository.Create(t.Context(), deletedTrade)
	require.NoError(t, deletedError)
	untaggedCount, untaggedError := repository.CountByTag(t.Context(), mistakeTag.ID+1)
	require.NoError(t, repository.MarkDeleted(t.Context(), deleted.ID, journalDeletedAt))
	countWithOnlyDeleted, onlyDeletedError := repository.CountByTag(t.Context(), mistakeTag.ID)
	keptTrade := anOpenTrade(owner.ID, "ETHUSDT", "long", journalOpenedAt)
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

func TestContractTradeRecordRepositoryReportsStorageFailures(t *testing.T) {
	database := closedDatabase(t)
	repository := persistence.NewContractTradeRecordRepository(database)

	_, createError := repository.Create(t.Context(), anOpenTrade(1, "BTCUSDT", "long", journalOpenedAt))
	_, saveError := repository.Save(t.Context(), anOpenTrade(1, "BTCUSDT", "long", journalOpenedAt))
	_, findError := repository.FindOne(t.Context(), 1)
	_, _, pageError := repository.FindPageByOwner(t.Context(), 1, vo.TradeListFilterVo{Limit: 1})
	_, closedError := repository.FindClosedByOwner(t.Context(), 1, nil)
	_, strategyError := repository.FindClosedByOwnerAndTradingStrategy(t.Context(), 1, 1)
	_, _, openError := repository.FindOpenByOwnerSymbolDirection(t.Context(), 1, "BTCUSDT", "long")
	deleteError := repository.MarkDeleted(t.Context(), 1, journalDeletedAt)
	_, countError := repository.CountByTag(t.Context(), 1)

	for _, storageError := range []error{
		createError, saveError, findError, pageError, closedError, strategyError, openError, deleteError, countError,
	} {
		require.Error(t, storageError)
		assert.NotErrorIs(t, storageError, domains.ErrContractTradeNotFound)
	}
}

func TestTradeTagRepository(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	repository := persistence.NewTradeTagRepository(database)

	breakout, createError := repository.Create(t.Context(),
		entities.TradeTag{OwnerID: owner.ID, Kind: string(vo.TradeTagKindSetup), Name: "突破"})
	require.NoError(t, createError)

	_, duplicateError := repository.Create(t.Context(),
		entities.TradeTag{OwnerID: owner.ID, Kind: string(vo.TradeTagKindSetup), Name: "突破"})
	require.ErrorIs(t, duplicateError, domains.ErrTradeTagNameConflict)
	assert.Contains(t, duplicateError.Error(), "已有同名的標籤「突破」")

	_, otherKindError := repository.Create(t.Context(),
		entities.TradeTag{OwnerID: owner.ID, Kind: string(vo.TradeTagKindMistake), Name: "突破"})
	require.NoError(t, otherKindError)

	require.NoError(t, repository.CreateIfAbsent(t.Context(), []entities.TradeTag{
		{OwnerID: owner.ID, Kind: string(vo.TradeTagKindMistake), Name: "突破"},
		{OwnerID: owner.ID, Kind: string(vo.TradeTagKindMistake), Name: "追價進場"},
	}))
	require.NoError(t, repository.CreateIfAbsent(t.Context(), nil))

	tags, listError := repository.FindAllByOwner(t.Context(), owner.ID)
	require.NoError(t, listError)
	assert.Len(t, tags, 3)
	assert.Equal(t, string(vo.TradeTagKindMistake), tags[0].Kind)

	renamed, renameError := repository.Rename(t.Context(), breakout.ID, "回踩")
	require.NoError(t, renameError)
	assert.Equal(t, "回踩", renamed.Name)

	_, renameConflictError := repository.Rename(t.Context(), tags[0].ID, "追價進場")
	require.ErrorIs(t, renameConflictError, domains.ErrTradeTagNameConflict)

	found, findError := repository.FindByIDs(t.Context(), []uint{breakout.ID})
	require.NoError(t, findError)
	require.Len(t, found, 1)
	none, noneError := repository.FindByIDs(t.Context(), nil)
	require.NoError(t, noneError)
	assert.Empty(t, none)

	require.NoError(t, repository.Delete(t.Context(), breakout.ID))
	_, goneError := repository.FindOne(t.Context(), breakout.ID)
	require.ErrorIs(t, goneError, domains.ErrTradeTagNotFound)
}

func TestTradeTagRepositoryReportsStorageFailures(t *testing.T) {
	repository := persistence.NewTradeTagRepository(closedDatabase(t))

	_, listError := repository.FindAllByOwner(t.Context(), 1)
	_, findError := repository.FindOne(t.Context(), 1)
	_, findManyError := repository.FindByIDs(t.Context(), []uint{1})
	_, createError := repository.Create(t.Context(), entities.TradeTag{OwnerID: 1, Kind: "setup", Name: "突破"})
	seedError := repository.CreateIfAbsent(t.Context(), []entities.TradeTag{{OwnerID: 1, Kind: "setup", Name: "突破"}})
	_, renameError := repository.Rename(t.Context(), 1, "回踩")
	deleteError := repository.Delete(t.Context(), 1)

	for _, storageError := range []error{listError, findError, findManyError, createError, seedError, renameError, deleteError} {
		require.Error(t, storageError)
		assert.NotErrorIs(t, storageError, domains.ErrTradeTagNotFound)
		assert.NotErrorIs(t, storageError, domains.ErrTradeTagNameConflict)
	}
}

func TestTradeJournalSettingRepository(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	repository := persistence.NewTradeJournalSettingRepository(database)

	_, hasSetting, emptyError := repository.FindOneByUser(t.Context(), owner.ID)
	require.NoError(t, emptyError)
	assert.False(t, hasSetting)

	seededAt := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	require.NoError(t, repository.MarkDefaultMistakeTagsSeeded(t.Context(), owner.ID, seededAt))

	saved, saveError := repository.SaveFeeRates(t.Context(), entities.TradeJournalSetting{
		UserID:       owner.ID,
		MakerFeeRate: decimal.NullDecimal{Decimal: decimal.RequireFromString("0.02"), Valid: true},
		TakerFeeRate: decimal.NullDecimal{Decimal: decimal.RequireFromString("0.05"), Valid: true},
	})
	require.NoError(t, saveError)
	assert.Equal(t, "0.05", saved.TakerFeeRate.Decimal.String())
	require.NotNil(t, saved.DefaultMistakeTagsSeededAt)
	assert.True(t, saved.DefaultMistakeTagsSeededAt.Equal(seededAt))

	stored, hasStored, findError := repository.FindOneByUser(t.Context(), owner.ID)
	require.NoError(t, findError)
	assert.True(t, hasStored)
	assert.Equal(t, "0.02", stored.MakerFeeRate.Decimal.String())
}

func TestTradeJournalSettingRepositoryReportsStorageFailures(t *testing.T) {
	repository := persistence.NewTradeJournalSettingRepository(closedDatabase(t))

	_, _, findError := repository.FindOneByUser(t.Context(), 1)
	_, saveError := repository.SaveFeeRates(t.Context(), entities.TradeJournalSetting{UserID: 1})
	markError := repository.MarkDefaultMistakeTagsSeeded(t.Context(), 1, time.Now())

	require.Error(t, findError)
	require.Error(t, saveError)
	require.Error(t, markError)
}

func TestKCandleContractRepositoryFindsPriceExtremes(t *testing.T) {
	database := newTestDatabase(t)
	repository := persistence.NewKCandleContractRepository(database)
	first := contractCandleAt("BTCUSDT", journalOpenedAt, "97950")
	first.High = decimal.RequireFromString("98010")
	first.Low = decimal.RequireFromString("97110")
	second := contractCandleAt("BTCUSDT", journalOpenedAt.Add(time.Minute), "100500")
	second.High = decimal.RequireFromString("100960")
	second.Low = decimal.RequireFromString("100100")
	outside := contractCandleAt("BTCUSDT", journalOpenedAt.Add(time.Hour), "200000")
	outside.High = decimal.RequireFromString("200000")
	_, storeError := repository.SaveAllIfAbsent(t.Context(), []entities.KCandleContract{first, second, outside})
	require.NoError(t, storeError)

	extremes, findError := repository.FindPriceExtremesInRange(
		t.Context(), "BTCUSDT", journalOpenedAt, journalOpenedAt.Add(time.Minute))
	nothing, nothingError := repository.FindPriceExtremesInRange(
		t.Context(), "ETHUSDT", journalOpenedAt, journalOpenedAt.Add(time.Minute))

	require.NoError(t, findError)
	require.NoError(t, nothingError)
	assert.True(t, extremes.Has)
	assert.Equal(t, "100960", extremes.HighestPrice.String())
	assert.Equal(t, "97110", extremes.LowestPrice.String())
	assert.False(t, nothing.Has)

	_, failure := persistence.NewKCandleContractRepository(closedDatabase(t)).FindPriceExtremesInRange(
		t.Context(), "BTCUSDT", journalOpenedAt, journalOpenedAt)
	require.Error(t, failure)
}

func TestContractTradeRecordRepositoryWritesNothingWhenAChildIsRefused(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	repository := persistence.NewContractTradeRecordRepository(database)

	unstorableFill := anOpenTrade(owner.ID, "BTCUSDT", "long", journalOpenedAt)
	unstorableFill.Fills[0].Kind = "a-kind-too-long-to-store"
	_, createError := repository.Create(t.Context(), unstorableFill)

	created, storedError := repository.Create(t.Context(), anOpenTrade(owner.ID, "ETHUSDT", "long", journalOpenedAt))
	require.NoError(t, storedError)
	created.Notes = []entities.ContractTradeNote{{Content: "nul\x00byte", CreatedAt: journalOpenedAt}}
	_, saveError := repository.Save(t.Context(), created)

	trades, totalCount, listError := repository.FindPageByOwner(t.Context(), owner.ID, vo.TradeListFilterVo{Limit: 20})
	require.Error(t, createError)
	require.Error(t, saveError)
	require.NoError(t, listError)
	assert.Equal(t, int64(1), totalCount)
	assert.Empty(t, trades[0].Notes)
}

func TestSchemaMigratorStopsDeletedContractTradesHoldingTheOpenPosition(t *testing.T) {
	// A database synced before deletion kept trades still carries the index that counted them as open.
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	require.NoError(t, database.Exec(`DROP INDEX IF EXISTS ?`,
		clause.Column{Name: persistence.ContractTradeOneOpenPerSymbolDirectionIndex}).Error)
	require.NoError(t, database.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS "idx_contract_trade_records_one_open_per_symbol_direction"
		ON "ContractTradeRecords" (owner_id, symbol, direction) WHERE status = 'open'`).Error)

	_, migrateError := persistence.NewSchemaMigrator(database).Migrate()

	require.NoError(t, migrateError)
	repository := persistence.NewContractTradeRecordRepository(database)
	deleted, deletedError := repository.Create(t.Context(), anOpenTrade(owner.ID, "BTCUSDT", "long", journalOpenedAt))
	require.NoError(t, deletedError)
	require.NoError(t, repository.MarkDeleted(t.Context(), deleted.ID, journalDeletedAt))
	_, recordError := repository.Create(t.Context(), anOpenTrade(owner.ID, "BTCUSDT", "long", journalOpenedAt))
	require.NoError(t, recordError)
}

func TestSchemaMigratorTreatsTradesRecordedBeforeDeletionWasKeptAsNotDeleted(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	migrator := database.Migrator()
	for _, table := range []any{&entities.ContractTradeRecord{}, &entities.SpotTradeRecord{}} {
		require.NoError(t, migrator.DropColumn(table, "IsDeleted"))
		require.NoError(t, migrator.DropColumn(table, "DeletedAt"))
	}
	earlierContractTrade := anOpenTrade(owner.ID, "BTCUSDT", "long", journalOpenedAt)
	earlierContractTrade.Fills = nil
	require.NoError(t, database.Omit("IsDeleted", "DeletedAt", clause.Associations).Create(&earlierContractTrade).Error)
	earlierSpotTrade := anOpenSpotTrade(owner.ID, "2330", "taiwanStock", journalOpenedAt)
	earlierSpotTrade.Fills = nil
	require.NoError(t, database.Omit("IsDeleted", "DeletedAt", clause.Associations).Create(&earlierSpotTrade).Error)

	_, migrateError := persistence.NewSchemaMigrator(database).Migrate()

	require.NoError(t, migrateError)
	contractPage, contractCount, contractError := persistence.NewContractTradeRecordRepository(database).
		FindPageByOwner(t.Context(), owner.ID, vo.TradeListFilterVo{Limit: 20})
	spotPage, spotCount, spotError := persistence.NewSpotTradeRecordRepository(database).
		FindPageByOwner(t.Context(), owner.ID, vo.TradeListFilterVo{Limit: 20})
	require.NoError(t, contractError)
	require.NoError(t, spotError)
	assert.Equal(t, int64(1), contractCount)
	require.Len(t, contractPage, 1)
	assert.Equal(t, earlierContractTrade.ID, contractPage[0].ID)
	assert.Equal(t, int64(1), spotCount)
	require.Len(t, spotPage, 1)
	assert.Equal(t, earlierSpotTrade.ID, spotPage[0].ID)
}
