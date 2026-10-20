package application_test

import (
	"context"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// sharesOf writes symbol and percentage pairs as the owner would send them.
func sharesOf(symbolsAndPercentages ...string) []dto.StrategyBotCapitalAllocationDto {
	shares := []dto.StrategyBotCapitalAllocationDto{}
	for index := 0; index < len(symbolsAndPercentages); index += 2 {
		shares = append(shares, dto.StrategyBotCapitalAllocationDto{
			Symbol:               symbolsAndPercentages[index],
			AllocationPercentage: decimal.RequireFromString(symbolsAndPercentages[index+1]),
		})
	}

	return shares
}

// savedShares reads back each saved symbol's share as text, in the saved order.
func savedShares(bot entities.StrategyBot) map[string]string {
	shares := map[string]string{}
	for _, watchedSymbol := range bot.WatchedSymbols {
		shares[watchedSymbol.Symbol] = watchedSymbol.AllocationPercentage.String()
	}

	return shares
}

func TestStrategyBotApplicationSharesTheCapitalAmongTheWatchedSymbols(t *testing.T) {
	sevenSymbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT", "ADAUSDT", "DOGEUSDT"}
	sevenEqualShares := map[string]string{}
	for _, symbol := range sevenSymbols {
		sevenEqualShares[symbol] = "14.28"
	}

	testCases := []struct {
		name           string
		symbols        []string
		shares         []dto.StrategyBotCapitalAllocationDto
		expectedShares map[string]string
	}{
		{name: "every symbol given its share", symbols: []string{"BTCUSDT", "ETHUSDT"},
			shares: sharesOf("BTCUSDT", "60", "ETHUSDT", "40"), expectedShares: map[string]string{"BTCUSDT": "60", "ETHUSDT": "40"}},
		{name: "shares summing to exactly 100", symbols: []string{"BTCUSDT", "ETHUSDT"},
			shares: sharesOf("BTCUSDT", "50", "ETHUSDT", "50"), expectedShares: map[string]string{"BTCUSDT": "50", "ETHUSDT": "50"}},
		{name: "shares leaving some capital unused", symbols: []string{"BTCUSDT", "ETHUSDT"},
			shares: sharesOf("BTCUSDT", "30", "ETHUSDT", "30"), expectedShares: map[string]string{"BTCUSDT": "30", "ETHUSDT": "30"}},
		{name: "shares named in other spellings and order", symbols: []string{"BTCUSDT", "ETHUSDT"},
			shares: sharesOf(" ethusdt ", "40", "btcusdt", "60"), expectedShares: map[string]string{"BTCUSDT": "60", "ETHUSDT": "40"}},
		{name: "one symbol given nothing takes the whole", symbols: []string{"BTCUSDT"},
			expectedShares: map[string]string{"BTCUSDT": "100"}},
		{name: "three symbols given nothing split equally, the remainder unused", symbols: []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"},
			expectedShares: map[string]string{"BTCUSDT": "33.33", "ETHUSDT": "33.33", "SOLUSDT": "33.33"}},
		{name: "seven symbols given nothing split equally", symbols: sevenSymbols, expectedShares: sevenEqualShares},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newStrategyBotApplicationUnderTest(t)
			underTest.expectTheNamedTradingStrategyIsThisPersons()
			underTest.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, bot entities.StrategyBot) (entities.StrategyBot, error) {
					assert.Equal(t, testCase.expectedShares, savedShares(bot))

					return bot, nil
				})
			writeDto := aBotWrite()
			writeDto.Symbols = testCase.symbols
			writeDto.CapitalAllocations = testCase.shares

			botDto, createError := underTest.strategyBotApplication.CreateStrategyBot(
				context.Background(), strategyBotOwnerID, writeDto)

			require.NoError(t, createError)
			shownShares := map[string]string{}
			for _, watchedSymbol := range botDto.WatchedSymbols {
				shownShares[watchedSymbol.Symbol] = watchedSymbol.AllocationPercentage.String()
			}
			assert.Equal(t, testCase.expectedShares, shownShares)
		})
	}
}

func TestStrategyBotApplicationRefusesSharesItCannotHonour(t *testing.T) {
	testCases := []struct {
		name            string
		shares          []dto.StrategyBotCapitalAllocationDto
		expectedMessage string
	}{
		{name: "shares summing past 100", shares: sharesOf("BTCUSDT", "70", "ETHUSDT", "40"),
			expectedMessage: "分配比例加起來是 110%，不得超過 100%"},
		{name: "a share of nothing", shares: sharesOf("BTCUSDT", "0", "ETHUSDT", "40"),
			expectedMessage: "BTCUSDT 的分配比例必須大於 0、不超過 100（單位：%）"},
		{name: "a negative share", shares: sharesOf("BTCUSDT", "-5", "ETHUSDT", "40"),
			expectedMessage: "BTCUSDT 的分配比例必須大於 0、不超過 100（單位：%）"},
		{name: "a share past the whole", shares: sharesOf("BTCUSDT", "100.5", "ETHUSDT", "40"),
			expectedMessage: "BTCUSDT 的分配比例必須大於 0、不超過 100（單位：%）"},
		{name: "a share finer than two decimal places", shares: sharesOf("BTCUSDT", "33.333", "ETHUSDT", "40"),
			expectedMessage: "BTCUSDT 的分配比例最多到小數點後兩位"},
		{name: "a symbol left without a share", shares: sharesOf("BTCUSDT", "60"),
			expectedMessage: "ETHUSDT 沒有分配比例——要嘛每個監聽標的都給，要嘛都不給"},
		{name: "a share for a symbol the bot does not watch", shares: sharesOf("BTCUSDT", "40", "ETHUSDT", "40", "SOLUSDT", "10"),
			expectedMessage: "SOLUSDT 不是這台機器人的監聽標的"},
		{name: "one symbol given two shares", shares: sharesOf("btcusdt", "50", "BTCUSDT", "30"),
			expectedMessage: "BTCUSDT 的分配比例給了兩次"},
		{name: "a share naming no symbol", shares: sharesOf("  ", "50"),
			expectedMessage: "資金分配裡的交易標的有誤"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newStrategyBotApplicationUnderTest(t)
			underTest.expectTheNamedTradingStrategyIsThisPersons()
			writeDto := aBotWrite()
			writeDto.Symbols = []string{"BTCUSDT", "ETHUSDT"}
			writeDto.CapitalAllocations = testCase.shares

			_, createError := underTest.strategyBotApplication.CreateStrategyBot(
				context.Background(), strategyBotOwnerID, writeDto)

			require.ErrorIs(t, createError, domains.ErrStrategyBotValidation)
			assert.ErrorContains(t, createError, testCase.expectedMessage)
		})
	}
}

func TestStrategyBotApplicationRewriteKeepsOrResplitsTheShares(t *testing.T) {
	testCases := []struct {
		name           string
		symbols        []string
		shares         []dto.StrategyBotCapitalAllocationDto
		expectedShares map[string]string
	}{
		{name: "the same symbols given no shares keep theirs", symbols: []string{"BTCUSDT", "ETHUSDT"},
			expectedShares: map[string]string{"BTCUSDT": "70", "ETHUSDT": "30"}},
		{name: "the same symbols in another order keep theirs", symbols: []string{"ETHUSDT", "BTCUSDT"},
			expectedShares: map[string]string{"BTCUSDT": "70", "ETHUSDT": "30"}},
		{name: "a symbol added splits equally", symbols: []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"},
			expectedShares: map[string]string{"BTCUSDT": "33.33", "ETHUSDT": "33.33", "SOLUSDT": "33.33"}},
		{name: "a symbol removed splits equally", symbols: []string{"BTCUSDT"},
			expectedShares: map[string]string{"BTCUSDT": "100"}},
		{name: "shares given replace the stored ones", symbols: []string{"BTCUSDT", "ETHUSDT"},
			shares: sharesOf("BTCUSDT", "20", "ETHUSDT", "80"), expectedShares: map[string]string{"BTCUSDT": "20", "ETHUSDT": "80"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newStrategyBotApplicationUnderTest(t)
			underTest.expectTheNamedTradingStrategyIsThisPersons()
			stored := aStoppedBotWatchingBitcoinAndEthereum("")
			stored.WatchedSymbols[0].AllocationPercentage = decimal.NewFromInt(70)
			stored.WatchedSymbols[1].AllocationPercentage = decimal.NewFromInt(30)
			underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).Return(stored, nil).AnyTimes()
			underTest.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, bot entities.StrategyBot) (entities.StrategyBot, error) {
					assert.Equal(t, testCase.expectedShares, savedShares(bot))

					return bot, nil
				})
			writeDto := aBotWrite()
			writeDto.ID = strategyBotID
			writeDto.Symbols = testCase.symbols
			writeDto.CapitalAllocations = testCase.shares

			_, updateError := underTest.strategyBotApplication.UpdateStrategyBot(
				context.Background(), strategyBotOwnerID, writeDto)

			require.NoError(t, updateError)
		})
	}
}

func TestStrategyBotApplicationRefusesToReshareARunningBot(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)
	underTest.expectTheNamedTradingStrategyIsThisPersons()
	running := storedBot(vo.StrategyBotRunning)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).Return(running, nil).AnyTimes()
	writeDto := aBotWrite()
	writeDto.ID = strategyBotID
	writeDto.CapitalAllocations = sharesOf("BTCUSDT", "50")

	_, updateError := underTest.strategyBotApplication.UpdateStrategyBot(
		context.Background(), strategyBotOwnerID, writeDto)

	require.ErrorIs(t, updateError, domains.ErrStrategyBotRunning)
}
