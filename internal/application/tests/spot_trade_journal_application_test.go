package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var spotBoughtAt = time.Date(2026, 9, 24, 1, 5, 0, 0, time.UTC)

type spotTradeJournalApplicationUnderTest struct {
	application                    *application.SpotTradeJournalApplication
	spotTradeRecordRepository      *mocks.MockISpotTradeRecordRepository
	tradeTagRepository             *mocks.MockITradeTagRepository
	tradingStrategyRepository      *mocks.MockITradingStrategyRepository
	tradingSymbolRepository        *mocks.MockITradingSymbolRepository
	kCandleRepository              *mocks.MockIKCandleRepository
	strategyBotRepository          *mocks.MockIStrategyBotRepository
	strategyBotRunRecordRepository *mocks.MockIStrategyBotRunRecordRepository
}

func newSpotTradeJournalApplicationUnderTest(t *testing.T) spotTradeJournalApplicationUnderTest {
	mockController := gomock.NewController(t)
	fixture := spotTradeJournalApplicationUnderTest{
		spotTradeRecordRepository:      mocks.NewMockISpotTradeRecordRepository(mockController),
		tradeTagRepository:             mocks.NewMockITradeTagRepository(mockController),
		tradingStrategyRepository:      mocks.NewMockITradingStrategyRepository(mockController),
		tradingSymbolRepository:        mocks.NewMockITradingSymbolRepository(mockController),
		kCandleRepository:              mocks.NewMockIKCandleRepository(mockController),
		strategyBotRepository:          mocks.NewMockIStrategyBotRepository(mockController),
		strategyBotRunRecordRepository: mocks.NewMockIStrategyBotRunRecordRepository(mockController),
	}
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(journalMoment).AnyTimes()
	fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), gomock.Len(0)).Return(nil, nil).AnyTimes()

	fixture.application = application.NewSpotTradeJournalApplication(
		service.NewSpotTradeJournalService(
			fixture.spotTradeRecordRepository, fixture.tradeTagRepository, fixture.tradingStrategyRepository,
			fixture.tradingSymbolRepository, fixture.kCandleRepository, clockProxy),
		service.NewTradeJournalLinkService(mocks.NewMockIOpaqueIdentifierProxy(mockController),
			fixture.strategyBotRunRecordRepository, fixture.strategyBotRepository, "https://console.example"))

	return fixture
}

// quietMarket answers every spot market read with nothing, so a test only arranges the reads it is about.
func (fixture spotTradeJournalApplicationUnderTest) quietMarket() {
	fixture.kCandleRepository.EXPECT().FindLatest(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	fixture.kCandleRepository.EXPECT().FindPriceExtremesInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(vo.PriceExtremesVo{}, nil).AnyTimes()
}

func (fixture spotTradeJournalApplicationUnderTest) knowsSymbol(symbol string, market string) {
	fixture.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), symbol).
		Return(entities.TradingSymbol{Symbol: symbol, Market: market}, true, nil).AnyTimes()
}

// createsWhatItIsGiven hands back the trade as stored, with identifiers filled in.
func (fixture spotTradeJournalApplicationUnderTest) createsWhatItIsGiven() {
	fixture.spotTradeRecordRepository.EXPECT().FindOpenByOwnerSymbol(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entities.SpotTradeRecord{}, false, nil).AnyTimes()
	fixture.spotTradeRecordRepository.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, record entities.SpotTradeRecord) (entities.SpotTradeRecord, error) {
			record.ID = 5
			for index := range record.Fills {
				record.Fills[index].ID = uint(index + 1)
			}
			return record, nil
		}).AnyTimes()
}

// savesWhatItIsGiven hands back the trade as saved.
func (fixture spotTradeJournalApplicationUnderTest) savesWhatItIsGiven() {
	fixture.spotTradeRecordRepository.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, record entities.SpotTradeRecord) (entities.SpotTradeRecord, error) {
			return record, nil
		}).AnyTimes()
}

func spotFillAt(id uint, kind vo.SpotTradeFillKindVo, filledAt time.Time, price string, quantity string) entities.SpotTradeFill {
	return entities.SpotTradeFill{
		ID: id, SpotTradeRecordID: 5, Kind: string(kind), FilledAt: filledAt,
		Price: decimal.RequireFromString(price), Quantity: decimal.RequireFromString(quantity),
	}
}

func aHeldTaiwanTrade() entities.SpotTradeRecord {
	return entities.SpotTradeRecord{
		ID: 5, OwnerID: journalOwnerID, Symbol: "2330", Market: "taiwanStock", Status: "open", OpenedAt: spotBoughtAt,
		Fills: []entities.SpotTradeFill{spotFillAt(1, vo.SpotTradeFillKindBuy, spotBoughtAt, "1050", "1000")},
	}
}

func aTaiwanBuy(price string, quantity string) dto.SpotTradeFillWriteDto {
	filledAt := spotBoughtAt

	return dto.SpotTradeFillWriteDto{
		FilledAt: &filledAt, Price: decimal.RequireFromString(price), Quantity: decimal.RequireFromString(quantity),
	}
}

func TestSpotTradeJournalApplicationRecordTrade(t *testing.T) {
	t.Run("a Taiwan stock buy is recorded as a holding counted in New Taiwan dollars", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		fixture.quietMarket()
		fixture.knowsSymbol("2330", "taiwanStock")
		fixture.createsWhatItIsGiven()

		trade, err := fixture.application.RecordTrade(context.Background(), journalOwnerID, dto.SpotTradeRecordWriteDto{
			Symbol: " 2330 ", FirstBuyFill: aTaiwanBuy("1050", "1000")})

		require.NoError(t, err)
		assert.Equal(t, "open", trade.Status)
		assert.Equal(t, "taiwanStock", trade.Market)
		assert.Equal(t, "TWD", trade.Currency)
		assert.Equal(t, "1000", trade.Holding.String())
		assert.Equal(t, "1050", trade.AverageBuyPrice.String())
		assert.False(t, trade.AverageSellPrice.Valid)
		assert.Equal(t, "buy", trade.Fills[0].Kind)
		assert.True(t, trade.Fills[0].Fee.IsZero())
		assert.Equal(t, "noLatestPrice", trade.Outcome.FloatingProfit.UnavailableReason)
	})

	t.Run("a crypto buy with no time is recorded now and counted in USDT", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		fixture.quietMarket()
		fixture.knowsSymbol("BTCUSDT", "crypto")
		fixture.createsWhatItIsGiven()

		trade, err := fixture.application.RecordTrade(context.Background(), journalOwnerID, dto.SpotTradeRecordWriteDto{
			Symbol: "btcusdt", FirstBuyFill: dto.SpotTradeFillWriteDto{
				Price: decimal.RequireFromString("97900"), Quantity: decimal.RequireFromString("0.05")}})

		require.NoError(t, err)
		assert.Equal(t, "USDT", trade.Currency)
		assert.True(t, trade.Fills[0].FilledAt.Equal(journalMoment))
	})

	t.Run("a symbol already held points at the trade to add to", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		fixture.knowsSymbol("2330", "taiwanStock")
		fixture.spotTradeRecordRepository.EXPECT().FindOpenByOwnerSymbol(gomock.Any(), journalOwnerID, "2330").
			Return(aHeldTaiwanTrade(), true, nil)

		_, err := fixture.application.RecordTrade(context.Background(), journalOwnerID, dto.SpotTradeRecordWriteDto{
			Symbol: "2330", FirstBuyFill: aTaiwanBuy("1050", "1000")})

		openHolding, isOpenHolding := errors.AsType[domains.SpotTradeOpenHoldingExistsError](err)
		require.True(t, isOpenHolding)
		assert.Equal(t, uint(5), openHolding.OpenTradeID)
		assert.Contains(t, err.Error(), "2330 已有持有中的 #5，請在那一筆加買進")
	})

	t.Run("an unknown symbol, a leverage and part of a share are refused", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		fixture.knowsSymbol("2330", "taiwanStock")
		fixture.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "ABCXYZ").
			Return(entities.TradingSymbol{}, false, nil)

		_, unknownError := fixture.application.RecordTrade(context.Background(), journalOwnerID, dto.SpotTradeRecordWriteDto{
			Symbol: "ABCXYZ", FirstBuyFill: aTaiwanBuy("1", "1")})
		_, leverageError := fixture.application.RecordTrade(context.Background(), journalOwnerID, dto.SpotTradeRecordWriteDto{
			Symbol: "2330", FirstBuyFill: aTaiwanBuy("1050", "1000"),
			Leverage: decimal.NewNullDecimal(decimal.NewFromInt(10))})
		_, shareError := fixture.application.RecordTrade(context.Background(), journalOwnerID, dto.SpotTradeRecordWriteDto{
			Symbol: "2330", FirstBuyFill: aTaiwanBuy("1050", "1000.5")})
		_, blankError := fixture.application.RecordTrade(context.Background(), journalOwnerID, dto.SpotTradeRecordWriteDto{
			Symbol: "  ", FirstBuyFill: aTaiwanBuy("1050", "1000")})

		require.ErrorIs(t, unknownError, domains.ErrSpotTradeValidation)
		assert.Contains(t, unknownError.Error(), "找不到這個現貨標的 ABCXYZ")
		require.ErrorIs(t, leverageError, domains.ErrSpotTradeValidation)
		assert.Contains(t, leverageError.Error(), "現貨只有先買後賣，沒有槓桿")
		require.ErrorIs(t, shareError, domains.ErrSpotTradeValidation)
		assert.Contains(t, shareError.Error(), "台股數量以股計，必須是整數")
		require.ErrorIs(t, blankError, domains.ErrSpotTradeValidation)
	})

	t.Run("only the person's own spot trading strategy can be named", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		fixture.quietMarket()
		fixture.knowsSymbol("2330", "taiwanStock")
		fixture.createsWhatItIsGiven()
		ownSpot, ownContract, strangers := uint(11), uint(12), uint(13)
		fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), ownSpot).
			Return(entities.TradingStrategy{ID: ownSpot, OwnerID: journalOwnerID, Name: "台積電波段", MarketDataKind: "kCandle"}, nil).AnyTimes()
		fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), ownContract).
			Return(entities.TradingStrategy{ID: ownContract, OwnerID: journalOwnerID, MarketDataKind: "contractKCandle"}, nil)
		fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), strangers).
			Return(entities.TradingStrategy{ID: strangers, OwnerID: journalOwnerID + 1}, nil)

		trade, ownError := fixture.application.RecordTrade(context.Background(), journalOwnerID, dto.SpotTradeRecordWriteDto{
			Symbol: "2330", FirstBuyFill: aTaiwanBuy("1050", "1000"), TradingStrategyID: &ownSpot})
		_, contractError := fixture.application.RecordTrade(context.Background(), journalOwnerID, dto.SpotTradeRecordWriteDto{
			Symbol: "2330", FirstBuyFill: aTaiwanBuy("1050", "1000"), TradingStrategyID: &ownContract})
		_, strangerError := fixture.application.RecordTrade(context.Background(), journalOwnerID, dto.SpotTradeRecordWriteDto{
			Symbol: "2330", FirstBuyFill: aTaiwanBuy("1050", "1000"), TradingStrategyID: &strangers})

		require.NoError(t, ownError)
		assert.Equal(t, "台積電波段", trade.TradingStrategyName)
		require.ErrorIs(t, contractError, domains.ErrSpotTradeValidation)
		assert.Contains(t, contractError.Error(), "只能指名 K 線（現貨）交易策略")
		require.ErrorIs(t, strangerError, domains.ErrTradingStrategyNotFound)
	})

	t.Run("a strategy that cannot be read stops the recording", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		fixture.knowsSymbol("2330", "taiwanStock")
		unreadable := uint(14)
		fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), unreadable).Return(entities.TradingStrategy{}, errStorageDown)

		_, err := fixture.application.RecordTrade(context.Background(), journalOwnerID, dto.SpotTradeRecordWriteDto{
			Symbol: "2330", FirstBuyFill: aTaiwanBuy("1050", "1000"), TradingStrategyID: &unreadable})

		require.ErrorIs(t, err, errStorageDown)
	})

	t.Run("somebody else's setup tag is not found", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		fixture.knowsSymbol("2330", "taiwanStock")
		fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), []uint{9}).
			Return([]entities.TradeTag{{ID: 9, OwnerID: journalOwnerID + 1, Kind: "setup", Name: "突破"}}, nil)

		_, err := fixture.application.RecordTrade(context.Background(), journalOwnerID, dto.SpotTradeRecordWriteDto{
			Symbol: "2330", FirstBuyFill: aTaiwanBuy("1050", "1000"), SetupTagIDs: []uint{9}})

		require.ErrorIs(t, err, domains.ErrTradeTagNotFound)
	})

	t.Run("storage failures come back as they are", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		fixture.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "0050").Return(entities.TradingSymbol{}, false, errStorageDown)
		fixture.knowsSymbol("2330", "taiwanStock")
		fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
		fixture.spotTradeRecordRepository.EXPECT().FindOpenByOwnerSymbol(gomock.Any(), journalOwnerID, "2330").
			Return(entities.SpotTradeRecord{}, false, errStorageDown).Times(1)

		_, symbolError := fixture.application.RecordTrade(context.Background(), journalOwnerID, dto.SpotTradeRecordWriteDto{
			Symbol: "0050", FirstBuyFill: aTaiwanBuy("1", "1")})
		_, openError := fixture.application.RecordTrade(context.Background(), journalOwnerID, dto.SpotTradeRecordWriteDto{
			Symbol: "2330", FirstBuyFill: aTaiwanBuy("1050", "1000")})

		require.ErrorIs(t, symbolError, errStorageDown)
		require.ErrorIs(t, openError, errStorageDown)
	})
}

func TestSpotTradeJournalApplicationChangesATrade(t *testing.T) {
	t.Run("selling everything closes the trade with its return", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		fixture.quietMarket()
		fixture.savesWhatItIsGiven()
		fixture.spotTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(5)).Return(aHeldTaiwanTrade(), nil)
		soldAt := spotBoughtAt.Add(26 * time.Hour)
		fee := decimal.NewNullDecimal(decimal.RequireFromString("2100"))

		trade, err := fixture.application.AddFill(context.Background(), journalOwnerID, 5, dto.SpotTradeFillWriteDto{
			Kind: "sell", FilledAt: &soldAt, Price: decimal.RequireFromString("1120"),
			Quantity: decimal.RequireFromString("1000"), Fee: fee})

		require.NoError(t, err)
		assert.Equal(t, "closed", trade.Status)
		assert.Equal(t, "1120", trade.AverageSellPrice.Decimal.String())
		assert.Equal(t, "67900", trade.Outcome.NetProfit.String())
		assert.InDelta(t, 0.0647, *trade.Outcome.ReturnRate, 0.00005)
		assert.True(t, trade.Plan.Locked)
		assert.Equal(t, "notOpen", trade.Outcome.FloatingProfit.UnavailableReason)
	})

	t.Run("selling more than is held is refused", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		fixture.spotTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(5)).Return(aHeldTaiwanTrade(), nil)
		soldAt := spotBoughtAt.Add(time.Hour)

		_, err := fixture.application.AddFill(context.Background(), journalOwnerID, 5, dto.SpotTradeFillWriteDto{
			Kind: "sell", FilledAt: &soldAt, Price: decimal.RequireFromString("1120"), Quantity: decimal.RequireFromString("1200")})

		require.ErrorIs(t, err, domains.ErrSpotTradeValidation)
		assert.Contains(t, err.Error(), "賣出數量超過目前持有 1000")
	})

	t.Run("a buy is corrected and a spare buy removed while held", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		fixture.quietMarket()
		fixture.savesWhatItIsGiven()
		held := aHeldTaiwanTrade()
		held.Fills = append(held.Fills, spotFillAt(2, vo.SpotTradeFillKindBuy, spotBoughtAt, "1060", "100"))
		fixture.spotTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(5)).Return(held, nil).Times(2)

		amended, amendError := fixture.application.AmendFill(context.Background(), journalOwnerID, 5, 1, dto.SpotTradeFillWriteDto{
			Kind: "buy", FilledAt: &spotBoughtAt, Price: decimal.RequireFromString("1040"), Quantity: decimal.RequireFromString("1000")})
		removed, removeError := fixture.application.RemoveFill(context.Background(), journalOwnerID, 5, 2)

		require.NoError(t, amendError)
		assert.Equal(t, "1100", amended.Holding.String())
		require.NoError(t, removeError)
		assert.Equal(t, "1000", removed.Holding.String())
	})

	t.Run("the plan, a note, the review and setup tags go through the trade", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		fixture.quietMarket()
		fixture.savesWhatItIsGiven()
		closed := aHeldTaiwanTrade()
		closedAt := spotBoughtAt.Add(time.Hour)
		closed.Fills = append(closed.Fills, spotFillAt(2, vo.SpotTradeFillKindSell, closedAt, "1100", "1000"))
		closed.Status = "closed"
		closed.ClosedAt = &closedAt
		fixture.spotTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(5)).Return(aHeldTaiwanTrade(), nil).Times(1)
		fixture.spotTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(5)).Return(closed, nil).Times(3)
		chasing := entities.TradeTag{ID: 3, OwnerID: journalOwnerID, Kind: "mistake", Name: "追價進場"}
		breakout := entities.TradeTag{ID: 4, OwnerID: journalOwnerID, Kind: "setup", Name: "突破"}
		fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), []uint{3}).Return([]entities.TradeTag{chasing}, nil)
		fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), []uint{4}).Return([]entities.TradeTag{breakout}, nil)

		planned, planError := fixture.application.AmendPlan(context.Background(), journalOwnerID, 5, dto.SpotTradePlanWriteDto{
			PlannedStopLossPrice: decimal.NewNullDecimal(decimal.RequireFromString("1000"))})
		noted, noteError := fixture.application.AddNote(context.Background(), journalOwnerID, 5, "盤後補記")
		reviewed, reviewError := fixture.application.WriteReview(context.Background(), journalOwnerID, 5, dto.SpotTradeReviewWriteDto{
			ExecutionScore: 4, MistakeTagIDs: []uint{3}})
		tagged, tagError := fixture.application.AssignSetupTags(context.Background(), journalOwnerID, 5, []uint{4})

		require.NoError(t, planError)
		assert.Equal(t, "1000", planned.Plan.PlannedStopLossPrice.Decimal.String())
		require.NoError(t, noteError)
		assert.Equal(t, "盤後補記", noted.Notes[0].Content)
		require.NoError(t, reviewError)
		assert.Equal(t, "reviewed", reviewed.Status)
		assert.Equal(t, "追價進場", reviewed.MistakeTags[0].Name)
		require.NoError(t, tagError)
		assert.Equal(t, "突破", tagged.SetupTags[0].Name)
	})

	t.Run("a closed trade's plan is locked and unreadable tags stop the change", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		closed := aHeldTaiwanTrade()
		closed.Status = "closed"
		fixture.spotTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(5)).Return(closed, nil)
		fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), gomock.Len(1)).Return(nil, errStorageDown).Times(2)

		_, planError := fixture.application.AmendPlan(context.Background(), journalOwnerID, 5, dto.SpotTradePlanWriteDto{})
		_, reviewError := fixture.application.WriteReview(context.Background(), journalOwnerID, 5, dto.SpotTradeReviewWriteDto{
			ExecutionScore: 4, MistakeTagIDs: []uint{3}})
		_, tagError := fixture.application.AssignSetupTags(context.Background(), journalOwnerID, 5, []uint{4})

		require.ErrorIs(t, planError, domains.ErrSpotTradeLocked)
		require.ErrorIs(t, reviewError, errStorageDown)
		require.ErrorIs(t, tagError, errStorageDown)
	})

	t.Run("somebody else's trade is not found for any change or delete", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		strangers := aHeldTaiwanTrade()
		strangers.OwnerID = journalOwnerID + 1
		fixture.spotTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(5)).Return(strangers, nil).AnyTimes()

		_, getError := fixture.application.GetTrade(context.Background(), journalOwnerID, 5)
		_, noteError := fixture.application.AddNote(context.Background(), journalOwnerID, 5, "偷看")
		deleteError := fixture.application.DeleteTrade(context.Background(), journalOwnerID, 5)

		for _, err := range []error{getError, noteError, deleteError} {
			require.ErrorIs(t, err, domains.ErrSpotTradeNotFound)
			assert.Contains(t, err.Error(), "找不到這筆交易")
		}
	})

	t.Run("the owner deletes a trade, and failed reads or saves come back", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		fixture.spotTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(5)).Return(aHeldTaiwanTrade(), nil).Times(2)
		fixture.spotTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(6)).Return(entities.SpotTradeRecord{}, errStorageDown).Times(2)
		fixture.spotTradeRecordRepository.EXPECT().Delete(gomock.Any(), uint(5)).Return(nil)
		fixture.spotTradeRecordRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Return(entities.SpotTradeRecord{}, errStorageDown)

		require.NoError(t, fixture.application.DeleteTrade(context.Background(), journalOwnerID, 5))
		_, saveError := fixture.application.AddNote(context.Background(), journalOwnerID, 5, "盤後補記")
		_, readError := fixture.application.AddNote(context.Background(), journalOwnerID, 6, "盤後補記")
		deleteReadError := fixture.application.DeleteTrade(context.Background(), journalOwnerID, 6)

		require.ErrorIs(t, saveError, errStorageDown)
		require.ErrorIs(t, readError, errStorageDown)
		require.ErrorIs(t, deleteReadError, errStorageDown)
	})
}

func TestSpotTradeJournalApplicationReadsTrades(t *testing.T) {
	t.Run("the detail marks a held trade at the latest price and reads the swing while held", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		held := aHeldTaiwanTrade()
		held.PlannedStopLossPrice = decimal.NewNullDecimal(decimal.RequireFromString("1000"))
		followed := uint(11)
		held.TradingStrategyID = &followed
		fixture.spotTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(5)).Return(held, nil)
		fixture.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "2330", 1).
			Return([]entities.KCandle{{Symbol: "2330", Close: decimal.RequireFromString("1080")}}, nil)
		fixture.kCandleRepository.EXPECT().FindPriceExtremesInRange(gomock.Any(), "2330", spotBoughtAt, journalMoment).
			Return(vo.PriceExtremesVo{Has: true, LowestPrice: decimal.RequireFromString("1020"),
				HighestPrice: decimal.RequireFromString("1090")}, nil)
		fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), followed).
			Return(entities.TradingStrategy{}, domains.TradingStrategyNotFound(followed))

		trade, err := fixture.application.GetTrade(context.Background(), journalOwnerID, 5)

		require.NoError(t, err)
		assert.Equal(t, "30000", trade.Outcome.FloatingProfit.Amount.String())
		assert.Equal(t, "1020", trade.Outcome.Excursion.AdversePrice.String())
		assert.InDelta(t, -0.6, *trade.Outcome.Excursion.AdverseRMultiple, 0.0001)
		assert.True(t, trade.TradingStrategyDeleted)
	})

	t.Run("an unreadable market only leaves its own figures unavailable", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		fixture.spotTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(5)).Return(aHeldTaiwanTrade(), nil)
		fixture.kCandleRepository.EXPECT().FindLatest(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errStorageDown)
		fixture.kCandleRepository.EXPECT().FindPriceExtremesInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(vo.PriceExtremesVo{}, errStorageDown)

		trade, err := fixture.application.GetTrade(context.Background(), journalOwnerID, 5)

		require.NoError(t, err)
		assert.Equal(t, "noLatestPrice", trade.Outcome.FloatingProfit.UnavailableReason)
		assert.Equal(t, "noMarketData", trade.Outcome.Excursion.UnavailableReason)
	})

	t.Run("the list narrows as asked and names the strategies it can read", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		fixture.quietMarket()
		followed, deleted := uint(11), uint(12)
		first := aHeldTaiwanTrade()
		first.TradingStrategyID = &followed
		second := aHeldTaiwanTrade()
		second.ID = 6
		second.TradingStrategyID = &deleted
		openedSince := journalMoment.AddDate(0, 0, -7)
		fixture.spotTradeRecordRepository.EXPECT().FindPageByOwner(gomock.Any(), journalOwnerID, vo.TradeListFilterVo{
			Status: "open", Symbol: "2330", Market: "taiwanStock", OpenedSince: &openedSince, Limit: 200,
		}).Return([]entities.SpotTradeRecord{first, second}, int64(2), nil)
		fixture.tradingStrategyRepository.EXPECT().FindAllByOwner(gomock.Any(), journalOwnerID).
			Return([]entities.TradingStrategy{{ID: followed, Name: "台積電波段"}}, nil)

		page, err := fixture.application.ListTrades(context.Background(), journalOwnerID, dto.SpotTradeListQueryDto{
			Status: "open", Symbol: "2330", Market: "taiwanStock", Period: "7d", Limit: 500})

		require.NoError(t, err)
		assert.Equal(t, int64(2), page.TotalCount)
		assert.Equal(t, "台積電波段", page.Trades[0].TradingStrategyName)
		assert.True(t, page.Trades[1].TradingStrategyDeleted)
		assert.Equal(t, "notComputed", page.Trades[0].Outcome.Excursion.UnavailableReason)
	})

	t.Run("a list without limits takes twenty and unreadable strategy names orphan nothing", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		fixture.quietMarket()
		followed := uint(11)
		trade := aHeldTaiwanTrade()
		trade.TradingStrategyID = &followed
		fixture.spotTradeRecordRepository.EXPECT().FindPageByOwner(gomock.Any(), journalOwnerID, vo.TradeListFilterVo{Limit: 20}).
			Return([]entities.SpotTradeRecord{trade}, int64(1), nil)
		fixture.tradingStrategyRepository.EXPECT().FindAllByOwner(gomock.Any(), journalOwnerID).Return(nil, errStorageDown)

		page, err := fixture.application.ListTrades(context.Background(), journalOwnerID, dto.SpotTradeListQueryDto{})

		require.NoError(t, err)
		assert.False(t, page.Trades[0].TradingStrategyDeleted)
	})

	t.Run("an unknown status, market, symbol or period is refused, and a failed read comes back", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		fixture.spotTradeRecordRepository.EXPECT().FindPageByOwner(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, int64(0), errStorageDown)

		_, statusError := fixture.application.ListTrades(context.Background(), journalOwnerID, dto.SpotTradeListQueryDto{Status: "planned"})
		_, marketError := fixture.application.ListTrades(context.Background(), journalOwnerID, dto.SpotTradeListQueryDto{Market: "futures"})
		_, symbolError := fixture.application.ListTrades(context.Background(), journalOwnerID, dto.SpotTradeListQueryDto{Symbol: "BTC\x00USDT"})
		_, periodError := fixture.application.ListTrades(context.Background(), journalOwnerID, dto.SpotTradeListQueryDto{Period: "1y"})
		_, readError := fixture.application.ListTrades(context.Background(), journalOwnerID, dto.SpotTradeListQueryDto{})

		require.ErrorIs(t, statusError, domains.ErrSpotTradeValidation)
		assert.Contains(t, statusError.Error(), "狀態只有 open、closed 與 reviewed")
		require.ErrorIs(t, marketError, domains.ErrSpotTradeValidation)
		assert.Contains(t, marketError.Error(), "市場只有 crypto 與 taiwanStock")
		require.ErrorIs(t, symbolError, domains.ErrSpotTradeValidation)
		require.ErrorIs(t, periodError, domains.ErrSpotTradeValidation)
		assert.Contains(t, periodError.Error(), "期間只有 7d、30d、90d 與 all")
		require.ErrorIs(t, readError, errStorageDown)
	})

	t.Run("statistics are grouped by market over the period asked", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		closed := aHeldTaiwanTrade()
		closedAt := spotBoughtAt.Add(time.Hour)
		closed.Fills = append(closed.Fills, spotFillAt(2, vo.SpotTradeFillKindSell, closedAt, "1100", "1000"))
		closed.Status = "closed"
		closed.ClosedAt = &closedAt
		since := journalMoment.AddDate(0, 0, -30)
		fixture.spotTradeRecordRepository.EXPECT().FindClosedByOwner(gomock.Any(), journalOwnerID, &since).
			Return([]entities.SpotTradeRecord{closed}, nil)
		fixture.spotTradeRecordRepository.EXPECT().FindClosedByOwner(gomock.Any(), journalOwnerID, gomock.Nil()).
			Return(nil, errStorageDown)

		statistics, err := fixture.application.GetStatistics(context.Background(), journalOwnerID, "")
		_, readError := fixture.application.GetStatistics(context.Background(), journalOwnerID, "all")
		_, periodError := fixture.application.GetStatistics(context.Background(), journalOwnerID, "1y")

		require.NoError(t, err)
		assert.Equal(t, "30d", statistics.Period)
		assert.Equal(t, 1, statistics.Markets[0].ClosedTradeCount)
		assert.Equal(t, "50000", statistics.Markets[0].NetProfit.String())
		assert.Equal(t, 0, statistics.Markets[1].ClosedTradeCount)
		require.ErrorIs(t, readError, errStorageDown)
		require.ErrorIs(t, periodError, domains.ErrSpotTradeValidation)
	})
}

func aSpotBot() entities.StrategyBot {
	return entities.StrategyBot{ID: 3, OwnerID: journalOwnerID, Name: "台積電波段", Symbol: "2330",
		MarketDataKind: "kCandle", TradingStrategyID: 11}
}

func aSpotRound(result string) entities.StrategyBotRunRecord {
	return entities.StrategyBotRunRecord{
		StrategyBotID: 3, RunNumber: 88, RanAt: spotBoughtAt, Result: result,
		ReferencePrice: decimal.NewNullDecimal(decimal.RequireFromString("1045")),
		SuggestedStake: decimal.NewNullDecimal(decimal.RequireFromString("105000")),
	}
}

func TestSpotTradeJournalApplicationJournalLinks(t *testing.T) {
	t.Run("a spot buy prefills a new trade and records nothing", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		fixture.knowsSymbol("2330", "taiwanStock")
		fixture.strategyBotRunRecordRepository.EXPECT().FindByJournalLinkIdentifier(gomock.Any(), "abc").
			Return(aSpotRound("buy"), true, nil)
		fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).Return(aSpotBot(), nil)
		fixture.spotTradeRecordRepository.EXPECT().FindOpenByOwnerSymbol(gomock.Any(), journalOwnerID, "2330").
			Return(entities.SpotTradeRecord{}, false, nil)

		prefill, err := fixture.application.PrepareJournalLink(context.Background(), journalOwnerID, "abc")

		require.NoError(t, err)
		assert.Equal(t, "newTrade", prefill.Mode)
		assert.Equal(t, "1045", prefill.Price.Decimal.String())
		assert.Equal(t, "100", prefill.Quantity.Decimal.String())
		assert.Equal(t, "taiwanStock", prefill.Market)
	})

	t.Run("a spot exit while holding sells what is held", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		fixture.knowsSymbol("2330", "taiwanStock")
		fixture.strategyBotRunRecordRepository.EXPECT().FindByJournalLinkIdentifier(gomock.Any(), "abc").
			Return(aSpotRound("sell"), true, nil)
		fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).Return(aSpotBot(), nil)
		fixture.spotTradeRecordRepository.EXPECT().FindOpenByOwnerSymbol(gomock.Any(), journalOwnerID, "2330").
			Return(aHeldTaiwanTrade(), true, nil)

		prefill, err := fixture.application.PrepareJournalLink(context.Background(), journalOwnerID, "abc")

		require.NoError(t, err)
		assert.Equal(t, "addSellFill", prefill.Mode)
		assert.Equal(t, uint(5), *prefill.TargetTradeID)
		assert.Equal(t, "1000", prefill.Quantity.Decimal.String())
	})

	t.Run("a contract bot's link does not open in the spot journal", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		contractBot := aSpotBot()
		contractBot.MarketDataKind = "contractKCandle"
		fixture.strategyBotRunRecordRepository.EXPECT().FindByJournalLinkIdentifier(gomock.Any(), "abc").
			Return(aSpotRound("buy"), true, nil)
		fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).Return(contractBot, nil)

		_, err := fixture.application.PrepareJournalLink(context.Background(), journalOwnerID, "abc")

		require.ErrorIs(t, err, domains.ErrJournalLinkNotFound)
		assert.Contains(t, err.Error(), "這條連結屬於合約交易日誌")
	})

	t.Run("a forgotten round, an unreadable symbol and an unreadable holding come back", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		fixture.strategyBotRunRecordRepository.EXPECT().FindByJournalLinkIdentifier(gomock.Any(), "gone").
			Return(entities.StrategyBotRunRecord{}, false, nil)
		fixture.strategyBotRunRecordRepository.EXPECT().FindByJournalLinkIdentifier(gomock.Any(), "abc").
			Return(aSpotRound("buy"), true, nil).Times(2)
		fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).Return(aSpotBot(), nil).Times(2)
		fixture.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "2330").Return(entities.TradingSymbol{}, false, errStorageDown)
		fixture.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "2330").Return(entities.TradingSymbol{Market: "taiwanStock"}, true, nil)
		fixture.spotTradeRecordRepository.EXPECT().FindOpenByOwnerSymbol(gomock.Any(), journalOwnerID, "2330").
			Return(entities.SpotTradeRecord{}, false, errStorageDown)

		_, goneError := fixture.application.PrepareJournalLink(context.Background(), journalOwnerID, "gone")
		_, symbolError := fixture.application.PrepareJournalLink(context.Background(), journalOwnerID, "abc")
		_, holdingError := fixture.application.PrepareJournalLink(context.Background(), journalOwnerID, "abc")

		require.ErrorIs(t, goneError, domains.ErrJournalLinkNotFound)
		assert.Contains(t, goneError.Error(), "這一輪的建議已不在紀錄中")
		require.ErrorIs(t, symbolError, errStorageDown)
		require.ErrorIs(t, holdingError, errStorageDown)
	})

	t.Run("saving from a spot link keeps the round's suggestion and measures the slippage", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		fixture.quietMarket()
		fixture.knowsSymbol("2330", "taiwanStock")
		fixture.createsWhatItIsGiven()
		fixture.strategyBotRunRecordRepository.EXPECT().FindByJournalLinkIdentifier(gomock.Any(), "abc").
			Return(aSpotRound("buy"), true, nil)
		fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).Return(aSpotBot(), nil)

		trade, err := fixture.application.RecordTrade(context.Background(), journalOwnerID, dto.SpotTradeRecordWriteDto{
			Symbol: "2330", FirstBuyFill: aTaiwanBuy("1050", "100"), JournalLinkIdentifier: "abc"})

		require.NoError(t, err)
		require.NotNil(t, trade.Source)
		assert.Equal(t, 88, trade.Source.RunNumber)
		assert.Equal(t, "1045", trade.Source.ReferencePrice.Decimal.String())
		require.NotNil(t, trade.Outcome.EntrySlippagePercentage)
		assert.InDelta(t, 0.4785, *trade.Outcome.EntrySlippagePercentage, 0.0001)
	})

	t.Run("saving from a contract bot's link records the spot trade without a source", func(t *testing.T) {
		fixture := newSpotTradeJournalApplicationUnderTest(t)
		fixture.quietMarket()
		fixture.knowsSymbol("2330", "taiwanStock")
		fixture.createsWhatItIsGiven()
		contractBot := aSpotBot()
		contractBot.MarketDataKind = "contractKCandle"
		fixture.strategyBotRunRecordRepository.EXPECT().FindByJournalLinkIdentifier(gomock.Any(), "abc").
			Return(aSpotRound("buy"), true, nil)
		fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).Return(contractBot, nil)

		trade, err := fixture.application.RecordTrade(context.Background(), journalOwnerID, dto.SpotTradeRecordWriteDto{
			Symbol: "2330", FirstBuyFill: aTaiwanBuy("1050", "100"), JournalLinkIdentifier: "abc"})

		require.NoError(t, err)
		assert.Nil(t, trade.Source)
	})
}

func aLinkedClosedSpotTrade(id uint, symbol string, buy string, sell string, closedDay int) entities.SpotTradeRecord {
	tradingStrategyID := uint(41)
	openedAt := backtestStart.AddDate(0, 0, closedDay-1)
	closedAt := backtestStart.AddDate(0, 0, closedDay)

	return entities.SpotTradeRecord{
		ID: id, OwnerID: backtestViewerID, Symbol: symbol, Market: "taiwanStock", Status: "closed",
		OpenedAt: openedAt, ClosedAt: &closedAt, TradingStrategyID: &tradingStrategyID,
		Fills: []entities.SpotTradeFill{
			spotFillAt(id*10+1, vo.SpotTradeFillKindBuy, openedAt, buy, "1"),
			spotFillAt(id*10+2, vo.SpotTradeFillKindSell, closedAt, sell, "1"),
		},
	}
}

func TestSpotTradeLiveComparisonApplication(t *testing.T) {
	const spotTradingStrategyID = uint(41)
	const spotScriptID = uint(42)
	spotTradingStrategy := entities.TradingStrategy{
		ID: spotTradingStrategyID, OwnerID: backtestViewerID, Name: "台股均線", MarketDataKind: "kCandle",
		SignalSources: []entities.TradingStrategySignalSource{{
			ID: 50, TradingStrategyID: spotTradingStrategyID, Label: "A", StrategyScriptID: spotScriptID, AggregationInterval: "1h",
		}},
		ConditionNodes: []entities.TradingStrategyConditionNode{
			{ID: 51, TradingStrategyID: spotTradingStrategyID, Side: "buy", SourceLabel: "A", ExpectedSignal: "buy"},
			{ID: 52, TradingStrategyID: spotTradingStrategyID, Side: "sell", SourceLabel: "A", ExpectedSignal: "sell"},
		},
	}

	newComparison := func(t *testing.T) (*application.SpotTradeLiveComparisonApplication, *mocks.MockISpotTradeRecordRepository,
		*mocks.MockITradingStrategyRepository, *mocks.MockIStrategyScriptRepository, *mocks.MockIKCandleRepository,
		*mocks.MockIIndicatorScriptProxy) {
		controller := gomock.NewController(t)
		clockProxy := mocks.NewMockIClockProxy(controller)
		clockProxy.EXPECT().Now().Return(backtestStart.Add(30 * 24 * time.Hour)).AnyTimes()
		spotTradeRecordRepository := mocks.NewMockISpotTradeRecordRepository(controller)
		tradingStrategyRepository := mocks.NewMockITradingStrategyRepository(controller)
		strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(controller)
		kCandleRepository := mocks.NewMockIKCandleRepository(controller)
		indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(controller)
		publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(controller)
		publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
			Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished).AnyTimes()

		comparisonApplication := application.NewSpotTradeLiveComparisonApplication(
			service.NewSpotTradeJournalService(spotTradeRecordRepository, mocks.NewMockITradeTagRepository(controller),
				tradingStrategyRepository, mocks.NewMockITradingSymbolRepository(controller), kCandleRepository, clockProxy),
			service.NewTradingStrategyService(tradingStrategyRepository),
			service.NewStrategyScriptService(strategyScriptRepository, publishedStrategyScriptRepository),
			service.NewBacktestService(kCandleRepository, indicatorScriptProxy, clockProxy, queryMaxResults, time.Minute))

		return comparisonApplication, spotTradeRecordRepository, tradingStrategyRepository, strategyScriptRepository,
			kCandleRepository, indicatorScriptProxy
	}

	t.Run("each symbol is replayed without costs beside its live figures", func(t *testing.T) {
		comparisonApplication, spotTradeRecordRepository, tradingStrategyRepository, strategyScriptRepository,
			kCandleRepository, indicatorScriptProxy := newComparison(t)
		spotTradeRecordRepository.EXPECT().FindClosedByOwnerAndTradingStrategy(gomock.Any(), backtestViewerID, spotTradingStrategyID).
			Return([]entities.SpotTradeRecord{
				aLinkedClosedSpotTrade(1, "2330", "100", "110", 2),
				aLinkedClosedSpotTrade(2, "2330", "100", "95", 3),
			}, nil)
		tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), spotTradingStrategyID).Return(spotTradingStrategy, nil)
		strategyScriptRepository.EXPECT().FindOne(gomock.Any(), spotScriptID).Return(entities.StrategyScript{
			ID: spotScriptID, OwnerID: backtestViewerID, Script: "the script", MarketDataKind: "kCandle"}, nil).AnyTimes()
		kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).Return([]entities.KCandle{
			{Symbol: "2330", OpenTime: backtestStart, Open: decimal.NewFromInt(100), High: decimal.NewFromInt(100),
				Low: decimal.NewFromInt(100), Close: decimal.NewFromInt(100), Volume: decimal.NewFromInt(1)},
			{Symbol: "2330", OpenTime: backtestStart.Add(time.Hour), Open: decimal.NewFromInt(110), High: decimal.NewFromInt(110),
				Low: decimal.NewFromInt(110), Close: decimal.NewFromInt(110), Volume: decimal.NewFromInt(1)},
		}, nil).AnyTimes()
		indicatorScriptProxy.EXPECT().ExecuteForEachCandle(gomock.Any(), "the script", gomock.Any(), gomock.Any(), gomock.Any()).
			Return(signalsSaying(vo.SignalBuy, vo.SignalSell), nil).AnyTimes()

		comparison, err := comparisonApplication.CompareWithBacktest(context.Background(), backtestViewerID, spotTradingStrategyID)

		require.NoError(t, err)
		assert.Equal(t, "台股均線", comparison.TradingStrategyName)
		require.Len(t, comparison.Rows, 1)
		row := comparison.Rows[0]
		assert.Equal(t, "2330", row.Symbol)
		assert.Equal(t, "taiwanStock", row.Market)
		assert.Equal(t, 2, row.Live.ClosedTradeCount)
		assert.InDelta(t, 0.5, *row.Live.WinRate, 0.0001)
		require.NotNil(t, row.Backtest, row.BacktestUnavailableReason)
		assert.Equal(t, 1, row.Backtest.ClosedTradeCount)
		assert.InDelta(t, 1.0, *row.Backtest.WinRate, 0.0001)
	})

	t.Run("a deleted strategy keeps its live figures without replaying", func(t *testing.T) {
		comparisonApplication, spotTradeRecordRepository, tradingStrategyRepository, _, _, _ := newComparison(t)
		spotTradeRecordRepository.EXPECT().FindClosedByOwnerAndTradingStrategy(gomock.Any(), backtestViewerID, spotTradingStrategyID).
			Return([]entities.SpotTradeRecord{aLinkedClosedSpotTrade(1, "2330", "100", "110", 2)}, nil)
		tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), spotTradingStrategyID).
			Return(entities.TradingStrategy{}, domains.TradingStrategyNotFound(spotTradingStrategyID))

		comparison, err := comparisonApplication.CompareWithBacktest(context.Background(), backtestViewerID, spotTradingStrategyID)

		require.NoError(t, err)
		assert.True(t, comparison.TradingStrategyDeleted)
		assert.Equal(t, "交易策略已刪除，無法重演", comparison.Rows[0].BacktestUnavailableReason)
	})

	t.Run("sources that cannot be resolved leave the row without a replay", func(t *testing.T) {
		comparisonApplication, spotTradeRecordRepository, tradingStrategyRepository, strategyScriptRepository, _, _ := newComparison(t)
		spotTradeRecordRepository.EXPECT().FindClosedByOwnerAndTradingStrategy(gomock.Any(), backtestViewerID, spotTradingStrategyID).
			Return([]entities.SpotTradeRecord{aLinkedClosedSpotTrade(1, "2330", "100", "110", 2)}, nil)
		tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), spotTradingStrategyID).Return(spotTradingStrategy, nil)
		strategyScriptRepository.EXPECT().FindOne(gomock.Any(), spotScriptID).
			Return(entities.StrategyScript{}, domains.StrategyScriptNotFound(spotScriptID))

		comparison, err := comparisonApplication.CompareWithBacktest(context.Background(), backtestViewerID, spotTradingStrategyID)

		require.NoError(t, err)
		assert.Nil(t, comparison.Rows[0].Backtest)
		assert.NotEmpty(t, comparison.Rows[0].BacktestUnavailableReason)
	})

	t.Run("a replay that fails costs only its own row, and failed reads come back", func(t *testing.T) {
		comparisonApplication, spotTradeRecordRepository, tradingStrategyRepository, strategyScriptRepository,
			kCandleRepository, _ := newComparison(t)
		spotTradeRecordRepository.EXPECT().FindClosedByOwnerAndTradingStrategy(gomock.Any(), backtestViewerID, spotTradingStrategyID).
			Return([]entities.SpotTradeRecord{aLinkedClosedSpotTrade(1, "2330", "100", "110", 2)}, nil)
		spotTradeRecordRepository.EXPECT().FindClosedByOwnerAndTradingStrategy(gomock.Any(), backtestViewerID, uint(99)).
			Return(nil, errStorageDown)
		tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), spotTradingStrategyID).Return(spotTradingStrategy, nil)
		strategyScriptRepository.EXPECT().FindOne(gomock.Any(), spotScriptID).Return(entities.StrategyScript{
			ID: spotScriptID, OwnerID: backtestViewerID, Script: "the script", MarketDataKind: "kCandle"}, nil).AnyTimes()
		kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errStorageDown).AnyTimes()

		comparison, err := comparisonApplication.CompareWithBacktest(context.Background(), backtestViewerID, spotTradingStrategyID)
		_, readError := comparisonApplication.CompareWithBacktest(context.Background(), backtestViewerID, 99)

		require.NoError(t, err)
		assert.Nil(t, comparison.Rows[0].Backtest)
		assert.NotEmpty(t, comparison.Rows[0].BacktestUnavailableReason)
		require.ErrorIs(t, readError, errStorageDown)
	})

	t.Run("a strategy that is not the person's and has no trades is not found", func(t *testing.T) {
		comparisonApplication, spotTradeRecordRepository, tradingStrategyRepository, _, _, _ := newComparison(t)
		spotTradeRecordRepository.EXPECT().FindClosedByOwnerAndTradingStrategy(gomock.Any(), backtestViewerID, spotTradingStrategyID).
			Return(nil, nil)
		tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), spotTradingStrategyID).
			Return(entities.TradingStrategy{}, domains.TradingStrategyNotFound(spotTradingStrategyID))

		_, err := comparisonApplication.CompareWithBacktest(context.Background(), backtestViewerID, spotTradingStrategyID)

		require.ErrorIs(t, err, domains.ErrTradingStrategyNotFound)
	})
}
