package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// ContractAutoOrderMessageDomain writes what an auto order did for its owner's Telegram; a missing stop leads the message, since it is the one thing he must act on at once.
type ContractAutoOrderMessageDomain struct {
	botName string
	symbol  string
	order   ContractAutoOrderDomain
}

func NewContractAutoOrderMessageDomain(
	botName string, symbol string, order ContractAutoOrderDomain,
) ContractAutoOrderMessageDomain {
	return ContractAutoOrderMessageDomain{botName: botName, symbol: symbol, order: order}
}

func (messageDomain ContractAutoOrderMessageDomain) Text() string {
	result := messageDomain.order.ToResultDto()
	lines := []string{}

	if missing := messageDomain.order.MissingProtectionInWords(); missing != "" {
		lines = append(lines, fmt.Sprintf("⚠️ %s沒有掛上，請立刻到幣安自己處理", missing))
	}

	lines = append(lines,
		fmt.Sprintf("🤖【自動下單】%s", messageDomain.botName),
		fmt.Sprintf("%s%s｜%s", messageDomain.symbol, strategyBotContractSymbolSuffix, result.Action))

	if result.ClosedQuantity != nil && result.CloseAveragePrice != nil {
		lines = append(lines, fmt.Sprintf("✅ 平倉 %s @ %s",
			result.ClosedQuantity.String(), result.CloseAveragePrice.String()))
	}
	if result.OpenedQuantity != nil && result.OpenAveragePrice != nil {
		lines = append(lines, fmt.Sprintf("✅ 成交 %s @ %s",
			result.OpenedQuantity.String(), result.OpenAveragePrice.String()))
	}

	exits := []string{}
	if result.StopLossPrice != nil {
		exits = append(exits, "止損 "+result.StopLossPrice.String())
	}
	if result.TakeProfitPrice != nil {
		exits = append(exits, "止盈 "+result.TakeProfitPrice.String())
	}
	if len(exits) > 0 {
		lines = append(lines, "🛡 "+strings.Join(exits, "｜"))
	}

	if result.Reason != "" {
		mark := "❌ "
		switch messageDomain.order.Outcome() {
		case vo.ContractAutoOrderFilled:
			mark = "ℹ️ "
		case vo.ContractAutoOrderPartiallyDone:
			mark = "⚠️ "
		}
		lines = append(lines, mark+result.Reason)
	}

	return strings.Join(lines, "\n")
}
