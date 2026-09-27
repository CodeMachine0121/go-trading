package application_test

import (
	"context"
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

var journalEntryAt = time.Date(2026, 9, 25, 14, 3, 0, 0, time.UTC)

type contractTradeJournalApplicationUnderTest struct {
	application                             *application.ContractTradeJournalApplication
	journalService                          *service.ContractTradeJournalService
	contractTradeRecordRepository           *mocks.MockIContractTradeRecordRepository
	tradeTagRepository                      *mocks.MockITradeTagRepository
	tradeJournalSettingRepository           *mocks.MockITradeJournalSettingRepository
	tradingStrategyRepository               *mocks.MockITradingStrategyRepository
	contractTradingSymbolRepository         *mocks.MockIContractTradingSymbolRepository
	contractMaintenanceMarginTierRepository *mocks.MockIContractMaintenanceMarginTierRepository
	contractFundingRateSettlementRepository *mocks.MockIContractFundingRateSettlementRepository
	kCandleContractRepository               *mocks.MockIKCandleContractRepository
	strategyBotRepository                   *mocks.MockIStrategyBotRepository
	strategyBotRunRecordRepository          *mocks.MockIStrategyBotRunRecordRepository
}

func newContractTradeJournalApplicationUnderTest(t *testing.T) contractTradeJournalApplicationUnderTest {
	mockController := gomock.NewController(t)
	fixture := contractTradeJournalApplicationUnderTest{
		contractTradeRecordRepository:           mocks.NewMockIContractTradeRecordRepository(mockController),
		tradeTagRepository:                      mocks.NewMockITradeTagRepository(mockController),
		tradeJournalSettingRepository:           mocks.NewMockITradeJournalSettingRepository(mockController),
		tradingStrategyRepository:               mocks.NewMockITradingStrategyRepository(mockController),
		contractTradingSymbolRepository:         mocks.NewMockIContractTradingSymbolRepository(mockController),
		contractMaintenanceMarginTierRepository: mocks.NewMockIContractMaintenanceMarginTierRepository(mockController),
		contractFundingRateSettlementRepository: mocks.NewMockIContractFundingRateSettlementRepository(mockController),
		kCandleContractRepository:               mocks.NewMockIKCandleContractRepository(mockController),
		strategyBotRepository:                   mocks.NewMockIStrategyBotRepository(mockController),
		strategyBotRunRecordRepository:          mocks.NewMockIStrategyBotRunRecordRepository(mockController),
	}
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(journalMoment).AnyTimes()

	fixture.journalService = service.NewContractTradeJournalService(
		fixture.contractTradeRecordRepository, fixture.tradeTagRepository, fixture.tradeJournalSettingRepository,
		fixture.tradingStrategyRepository, fixture.contractTradingSymbolRepository,
		fixture.contractMaintenanceMarginTierRepository, fixture.contractFundingRateSettlementRepository,
		fixture.kCandleContractRepository, fixture.strategyBotRepository, fixture.strategyBotRunRecordRepository,
		clockProxy)
	fixture.application = application.NewContractTradeJournalApplication(fixture.journalService)

	return fixture
}

// quietMarket answers every market read with nothing, so a test only arranges the reads it is about.
func (fixture contractTradeJournalApplicationUnderTest) quietMarket() {
	fixture.contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
		Return(entities.ContractTradingSymbol{}, true, nil).AnyTimes()
	fixture.contractFundingRateSettlementRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entities.ContractFundingRateSettlement{}, nil).AnyTimes()
	fixture.kCandleContractRepository.EXPECT().FindLatest(gomock.Any(), gomock.Any(), 1).
		Return([]entities.KCandleContract{}, nil).AnyTimes()
	fixture.kCandleContractRepository.EXPECT().FindPriceExtremesInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(vo.PriceExtremesVo{}, nil).AnyTimes()
	fixture.contractMaintenanceMarginTierRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
		Return([]entities.ContractMaintenanceMarginTier{}, nil).AnyTimes()
}

func (fixture contractTradeJournalApplicationUnderTest) withoutFeeRates() {
	fixture.tradeJournalSettingRepository.EXPECT().FindOneByUser(gomock.Any(), journalOwnerID).
		Return(entities.TradeJournalSetting{}, false, nil).AnyTimes()
}

func journalFill(id uint, kind vo.ContractTradeFillKindVo, filledAt time.Time, price string, quantity string) entities.ContractTradeFill {
	return entities.ContractTradeFill{
		ID: id, Kind: string(kind), FilledAt: filledAt, Price: decimal.RequireFromString(price),
		Quantity: decimal.RequireFromString(quantity), Liquidity: "taker",
	}
}

func aStoredOpenTrade() entities.ContractTradeRecord {
	return entities.ContractTradeRecord{
		ID: 27, OwnerID: journalOwnerID, Symbol: "BTCUSDT", Direction: "long", Leverage: decimal.NewFromInt(10),
		Status: string(vo.ContractTradeStatusOpen), OpenedAt: journalEntryAt,
		Fills: []entities.ContractTradeFill{journalFill(1, vo.ContractTradeFillKindEntry, journalEntryAt, "97905", "0.051")},
	}
}

func aStoredClosedTrade() entities.ContractTradeRecord {
	record := aStoredOpenTrade()
	closedAt := journalEntryAt.Add(26 * time.Hour)
	record.Status = string(vo.ContractTradeStatusClosed)
	record.ClosedAt = &closedAt
	record.Fills = append(record.Fills, journalFill(2, vo.ContractTradeFillKindExit, closedAt, "100420", "0.051"))

	return record
}

func aLongWrite() dto.ContractTradeRecordWriteDto {
	return dto.ContractTradeRecordWriteDto{
		Symbol: "btcusdt", Direction: "long", Leverage: decimal.NewFromInt(10),
		FirstEntryFill: dto.ContractTradeFillWriteDto{
			Kind: "entry", FilledAt: &journalEntryAt,
			Price: decimal.RequireFromString("97905"), Quantity: decimal.RequireFromString("0.030"),
		},
	}
}

func echoCreated(_ context.Context, record entities.ContractTradeRecord) (entities.ContractTradeRecord, error) {
	record.ID = 31
	return record, nil
}

func echoSaved(_ context.Context, record entities.ContractTradeRecord) (entities.ContractTradeRecord, error) {
	return record, nil
}

func TestContractTradeJournalApplicationRecordTrade(t *testing.T) {
	t.Run("a long is recorded for the person, priced at their taker rate", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		fixture.tradeJournalSettingRepository.EXPECT().FindOneByUser(gomock.Any(), journalOwnerID).
			Return(entities.TradeJournalSetting{TakerFeeRate: percentage("0.05")}, true, nil)
		fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), gomock.Len(0)).Return([]entities.TradeTag{}, nil)
		fixture.contractTradeRecordRepository.EXPECT().
			FindOpenByOwnerSymbolDirection(gomock.Any(), journalOwnerID, "BTCUSDT", "long").
			Return(entities.ContractTradeRecord{}, false, nil)
		fixture.contractTradeRecordRepository.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(echoCreated)
		fixture.quietMarket()

		recordDto, err := fixture.application.RecordTrade(context.Background(), journalOwnerID, aLongWrite())

		require.NoError(t, err)
		assert.Equal(t, uint(31), recordDto.ID)
		assert.Equal(t, "BTCUSDT", recordDto.Symbol)
		assert.Equal(t, "open", recordDto.Status)
		require.Len(t, recordDto.Fills, 1)
		assert.Equal(t, "1.47", recordDto.Fills[0].Fee.StringFixed(2))
		assert.Equal(t, "0.03", recordDto.Position.String())
		assert.Nil(t, recordDto.TradingStrategyID)
	})

	t.Run("an unknown contract is refused", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		fixture.contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "ABCUSDT").
			Return(entities.ContractTradingSymbol{}, false, nil)
		write := aLongWrite()
		write.Symbol = "ABCUSDT"

		_, err := fixture.application.RecordTrade(context.Background(), journalOwnerID, write)

		require.ErrorIs(t, err, domains.ErrContractTradeValidation)
		assert.Contains(t, err.Error(), "找不到這個合約標的 ABCUSDT")
	})

	t.Run("a blank symbol is refused", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		write := aLongWrite()
		write.Symbol = " "

		_, err := fixture.application.RecordTrade(context.Background(), journalOwnerID, write)

		require.ErrorIs(t, err, domains.ErrContractTradeValidation)
	})

	t.Run("a second open long on the same contract points at the first", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		fixture.quietMarket()
		fixture.withoutFeeRates()
		fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), gomock.Any()).Return([]entities.TradeTag{}, nil)
		fixture.contractTradeRecordRepository.EXPECT().
			FindOpenByOwnerSymbolDirection(gomock.Any(), journalOwnerID, "BTCUSDT", "long").
			Return(aStoredOpenTrade(), true, nil)

		_, err := fixture.application.RecordTrade(context.Background(), journalOwnerID, aLongWrite())

		require.ErrorIs(t, err, domains.ErrContractTradeOpenPositionExists)
		assert.Contains(t, err.Error(), "BTCUSDT 做多 已有持倉中的 #27，請在那一筆加成交")
	})

	t.Run("the person's own contract strategy is linked", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		fixture.quietMarket()
		fixture.withoutFeeRates()
		strategy := entities.TradingStrategy{ID: 12, OwnerID: journalOwnerID, Name: "BTC 趨勢跟隨", MarketDataKind: "contractKCandle"}
		fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), uint(12)).Return(strategy, nil).Times(2)
		fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), gomock.Any()).Return([]entities.TradeTag{}, nil)
		fixture.contractTradeRecordRepository.EXPECT().FindOpenByOwnerSymbolDirection(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entities.ContractTradeRecord{}, false, nil)
		fixture.contractTradeRecordRepository.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(echoCreated)
		write := aLongWrite()
		write.TradingStrategyID = new(uint(12))

		recordDto, err := fixture.application.RecordTrade(context.Background(), journalOwnerID, write)

		require.NoError(t, err)
		assert.Equal(t, uint(12), *recordDto.TradingStrategyID)
		assert.Equal(t, "BTC 趨勢跟隨", recordDto.TradingStrategyName)
	})

	t.Run("a spot strategy or somebody else's strategy cannot be linked", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		fixture.quietMarket()
		fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), uint(13)).
			Return(entities.TradingStrategy{ID: 13, OwnerID: journalOwnerID, MarketDataKind: "kCandle"}, nil)
		fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), uint(14)).
			Return(entities.TradingStrategy{ID: 14, OwnerID: 99, MarketDataKind: "contractKCandle"}, nil)
		fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), uint(15)).
			Return(entities.TradingStrategy{}, domains.TradingStrategyNotFound(15))
		spotWrite, strangerWrite, missingWrite := aLongWrite(), aLongWrite(), aLongWrite()
		spotWrite.TradingStrategyID = new(uint(13))
		strangerWrite.TradingStrategyID = new(uint(14))
		missingWrite.TradingStrategyID = new(uint(15))

		_, spotError := fixture.application.RecordTrade(context.Background(), journalOwnerID, spotWrite)
		_, strangerError := fixture.application.RecordTrade(context.Background(), journalOwnerID, strangerWrite)
		_, missingError := fixture.application.RecordTrade(context.Background(), journalOwnerID, missingWrite)

		require.ErrorIs(t, spotError, domains.ErrContractTradeValidation)
		assert.Contains(t, spotError.Error(), "只能指名合約交易策略")
		require.ErrorIs(t, strangerError, domains.ErrTradingStrategyNotFound)
		require.ErrorIs(t, missingError, domains.ErrTradingStrategyNotFound)
	})

	t.Run("somebody else's tag cannot be put on the trade", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		fixture.quietMarket()
		fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), []uint{6}).
			Return([]entities.TradeTag{{ID: 6, OwnerID: 99, Kind: "setup", Name: "突破"}}, nil)
		write := aLongWrite()
		write.SetupTagIDs = []uint{6}

		_, err := fixture.application.RecordTrade(context.Background(), journalOwnerID, write)

		require.ErrorIs(t, err, domains.ErrTradeTagNotFound)
	})

	t.Run("rule breaks and storage failures stop the recording", func(t *testing.T) {
		for name, arrange := range map[string]func(fixture contractTradeJournalApplicationUnderTest) (dto.ContractTradeRecordWriteDto, error){
			"reading the contract": func(fixture contractTradeJournalApplicationUnderTest) (dto.ContractTradeRecordWriteDto, error) {
				fixture.contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
					Return(entities.ContractTradingSymbol{}, false, errStorageDown)
				return aLongWrite(), errStorageDown
			},
			"reading tags": func(fixture contractTradeJournalApplicationUnderTest) (dto.ContractTradeRecordWriteDto, error) {
				fixture.quietMarket()
				fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), gomock.Any()).Return(nil, errStorageDown)
				return aLongWrite(), errStorageDown
			},
			"reading fee rates": func(fixture contractTradeJournalApplicationUnderTest) (dto.ContractTradeRecordWriteDto, error) {
				fixture.quietMarket()
				fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), gomock.Any()).Return(nil, nil)
				fixture.tradeJournalSettingRepository.EXPECT().FindOneByUser(gomock.Any(), gomock.Any()).
					Return(entities.TradeJournalSetting{}, false, errStorageDown)
				return aLongWrite(), errStorageDown
			},
			"a leverage below one": func(fixture contractTradeJournalApplicationUnderTest) (dto.ContractTradeRecordWriteDto, error) {
				fixture.quietMarket()
				fixture.withoutFeeRates()
				fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), gomock.Any()).Return(nil, nil)
				write := aLongWrite()
				write.Leverage = decimal.RequireFromString("0.5")
				return write, domains.ErrContractTradeValidation
			},
			"looking for an open trade": func(fixture contractTradeJournalApplicationUnderTest) (dto.ContractTradeRecordWriteDto, error) {
				fixture.quietMarket()
				fixture.withoutFeeRates()
				fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), gomock.Any()).Return(nil, nil)
				fixture.contractTradeRecordRepository.EXPECT().FindOpenByOwnerSymbolDirection(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(entities.ContractTradeRecord{}, false, errStorageDown)
				return aLongWrite(), errStorageDown
			},
			"a race lost to the open-trade rule": func(fixture contractTradeJournalApplicationUnderTest) (dto.ContractTradeRecordWriteDto, error) {
				fixture.quietMarket()
				fixture.withoutFeeRates()
				fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), gomock.Any()).Return(nil, nil)
				fixture.contractTradeRecordRepository.EXPECT().FindOpenByOwnerSymbolDirection(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(entities.ContractTradeRecord{}, false, nil)
				fixture.contractTradeRecordRepository.EXPECT().Create(gomock.Any(), gomock.Any()).
					Return(entities.ContractTradeRecord{}, domains.ContractTradeOpenPositionExists("BTCUSDT", "做多", 0))
				return aLongWrite(), domains.ErrContractTradeOpenPositionExists
			},
		} {
			t.Run(name, func(t *testing.T) {
				fixture := newContractTradeJournalApplicationUnderTest(t)
				write, expectedError := arrange(fixture)

				_, err := fixture.application.RecordTrade(context.Background(), journalOwnerID, write)

				require.ErrorIs(t, err, expectedError)
			})
		}
	})
}

func TestContractTradeJournalApplicationChangesATrade(t *testing.T) {
	t.Run("selling everything back closes the trade", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		fixture.quietMarket()
		fixture.withoutFeeRates()
		fixture.contractTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(27)).Return(aStoredOpenTrade(), nil)
		fixture.contractTradeRecordRepository.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(echoSaved)
		exitAt := journalEntryAt.Add(26 * time.Hour)

		recordDto, err := fixture.application.AddFill(context.Background(), journalOwnerID, 27, dto.ContractTradeFillWriteDto{
			Kind: "exit", FilledAt: &exitAt, Price: decimal.RequireFromString("100420"), Quantity: decimal.RequireFromString("0.051"),
		})

		require.NoError(t, err)
		assert.Equal(t, "closed", recordDto.Status)
		require.NotNil(t, recordDto.ClosedAt)
		assert.True(t, recordDto.ClosedAt.Equal(exitAt))
		assert.True(t, recordDto.Plan.Locked)
		assert.Equal(t, "100420", recordDto.AverageExitPrice.Decimal.String())
		assert.True(t, recordDto.Outcome.FeeRateMissing)
	})

	t.Run("a fill left without a time is dated now", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		fixture.quietMarket()
		fixture.withoutFeeRates()
		fixture.contractTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(27)).Return(aStoredOpenTrade(), nil)
		fixture.contractTradeRecordRepository.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(echoSaved)

		recordDto, err := fixture.application.AddFill(context.Background(), journalOwnerID, 27, dto.ContractTradeFillWriteDto{
			Kind: "entry", Price: decimal.RequireFromString("97960"), Quantity: decimal.RequireFromString("0.021"),
		})

		require.NoError(t, err)
		assert.True(t, recordDto.Fills[1].FilledAt.Equal(journalMoment))
	})

	t.Run("somebody else's trade answers not found to every change", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		strangersTrade := aStoredOpenTrade()
		strangersTrade.OwnerID = 99
		fixture.contractTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(27)).Return(strangersTrade, nil).AnyTimes()
		fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
		ctx := context.Background()

		_, addError := fixture.application.AddFill(ctx, journalOwnerID, 27, dto.ContractTradeFillWriteDto{})
		_, amendError := fixture.application.AmendFill(ctx, journalOwnerID, 27, 1, dto.ContractTradeFillWriteDto{})
		_, removeError := fixture.application.RemoveFill(ctx, journalOwnerID, 27, 1)
		_, planError := fixture.application.AmendPlan(ctx, journalOwnerID, 27, dto.ContractTradePlanWriteDto{})
		_, noteError := fixture.application.AddNote(ctx, journalOwnerID, 27, "hi")
		_, reviewError := fixture.application.WriteReview(ctx, journalOwnerID, 27, dto.ContractTradeReviewWriteDto{})
		_, tagError := fixture.application.AssignSetupTags(ctx, journalOwnerID, 27, nil)
		_, getError := fixture.application.GetTrade(ctx, journalOwnerID, 27)
		deleteError := fixture.application.DeleteTrade(ctx, journalOwnerID, 27)

		for _, err := range []error{addError, amendError, removeError, planError, noteError, reviewError, tagError, getError, deleteError} {
			require.ErrorIs(t, err, domains.ErrContractTradeNotFound)
			assert.Contains(t, err.Error(), "找不到這筆交易")
		}
	})

	t.Run("a closed trade's plan stays as it was", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		fixture.withoutFeeRates()
		fixture.contractTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(27)).Return(aStoredClosedTrade(), nil)

		_, err := fixture.application.AmendPlan(context.Background(), journalOwnerID, 27, dto.ContractTradePlanWriteDto{
			PlannedStopLossPrice: percentage("96380")})

		require.ErrorIs(t, err, domains.ErrContractTradeLocked)
	})

	t.Run("an open trade's fill, plan, note and tags change", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		fixture.quietMarket()
		fixture.withoutFeeRates()
		stored := aStoredOpenTrade()
		stored.Fills = append(stored.Fills, journalFill(2, vo.ContractTradeFillKindEntry, journalEntryAt.Add(time.Minute), "97960", "0.021"))
		fixture.contractTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(27)).Return(stored, nil).AnyTimes()
		fixture.contractTradeRecordRepository.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(echoSaved).AnyTimes()
		fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), []uint{8}).
			Return([]entities.TradeTag{{ID: 8, OwnerID: journalOwnerID, Kind: "setup", Name: "突破"}}, nil)
		ctx := context.Background()

		amended, amendError := fixture.application.AmendFill(ctx, journalOwnerID, 27, 1, dto.ContractTradeFillWriteDto{
			Kind: "entry", FilledAt: &journalEntryAt, Price: decimal.RequireFromString("97900"), Quantity: decimal.RequireFromString("0.051")})
		removed, removeError := fixture.application.RemoveFill(ctx, journalOwnerID, 27, 2)
		planned, planError := fixture.application.AmendPlan(ctx, journalOwnerID, 27, dto.ContractTradePlanWriteDto{
			PlannedStopLossPrice: percentage("96380"), EntryReason: "突破前高"})
		noted, noteError := fixture.application.AddNote(ctx, journalOwnerID, 27, "加碼太急")
		tagged, tagError := fixture.application.AssignSetupTags(ctx, journalOwnerID, 27, []uint{8})

		require.NoError(t, amendError)
		require.NoError(t, removeError)
		require.NoError(t, planError)
		require.NoError(t, noteError)
		require.NoError(t, tagError)
		assert.Equal(t, "97900", amended.Fills[0].Price.String())
		assert.Len(t, removed.Fills, 1)
		assert.Equal(t, "突破前高", planned.Plan.EntryReason)
		assert.Equal(t, "加碼太急", noted.Notes[0].Content)
		assert.Equal(t, "突破", tagged.SetupTags[0].Name)
	})

	t.Run("a closed trade is reviewed with its mistakes", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		fixture.quietMarket()
		fixture.withoutFeeRates()
		fixture.contractTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(27)).Return(aStoredClosedTrade(), nil)
		fixture.contractTradeRecordRepository.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(echoSaved)
		fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), []uint{4}).
			Return([]entities.TradeTag{{ID: 4, OwnerID: journalOwnerID, Kind: "mistake", Name: "提早出場"}}, nil)

		recordDto, err := fixture.application.WriteReview(context.Background(), journalOwnerID, 27,
			dto.ContractTradeReviewWriteDto{WentWrong: "提早出場", ExecutionScore: 4, MistakeTagIDs: []uint{4}})

		require.NoError(t, err)
		assert.Equal(t, "reviewed", recordDto.Status)
		require.NotNil(t, recordDto.Review)
		assert.Equal(t, 4, recordDto.Review.ExecutionScore)
		assert.Equal(t, "提早出場", recordDto.MistakeTags[0].Name)
	})

	t.Run("failures while changing come back as they are", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		fixture.contractTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(40)).Return(entities.ContractTradeRecord{}, domains.ContractTradeNotFound(40))
		fixture.contractTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(27)).Return(aStoredOpenTrade(), nil).AnyTimes()
		fixture.tradeJournalSettingRepository.EXPECT().FindOneByUser(gomock.Any(), journalOwnerID).
			Return(entities.TradeJournalSetting{}, false, errStorageDown)
		fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), gomock.Any()).Return(nil, errStorageDown).Times(2)
		ctx := context.Background()

		_, missingError := fixture.application.AddNote(ctx, journalOwnerID, 40, "x")
		_, settingError := fixture.application.AddNote(ctx, journalOwnerID, 27, "x")
		_, reviewTagError := fixture.application.WriteReview(ctx, journalOwnerID, 27, dto.ContractTradeReviewWriteDto{})
		_, setupTagError := fixture.application.AssignSetupTags(ctx, journalOwnerID, 27, []uint{1})

		require.ErrorIs(t, missingError, domains.ErrContractTradeNotFound)
		require.ErrorIs(t, settingError, errStorageDown)
		require.ErrorIs(t, reviewTagError, errStorageDown)
		require.ErrorIs(t, setupTagError, errStorageDown)
	})

	t.Run("a refused change or a failed save writes nothing more", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		fixture.withoutFeeRates()
		fixture.contractTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(27)).Return(aStoredOpenTrade(), nil).Times(2)
		fixture.contractTradeRecordRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Return(entities.ContractTradeRecord{}, errStorageDown)

		_, blankNoteError := fixture.application.AddNote(context.Background(), journalOwnerID, 27, " ")
		_, saveError := fixture.application.AddNote(context.Background(), journalOwnerID, 27, "ok")

		require.ErrorIs(t, blankNoteError, domains.ErrContractTradeValidation)
		require.ErrorIs(t, saveError, errStorageDown)
	})

	t.Run("the person's own trade is deleted", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		fixture.contractTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(33)).Return(aStoredClosedTrade(), nil)
		fixture.contractTradeRecordRepository.EXPECT().Delete(gomock.Any(), uint(33)).Return(nil)

		require.NoError(t, fixture.application.DeleteTrade(context.Background(), journalOwnerID, 33))
	})
}

func TestContractTradeJournalApplicationGetTrade(t *testing.T) {
	t.Run("the detail brings funding, excursions, the floating profit and a liquidation estimate", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		stored := aStoredOpenTrade()
		stored.PlannedStopLossPrice = percentage("96380")
		stored.TradingStrategyID = new(uint(12))
		specifiedAt := journalMoment
		fixture.contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(entities.ContractTradingSymbol{
			Symbol: "BTCUSDT", TickSize: percentage("0.1"), MaintenanceMarginRate: percentage("0.005"), SpecificationUpdatedAt: &specifiedAt,
		}, true, nil).AnyTimes()
		fixture.contractMaintenanceMarginTierRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(nil, nil)
		fixture.contractFundingRateSettlementRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), 1000).Return(
			[]entities.ContractFundingRateSettlement{{
				Symbol: "BTCUSDT", SettlementTime: time.Date(2026, 9, 25, 16, 0, 0, 0, time.UTC),
				FundingRate: decimal.RequireFromString("0.0001"), MarkPrice: percentage("100000"),
			}}, nil)
		fixture.kCandleContractRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).
			Return([]entities.KCandleContract{{Close: decimal.RequireFromString("98500")}}, nil)
		fixture.kCandleContractRepository.EXPECT().FindPriceExtremesInRange(gomock.Any(), "BTCUSDT", journalEntryAt, journalMoment).
			Return(vo.PriceExtremesVo{HighestPrice: decimal.RequireFromString("100960"), LowestPrice: decimal.RequireFromString("97110"), Has: true}, nil)
		fixture.contractTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(27)).Return(stored, nil)
		fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), uint(12)).Return(entities.TradingStrategy{}, domains.TradingStrategyNotFound(12))

		recordDto, err := fixture.application.GetTrade(context.Background(), journalOwnerID, 27)

		require.NoError(t, err)
		outcome := recordDto.Outcome
		assert.Equal(t, "-0.51", outcome.Funding.Amount.StringFixed(2))
		assert.True(t, outcome.Excursion.Available)
		assert.Equal(t, "30.35", outcome.FloatingProfit.Amount.StringFixed(2))
		assert.True(t, outcome.LiquidationPrice.Available)
		assert.True(t, recordDto.TradingStrategyDeleted)
	})

	t.Run("failed market reads only leave their own figures unavailable", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		fixture.contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
			Return(entities.ContractTradingSymbol{}, false, errStorageDown).AnyTimes()
		fixture.contractFundingRateSettlementRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errStorageDown)
		fixture.kCandleContractRepository.EXPECT().FindLatest(gomock.Any(), gomock.Any(), 1).Return(nil, errStorageDown)
		fixture.kCandleContractRepository.EXPECT().FindPriceExtremesInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(vo.PriceExtremesVo{}, errStorageDown)
		fixture.contractTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(27)).Return(aStoredOpenTrade(), nil)

		recordDto, err := fixture.application.GetTrade(context.Background(), journalOwnerID, 27)

		require.NoError(t, err)
		assert.Equal(t, "noSettlementData", recordDto.Outcome.Funding.UnavailableReason)
		assert.Equal(t, "noLatestPrice", recordDto.Outcome.FloatingProfit.UnavailableReason)
		assert.Equal(t, "noMarketData", recordDto.Outcome.Excursion.UnavailableReason)
		assert.Equal(t, "noTradingSpecification", recordDto.Outcome.LiquidationPrice.UnavailableReason)
	})

	t.Run("margin tiers that cannot be read leave no estimate", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		fixture.contractMaintenanceMarginTierRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).Return(nil, errStorageDown)
		fixture.quietMarket()
		fixture.contractTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(27)).Return(aStoredOpenTrade(), nil)

		recordDto, err := fixture.application.GetTrade(context.Background(), journalOwnerID, 27)

		require.NoError(t, err)
		assert.Equal(t, "noTradingSpecification", recordDto.Outcome.LiquidationPrice.UnavailableReason)
	})

	t.Run("a long holding reads its settlements page by page", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		fullPage := make([]entities.ContractFundingRateSettlement, 1000)
		for index := range fullPage {
			fullPage[index] = entities.ContractFundingRateSettlement{
				SettlementTime: journalEntryAt.Add(time.Duration(index+1) * time.Minute), FundingRate: decimal.Zero,
			}
		}
		gomock.InOrder(
			fixture.contractFundingRateSettlementRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), 1000).Return(fullPage, nil),
			fixture.contractFundingRateSettlementRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), 1000).
				Return([]entities.ContractFundingRateSettlement{{SettlementTime: journalMoment.Add(-time.Hour), FundingRate: decimal.Zero}}, nil),
		)
		fixture.quietMarket()
		fixture.contractTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(27)).Return(aStoredOpenTrade(), nil)

		recordDto, err := fixture.application.GetTrade(context.Background(), journalOwnerID, 27)

		require.NoError(t, err)
		assert.Equal(t, 1001, recordDto.Outcome.Funding.SettlementCount)
	})
}

func TestContractTradeJournalApplicationListTrades(t *testing.T) {
	t.Run("the list narrows as asked and names the linked strategies", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		fixture.quietMarket()
		linked := aStoredOpenTrade()
		linked.TradingStrategyID = new(uint(12))
		orphaned := aStoredClosedTrade()
		orphaned.ID = 26
		orphaned.TradingStrategyID = new(uint(13))
		openedSince := journalMoment.AddDate(0, 0, -7)
		fixture.contractTradeRecordRepository.EXPECT().FindPageByOwner(gomock.Any(), journalOwnerID, vo.ContractTradeListFilterVo{
			Status: "open", Symbol: "BTCUSDT", OpenedSince: &openedSince, Limit: 20,
		}).Return([]entities.ContractTradeRecord{linked, orphaned}, int64(2), nil)
		fixture.tradingStrategyRepository.EXPECT().FindAllByOwner(gomock.Any(), journalOwnerID).
			Return([]entities.TradingStrategy{{ID: 12, Name: "BTC 趨勢跟隨"}}, nil)

		page, err := fixture.application.ListTrades(context.Background(), journalOwnerID,
			dto.ContractTradeListQueryDto{Status: "open", Symbol: "btcusdt", Period: "7d"})

		require.NoError(t, err)
		assert.Equal(t, int64(2), page.TotalCount)
		assert.Equal(t, "BTC 趨勢跟隨", page.Trades[0].TradingStrategyName)
		assert.False(t, page.Trades[0].TradingStrategyDeleted)
		assert.True(t, page.Trades[1].TradingStrategyDeleted)
		assert.Equal(t, "notComputed", page.Trades[0].Outcome.Excursion.UnavailableReason)
	})

	t.Run("the limit is capped and names that cannot be read orphan nothing", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		fixture.quietMarket()
		linked := aStoredOpenTrade()
		linked.TradingStrategyID = new(uint(12))
		fixture.contractTradeRecordRepository.EXPECT().FindPageByOwner(gomock.Any(), journalOwnerID, vo.ContractTradeListFilterVo{Limit: 200}).
			Return([]entities.ContractTradeRecord{linked}, int64(1), nil)
		fixture.tradingStrategyRepository.EXPECT().FindAllByOwner(gomock.Any(), journalOwnerID).Return(nil, errStorageDown)

		page, err := fixture.application.ListTrades(context.Background(), journalOwnerID, dto.ContractTradeListQueryDto{Limit: 500})

		require.NoError(t, err)
		assert.False(t, page.Trades[0].TradingStrategyDeleted)
	})

	t.Run("an unknown status, symbol or period is refused, and a failed read comes back", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		fixture.contractTradeRecordRepository.EXPECT().FindPageByOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, int64(0), errStorageDown)

		_, statusError := fixture.application.ListTrades(context.Background(), journalOwnerID, dto.ContractTradeListQueryDto{Status: "planned"})
		_, symbolError := fixture.application.ListTrades(context.Background(), journalOwnerID, dto.ContractTradeListQueryDto{Symbol: "BTC\x00USDT"})
		_, periodError := fixture.application.ListTrades(context.Background(), journalOwnerID, dto.ContractTradeListQueryDto{Period: "1y"})
		_, readError := fixture.application.ListTrades(context.Background(), journalOwnerID, dto.ContractTradeListQueryDto{})

		require.ErrorIs(t, statusError, domains.ErrContractTradeValidation)
		assert.Contains(t, statusError.Error(), "狀態只有 open、closed 與 reviewed")
		require.ErrorIs(t, symbolError, domains.ErrContractTradeValidation)
		require.ErrorIs(t, periodError, domains.ErrContractTradeValidation)
		require.ErrorIs(t, readError, errStorageDown)
	})
}

func TestContractTradeJournalApplicationGivesEachTradeOnlyItsOwnSettlements(t *testing.T) {
	fixture := newContractTradeJournalApplicationUnderTest(t)
	earlier := aStoredClosedTrade()
	earlier.ID = 20
	earlierClosedAt := journalEntryAt.Add(-20 * time.Hour)
	earlier.OpenedAt = journalEntryAt.Add(-30 * time.Hour)
	earlier.ClosedAt = &earlierClosedAt
	earlier.Fills[0].FilledAt = earlier.OpenedAt
	earlier.Fills[1].FilledAt = earlierClosedAt
	later := aStoredClosedTrade()
	stillOpen := aStoredOpenTrade()
	stillOpen.ID = 21
	stillOpen.Direction = "short"
	fixture.contractFundingRateSettlementRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), 1000).Return(
		[]entities.ContractFundingRateSettlement{
			{SettlementTime: journalEntryAt.Add(2 * time.Hour), FundingRate: decimal.RequireFromString("0.0001"), MarkPrice: percentage("100000")},
		}, nil)
	fixture.quietMarket()
	fixture.contractTradeRecordRepository.EXPECT().FindPageByOwner(gomock.Any(), journalOwnerID, gomock.Any()).
		Return([]entities.ContractTradeRecord{later, earlier, stillOpen}, int64(3), nil)
	fixture.tradingStrategyRepository.EXPECT().FindAllByOwner(gomock.Any(), journalOwnerID).Return(nil, nil)

	page, err := fixture.application.ListTrades(context.Background(), journalOwnerID, dto.ContractTradeListQueryDto{})

	require.NoError(t, err)
	require.Len(t, page.Trades, 3)
	assert.Equal(t, 1, page.Trades[0].Outcome.Funding.SettlementCount)
	assert.Equal(t, "noSettlementData", page.Trades[1].Outcome.Funding.UnavailableReason)
}

func TestContractTradeJournalApplicationGetStatistics(t *testing.T) {
	t.Run("the last thirty days of closed trades by default", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		fixture.quietMarket()
		closedSince := journalMoment.AddDate(0, 0, -30)
		winning := aStoredClosedTrade()
		winning.PlannedStopLossPrice = percentage("96380")
		losing := aStoredClosedTrade()
		losing.ID = 28
		losing.Fills[1].Price = decimal.RequireFromString("97000")
		fixture.contractTradeRecordRepository.EXPECT().FindClosedByOwner(gomock.Any(), journalOwnerID, &closedSince).
			Return([]entities.ContractTradeRecord{winning, losing}, nil)

		statistics, err := fixture.application.GetStatistics(context.Background(), journalOwnerID, "")

		require.NoError(t, err)
		assert.Equal(t, "30d", statistics.Period)
		assert.Equal(t, 2, statistics.ClosedTradeCount)
		assert.InDelta(t, 0.5, *statistics.WinRate, 0.0001)
		assert.Equal(t, 1, statistics.RExcludedCount)
	})

	t.Run("an unknown period is refused and a failed read comes back", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		fixture.contractTradeRecordRepository.EXPECT().FindClosedByOwner(gomock.Any(), journalOwnerID, nil).Return(nil, errStorageDown)

		_, periodError := fixture.application.GetStatistics(context.Background(), journalOwnerID, "1y")
		_, readError := fixture.application.GetStatistics(context.Background(), journalOwnerID, "all")

		require.ErrorIs(t, periodError, domains.ErrContractTradeValidation)
		require.ErrorIs(t, readError, errStorageDown)
	})
}

func theBotAndItsRound() (entities.StrategyBot, entities.StrategyBotRunRecord) {
	return entities.StrategyBot{ID: 3, OwnerID: journalOwnerID, Name: "BTC 趨勢跟隨", Symbol: "BTCUSDT", TradingStrategyID: 12},
		entities.StrategyBotRunRecord{
			StrategyBotID: 3, RunNumber: 412, RanAt: journalEntryAt.Add(-3 * time.Minute),
			SuggestedDirection: "long", SuggestedLeverage: percentage("10"),
			SuggestedStopLossPrice: percentage("96380"), SuggestedTakeProfitPrice: percentage("100785"),
			ReferencePrice: percentage("97850"), SuggestedQuantity: percentage("0.051"),
			JournalLinkIdentifier: "round-link-1",
		}
}

func TestContractTradeJournalApplicationPrepareJournalLink(t *testing.T) {
	t.Run("the round as it was fills in a new trade", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		strategyBot, runRecord := theBotAndItsRound()
		fixture.strategyBotRunRecordRepository.EXPECT().FindByJournalLinkIdentifier(gomock.Any(), "round-link-1").Return(runRecord, true, nil)
		fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).Return(strategyBot, nil)
		fixture.contractTradeRecordRepository.EXPECT().FindOpenByOwnerSymbolDirection(gomock.Any(), journalOwnerID, "BTCUSDT", "long").
			Return(entities.ContractTradeRecord{}, false, nil)

		prefill, err := fixture.application.PrepareJournalLink(context.Background(), journalOwnerID, "round-link-1")

		require.NoError(t, err)
		assert.Equal(t, "newTrade", prefill.Mode)
		assert.Equal(t, "BTCUSDT", prefill.Symbol)
		assert.Equal(t, "long", prefill.Direction)
		assert.Equal(t, "10", prefill.Leverage.Decimal.String())
		assert.Equal(t, "96380", prefill.PlannedStopLossPrice.Decimal.String())
		assert.Equal(t, "100785", prefill.PlannedTakeProfitPrice.Decimal.String())
		assert.Equal(t, uint(12), *prefill.TradingStrategyID)
		assert.Equal(t, "97850", prefill.EntryPrice.Decimal.String())
		assert.Equal(t, "0.051", prefill.Quantity.Decimal.String())
		assert.True(t, prefill.EntryPriceNeedsConfirmation)
		assert.Equal(t, 412, prefill.RunNumber)
		assert.Equal(t, "BTC 趨勢跟隨", prefill.StrategyBotName)
		assert.Empty(t, prefill.MissingReferenceReason)
	})

	t.Run("holding the symbol already turns it into an entry fill for that trade", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		strategyBot, runRecord := theBotAndItsRound()
		fixture.strategyBotRunRecordRepository.EXPECT().FindByJournalLinkIdentifier(gomock.Any(), "round-link-1").Return(runRecord, true, nil)
		fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).Return(strategyBot, nil)
		fixture.contractTradeRecordRepository.EXPECT().FindOpenByOwnerSymbolDirection(gomock.Any(), journalOwnerID, "BTCUSDT", "long").
			Return(aStoredOpenTrade(), true, nil)

		prefill, err := fixture.application.PrepareJournalLink(context.Background(), journalOwnerID, "round-link-1")

		require.NoError(t, err)
		assert.Equal(t, "addEntryFill", prefill.Mode)
		assert.Equal(t, uint(27), *prefill.TargetTradeID)
		assert.Equal(t, "97850", prefill.EntryPrice.Decimal.String())
	})

	t.Run("a round from before reference prices were kept says so", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		strategyBot, runRecord := theBotAndItsRound()
		runRecord.ReferencePrice = decimal.NullDecimal{}
		runRecord.SuggestedQuantity = decimal.NullDecimal{}
		fixture.strategyBotRunRecordRepository.EXPECT().FindByJournalLinkIdentifier(gomock.Any(), "round-link-1").Return(runRecord, true, nil)
		fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).Return(strategyBot, nil)
		fixture.contractTradeRecordRepository.EXPECT().FindOpenByOwnerSymbolDirection(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entities.ContractTradeRecord{}, false, nil)

		prefill, err := fixture.application.PrepareJournalLink(context.Background(), journalOwnerID, "round-link-1")

		require.NoError(t, err)
		assert.False(t, prefill.EntryPrice.Valid)
		assert.Equal(t, "roundPredatesReferencePrices", prefill.MissingReferenceReason)
	})

	t.Run("a forgotten round, somebody else's bot or a deleted bot answers not found", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		strategyBot, runRecord := theBotAndItsRound()
		strangersBot := strategyBot
		strangersBot.OwnerID = 99
		fixture.strategyBotRunRecordRepository.EXPECT().FindByJournalLinkIdentifier(gomock.Any(), "gone").Return(entities.StrategyBotRunRecord{}, false, nil)
		fixture.strategyBotRunRecordRepository.EXPECT().FindByJournalLinkIdentifier(gomock.Any(), "round-link-1").Return(runRecord, true, nil).Times(2)
		gomock.InOrder(
			fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).Return(strangersBot, nil),
			fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).Return(entities.StrategyBot{}, domains.StrategyBotNotFound(3)),
		)

		_, forgottenError := fixture.application.PrepareJournalLink(context.Background(), journalOwnerID, "gone")
		_, strangerError := fixture.application.PrepareJournalLink(context.Background(), journalOwnerID, "round-link-1")
		_, deletedError := fixture.application.PrepareJournalLink(context.Background(), journalOwnerID, "round-link-1")

		require.ErrorIs(t, forgottenError, domains.ErrJournalLinkNotFound)
		assert.Contains(t, forgottenError.Error(), "這一輪的建議已不在紀錄中")
		require.ErrorIs(t, strangerError, domains.ErrJournalLinkNotFound)
		assert.Contains(t, strangerError.Error(), "找不到這一輪的建議")
		require.ErrorIs(t, deletedError, domains.ErrJournalLinkNotFound)
	})

	t.Run("storage failures come back as they are", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		strategyBot, runRecord := theBotAndItsRound()
		fixture.strategyBotRunRecordRepository.EXPECT().FindByJournalLinkIdentifier(gomock.Any(), "a").Return(entities.StrategyBotRunRecord{}, false, errStorageDown)
		fixture.strategyBotRunRecordRepository.EXPECT().FindByJournalLinkIdentifier(gomock.Any(), "round-link-1").Return(runRecord, true, nil).Times(2)
		gomock.InOrder(
			fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).Return(entities.StrategyBot{}, errStorageDown),
			fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).Return(strategyBot, nil),
		)
		fixture.contractTradeRecordRepository.EXPECT().FindOpenByOwnerSymbolDirection(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entities.ContractTradeRecord{}, false, errStorageDown)

		_, runError := fixture.application.PrepareJournalLink(context.Background(), journalOwnerID, "a")
		_, botError := fixture.application.PrepareJournalLink(context.Background(), journalOwnerID, "round-link-1")
		_, openError := fixture.application.PrepareJournalLink(context.Background(), journalOwnerID, "round-link-1")

		require.ErrorIs(t, runError, errStorageDown)
		require.ErrorIs(t, botError, errStorageDown)
		require.ErrorIs(t, openError, errStorageDown)
	})
}

func TestContractTradeJournalApplicationRecordsWhereATradeCameFrom(t *testing.T) {
	t.Run("saving from a link keeps the round's suggestion and measures the slippage", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		fixture.quietMarket()
		fixture.withoutFeeRates()
		strategyBot, runRecord := theBotAndItsRound()
		fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), gomock.Any()).Return(nil, nil)
		fixture.strategyBotRunRecordRepository.EXPECT().FindByJournalLinkIdentifier(gomock.Any(), "round-link-1").Return(runRecord, true, nil)
		fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).Return(strategyBot, nil)
		fixture.contractTradeRecordRepository.EXPECT().FindOpenByOwnerSymbolDirection(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entities.ContractTradeRecord{}, false, nil)
		fixture.contractTradeRecordRepository.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(echoCreated)
		write := aLongWrite()
		write.JournalLinkIdentifier = "round-link-1"

		recordDto, err := fixture.application.RecordTrade(context.Background(), journalOwnerID, write)

		require.NoError(t, err)
		require.NotNil(t, recordDto.Source)
		assert.Equal(t, 412, recordDto.Source.RunNumber)
		assert.Equal(t, "BTC 趨勢跟隨", recordDto.Source.StrategyBotName)
		assert.Equal(t, "97850", recordDto.Source.ReferencePrice.Decimal.String())
		assert.Equal(t, "96380", recordDto.Source.SuggestedStopLossPrice.Decimal.String())
		assert.Equal(t, "100785", recordDto.Source.SuggestedTakeProfitPrice.Decimal.String())
		require.NotNil(t, recordDto.Outcome.EntrySlippagePercentage)
		assert.InDelta(t, 0.06, *recordDto.Outcome.EntrySlippagePercentage, 0.005)
	})

	t.Run("a link forgotten since the page opened still records the trade, without a source", func(t *testing.T) {
		fixture := newContractTradeJournalApplicationUnderTest(t)
		fixture.quietMarket()
		fixture.withoutFeeRates()
		fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), gomock.Any()).Return(nil, nil)
		fixture.strategyBotRunRecordRepository.EXPECT().FindByJournalLinkIdentifier(gomock.Any(), "gone").
			Return(entities.StrategyBotRunRecord{}, false, nil)
		fixture.contractTradeRecordRepository.EXPECT().FindOpenByOwnerSymbolDirection(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entities.ContractTradeRecord{}, false, nil)
		fixture.contractTradeRecordRepository.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(echoCreated)
		write := aLongWrite()
		write.JournalLinkIdentifier = "gone"

		recordDto, err := fixture.application.RecordTrade(context.Background(), journalOwnerID, write)

		require.NoError(t, err)
		assert.Nil(t, recordDto.Source)
	})
}
