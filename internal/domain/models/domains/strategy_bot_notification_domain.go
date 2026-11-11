package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// StrategyBotNotificationDomain decides which conclusions send their round message; it only decides whether to speak, never what the bot concludes or does.
type StrategyBotNotificationDomain struct {
	notifications dto.StrategyBotNotificationsDto
}

// NewStrategyBotNotificationDomain reads settings already settled, as a stored bot holds them.
func NewStrategyBotNotificationDomain(notifications dto.StrategyBotNotificationsDto) StrategyBotNotificationDomain {
	return StrategyBotNotificationDomain{notifications: notifications}
}

// NewStrategyBotNotificationDomainSettling fills what was not said with the retained settings on a rewrite, or else the defaults: buy and sell on, hold off, as every bot behaved before the choice existed.
func NewStrategyBotNotificationDomainSettling(
	declared dto.StrategyBotNotificationsWriteDto, retained *dto.StrategyBotNotificationsDto,
) StrategyBotNotificationDomain {
	notifications := dto.StrategyBotNotificationsDto{Buy: true, Sell: true, Hold: false}
	if retained != nil {
		notifications = *retained
	}
	if declared.Buy != nil {
		notifications.Buy = *declared.Buy
	}
	if declared.Sell != nil {
		notifications.Sell = *declared.Sell
	}
	if declared.Hold != nil {
		notifications.Hold = *declared.Hold
	}

	return StrategyBotNotificationDomain{notifications: notifications}
}

// NotifiesSignal answers for a buy or sell the round decided to send.
func (notificationDomain StrategyBotNotificationDomain) NotifiesSignal(signal vo.SignalVo) bool {
	switch signal {
	case vo.SignalBuy:
		return notificationDomain.notifications.Buy
	case vo.SignalSell:
		return notificationDomain.notifications.Sell
	default:
		return false
	}
}

// NotifiesHold answers for a watched symbol returning to holding.
func (notificationDomain StrategyBotNotificationDomain) NotifiesHold() bool {
	return notificationDomain.notifications.Hold
}

func (notificationDomain StrategyBotNotificationDomain) ToDto() dto.StrategyBotNotificationsDto {
	return notificationDomain.notifications
}
