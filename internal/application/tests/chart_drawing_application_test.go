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

const chartDrawingOwnerID = uint(7)

// chartDrawingAnchorHour is the 08:00 hourly candle of the day the clock stands on, so the hour is merged from minutes.
var chartDrawingAnchorHour = time.Date(2026, 10, 19, 8, 0, 0, 0, time.UTC)

var errChartDrawingStorageDown = errors.New("storage down")

type chartDrawingApplicationUnderTest struct {
	application                 *application.ChartDrawingApplication
	chartDrawingBoardRepository *mocks.MockIChartDrawingBoardRepository
	kCandleRepository           *mocks.MockIKCandleRepository
	kCandleContractRepository   *mocks.MockIKCandleContractRepository
}

func newChartDrawingApplicationUnderTest(t *testing.T) chartDrawingApplicationUnderTest {
	mockController := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)).AnyTimes()
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
		Return(entities.TradingSymbol{Symbol: "BTCUSDT", Market: string(vo.MarketCrypto)}, true, nil).AnyTimes()
	marketCatalog := domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}})
	fixture := chartDrawingApplicationUnderTest{
		chartDrawingBoardRepository: mocks.NewMockIChartDrawingBoardRepository(mockController),
		kCandleRepository:           mocks.NewMockIKCandleRepository(mockController),
		kCandleContractRepository:   mocks.NewMockIKCandleContractRepository(mockController),
	}
	fixture.application = application.NewChartDrawingApplication(service.NewChartDrawingService(
		fixture.chartDrawingBoardRepository,
		service.NewKCandleService(fixture.kCandleRepository, mocks.NewMockIKCandleSummaryDayRepository(mockController),
			tradingSymbolRepository, clockProxy, marketCatalog, 1000),
		service.NewKCandleContractService(fixture.kCandleContractRepository,
			mocks.NewMockIKCandleContractSummaryDayRepository(mockController), clockProxy, marketCatalog, 1000),
		vo.ChartDrawingLimitsVo{BoardCapacity: 200, AssistantBatchCapacity: 20},
	))

	return fixture
}

func spotPlace() dto.ChartDrawingBoardPlaceDto {
	return dto.ChartDrawingBoardPlaceDto{OwnerID: chartDrawingOwnerID, Market: "spot", Symbol: "btcusdt"}
}

func aBoardDrawing(id uint, source vo.ChartDrawingSourceVo) dto.ChartDrawingDto {
	return dto.ChartDrawingDto{
		ID: id, Kind: "horizontalLine",
		Anchors: []dto.ChartDrawingAnchorDto{{OpenTime: chartDrawingAnchorHour, Price: decimal.RequireFromString("100")}},
		Color:   "blue", LineWidth: 1, LineStyle: "solid", Visible: true, Source: string(source),
	}
}

func aStoredBoard(revision int64, drawings ...dto.ChartDrawingDto) entities.ChartDrawingBoard {
	lastDrawingID := uint(0)
	for _, drawing := range drawings {
		lastDrawingID = max(lastDrawingID, drawing.ID)
	}

	return entities.ChartDrawingBoard{
		ID: 3, OwnerID: chartDrawingOwnerID, Market: "spot", Symbol: "BTCUSDT",
		Revision: revision, LastDrawingID: lastDrawingID, Drawings: drawings,
	}
}

func (fixture chartDrawingApplicationUnderTest) boardIs(board entities.ChartDrawingBoard, found bool) {
	fixture.chartDrawingBoardRepository.EXPECT().FindOne(gomock.Any(), chartDrawingOwnerID, board.Market, "BTCUSDT").
		Return(board, found, nil)
}

// savesEcho accepts every write and records it.
func (fixture chartDrawingApplicationUnderTest) savesEcho(expectedRevision int64) *entities.ChartDrawingBoard {
	saved := &entities.ChartDrawingBoard{}
	fixture.chartDrawingBoardRepository.EXPECT().Save(gomock.Any(), gomock.Any(), expectedRevision).
		DoAndReturn(func(_ context.Context, board entities.ChartDrawingBoard, _ int64) (entities.ChartDrawingBoard, error) {
			*saved = board

			return board, nil
		})

	return saved
}

// minutesOfTheHour answers the hourly bucket at 08:00 with three minutes whose lowest low is 61200.
func (fixture chartDrawingApplicationUnderTest) minutesOfTheHour() {
	fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, query domains.KCandleQueryDomain, _ int) ([]entities.KCandle, error) {
			if !query.StartTime().Equal(chartDrawingAnchorHour) {
				return []entities.KCandle{}, nil
			}

			minute := func(offset time.Duration, low string, closePrice string) entities.KCandle {
				return entities.KCandle{
					Symbol: "BTCUSDT", OpenTime: chartDrawingAnchorHour.Add(offset),
					Open: decimal.RequireFromString("61500"), High: decimal.RequireFromString("62400"),
					Low: decimal.RequireFromString(low), Close: decimal.RequireFromString(closePrice),
					Volume: decimal.RequireFromString("1"),
				}
			}

			return []entities.KCandle{
				minute(0, "61400", "61450"), minute(time.Minute, "61200", "61300"), minute(59*time.Minute, "61350", "61800"),
			}, nil
		}).AnyTimes()
}

func snapLowBatch(market string) dto.AssistantChartDrawingBatchDto {
	return dto.AssistantChartDrawingBatchDto{
		Place:    dto.ChartDrawingBoardPlaceDto{OwnerID: chartDrawingOwnerID, Market: market, Symbol: "BTCUSDT"},
		Interval: "1h", ConversationID: 12, TurnID: 34,
		Drawings: []dto.AssistantChartDrawingWriteDto{{
			Kind:    "horizontalLine",
			Anchors: []dto.AssistantChartDrawingAnchorWriteDto{{OpenTime: chartDrawingAnchorHour, SnapTo: "low"}},
		}},
	}
}

func TestChartDrawingApplicationGetBoard(t *testing.T) {
	t.Run("a board never drawn on is empty at revision zero", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)
		fixture.chartDrawingBoardRepository.EXPECT().FindOne(gomock.Any(), chartDrawingOwnerID, "contract", "BTCUSDT").
			Return(entities.ChartDrawingBoard{}, false, nil)

		boardDto, getError := fixture.application.GetBoard(context.Background(),
			dto.ChartDrawingBoardPlaceDto{OwnerID: chartDrawingOwnerID, Market: "contract", Symbol: "BTCUSDT"})

		require.NoError(t, getError)
		assert.Equal(t, dto.ChartDrawingBoardDto{
			Market: "contract", Symbol: "BTCUSDT", Revision: 0, Drawings: []dto.ChartDrawingDto{},
		}, boardDto)
	})

	t.Run("the stored drawings come back in their order", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)
		fixture.boardIs(aStoredBoard(2, aBoardDrawing(2, vo.ChartDrawingSourceManual), aBoardDrawing(1, vo.ChartDrawingSourceAssistant)), true)

		boardDto, getError := fixture.application.GetBoard(context.Background(), spotPlace())

		require.NoError(t, getError)
		assert.Equal(t, int64(2), boardDto.Revision)
		assert.Equal(t, []uint{2, 1}, []uint{boardDto.Drawings[0].ID, boardDto.Drawings[1].ID})
	})

	t.Run("a market other than spot or contract is refused", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)

		_, getError := fixture.application.GetBoard(context.Background(),
			dto.ChartDrawingBoardPlaceDto{OwnerID: chartDrawingOwnerID, Market: "option", Symbol: "BTCUSDT"})

		assert.ErrorIs(t, getError, domains.ErrChartDrawingValidation)
	})

	t.Run("a failed read passes through", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)
		fixture.chartDrawingBoardRepository.EXPECT().FindOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entities.ChartDrawingBoard{}, false, errChartDrawingStorageDown)

		_, getError := fixture.application.GetBoard(context.Background(), spotPlace())

		assert.ErrorIs(t, getError, errChartDrawingStorageDown)
	})
}

func TestChartDrawingApplicationReplaceBoard(t *testing.T) {
	newHorizontalLine := dto.ChartDrawingWriteDto{
		Kind:    "horizontalLine",
		Anchors: []dto.ChartDrawingAnchorDto{{OpenTime: chartDrawingAnchorHour.Add(48 * time.Hour), Price: decimal.RequireFromString("250")}},
	}

	t.Run("the saved board replaces the stored one over its revision", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)
		fixture.boardIs(aStoredBoard(3, aBoardDrawing(1, vo.ChartDrawingSourceManual), aBoardDrawing(2, vo.ChartDrawingSourceAssistant)), true)
		saved := fixture.savesEcho(3)

		boardDto, replaceError := fixture.application.ReplaceBoard(context.Background(), dto.ChartDrawingBoardReplaceDto{
			Place: spotPlace(), Revision: 3, Drawings: []dto.ChartDrawingWriteDto{
				{ID: 1, Kind: "horizontalLine", Anchors: aBoardDrawing(1, vo.ChartDrawingSourceManual).Anchors}, newHorizontalLine,
			},
		})

		require.NoError(t, replaceError)
		assert.Equal(t, int64(4), boardDto.Revision)
		assert.Equal(t, []uint{1, 3}, []uint{boardDto.Drawings[0].ID, boardDto.Drawings[1].ID})
		assert.Equal(t, "manual", boardDto.Drawings[1].Source)
		assert.Equal(t, int64(4), saved.Revision)
		assert.Equal(t, uint(3), saved.ID)
	})

	t.Run("the first save of a board inserts it", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)
		fixture.boardIs(entities.ChartDrawingBoard{Market: "spot"}, false)
		saved := fixture.savesEcho(0)

		boardDto, replaceError := fixture.application.ReplaceBoard(context.Background(), dto.ChartDrawingBoardReplaceDto{
			Place: spotPlace(), Revision: 0, Drawings: []dto.ChartDrawingWriteDto{newHorizontalLine},
		})

		require.NoError(t, replaceError)
		assert.Equal(t, int64(1), boardDto.Revision)
		assert.Equal(t, chartDrawingOwnerID, saved.OwnerID)
		assert.Equal(t, "BTCUSDT", saved.Symbol)
	})

	t.Run("a hand drawing in the future is saved without reading the market", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)
		fixture.boardIs(entities.ChartDrawingBoard{Market: "spot"}, false)
		fixture.savesEcho(0)

		boardDto, replaceError := fixture.application.ReplaceBoard(context.Background(), dto.ChartDrawingBoardReplaceDto{
			Place: spotPlace(), Revision: 0, Drawings: []dto.ChartDrawingWriteDto{newHorizontalLine},
		})

		require.NoError(t, replaceError)
		assert.True(t, boardDto.Drawings[0].Anchors[0].OpenTime.After(time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)))
	})

	t.Run("a board changed since is refused with the board as it is, writing nothing", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)
		fixture.boardIs(aStoredBoard(4, aBoardDrawing(1, vo.ChartDrawingSourceAssistant)), true)

		_, replaceError := fixture.application.ReplaceBoard(context.Background(), dto.ChartDrawingBoardReplaceDto{
			Place: spotPlace(), Revision: 3,
		})

		revisionConflictError, isRevisionConflict := errors.AsType[domains.ChartDrawingBoardRevisionConflictError](replaceError)
		require.True(t, isRevisionConflict)
		assert.Equal(t, int64(4), revisionConflictError.CurrentBoard.Revision)
		assert.Len(t, revisionConflictError.CurrentBoard.Drawings, 1)
	})

	t.Run("a write that loses the race is refused with the board the other write left", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)
		gomock.InOrder(
			fixture.chartDrawingBoardRepository.EXPECT().FindOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(aStoredBoard(3), true, nil),
			fixture.chartDrawingBoardRepository.EXPECT().Save(gomock.Any(), gomock.Any(), int64(3)).
				Return(entities.ChartDrawingBoard{}, domains.ErrChartDrawingBoardRevisionConflict),
			fixture.chartDrawingBoardRepository.EXPECT().FindOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(aStoredBoard(4, aBoardDrawing(1, vo.ChartDrawingSourceAssistant)), true, nil),
		)

		_, replaceError := fixture.application.ReplaceBoard(context.Background(), dto.ChartDrawingBoardReplaceDto{
			Place: spotPlace(), Revision: 3,
		})

		revisionConflictError, isRevisionConflict := errors.AsType[domains.ChartDrawingBoardRevisionConflictError](replaceError)
		require.True(t, isRevisionConflict)
		assert.Equal(t, int64(4), revisionConflictError.CurrentBoard.Revision)
	})

	t.Run("a failed write passes through", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)
		fixture.boardIs(aStoredBoard(1), true)
		fixture.chartDrawingBoardRepository.EXPECT().Save(gomock.Any(), gomock.Any(), int64(1)).
			Return(entities.ChartDrawingBoard{}, errChartDrawingStorageDown)

		_, replaceError := fixture.application.ReplaceBoard(context.Background(), dto.ChartDrawingBoardReplaceDto{
			Place: spotPlace(), Revision: 1,
		})

		assert.ErrorIs(t, replaceError, errChartDrawingStorageDown)
	})

	t.Run("a bad place is refused before reading", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)

		_, replaceError := fixture.application.ReplaceBoard(context.Background(), dto.ChartDrawingBoardReplaceDto{
			Place: dto.ChartDrawingBoardPlaceDto{Market: "spot"},
		})

		assert.ErrorIs(t, replaceError, domains.ErrChartDrawingValidation)
	})

	t.Run("a failed read passes through", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)
		fixture.chartDrawingBoardRepository.EXPECT().FindOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entities.ChartDrawingBoard{}, false, errChartDrawingStorageDown)

		_, replaceError := fixture.application.ReplaceBoard(context.Background(), dto.ChartDrawingBoardReplaceDto{Place: spotPlace()})

		assert.ErrorIs(t, replaceError, errChartDrawingStorageDown)
	})

	t.Run("a conflict whose reread fails reports the failed read", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)
		gomock.InOrder(
			fixture.chartDrawingBoardRepository.EXPECT().FindOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(aStoredBoard(3), true, nil),
			fixture.chartDrawingBoardRepository.EXPECT().Save(gomock.Any(), gomock.Any(), int64(3)).
				Return(entities.ChartDrawingBoard{}, domains.ErrChartDrawingBoardRevisionConflict),
			fixture.chartDrawingBoardRepository.EXPECT().FindOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(entities.ChartDrawingBoard{}, false, errChartDrawingStorageDown),
		)

		_, replaceError := fixture.application.ReplaceBoard(context.Background(), dto.ChartDrawingBoardReplaceDto{
			Place: spotPlace(), Revision: 3,
		})

		assert.ErrorIs(t, replaceError, errChartDrawingStorageDown)
	})
}

func TestChartDrawingApplicationAddAssistantDrawings(t *testing.T) {
	t.Run("a snapped anchor takes the lowest low of the hour and the drawing records the answer", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)
		fixture.minutesOfTheHour()
		fixture.boardIs(aStoredBoard(2, aBoardDrawing(1, vo.ChartDrawingSourceManual)), true)
		saved := fixture.savesEcho(2)

		additionDto, addError := fixture.application.AddAssistantDrawings(context.Background(), snapLowBatch("spot"))

		require.NoError(t, addError)
		assert.Equal(t, int64(3), additionDto.Revision)
		require.Len(t, additionDto.AddedDrawings, 1)
		addedDrawing := additionDto.AddedDrawings[0]
		assert.Equal(t, uint(2), addedDrawing.ID)
		assert.Equal(t, "61200", addedDrawing.Anchors[0].Price.String())
		assert.Equal(t, "assistant", addedDrawing.Source)
		assert.Equal(t, uint(34), *addedDrawing.AssistantTurnID)
		assert.Len(t, saved.Drawings, 2)
	})

	t.Run("a contract board pins anchors to contract candles", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)
		fixture.kCandleContractRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandleContract{{
				Symbol: "BTCUSDT", OpenTime: chartDrawingAnchorHour,
				Open: decimal.RequireFromString("61500"), High: decimal.RequireFromString("62400"),
				Low: decimal.RequireFromString("61150"), Close: decimal.RequireFromString("61850"),
			}}, nil)
		fixture.boardIs(entities.ChartDrawingBoard{Market: "contract"}, false)
		fixture.savesEcho(0)

		additionDto, addError := fixture.application.AddAssistantDrawings(context.Background(), snapLowBatch("contract"))

		require.NoError(t, addError)
		assert.Equal(t, "61150", additionDto.AddedDrawings[0].Anchors[0].Price.String())
		assert.Equal(t, int64(1), additionDto.Revision)
	})

	t.Run("an anchor with no candle refuses the batch and writes nothing", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandle{}, nil)

		_, addError := fixture.application.AddAssistantDrawings(context.Background(), snapLowBatch("spot"))

		assert.ErrorIs(t, addError, domains.ErrChartDrawingValidation)
		assert.ErrorContains(t, addError, "沒有 1h K 線")
	})

	t.Run("a closed day of a market that closes has no candle to pin to", func(t *testing.T) {
		mockController := gomock.NewController(t)
		clockProxy := mocks.NewMockIClockProxy(mockController)
		clockProxy.EXPECT().Now().Return(time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)).AnyTimes()
		tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
		tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "2330").
			Return(entities.TradingSymbol{Symbol: "2330", Market: string(vo.MarketTaiwanStock)}, true, nil)
		kCandleRepository := mocks.NewMockIKCandleRepository(mockController)
		kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).Return([]entities.KCandle{}, nil).AnyTimes()
		summaryDayRepository := mocks.NewMockIKCandleSummaryDayRepository(mockController)
		summaryDayRepository.EXPECT().FindDaysInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, nil).AnyTimes()
		marketCatalog := domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}, vo.MarketTaiwanStock: {}})
		taiwanApplication := application.NewChartDrawingApplication(service.NewChartDrawingService(
			mocks.NewMockIChartDrawingBoardRepository(mockController),
			service.NewKCandleService(kCandleRepository, summaryDayRepository, tradingSymbolRepository, clockProxy, marketCatalog, 1000),
			service.NewKCandleContractService(mocks.NewMockIKCandleContractRepository(mockController),
				mocks.NewMockIKCandleContractSummaryDayRepository(mockController), clockProxy, marketCatalog, 1000),
			vo.ChartDrawingLimitsVo{BoardCapacity: 200, AssistantBatchCapacity: 20}))
		saturday := time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)

		_, addError := taiwanApplication.AddAssistantDrawings(context.Background(), dto.AssistantChartDrawingBatchDto{
			Place:    dto.ChartDrawingBoardPlaceDto{OwnerID: chartDrawingOwnerID, Market: "spot", Symbol: "2330"},
			Interval: "1d",
			Drawings: []dto.AssistantChartDrawingWriteDto{{
				Kind: "horizontalLine", Anchors: []dto.AssistantChartDrawingAnchorWriteDto{{OpenTime: saturday, SnapTo: "close"}},
			}},
		})

		assert.ErrorIs(t, addError, domains.ErrChartDrawingValidation)
		assert.ErrorContains(t, addError, "2026-10-17T00:00:00Z 沒有 1d K 線")
	})

	t.Run("one drawing breaking the rules refuses the batch, naming it, before anything is written", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)
		batch := snapLowBatch("spot")
		batch.Drawings = append(batch.Drawings,
			dto.AssistantChartDrawingWriteDto{Kind: "trendLine", Anchors: batch.Drawings[0].Anchors},
			batch.Drawings[0])

		_, addError := fixture.application.AddAssistantDrawings(context.Background(), batch)

		assert.ErrorIs(t, addError, domains.ErrChartDrawingValidation)
		assert.ErrorContains(t, addError, "第 2 個繪圖：趨勢線要 2 個錨點，收到 1 個")
	})

	t.Run("a batch that cannot be judged is refused without reading the market", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)
		batch := snapLowBatch("spot")
		batch.Interval = "2h"

		_, addError := fixture.application.AddAssistantDrawings(context.Background(), batch)

		assert.ErrorIs(t, addError, domains.ErrChartDrawingValidation)
	})

	t.Run("a failed market read refuses the batch", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, errChartDrawingStorageDown)

		_, addError := fixture.application.AddAssistantDrawings(context.Background(), snapLowBatch("spot"))

		assert.ErrorIs(t, addError, errChartDrawingStorageDown)
	})

	t.Run("a failed contract market read refuses the batch", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)
		fixture.kCandleContractRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, errChartDrawingStorageDown)

		_, addError := fixture.application.AddAssistantDrawings(context.Background(), snapLowBatch("contract"))

		assert.ErrorIs(t, addError, errChartDrawingStorageDown)
	})

	t.Run("a board too full for the batch is refused", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)
		fixture.minutesOfTheHour()
		fullDrawings := make([]dto.ChartDrawingDto, 0, 200)
		for position := range 200 {
			fullDrawings = append(fullDrawings, aBoardDrawing(uint(position+1), vo.ChartDrawingSourceManual))
		}
		fixture.boardIs(aStoredBoard(9, fullDrawings...), true)

		_, addError := fixture.application.AddAssistantDrawings(context.Background(), snapLowBatch("spot"))

		assert.ErrorContains(t, addError, "這份只剩 0 個空位")
	})

	t.Run("a board written in between is reread and the batch appended to it", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)
		fixture.minutesOfTheHour()
		gomock.InOrder(
			fixture.chartDrawingBoardRepository.EXPECT().FindOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(aStoredBoard(2), true, nil),
			fixture.chartDrawingBoardRepository.EXPECT().Save(gomock.Any(), gomock.Any(), int64(2)).
				Return(entities.ChartDrawingBoard{}, domains.ErrChartDrawingBoardRevisionConflict),
			fixture.chartDrawingBoardRepository.EXPECT().FindOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(aStoredBoard(3, aBoardDrawing(1, vo.ChartDrawingSourceManual)), true, nil),
			fixture.chartDrawingBoardRepository.EXPECT().Save(gomock.Any(), gomock.Any(), int64(3)).
				Return(entities.ChartDrawingBoard{}, nil),
		)

		additionDto, addError := fixture.application.AddAssistantDrawings(context.Background(), snapLowBatch("spot"))

		require.NoError(t, addError)
		assert.Equal(t, int64(4), additionDto.Revision)
		assert.Equal(t, uint(2), additionDto.AddedDrawings[0].ID)
	})

	t.Run("losing the race three times reports a revision conflict", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)
		fixture.minutesOfTheHour()
		fixture.chartDrawingBoardRepository.EXPECT().FindOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(aStoredBoard(2), true, nil).Times(4)
		fixture.chartDrawingBoardRepository.EXPECT().Save(gomock.Any(), gomock.Any(), int64(2)).
			Return(entities.ChartDrawingBoard{}, domains.ErrChartDrawingBoardRevisionConflict).Times(3)

		_, addError := fixture.application.AddAssistantDrawings(context.Background(), snapLowBatch("spot"))

		_, isRevisionConflict := errors.AsType[domains.ChartDrawingBoardRevisionConflictError](addError)
		assert.True(t, isRevisionConflict)
	})

	t.Run("a failed board read passes through", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)
		fixture.minutesOfTheHour()
		fixture.chartDrawingBoardRepository.EXPECT().FindOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entities.ChartDrawingBoard{}, false, errChartDrawingStorageDown)

		_, addError := fixture.application.AddAssistantDrawings(context.Background(), snapLowBatch("spot"))

		assert.ErrorIs(t, addError, errChartDrawingStorageDown)
	})

	t.Run("a failed write passes through", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)
		fixture.minutesOfTheHour()
		fixture.boardIs(aStoredBoard(2), true)
		fixture.chartDrawingBoardRepository.EXPECT().Save(gomock.Any(), gomock.Any(), int64(2)).
			Return(entities.ChartDrawingBoard{}, errChartDrawingStorageDown)

		_, addError := fixture.application.AddAssistantDrawings(context.Background(), snapLowBatch("spot"))

		assert.ErrorIs(t, addError, errChartDrawingStorageDown)
	})
}

func TestChartDrawingApplicationRemoveAssistantDrawings(t *testing.T) {
	bothSources := func() entities.ChartDrawingBoard {
		return aStoredBoard(5, aBoardDrawing(1, vo.ChartDrawingSourceManual),
			aBoardDrawing(2, vo.ChartDrawingSourceAssistant), aBoardDrawing(3, vo.ChartDrawingSourceAssistant))
	}

	t.Run("every assistant drawing goes and the hand drawing stays", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)
		fixture.boardIs(bothSources(), true)
		saved := fixture.savesEcho(5)

		resultDto, removeError := fixture.application.RemoveAssistantDrawings(context.Background(),
			dto.AssistantChartDrawingRemovalDto{Place: spotPlace()})

		require.NoError(t, removeError)
		assert.Equal(t, dto.AssistantChartDrawingRemovalResultDto{
			Revision: 6, RemovedCount: 2, RemovedDrawingIDs: []uint{2, 3},
		}, resultDto)
		require.Len(t, saved.Drawings, 1)
		assert.Equal(t, uint(1), saved.Drawings[0].ID)
	})

	t.Run("naming a hand drawing removes nothing", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)
		fixture.boardIs(bothSources(), true)

		_, removeError := fixture.application.RemoveAssistantDrawings(context.Background(),
			dto.AssistantChartDrawingRemovalDto{Place: spotPlace(), DrawingIDs: []uint{1, 2}})

		assert.ErrorIs(t, removeError, domains.ErrChartDrawingHandDrawn)
	})

	t.Run("nothing of the assistant's writes nothing", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)
		fixture.boardIs(aStoredBoard(5, aBoardDrawing(1, vo.ChartDrawingSourceManual)), true)

		resultDto, removeError := fixture.application.RemoveAssistantDrawings(context.Background(),
			dto.AssistantChartDrawingRemovalDto{Place: spotPlace()})

		require.NoError(t, removeError)
		assert.Equal(t, dto.AssistantChartDrawingRemovalResultDto{
			Revision: 5, RemovedCount: 0, RemovedDrawingIDs: []uint{},
		}, resultDto)
	})

	t.Run("a bad place is refused", func(t *testing.T) {
		fixture := newChartDrawingApplicationUnderTest(t)

		_, removeError := fixture.application.RemoveAssistantDrawings(context.Background(),
			dto.AssistantChartDrawingRemovalDto{Place: dto.ChartDrawingBoardPlaceDto{Market: "option", Symbol: "BTCUSDT"}})

		assert.ErrorIs(t, removeError, domains.ErrChartDrawingValidation)
	})
}
