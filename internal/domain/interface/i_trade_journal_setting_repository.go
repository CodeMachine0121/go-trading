package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_trade_journal_setting_repository.go -destination=mocks/mock_i_trade_journal_setting_repository.go -package=mocks

// ITradeJournalSettingRepository keys every method by user, since each person has at most one setting.
type ITradeJournalSettingRepository interface {
	FindOneByUser(
		executionContext context.Context, userID uint,
	) (entities.TradeJournalSetting, bool, error)
	// SaveFeeRates writes only the rates, so it never undoes the default-tag marker.
	SaveFeeRates(
		executionContext context.Context, setting entities.TradeJournalSetting,
	) (entities.TradeJournalSetting, error)
	MarkDefaultMistakeTagsSeeded(executionContext context.Context, userID uint, seededAt time.Time) error
}
