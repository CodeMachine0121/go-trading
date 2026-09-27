package domains_test

import (
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var tradeOpenedAt = time.Date(2026, 9, 25, 14, 3, 0, 0, time.UTC)

func entryFill(id uint, filledAt time.Time, price string, quantity string) entities.ContractTradeFill {
	return entities.ContractTradeFill{
		ID: id, Kind: string(vo.ContractTradeFillKindEntry), FilledAt: filledAt,
		Price: decimal.RequireFromString(price), Quantity: decimal.RequireFromString(quantity),
		Liquidity: string(vo.TradeFillLiquidityTaker),
	}
}

func exitFill(id uint, filledAt time.Time, price string, quantity string) entities.ContractTradeFill {
	fill := entryFill(id, filledAt, price, quantity)
	fill.Kind = string(vo.ContractTradeFillKindExit)

	return fill
}

func price(value string) decimal.NullDecimal {
	return decimal.NullDecimal{Decimal: decimal.RequireFromString(value), Valid: true}
}

func confidenceOf(value int) *int {
	return &value
}

func openingTrade(t *testing.T, direction string, firstEntryPrice string) domains.ContractTradeRecordDomain {
	recordDomain, err := domains.NewOpeningContractTradeRecordDomain(
		7, "BTCUSDT",
		dto.ContractTradeRecordWriteDto{Direction: direction, Leverage: decimal.NewFromInt(10)},
		entryFill(0, tradeOpenedAt, firstEntryPrice, "0.030"), nil, ledgerNow)
	require.NoError(t, err)

	return recordDomain
}

func closedTrade(t *testing.T) domains.ContractTradeRecordDomain {
	recordDomain := openingTrade(t, "long", "97905")
	require.NoError(t, recordDomain.AddFill(exitFill(0, tradeOpenedAt.Add(time.Hour), "99000", "0.030"), ledgerNow))

	return recordDomain
}

func TestNewOpeningContractTradeRecordDomain(t *testing.T) {
	t.Run("a long opens held by its owner", func(t *testing.T) {
		recordDomain := openingTrade(t, "long", "97905")

		entity := recordDomain.ToEntity()
		assert.Equal(t, uint(7), entity.OwnerID)
		assert.Equal(t, string(vo.ContractTradeStatusOpen), entity.Status)
		assert.True(t, entity.OpenedAt.Equal(tradeOpenedAt))
		assert.Equal(t, "做多", recordDomain.DirectionInWords())
		assert.True(t, recordDomain.IsOpen())
	})

	t.Run("a blank leverage is one", func(t *testing.T) {
		recordDomain, err := domains.NewOpeningContractTradeRecordDomain(
			7, "BTCUSDT", dto.ContractTradeRecordWriteDto{Direction: "short"},
			entryFill(0, tradeOpenedAt, "100", "1"), nil, ledgerNow)

		require.NoError(t, err)
		assert.Equal(t, "1", recordDomain.ToEntity().Leverage.String())
		assert.Equal(t, "做空", recordDomain.DirectionInWords())
	})

	t.Run("a first fill that does not say its kind is the entry", func(t *testing.T) {
		unnamedFill := entryFill(0, tradeOpenedAt, "97905", "0.030")
		unnamedFill.Kind = ""

		recordDomain, err := domains.NewOpeningContractTradeRecordDomain(
			7, "BTCUSDT", dto.ContractTradeRecordWriteDto{Direction: "long"}, unnamedFill, nil, ledgerNow)

		require.NoError(t, err)
		assert.Equal(t, "0.03", recordDomain.Ledger().Position().String())
		assert.Equal(t, "entry", recordDomain.ToEntity().Fills[0].Kind)
	})

	testCases := []struct {
		name            string
		writeDto        dto.ContractTradeRecordWriteDto
		firstEntryFill  entities.ContractTradeFill
		expectedMessage string
	}{
		{name: "a leverage below one",
			writeDto:        dto.ContractTradeRecordWriteDto{Direction: "long", Leverage: decimal.RequireFromString("0.5")},
			firstEntryFill:  entryFill(0, tradeOpenedAt, "100", "1"),
			expectedMessage: "槓桿倍數不得小於一"},
		{name: "a direction that is a close",
			writeDto:        dto.ContractTradeRecordWriteDto{Direction: "平多"},
			firstEntryFill:  entryFill(0, tradeOpenedAt, "100", "1"),
			expectedMessage: "方向只有做多（long）與做空（short）"},
		{name: "no entry fill",
			writeDto:        dto.ContractTradeRecordWriteDto{Direction: "long"},
			firstEntryFill:  exitFill(0, tradeOpenedAt, "100", "1"),
			expectedMessage: "一筆交易的第一筆必須是開倉"},
		{name: "an entry fill that is not a fill",
			writeDto:        dto.ContractTradeRecordWriteDto{Direction: "long"},
			firstEntryFill:  entryFill(0, tradeOpenedAt, "0", "1"),
			expectedMessage: "開倉價、平倉價與數量必須大於零"},
		{name: "a long's stop above its entry",
			writeDto: dto.ContractTradeRecordWriteDto{Direction: "long",
				Plan: dto.ContractTradePlanWriteDto{PlannedStopLossPrice: price("98000")}},
			firstEntryFill:  entryFill(0, tradeOpenedAt, "97905", "1"),
			expectedMessage: "做多的止損必須低於開倉價"},
		{name: "a short's stop below its entry",
			writeDto: dto.ContractTradeRecordWriteDto{Direction: "short",
				Plan: dto.ContractTradePlanWriteDto{PlannedStopLossPrice: price("3450")}},
			firstEntryFill:  entryFill(0, tradeOpenedAt, "3500", "1"),
			expectedMessage: "做空的止損必須高於開倉價"},
		{name: "a long's target below its entry",
			writeDto: dto.ContractTradeRecordWriteDto{Direction: "long",
				Plan: dto.ContractTradePlanWriteDto{PlannedTakeProfitPrice: price("97000")}},
			firstEntryFill:  entryFill(0, tradeOpenedAt, "97905", "1"),
			expectedMessage: "做多的止盈必須高於開倉價"},
		{name: "a short's target above its entry",
			writeDto: dto.ContractTradeRecordWriteDto{Direction: "short",
				Plan: dto.ContractTradePlanWriteDto{PlannedTakeProfitPrice: price("3600")}},
			firstEntryFill:  entryFill(0, tradeOpenedAt, "3500", "1"),
			expectedMessage: "做空的止盈必須低於開倉價"},
		{name: "a negative stop",
			writeDto: dto.ContractTradeRecordWriteDto{Direction: "long",
				Plan: dto.ContractTradePlanWriteDto{PlannedStopLossPrice: price("-1")}},
			firstEntryFill:  entryFill(0, tradeOpenedAt, "3500", "1"),
			expectedMessage: "計畫止損不得為負"},
		{name: "a negative target",
			writeDto: dto.ContractTradeRecordWriteDto{Direction: "short",
				Plan: dto.ContractTradePlanWriteDto{PlannedTakeProfitPrice: price("-1")}},
			firstEntryFill:  entryFill(0, tradeOpenedAt, "3500", "1"),
			expectedMessage: "計畫止盈不得為負"},
		{name: "a confidence of six",
			writeDto: dto.ContractTradeRecordWriteDto{Direction: "long",
				Plan: dto.ContractTradePlanWriteDto{Confidence: confidenceOf(6)}},
			firstEntryFill:  entryFill(0, tradeOpenedAt, "3500", "1"),
			expectedMessage: "信心只能是 1 到 5"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := domains.NewOpeningContractTradeRecordDomain(
				7, "BTCUSDT", testCase.writeDto, testCase.firstEntryFill, nil, ledgerNow)

			require.ErrorIs(t, err, domains.ErrContractTradeValidation)
			assert.Contains(t, err.Error(), testCase.expectedMessage)
		})
	}

	t.Run("a mistake tag cannot be a setup tag", func(t *testing.T) {
		_, err := domains.NewOpeningContractTradeRecordDomain(
			7, "BTCUSDT", dto.ContractTradeRecordWriteDto{Direction: "long"},
			entryFill(0, tradeOpenedAt, "100", "1"),
			[]entities.TradeTag{{ID: 3, Kind: string(vo.TradeTagKindMistake), Name: "追價進場"}}, ledgerNow)

		require.ErrorIs(t, err, domains.ErrContractTradeValidation)
		assert.Contains(t, err.Error(), "標籤「追價進場」不能用在這裡")
	})
}

func TestContractTradeRecordDomainClosesWhenFlat(t *testing.T) {
	recordDomain := closedTrade(t)

	entity := recordDomain.ToEntity()
	assert.Equal(t, string(vo.ContractTradeStatusClosed), entity.Status)
	require.NotNil(t, entity.ClosedAt)
	assert.True(t, entity.ClosedAt.Equal(tradeOpenedAt.Add(time.Hour)))
	assert.False(t, recordDomain.IsOpen())
}

func TestContractTradeRecordDomainLocksAClosedTrade(t *testing.T) {
	testCases := []struct {
		name            string
		change          func(recordDomain *domains.ContractTradeRecordDomain) error
		expectedMessage string
	}{
		{name: "adding a fill", expectedMessage: "這筆交易已經平倉，不能再加倉或減倉",
			change: func(recordDomain *domains.ContractTradeRecordDomain) error {
				return recordDomain.AddFill(entryFill(0, tradeOpenedAt.Add(2*time.Hour), "99000", "0.01"), ledgerNow)
			}},
		{name: "amending a fill", expectedMessage: "平倉後開平倉紀錄已鎖定，可以加附註或刪除整筆重記",
			change: func(recordDomain *domains.ContractTradeRecordDomain) error {
				return recordDomain.AmendFill(1, entryFill(0, tradeOpenedAt, "99000", "0.01"), ledgerNow)
			}},
		{name: "removing a fill", expectedMessage: "平倉後開平倉紀錄已鎖定，可以加附註或刪除整筆重記",
			change: func(recordDomain *domains.ContractTradeRecordDomain) error {
				return recordDomain.RemoveFill(1, ledgerNow)
			}},
		{name: "changing the plan", expectedMessage: "平倉後計畫已鎖定，可以加附註",
			change: func(recordDomain *domains.ContractTradeRecordDomain) error {
				return recordDomain.AmendPlan(dto.ContractTradePlanWriteDto{PlannedStopLossPrice: price("96380")})
			}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			recordDomain := closedTrade(t)

			err := testCase.change(&recordDomain)

			require.ErrorIs(t, err, domains.ErrContractTradeLocked)
			assert.Contains(t, err.Error(), testCase.expectedMessage)
		})
	}
}

func TestContractTradeRecordDomainChangesAnOpenTrade(t *testing.T) {
	t.Run("the plan changes while the trade is held", func(t *testing.T) {
		recordDomain := openingTrade(t, "long", "97905")

		err := recordDomain.AmendPlan(dto.ContractTradePlanWriteDto{
			PlannedStopLossPrice: price("96380"), PlannedTakeProfitPrice: price("100785"),
			EntryReason: "  突破前高  ", Confidence: confidenceOf(3),
		})

		require.NoError(t, err)
		entity := recordDomain.ToEntity()
		assert.Equal(t, "96380", entity.PlannedStopLossPrice.Decimal.String())
		assert.Equal(t, "100785", entity.PlannedTakeProfitPrice.Decimal.String())
		assert.Equal(t, "突破前高", entity.EntryReason)
		assert.Equal(t, 3, *entity.Confidence)
	})

	t.Run("amending the first entry past the stop is refused", func(t *testing.T) {
		recordDomain, err := domains.NewOpeningContractTradeRecordDomain(
			7, "BTCUSDT", dto.ContractTradeRecordWriteDto{Direction: "long",
				Plan: dto.ContractTradePlanWriteDto{PlannedStopLossPrice: price("96380")}},
			entryFill(1, tradeOpenedAt, "97905", "0.030"), nil, ledgerNow)
		require.NoError(t, err)

		amendError := recordDomain.AmendFill(1, entryFill(0, tradeOpenedAt, "96000", "0.030"), ledgerNow)

		require.ErrorIs(t, amendError, domains.ErrContractTradeValidation)
		assert.Contains(t, amendError.Error(), "做多的止損必須低於開倉價")
	})

	t.Run("a refused fill leaves the trade as it was", func(t *testing.T) {
		recordDomain := openingTrade(t, "long", "97905")

		err := recordDomain.AddFill(exitFill(0, tradeOpenedAt.Add(time.Hour), "99000", "1"), ledgerNow)

		require.ErrorIs(t, err, domains.ErrContractTradeValidation)
		assert.Equal(t, "0.03", recordDomain.Ledger().Position().String())
	})

	t.Run("refused amendments and removals pass the ledger's reason through", func(t *testing.T) {
		recordDomain := openingTrade(t, "long", "97905")

		amendError := recordDomain.AmendFill(99, entryFill(0, tradeOpenedAt, "1", "1"), ledgerNow)
		removeError := recordDomain.RemoveFill(99, ledgerNow)

		require.ErrorIs(t, amendError, domains.ErrContractTradeValidation)
		require.ErrorIs(t, removeError, domains.ErrContractTradeValidation)
	})

	t.Run("a removed exit keeps the trade open", func(t *testing.T) {
		recordDomain, err := domains.NewOpeningContractTradeRecordDomain(
			7, "BTCUSDT", dto.ContractTradeRecordWriteDto{Direction: "long"},
			entryFill(1, tradeOpenedAt, "97905", "0.050"), nil, ledgerNow)
		require.NoError(t, err)
		entity := recordDomain.ToEntity()
		entity.Fills = append(entity.Fills, exitFill(2, tradeOpenedAt.Add(time.Hour), "99000", "0.020"))
		reopened := domains.NewContractTradeRecordDomain(entity)

		require.NoError(t, reopened.RemoveFill(2, ledgerNow))
		assert.Equal(t, "0.05", reopened.Ledger().Position().String())
	})
}

func TestContractTradeRecordDomainNotes(t *testing.T) {
	t.Run("a note never touches the locked plan", func(t *testing.T) {
		recordDomain := openingTrade(t, "long", "97905")
		require.NoError(t, recordDomain.AmendPlan(dto.ContractTradePlanWriteDto{PlannedStopLossPrice: price("96380")}))
		require.NoError(t, recordDomain.AddFill(exitFill(0, tradeOpenedAt.Add(time.Hour), "99000", "0.030"), ledgerNow))

		err := recordDomain.AddNote("止損其實是 96,300，當時打錯", ledgerNow)

		require.NoError(t, err)
		entity := recordDomain.ToEntity()
		assert.Equal(t, "96380", entity.PlannedStopLossPrice.Decimal.String())
		require.Len(t, entity.Notes, 1)
		assert.Equal(t, "止損其實是 96,300，當時打錯", entity.Notes[0].Content)
	})

	t.Run("a blank note is refused", func(t *testing.T) {
		recordDomain := openingTrade(t, "long", "97905")

		err := recordDomain.AddNote("   ", ledgerNow)

		require.ErrorIs(t, err, domains.ErrContractTradeValidation)
		assert.Contains(t, err.Error(), "附註不得為空白")
	})
}

func TestContractTradeRecordDomainReviews(t *testing.T) {
	mistakeTag := entities.TradeTag{ID: 4, Kind: string(vo.TradeTagKindMistake), Name: "提早出場"}
	setupTag := entities.TradeTag{ID: 5, Kind: string(vo.TradeTagKindSetup), Name: "突破"}

	t.Run("a closed trade is reviewed", func(t *testing.T) {
		recordDomain := closedTrade(t)
		require.NoError(t, recordDomain.AssignSetupTags([]entities.TradeTag{setupTag}))

		err := recordDomain.WriteReview(dto.ContractTradeReviewWriteDto{
			WentWell: "照計畫進場", WentWrong: "提早出場", NextTime: "讓止盈單成交", ExecutionScore: 4,
		}, []entities.TradeTag{mistakeTag}, ledgerNow)

		require.NoError(t, err)
		entity := recordDomain.ToEntity()
		assert.Equal(t, string(vo.ContractTradeStatusReviewed), entity.Status)
		assert.Equal(t, 4, *entity.ExecutionScore)
		assert.ElementsMatch(t, []entities.TradeTag{setupTag, mistakeTag}, entity.Tags)
	})

	t.Run("a reviewed trade can be reviewed again and stays reviewed", func(t *testing.T) {
		recordDomain := closedTrade(t)
		require.NoError(t, recordDomain.WriteReview(
			dto.ContractTradeReviewWriteDto{ExecutionScore: 4}, nil, ledgerNow))

		err := recordDomain.WriteReview(dto.ContractTradeReviewWriteDto{ExecutionScore: 3}, nil, ledgerNow)

		require.NoError(t, err)
		assert.Equal(t, string(vo.ContractTradeStatusReviewed), recordDomain.ToEntity().Status)
		assert.Equal(t, 3, *recordDomain.ToEntity().ExecutionScore)
	})

	t.Run("an open trade cannot be reviewed", func(t *testing.T) {
		recordDomain := openingTrade(t, "long", "97905")

		err := recordDomain.WriteReview(dto.ContractTradeReviewWriteDto{ExecutionScore: 4}, nil, ledgerNow)

		require.ErrorIs(t, err, domains.ErrContractTradeValidation)
		assert.Contains(t, err.Error(), "平倉後才能檢討")
	})

	t.Run("a score of six is refused", func(t *testing.T) {
		recordDomain := closedTrade(t)

		err := recordDomain.WriteReview(dto.ContractTradeReviewWriteDto{ExecutionScore: 6}, nil, ledgerNow)

		require.ErrorIs(t, err, domains.ErrContractTradeValidation)
		assert.Contains(t, err.Error(), "執行評分只能是 1 到 5")
	})

	t.Run("a setup tag cannot be a mistake", func(t *testing.T) {
		recordDomain := closedTrade(t)

		err := recordDomain.WriteReview(
			dto.ContractTradeReviewWriteDto{ExecutionScore: 4}, []entities.TradeTag{setupTag}, ledgerNow)

		require.ErrorIs(t, err, domains.ErrContractTradeValidation)
	})
}

func TestContractTradeRecordDomainNamesPositionsAsTheVenueDoes(t *testing.T) {
	t.Run("closing more than is held is refused in the venue's words", func(t *testing.T) {
		recordDomain := openingTrade(t, "long", "97905")

		err := recordDomain.AddFill(exitFill(0, tradeOpenedAt.Add(time.Hour), "99000", "5"), ledgerNow)

		require.ErrorIs(t, err, domains.ErrContractTradeValidation)
		assert.Contains(t, err.Error(), "平倉數量超過目前持倉")
		assert.Contains(t, err.Error(), "要反手請先平倉再新增一筆反方向的交易")
		assert.NotContains(t, err.Error(), "成交")
	})

	t.Run("a liquidity that is neither maker nor taker is refused", func(t *testing.T) {
		recordDomain := openingTrade(t, "long", "97905")
		iceberg := entryFill(0, tradeOpenedAt.Add(time.Minute), "97950", "0.01")
		iceberg.Liquidity = "iceberg"

		err := recordDomain.AddFill(iceberg, ledgerNow)

		require.ErrorIs(t, err, domains.ErrContractTradeValidation)
		assert.Contains(t, err.Error(), "掛單或吃單只有 maker 與 taker")
	})

	t.Run("a position the trade does not have cannot be amended or removed", func(t *testing.T) {
		recordDomain := openingTrade(t, "long", "97905")

		amendError := recordDomain.AmendFill(9, entryFill(0, tradeOpenedAt, "1", "1"), ledgerNow)
		removeError := recordDomain.RemoveFill(9, ledgerNow)

		require.ErrorIs(t, amendError, domains.ErrContractTradeValidation)
		require.ErrorIs(t, removeError, domains.ErrContractTradeValidation)
		assert.Contains(t, amendError.Error(), "這筆交易沒有識別碼為 9 的開平倉紀錄")
	})

	t.Run("a fee zero only for want of a rate is remembered", func(t *testing.T) {
		unpriced := entryFill(1, tradeOpenedAt, "97905", "0.030")
		unpriced.FeeRateMissing = true
		recordDomain := domains.NewContractTradeRecordDomain(entities.ContractTradeRecord{
			Status: "open", Fills: []entities.ContractTradeFill{unpriced}})

		assert.True(t, recordDomain.FeeRateMissing())
		assert.False(t, openingTrade(t, "long", "97905").FeeRateMissing())
	})
}

func TestContractTradeRecordDomainNeverSpeaksOfFills(t *testing.T) {
	zeroPrice := entryFill(0, tradeOpenedAt.Add(time.Minute), "0", "0.01")
	future := entryFill(0, ledgerNow.Add(time.Hour), "97950", "0.01")
	earlyClose := exitFill(0, tradeOpenedAt.Add(-time.Hour), "97950", "0.01")
	closingFirst := exitFill(0, tradeOpenedAt, "97905", "0.03")
	_, firstIsCloseError := domains.NewOpeningContractTradeRecordDomain(
		7, "BTCUSDT", dto.ContractTradeRecordWriteDto{Direction: "long"}, closingFirst, nil, ledgerNow)

	held := openingTrade(t, "long", "97905")
	closed := closedTrade(t)
	refusals := []error{
		firstIsCloseError,
		held.AddFill(zeroPrice, ledgerNow),
		held.AddFill(future, ledgerNow),
		held.AddFill(earlyClose, ledgerNow),
		held.AddFill(exitFill(0, tradeOpenedAt.Add(time.Hour), "99000", "5"), ledgerNow),
		held.AmendFill(9, entryFill(0, tradeOpenedAt, "1", "1"), ledgerNow),
		held.AmendPlan(dto.ContractTradePlanWriteDto{PlannedStopLossPrice: price("99000")}),
		closed.AddFill(entryFill(0, tradeOpenedAt.Add(2*time.Hour), "97950", "0.01"), ledgerNow),
		closed.RemoveFill(1, ledgerNow),
	}

	for _, refusal := range refusals {
		require.Error(t, refusal)
		assert.NotContains(t, refusal.Error(), "成交")
		assert.True(t, strings.Contains(refusal.Error(), "開倉") || strings.Contains(refusal.Error(), "平倉") ||
			strings.Contains(refusal.Error(), "時間") || strings.Contains(refusal.Error(), "持倉"), refusal.Error())
	}
}

func TestContractTradeRecordDomainKeepsOnlyAMatchingSource(t *testing.T) {
	testCases := []struct {
		name           string
		roundSymbol    string
		roundDirection string
		expectsSource  bool
	}{
		{name: "the round that suggested this long is kept", roundSymbol: "BTCUSDT", roundDirection: "long", expectsSource: true},
		{name: "a round about another symbol is dropped", roundSymbol: "ETHUSDT", roundDirection: "long", expectsSource: false},
		{name: "a round suggesting the other direction is dropped", roundSymbol: "BTCUSDT", roundDirection: "short", expectsSource: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			recordDomain := openingTrade(t, "long", "97905")

			recordDomain.WithSource(dto.JournalLinkRoundDto{
				StrategyBotID: 5, RunNumber: 412, Symbol: testCase.roundSymbol,
				SuggestedDirection: testCase.roundDirection, ReferencePrice: price("97850"),
			})

			entity := recordDomain.ToEntity()
			assert.Equal(t, testCase.expectsSource, entity.SourceStrategyBotID != nil)
			assert.Equal(t, testCase.expectsSource, entity.SourceReferencePrice.Valid)
		})
	}
}
