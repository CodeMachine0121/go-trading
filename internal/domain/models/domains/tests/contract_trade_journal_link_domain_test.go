package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
)

func aRoundSuggestingALong() dto.StrategyBotRoundDto {
	return dto.StrategyBotRoundDto{
		BotName: "BTC 趨勢跟隨", Symbol: "BTCUSDT", MarketDataKind: string(vo.MarketDataKindContractKCandle),
		Verdict: string(vo.SignalBuy), HasPositionPlan: true,
		PositionPlan: dto.PositionPlanDto{Affordable: true, Direction: "long", ForContract: true},
	}
}

func TestContractTradeJournalLinkDomainOffersALinkOnlyForAPositionToOpen(t *testing.T) {
	testCases := []struct {
		name          string
		shape         func(round *dto.StrategyBotRoundDto)
		expectedOffer bool
	}{
		{name: "a contract long with a suggestion", shape: func(*dto.StrategyBotRoundDto) {}, expectedOffer: true},
		{name: "a contract short with a suggestion", shape: func(round *dto.StrategyBotRoundDto) {
			round.PositionPlan.Direction = "short"
		}, expectedOffer: true},
		{name: "a spot round", shape: func(round *dto.StrategyBotRoundDto) {
			round.MarketDataKind = string(vo.MarketDataKindKCandle)
		}},
		{name: "a close with no suggestion", shape: func(round *dto.StrategyBotRoundDto) {
			round.HasPositionPlan = false
			round.PositionPlan = dto.PositionPlanDto{}
		}},
		{name: "a stake the capital cannot cover", shape: func(round *dto.StrategyBotRoundDto) {
			round.PositionPlan.Affordable = false
		}},
		{name: "an order the venue would refuse", shape: func(round *dto.StrategyBotRoundDto) {
			round.PositionPlan.HasVenueRefusal = true
		}},
		{name: "a suggestion without a direction", shape: func(round *dto.StrategyBotRoundDto) {
			round.PositionPlan.Direction = ""
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			round := aRoundSuggestingALong()
			testCase.shape(&round)

			assert.Equal(t, testCase.expectedOffer, domains.NewContractTradeJournalLinkDomain(round).Offered())
		})
	}
}

func TestContractTradeJournalLinkDomainPointsAtTheNewTradePage(t *testing.T) {
	url := domains.NewContractTradeJournalLinkDomain(aRoundSuggestingALong()).
		UrlFor("https://app.example.com", "a b&c")

	assert.Equal(t, "https://app.example.com/contract-trade-journal/new?journalLink=a+b%26c", url)
}

func TestStrategyBotMessageEndsWithTheJournalLinkWhenOffered(t *testing.T) {
	round := aRoundSuggestingALong()
	withoutLink := domains.NewStrategyBotMessageDomain(round).Text()
	round.JournalLinkUrl = "https://app.example.com/contract-trade-journal/new?journalLink=abc"

	withLink := domains.NewStrategyBotMessageDomain(round).Text()

	assert.NotContains(t, withoutLink, "記到交易日誌")
	assert.Equal(t, withoutLink+"\n\n📝 記到交易日誌：https://app.example.com/contract-trade-journal/new?journalLink=abc", withLink)
}
