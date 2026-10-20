package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StrategyBotRepository stores bots; their rules live with the trading strategy they reference.
// It reads through ambientTransactionDatabase so a booked round can land in one transaction with what it caused.
type StrategyBotRepository struct {
	database ambientTransactionDatabase
}

func NewStrategyBotRepository(database *gorm.DB) *StrategyBotRepository {
	return &StrategyBotRepository{database: ambientTransactionDatabase{root: database}}
}

// Save writes the bot and its watched symbols in one transaction, so a bot is never left watching half of what was asked.
func (strategyBotRepository *StrategyBotRepository) Save(
	executionContext context.Context, bot entities.StrategyBot,
) (entities.StrategyBot, error) {
	botRow := bot
	botRow.Owner = entities.User{}
	botRow.TradingStrategy = entities.TradingStrategy{}
	botRow.RunRecords = nil
	botRow.WatchedSymbols = nil

	symbols := bot.Symbols()
	// The ORM's IN clause only takes untyped values.
	symbolValues := make([]any, 0, len(symbols))
	for _, symbol := range symbols {
		symbolValues = append(symbolValues, symbol)
	}

	transactionError := strategyBotRepository.database.within(executionContext).Transaction(
		func(transaction *gorm.DB) error {
			if botRow.ID == 0 {
				if createError := transaction.Omit(clause.Associations).Create(&botRow).Error; createError != nil {
					return createError
				}
			} else {
				// Named columns keep a rewrite from touching lifecycle fields or the immutable market kind, and make empty values written rather than skipped.
				updateError := transaction.
					Model(&entities.StrategyBot{}).
					Where(clause.Eq{Column: "id", Value: botRow.ID}).
					Select(
						"name", "trading_strategy_id", "trigger_interval_minutes",
						"position_plan_capital", "position_plan_sizing_mode",
						"position_plan_sizing_value",
						"position_plan_stop_loss_percentage", "position_plan_take_profit_percentage",
						"position_plan_contract_leverage", "auto_order_entry_slippage_tolerance_percentage").
					Updates(entities.StrategyBot{
						Name:                                      botRow.Name,
						TradingStrategyID:                         botRow.TradingStrategyID,
						TriggerIntervalMinutes:                    botRow.TriggerIntervalMinutes,
						PositionPlanCapital:                       botRow.PositionPlanCapital,
						PositionPlanSizingMode:                    botRow.PositionPlanSizingMode,
						PositionPlanSizingValue:                   botRow.PositionPlanSizingValue,
						PositionPlanStopLossPercentage:            botRow.PositionPlanStopLossPercentage,
						PositionPlanTakeProfitPercentage:          botRow.PositionPlanTakeProfitPercentage,
						PositionPlanLeverage:                      botRow.PositionPlanLeverage,
						AutoOrderEntrySlippageTolerancePercentage: botRow.AutoOrderEntrySlippageTolerancePercentage,
					}).Error
				if updateError != nil {
					return updateError
				}
			}

			// Symbols kept keep everything they remember and take only their new share; dropped ones go and new ones are added in the listed order.
			if deleteError := transaction.
				Where(clause.Eq{Column: "strategy_bot_id", Value: botRow.ID}).
				Where(clause.Not(clause.IN{Column: "symbol", Values: symbolValues})).
				Delete(&entities.StrategyBotWatchedSymbol{}).Error; deleteError != nil {
				return deleteError
			}

			for _, requestedSymbol := range bot.WatchedSymbols {
				watchedSymbol := entities.StrategyBotWatchedSymbol{
					StrategyBotID:        botRow.ID,
					Symbol:               requestedSymbol.Symbol,
					AllocationPercentage: requestedSymbol.AllocationPercentage,
				}
				if createError := transaction.
					Clauses(clause.OnConflict{
						Columns:   []clause.Column{{Name: "strategy_bot_id"}, {Name: "symbol"}},
						DoUpdates: clause.AssignmentColumns([]string{"allocation_percentage"}),
					}).
					Create(&watchedSymbol).Error; createError != nil {
					return createError
				}
			}

			return nil
		})
	if transactionError != nil {
		return entities.StrategyBot{}, strategyBotRepository.writeFailureOf(transactionError, bot.Name)
	}

	return strategyBotRepository.FindOne(executionContext, botRow.ID)
}

// withWatchedSymbols brings a bot's watched symbols along in the order they were added, so every read sees the whole bot.
func (strategyBotRepository *StrategyBotRepository) withWatchedSymbols(database *gorm.DB) *gorm.DB {
	return database.Preload("WatchedSymbols", func(watchedSymbols *gorm.DB) *gorm.DB {
		return watchedSymbols.Order("id ASC")
	})
}

// StrategyBotNameIndex enforces unique bot names per owner; the write path recognises its violation as a name conflict.
const StrategyBotNameIndex = "idx_strategy_bots_owner_name"

// writeFailureOf maps a name-index violation to a name conflict; any other error stays a fault.
func (strategyBotRepository *StrategyBotRepository) writeFailureOf(
	writeError error, name string,
) error {
	postgresError, isPostgresError := errors.AsType[*pgconn.PgError](writeError)
	if isPostgresError &&
		postgresError.Code == uniqueViolationCode &&
		postgresError.ConstraintName == StrategyBotNameIndex {
		return fmt.Errorf("%w: 機器人名稱「%s」已被使用", domains.ErrStrategyBotNameConflict, name)
	}

	return fmt.Errorf("save strategy bot: %w", writeError)
}

func (strategyBotRepository *StrategyBotRepository) FindOne(
	executionContext context.Context, id uint,
) (entities.StrategyBot, error) {
	bot := entities.StrategyBot{}

	// A string condition, because GORM drops zero-valued struct fields and ID zero would match the first bot.
	result := strategyBotRepository.withWatchedSymbols(strategyBotRepository.database.within(executionContext)).
		Preload("TradingStrategy").
		Where(clause.Eq{Column: "id", Value: id}).
		First(&bot)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.StrategyBot{}, domains.StrategyBotNotFound(id)
	}
	if result.Error != nil {
		return entities.StrategyBot{}, fmt.Errorf("find strategy bot: %w", result.Error)
	}

	return bot, nil
}

func (strategyBotRepository *StrategyBotRepository) FindAllByOwner(
	executionContext context.Context, ownerID uint,
) ([]entities.StrategyBot, error) {
	bots := []entities.StrategyBot{}

	result := strategyBotRepository.withWatchedSymbols(strategyBotRepository.database.within(executionContext)).
		Preload("TradingStrategy").
		Where(clause.Eq{Column: "owner_id", Value: ownerID}).
		Order("name ASC").
		Find(&bots)
	if result.Error != nil {
		return nil, fmt.Errorf("list strategy bots: %w", result.Error)
	}

	return bots, nil
}

// FindAllByStrategyScript reaches the script through the signal sources, because a bot names a trading strategy, never a script.
func (strategyBotRepository *StrategyBotRepository) FindAllByStrategyScript(
	executionContext context.Context, strategyScriptID uint,
) ([]entities.StrategyBot, error) {
	database := strategyBotRepository.database.within(executionContext)

	tradingStrategyIDs := []uint{}
	pluckResult := database.
		Model(&entities.TradingStrategySignalSource{}).
		Where(clause.Eq{Column: "strategy_id", Value: strategyScriptID}).
		Distinct().
		Pluck("trading_strategy_id", &tradingStrategyIDs)
	if pluckResult.Error != nil {
		return nil, fmt.Errorf("find trading strategies naming strategy script: %w", pluckResult.Error)
	}

	bots := []entities.StrategyBot{}
	if len(tradingStrategyIDs) == 0 {
		return bots, nil
	}

	// The ORM's IN clause only takes untyped values.
	values := make([]any, 0, len(tradingStrategyIDs))
	for _, tradingStrategyID := range tradingStrategyIDs {
		values = append(values, tradingStrategyID)
	}

	result := database.
		Where(clause.IN{Column: "trading_strategy_id", Values: values}).
		Order("name ASC").
		Find(&bots)
	if result.Error != nil {
		return nil, fmt.Errorf("find strategy bots by strategy script: %w", result.Error)
	}

	return bots, nil
}

// FindAllByTradingStrategy does not filter by owner, since a strategy already has exactly one owner who alone can reference it.
func (strategyBotRepository *StrategyBotRepository) FindAllByTradingStrategy(
	executionContext context.Context, tradingStrategyID uint,
) ([]entities.StrategyBot, error) {
	bots := []entities.StrategyBot{}

	result := strategyBotRepository.database.within(executionContext).
		Where(clause.Eq{Column: "trading_strategy_id", Value: tradingStrategyID}).
		Order("name ASC").
		Find(&bots)
	if result.Error != nil {
		return nil, fmt.Errorf("find strategy bots by trading strategy: %w", result.Error)
	}

	return bots, nil
}

// Delete cascades to the bot's runs but leaves its trading strategy, which other bots may use.
func (strategyBotRepository *StrategyBotRepository) Delete(
	executionContext context.Context, id uint,
) error {
	result := strategyBotRepository.database.within(executionContext).
		Where(clause.Eq{Column: "id", Value: id}).
		Delete(&entities.StrategyBot{})
	if result.Error != nil {
		return fmt.Errorf("delete strategy bot: %w", result.Error)
	}

	return nil
}

// UpdateRunState writes only lifecycle columns of the bot and its watched symbols in one transaction, so a finished round cannot alter the bot's configuration nor land on half its symbols.
func (strategyBotRepository *StrategyBotRepository) UpdateRunState(
	executionContext context.Context, bot entities.StrategyBot,
) error {
	return strategyBotRepository.database.within(executionContext).Transaction(func(transaction *gorm.DB) error {
		result := transaction.
			Model(&entities.StrategyBot{}).
			Where(clause.Eq{Column: "id", Value: bot.ID}).
			Select("run_state", "next_run_at", "halt_reason").
			// UpdateColumns rather than Updates, which would bump UpdatedAt on every round even though nothing was modified.
			UpdateColumns(entities.StrategyBot{
				RunState:   bot.RunState,
				NextRunAt:  bot.NextRunAt,
				HaltReason: bot.HaltReason,
			})
		if result.Error != nil {
			return fmt.Errorf("update strategy bot run state: %w", result.Error)
		}

		for _, watchedSymbol := range bot.WatchedSymbols {
			watchedSymbolResult := transaction.
				Model(&entities.StrategyBotWatchedSymbol{}).
				Where(clause.Eq{Column: "strategy_bot_id", Value: bot.ID}).
				Where(clause.Eq{Column: "symbol", Value: watchedSymbol.Symbol}).
				Select("last_sent_signal", "last_sent_round_due_at", "conflicting", "halt_reason").
				UpdateColumns(entities.StrategyBotWatchedSymbol{
					LastSentSignal:     watchedSymbol.LastSentSignal,
					LastSentRoundDueAt: watchedSymbol.LastSentRoundDueAt,
					Conflicting:        watchedSymbol.Conflicting,
					HaltReason:         watchedSymbol.HaltReason,
				})
			if watchedSymbolResult.Error != nil {
				return fmt.Errorf("update watched symbol %s run state: %w", watchedSymbol.Symbol, watchedSymbolResult.Error)
			}
		}

		return nil
	})
}

func (strategyBotRepository *StrategyBotRepository) CountRunningByOwner(
	executionContext context.Context, ownerID uint,
) (int, error) {
	runningBotCount := int64(0)

	result := strategyBotRepository.database.within(executionContext).
		Model(&entities.StrategyBot{}).
		Where(clause.Eq{Column: "owner_id", Value: ownerID}).
		Where(clause.Eq{Column: "run_state", Value: string(vo.StrategyBotRunning)}).
		Count(&runningBotCount)
	if result.Error != nil {
		return 0, fmt.Errorf("count running strategy bots: %w", result.Error)
	}

	return int(runningBotCount), nil
}

// EnableAutoOrder holds the owner's trading key row with a shared lock while writing, so a concurrent replace or removal either waits for this switch (and then turns it off) or has already changed the key (and this refuses).
func (strategyBotRepository *StrategyBotRepository) EnableAutoOrder(
	executionContext context.Context, id uint, ownerID uint, binanceTradingKeyConfiguredAt time.Time,
) error {
	return strategyBotRepository.database.within(executionContext).Transaction(
		func(transaction *gorm.DB) error {
			heldKeys := []entities.BinanceTradingKey{}
			lockResult := transaction.
				Clauses(clause.Locking{Strength: clause.LockingStrengthShare}).
				Where(clause.Eq{Column: "user_id", Value: ownerID}).
				Where(clause.Eq{Column: "updated_at", Value: binanceTradingKeyConfiguredAt}).
				Find(&heldKeys)
			if lockResult.Error != nil {
				return fmt.Errorf("hold binance trading key: %w", lockResult.Error)
			}
			if lockResult.RowsAffected == 0 {
				return fmt.Errorf("%w: 幣安交易金鑰剛剛變了，請再試一次",
					domains.ErrStrategyBotAutoOrderKeyChanged)
			}

			result := transaction.Model(&entities.StrategyBot{}).
				Where(clause.Eq{Column: "id", Value: id}).
				UpdateColumn("auto_order_enabled", true)
			if result.Error != nil {
				return fmt.Errorf("enable strategy bot auto order: %w", result.Error)
			}

			return nil
		})
}

// DisableAutoOrder leaves UpdatedAt alone, since the bot's configuration did not change.
func (strategyBotRepository *StrategyBotRepository) DisableAutoOrder(
	executionContext context.Context, id uint,
) error {
	result := strategyBotRepository.database.within(executionContext).
		Model(&entities.StrategyBot{}).
		Where(clause.Eq{Column: "id", Value: id}).
		UpdateColumn("auto_order_enabled", false)
	if result.Error != nil {
		return fmt.Errorf("disable strategy bot auto order: %w", result.Error)
	}

	return nil
}

// DisableAutoOrderByOwner leaves UpdatedAt alone, like DisableAutoOrder, and narrows to the given market data kinds when any are given.
func (strategyBotRepository *StrategyBotRepository) DisableAutoOrderByOwner(
	executionContext context.Context, ownerID uint, marketDataKinds []string,
) error {
	query := strategyBotRepository.database.within(executionContext).
		Model(&entities.StrategyBot{}).
		Where(clause.Eq{Column: "owner_id", Value: ownerID})
	if len(marketDataKinds) > 0 {
		// The ORM's IN clause only takes untyped values.
		kinds := make([]any, 0, len(marketDataKinds))
		for _, marketDataKind := range marketDataKinds {
			kinds = append(kinds, marketDataKind)
		}
		query = query.Where(clause.IN{Column: "market_data_kind", Values: kinds})
	}

	if result := query.UpdateColumn("auto_order_enabled", false); result.Error != nil {
		return fmt.Errorf("disable auto order of owner: %w", result.Error)
	}

	return nil
}

// UpdateAutoOrderPosition writes only one watched symbol's position columns, so neither a rewrite nor a round can disturb what auto orders opened.
func (strategyBotRepository *StrategyBotRepository) UpdateAutoOrderPosition(
	executionContext context.Context, id uint, symbol string, position vo.AutoOrderPositionVo,
) error {
	direction := string(position.Direction)
	if direction == string(vo.TargetPositionFlat) {
		direction = ""
	}

	result := strategyBotRepository.database.within(executionContext).
		Model(&entities.StrategyBotWatchedSymbol{}).
		Where(clause.Eq{Column: "strategy_bot_id", Value: id}).
		Where(clause.Eq{Column: "symbol", Value: symbol}).
		Select("auto_order_position_direction", "auto_order_position_quantity",
			"auto_order_stop_loss_client_id", "auto_order_take_profit_client_id").
		UpdateColumns(entities.StrategyBotWatchedSymbol{
			AutoOrderPositionDirection:  direction,
			AutoOrderPositionQuantity:   position.Quantity,
			AutoOrderStopLossClientID:   position.StopLossClientID,
			AutoOrderTakeProfitClientID: position.TakeProfitClientID,
		})
	if result.Error != nil {
		return fmt.Errorf("update strategy bot auto order position: %w", result.Error)
	}

	return nil
}

// ClaimDue picks and claims in one transaction: the row locks skip whatever another replica is claiming right now, and the claim columns keep it out of later scans until it expires.
func (strategyBotRepository *StrategyBotRepository) ClaimDue(
	executionContext context.Context, moment time.Time, limit int, claimant string, claimedUntil time.Time,
) ([]entities.StrategyBot, error) {
	claimedBots := []entities.StrategyBot{}
	claimedUntilUtc := claimedUntil.UTC()

	transactionError := strategyBotRepository.database.within(executionContext).Transaction(
		func(transaction *gorm.DB) error {
			dueIDs := []uint{}
			if pickError := transaction.Model(&entities.StrategyBot{}).
				Clauses(clause.Locking{
					Strength: clause.LockingStrengthUpdate, Options: clause.LockingOptionsSkipLocked,
				}).
				Where(clause.Eq{Column: "run_state", Value: string(vo.StrategyBotRunning)}).
				Where(clause.Lte{Column: "next_run_at", Value: moment.UTC()}).
				Where(strategyBotRepository.unclaimedAt(moment)).
				Order("next_run_at ASC").
				Limit(limit).
				Pluck("id", &dueIDs).Error; pickError != nil {
				return pickError
			}
			if len(dueIDs) == 0 {
				return nil
			}

			// A slice of primary keys is GORM's own form of "id IN (...)".
			if claimError := transaction.Model(&entities.StrategyBot{}).
				Where(dueIDs).
				Select("round_claimed_by", "round_claimed_until").
				UpdateColumns(entities.StrategyBot{
					RoundClaimedBy: claimant, RoundClaimedUntil: &claimedUntilUtc,
				}).Error; claimError != nil {
				return claimError
			}

			return strategyBotRepository.withWatchedSymbols(transaction).Preload("TradingStrategy").
				Order("next_run_at ASC").
				Find(&claimedBots, dueIDs).Error
		})
	if transactionError != nil {
		return nil, fmt.Errorf("claim due strategy bots: %w", transactionError)
	}

	return claimedBots, nil
}

// ClaimOne is one conditional update, so two replicas pressing at once cannot both win.
func (strategyBotRepository *StrategyBotRepository) ClaimOne(
	executionContext context.Context, id uint, claimant string, moment time.Time, claimedUntil time.Time,
) (bool, error) {
	claimedUntilUtc := claimedUntil.UTC()

	result := strategyBotRepository.database.within(executionContext).
		Model(&entities.StrategyBot{}).
		Where(clause.Eq{Column: "id", Value: id}).
		Where(strategyBotRepository.unclaimedAt(moment)).
		Select("round_claimed_by", "round_claimed_until").
		UpdateColumns(entities.StrategyBot{RoundClaimedBy: claimant, RoundClaimedUntil: &claimedUntilUtc})
	if result.Error != nil {
		return false, fmt.Errorf("claim strategy bot: %w", result.Error)
	}

	return result.RowsAffected == 1, nil
}

func (strategyBotRepository *StrategyBotRepository) ReleaseRoundClaim(
	executionContext context.Context, id uint, claimant string,
) error {
	result := strategyBotRepository.database.within(executionContext).
		Model(&entities.StrategyBot{}).
		Where(clause.Eq{Column: "id", Value: id}).
		Where(clause.Eq{Column: "round_claimed_by", Value: claimant}).
		Select("round_claimed_by", "round_claimed_until").
		UpdateColumns(entities.StrategyBot{})
	if result.Error != nil {
		return fmt.Errorf("release strategy bot round claim: %w", result.Error)
	}

	return nil
}

// unclaimedAt matches a bot nobody has claimed, or whose claim had run out by moment.
func (strategyBotRepository *StrategyBotRepository) unclaimedAt(moment time.Time) clause.Expression {
	return clause.Or(
		clause.Eq{Column: "round_claimed_until", Value: nil},
		clause.Lt{Column: "round_claimed_until", Value: moment.UTC()},
	)
}

// FindOneLocked holds the bot's row and its watched symbols' rows until the surrounding transaction ends, so a round being booked and a halt from a failed send never overwrite each other.
func (strategyBotRepository *StrategyBotRepository) FindOneLocked(
	executionContext context.Context, id uint,
) (entities.StrategyBot, error) {
	bot := entities.StrategyBot{}
	database := strategyBotRepository.database.within(executionContext)

	result := database.
		Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).
		Where(clause.Eq{Column: "id", Value: id}).
		First(&bot)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.StrategyBot{}, domains.StrategyBotNotFound(id)
	}
	if result.Error != nil {
		return entities.StrategyBot{}, fmt.Errorf("find strategy bot for update: %w", result.Error)
	}

	// Locked in their own read, since a preload takes no lock, so forgetting a symbol's signal waits for the round being booked instead of being written over by it.
	if lockError := database.
		Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).
		Where(clause.Eq{Column: "strategy_bot_id", Value: id}).
		Order("id ASC").
		Find(&bot.WatchedSymbols).Error; lockError != nil {
		return entities.StrategyBot{}, fmt.Errorf("find strategy bot watched symbols for update: %w", lockError)
	}

	return bot, nil
}

// ForgetSentSignal clears one watched symbol's last sent signal only while it still comes from the round due at roundDueAt, so a later round's signal is never lost.
func (strategyBotRepository *StrategyBotRepository) ForgetSentSignal(
	executionContext context.Context, id uint, symbol string, roundDueAt time.Time,
) error {
	result := strategyBotRepository.database.within(executionContext).
		Model(&entities.StrategyBotWatchedSymbol{}).
		Where(clause.Eq{Column: "strategy_bot_id", Value: id}).
		Where(clause.Eq{Column: "symbol", Value: symbol}).
		Where(clause.Eq{Column: "last_sent_round_due_at", Value: roundDueAt.UTC()}).
		Select("last_sent_signal", "last_sent_round_due_at").
		UpdateColumns(entities.StrategyBotWatchedSymbol{})
	if result.Error != nil {
		return fmt.Errorf("forget strategy bot sent signal: %w", result.Error)
	}

	return nil
}
