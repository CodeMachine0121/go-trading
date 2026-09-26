package persistence

import (
	"errors"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SchemaMigrator syncs the schema from the entities; the loose element type is required by GORM's AutoMigrate.
type SchemaMigrator struct {
	database *gorm.DB
}

func NewSchemaMigrator(database *gorm.DB) *SchemaMigrator {
	return &SchemaMigrator{database: database}
}

// retiredColumn names a column removed from an entity, since AutoMigrate never drops columns.
type retiredColumn struct {
	entity any
	name   string
}

// retiredColumns are dropped after the sync; already-gone columns are skipped, so entries may stay indefinitely.
var retiredColumns = []retiredColumn{
	// Moved onto the calculation request.
	{entity: &entities.StrategyScript{}, name: "aggregation_interval"},
	{entity: &entities.StrategyScript{}, name: "candle_count"},
	// Spot-only-era leftovers superseded by contract strategy mode and contract bot leverage columns.
	{entity: &entities.TradingStrategy{}, name: "trading_mode"},
	{entity: &entities.StrategyBot{}, name: "position_plan_leverage"},
}

// retiredIndex names a replaced index, since AutoMigrate never drops indexes and a leftover unique index would refuse valid writes.
type retiredIndex struct {
	entity any
	name   string
}

// retiredIndexes are dropped after the sync; dropping is idempotent.
var retiredIndexes = []retiredIndex{
	// Script names are now unique per owner, not system-wide.
	{entity: &entities.StrategyScript{}, name: "idx_strategies_name"},
}

// ownerlessTable is a table that gained a required owner column; rows predating it belong to nobody and are deleted.
type ownerlessTable struct {
	entity      any
	ownerColumn string
	description string
}

// ownerlessTables are emptied only while they still lack their owner column, so the condition fires once.
var ownerlessTables = []ownerlessTable{
	{entity: &entities.StrategyScript{}, ownerColumn: "owner_id", description: "strategy scripts"},
	// Conversations can contain the asker's own algorithms, so an ownerless one must not survive.
	{entity: &entities.Conversation{}, ownerColumn: "owner_id", description: "conversations"},
}

// Migrate syncs every registered entity, drops retired columns, and returns the table names; register new entities in the slice below.
func (schemaMigrator *SchemaMigrator) Migrate() ([]string, error) {
	migratedEntities := []any{
		&entities.KCandle{},
		&entities.TradingSymbol{},
		&entities.StrategyScript{},
		&entities.StrategyScriptParameter{},
		&entities.Conversation{},
		&entities.AssistantTurn{},
		&entities.AssistantQueryRecord{},
		&entities.AssistantPendingRevision{},
		&entities.AssistantCreatedSubject{},
		&entities.User{},
		&entities.Session{},
		&entities.PublishedStrategyScript{},
		&entities.TelegramDelivery{},
		&entities.TradingStrategy{},
		&entities.StrategyBot{},
		&entities.TradingStrategySignalSource{},
		&entities.TradingStrategySignalSourceParameterValue{},
		&entities.TradingStrategyConditionNode{},
		&entities.StrategyBotRunRecord{},
		&entities.KCandleHistorySyncRun{},
		&entities.KCandleContract{},
		&entities.ContractTradingSymbol{},
		&entities.KCandleContractHistorySyncRun{},
		&entities.ContractFundingRateSettlement{},
		&entities.ContractPositionStatistic{},
		&entities.ContractMaintenanceMarginTier{},
	}

	// Rename before syncing, or AutoMigrate would create empty new tables beside the old ones.
	if renameError := schemaMigrator.renameMovedRuleTables(); renameError != nil {
		return nil, renameError
	}

	// Move the rules before syncing, because the sync adds foreign keys to trading strategies that rows still carrying bot IDs would violate.
	if prepareError := schemaMigrator.prepareForTheMove(); prepareError != nil {
		return nil, prepareError
	}

	if moveError := schemaMigrator.moveRulesOntoTradingStrategies(); moveError != nil {
		return nil, moveError
	}

	// Clear before syncing, since a NOT NULL owner column cannot be added to a table with rows.
	if clearError := schemaMigrator.clearOwnerlessRows(); clearError != nil {
		return nil, clearError
	}

	migrateError := schemaMigrator.database.AutoMigrate(migratedEntities...)
	if migrateError != nil {
		return nil, fmt.Errorf("auto migrate schema: %w", migrateError)
	}

	// After the sync, so copies can be marked as adopted.
	if copyError := schemaMigrator.copyMarketplaceDependencies(); copyError != nil {
		return nil, copyError
	}

	if dropError := schemaMigrator.dropRetiredColumns(); dropError != nil {
		return nil, dropError
	}

	if dropError := schemaMigrator.dropRetiredIndexes(); dropError != nil {
		return nil, dropError
	}

	// After dropping retired indexes, so a reused name does not collide with the index it replaces.
	if createError := schemaMigrator.createPartialIndexes(); createError != nil {
		return nil, createError
	}

	if dropError := schemaMigrator.dropRetiredConstraints(); dropError != nil {
		return nil, dropError
	}

	migratedTables := make([]string, 0, len(migratedEntities))
	for _, migratedEntity := range migratedEntities {
		statement := &gorm.Statement{DB: schemaMigrator.database}
		if parseError := statement.Parse(migratedEntity); parseError != nil {
			return nil, fmt.Errorf("resolve migrated table name: %w", parseError)
		}
		migratedTables = append(migratedTables, statement.Schema.Table)
	}

	return migratedTables, nil
}

// dropRetiredColumns is idempotent, skipping columns already gone.
func (schemaMigrator *SchemaMigrator) dropRetiredColumns() error {
	migrator := schemaMigrator.database.Migrator()

	for _, column := range retiredColumns {
		if !migrator.HasColumn(column.entity, column.name) {
			continue
		}

		if dropError := migrator.DropColumn(column.entity, column.name); dropError != nil {
			return fmt.Errorf("drop retired column %s: %w", column.name, dropError)
		}
	}

	return nil
}

// AssistantTurnOneRunningPerConversationIndex enforces one running turn per conversation in the database, since a check-then-append lets concurrent requests both pass; the write path recognises its violation as a double submit, not a fault.
const AssistantTurnOneRunningPerConversationIndex = "idx_assistant_turns_one_running_per_conversation"

// KCandleHistorySyncOneRunningPerSymbolIndex enforces one running history sync per symbol in the database, since a check-then-start lets concurrent requests both pass; its violation means a double submit, not a fault.
const KCandleHistorySyncOneRunningPerSymbolIndex = "idx_k_candle_history_sync_runs_one_running_per_symbol"

// KCandleContractHistorySyncOneRunningPerSymbolIndex is the contract-venue counterpart, separate so a spot sync does not block a contract sync of the same name.
const KCandleContractHistorySyncOneRunningPerSymbolIndex = "idx_k_candle_contract_history_sync_runs_one_running_per_symbol"

// createPartialIndexes creates partial unique indexes GORM tags cannot express; the status is inlined from the shared constant because PostgreSQL does not accept parameters in an index predicate, and creation is idempotent.
// Creating it is idempotent, so running this twice is the same as running it once.
func (schemaMigrator *SchemaMigrator) createPartialIndexes() error {
	created := schemaMigrator.database.Exec(
		fmt.Sprintf(
			"CREATE UNIQUE INDEX IF NOT EXISTS ? ON ? (conversation_id) WHERE status = '%s'",
			vo.AssistantTurnRunning),
		clause.Column{Name: AssistantTurnOneRunningPerConversationIndex},
		clause.Table{Name: entities.AssistantTurn{}.TableName()},
	)
	if created.Error != nil {
		return fmt.Errorf("create index %s: %w",
			AssistantTurnOneRunningPerConversationIndex, created.Error)
	}

	createdHistorySyncIndex := schemaMigrator.database.Exec(
		fmt.Sprintf(
			"CREATE UNIQUE INDEX IF NOT EXISTS ? ON ? (symbol) WHERE status = '%s'",
			vo.KCandleHistorySyncRunning),
		clause.Column{Name: KCandleHistorySyncOneRunningPerSymbolIndex},
		clause.Table{Name: entities.KCandleHistorySyncRun{}.TableName()},
	)
	if createdHistorySyncIndex.Error != nil {
		return fmt.Errorf("create index %s: %w",
			KCandleHistorySyncOneRunningPerSymbolIndex, createdHistorySyncIndex.Error)
	}

	createdContractHistorySyncIndex := schemaMigrator.database.Exec(
		fmt.Sprintf(
			"CREATE UNIQUE INDEX IF NOT EXISTS ? ON ? (symbol) WHERE status = '%s'",
			vo.KCandleHistorySyncRunning),
		clause.Column{Name: KCandleContractHistorySyncOneRunningPerSymbolIndex},
		clause.Table{Name: entities.KCandleContractHistorySyncRun{}.TableName()},
	)
	if createdContractHistorySyncIndex.Error != nil {
		return fmt.Errorf("create index %s: %w",
			KCandleContractHistorySyncOneRunningPerSymbolIndex, createdContractHistorySyncIndex.Error)
	}

	return nil
}

// dropRetiredIndexes is idempotent, skipping indexes already gone.
func (schemaMigrator *SchemaMigrator) dropRetiredIndexes() error {
	migrator := schemaMigrator.database.Migrator()

	for _, index := range retiredIndexes {
		if !migrator.HasIndex(index.entity, index.name) {
			continue
		}

		// Raw SQL because GORM's DropIndex emits an invalid schema-qualified name on this driver; the name is a constant passed through identifier quoting.
		dropped := schemaMigrator.database.Exec("DROP INDEX IF EXISTS ?", clause.Column{Name: index.name})
		if dropped.Error != nil {
			return fmt.Errorf("drop retired index %s: %w", index.name, dropped.Error)
		}
	}

	return nil
}

// retiredConstraint is a foreign key left under an old name after the rules moved off the bot; two still point at StrategyBots and break almost every write.
type retiredConstraint struct {
	entity any
	name   string
}

// retiredConstraints are dropped after the sync; dropping is idempotent.
var retiredConstraints = []retiredConstraint{
	{entity: &entities.TradingStrategySignalSource{}, name: "fk_StrategyBots_signal_sources"},
	{entity: &entities.TradingStrategyConditionNode{}, name: "fk_StrategyBots_condition_nodes"},
	{
		entity: &entities.TradingStrategySignalSourceParameterValue{},
		name:   "fk_StrategyBotSignalSources_parameter_values",
	},
	{
		entity: &entities.TradingStrategyConditionNode{},
		name:   "fk_StrategyBotConditionNodes_children",
	},
	// Present on databases synced before the bot reference lost its constraint; it would block the rule move.
	{entity: &entities.StrategyBot{}, name: "fk_TradingStrategies_bots"},
	{entity: &entities.StrategyBot{}, name: "fk_StrategyBots_trading_strategy"},
}

// dropRetiredConstraints is idempotent, skipping constraints already gone.
func (schemaMigrator *SchemaMigrator) dropRetiredConstraints() error {
	migrator := schemaMigrator.database.Migrator()

	for _, constraint := range retiredConstraints {
		if !migrator.HasTable(constraint.entity) || !migrator.HasConstraint(constraint.entity, constraint.name) {
			continue
		}

		if dropError := migrator.DropConstraint(constraint.entity, constraint.name); dropError != nil {
			return fmt.Errorf("drop retired constraint %s: %w", constraint.name, dropError)
		}
	}

	return nil
}

// movedRuleTable is a table renamed when the rules moved from the bot onto the trading strategy.
type movedRuleTable struct {
	oldName string
	entity  any
}

// movedRuleTables renames are skipped once done.
var movedRuleTables = []movedRuleTable{
	{oldName: "StrategyBotSignalSources", entity: &entities.TradingStrategySignalSource{}},
	{oldName: "StrategyBotConditionNodes", entity: &entities.TradingStrategyConditionNode{}},
	{
		oldName: "StrategyBotSignalSourceParameterValues",
		entity:  &entities.TradingStrategySignalSourceParameterValue{},
	},
}

type movedRuleIndex struct {
	entity  any
	oldName string
	newName string
}

// movedRuleIndexes are renamed rather than recreated so the table stays indexed throughout.
var movedRuleIndexes = []movedRuleIndex{
	{
		entity:  &entities.TradingStrategySignalSource{},
		oldName: "idx_strategy_bot_signal_sources_bot",
		newName: "idx_trading_strategy_signal_sources_strategy",
	},
	{
		entity:  &entities.TradingStrategySignalSource{},
		oldName: "idx_strategy_bot_signal_sources_bot_label",
		newName: "idx_trading_strategy_signal_sources_strategy_label",
	},
	{
		entity:  &entities.TradingStrategySignalSource{},
		oldName: "idx_strategy_bot_signal_sources_strategy",
		newName: "idx_trading_strategy_signal_sources_script",
	},
	{
		entity:  &entities.TradingStrategyConditionNode{},
		oldName: "idx_strategy_bot_condition_nodes_bot",
		newName: "idx_trading_strategy_condition_nodes_strategy",
	},
	{
		entity:  &entities.TradingStrategyConditionNode{},
		oldName: "idx_strategy_bot_condition_nodes_parent",
		newName: "idx_trading_strategy_condition_nodes_parent",
	},
	{
		entity:  &entities.TradingStrategySignalSourceParameterValue{},
		oldName: "idx_strategy_bot_source_parameter_values_source",
		newName: "idx_trading_strategy_source_parameter_values_source",
	},
}

type movedRuleColumn struct {
	entity  any
	oldName string
	newName string
}

// The first two columns still hold bot IDs until moveRulesOntoTradingStrategies corrects them; renaming rather than adding avoids a leftover NOT NULL column that would fail every write.
var movedRuleColumns = []movedRuleColumn{
	{
		entity:  &entities.TradingStrategySignalSource{},
		oldName: "strategy_bot_id",
		newName: "trading_strategy_id",
	},
	{
		entity:  &entities.TradingStrategyConditionNode{},
		oldName: "strategy_bot_id",
		newName: "trading_strategy_id",
	},
	{
		entity:  &entities.TradingStrategySignalSourceParameterValue{},
		oldName: "strategy_bot_signal_source_id",
		newName: "trading_strategy_signal_source_id",
	},
}

// renameMovedRuleTables renames tables, indexes and columns in place rather than copying rows, so there is no interruptible half-way state.
func (schemaMigrator *SchemaMigrator) renameMovedRuleTables() error {
	migrator := schemaMigrator.database.Migrator()

	for _, table := range movedRuleTables {
		if !migrator.HasTable(table.oldName) || migrator.HasTable(table.entity) {
			continue
		}

		if renameError := migrator.RenameTable(table.oldName, table.entity); renameError != nil {
			return fmt.Errorf("rename moved rule table %s: %w", table.oldName, renameError)
		}
	}

	for _, column := range movedRuleColumns {
		if !migrator.HasColumn(column.entity, column.oldName) {
			continue
		}

		if renameError := migrator.RenameColumn(
			column.entity, column.oldName, column.newName); renameError != nil {
			return fmt.Errorf("rename moved rule column %s: %w", column.oldName, renameError)
		}
	}

	for _, index := range movedRuleIndexes {
		if !migrator.HasIndex(index.entity, index.oldName) || migrator.HasIndex(index.entity, index.newName) {
			continue
		}

		// Raw SQL because GORM's RenameIndex has the same invalid qualification as DropIndex; both names are constants passed through identifier quoting.
		renamed := schemaMigrator.database.Exec("ALTER INDEX IF EXISTS ? RENAME TO ?",
			clause.Column{Name: index.oldName}, clause.Column{Name: index.newName})
		if renamed.Error != nil {
			return fmt.Errorf("rename moved rule index %s: %w", index.oldName, renamed.Error)
		}
	}

	return nil
}

// prepareForTheMove creates the trading strategy table and the bot's reference column without the child foreign keys that cannot hold until the move has run.
func (schemaMigrator *SchemaMigrator) prepareForTheMove() error {
	// Only the table's own owner key is declared here; child-table keys come with the later sync.
	migrator := schemaMigrator.database.Migrator()

	// Nothing to move on a database that never had bots.
	if !migrator.HasTable(&entities.StrategyBot{}) {
		return nil
	}

	if !migrator.HasTable(&entities.TradingStrategy{}) {
		if createError := migrator.CreateTable(&entities.TradingStrategy{}); createError != nil {
			return fmt.Errorf("create the table the rules move into: %w", createError)
		}
	}

	if !migrator.HasColumn(&entities.StrategyBot{}, "trading_strategy_id") {
		if addError := migrator.AddColumn(
			&entities.StrategyBot{}, "TradingStrategyID"); addError != nil {
			return fmt.Errorf("add the column a bot names its rules with: %w", addError)
		}
	}

	// Two of these still point at StrategyBots and would refuse the move's writes, so they are dropped now.
	return schemaMigrator.dropRetiredConstraints()
}

// moveRulesOntoTradingStrategies gives each unmigrated bot a same-named trading strategy with its rules, repointing child rows by their own IDs (gathered up front) because bot and strategy ID ranges overlap; already-migrated bots are skipped.
func (schemaMigrator *SchemaMigrator) moveRulesOntoTradingStrategies() error {
	if !schemaMigrator.database.Migrator().HasTable(&entities.StrategyBot{}) {
		return nil
	}

	return schemaMigrator.database.Transaction(func(transaction *gorm.DB) error {
		unmovedBots := []entities.StrategyBot{}
		// Select only three columns: the table was just altered, and a cached plan whose result type changed is refused.
		if findError := transaction.Model(&entities.StrategyBot{}).
			Select("id", "owner_id", "name").
			Where(clause.Eq{Column: "trading_strategy_id", Value: 0}).
			Order("id ASC").
			Find(&unmovedBots).Error; findError != nil {
			return fmt.Errorf("find bots without a trading strategy: %w", findError)
		}

		// Push strategy IDs past every bot ID first, or a new strategy could collide with an unmoved bot's rows on the unique (trading_strategy_id, label), which the default label makes common.
		if pushError := schemaMigrator.pushIdentifiersPastEveryBot(transaction); pushError != nil {
			return pushError
		}

		// Gather every bot's rows before creating any strategy, so later reads are not disturbed by earlier writes.
		signalSourceIDsByBot := map[uint][]uint{}
		conditionNodeIDsByBot := map[uint][]uint{}

		for _, bot := range unmovedBots {
			signalSourceIDs, readError := schemaMigrator.ruleRowIDsOf(
				transaction, &entities.TradingStrategySignalSource{}, bot.ID)
			if readError != nil {
				return readError
			}

			conditionNodeIDs, readError := schemaMigrator.ruleRowIDsOf(
				transaction, &entities.TradingStrategyConditionNode{}, bot.ID)
			if readError != nil {
				return readError
			}

			signalSourceIDsByBot[bot.ID] = signalSourceIDs
			conditionNodeIDsByBot[bot.ID] = conditionNodeIDs
		}

		for _, bot := range unmovedBots {
			tradingStrategy := entities.TradingStrategy{OwnerID: bot.OwnerID, Name: bot.Name}
			if createError := transaction.Omit(clause.Associations).
				Create(&tradingStrategy).Error; createError != nil {
				return fmt.Errorf("create trading strategy for bot %d: %w", bot.ID, createError)
			}

			if repointError := schemaMigrator.repointRuleRows(
				transaction, &entities.TradingStrategySignalSource{},
				signalSourceIDsByBot[bot.ID], tradingStrategy.ID); repointError != nil {
				return repointError
			}

			if repointError := schemaMigrator.repointRuleRows(
				transaction, &entities.TradingStrategyConditionNode{},
				conditionNodeIDsByBot[bot.ID], tradingStrategy.ID); repointError != nil {
				return repointError
			}

			if updateError := transaction.Model(&entities.StrategyBot{}).
				Where(clause.Eq{Column: "id", Value: bot.ID}).
				UpdateColumn("trading_strategy_id", tradingStrategy.ID).Error; updateError != nil {
				return fmt.Errorf("point bot %d at its trading strategy: %w", bot.ID, updateError)
			}
		}

		return nil
	})
}

// pushIdentifiersPastEveryBot advances the trading strategy sequence past every bot and strategy ID; raw SQL because GORM cannot set a sequence, with the value bound.
func (schemaMigrator *SchemaMigrator) pushIdentifiersPastEveryBot(transaction *gorm.DB) error {
	highestBotID := uint(0)
	if readError := transaction.Model(&entities.StrategyBot{}).
		Select("COALESCE(MAX(id), 0)").Scan(&highestBotID).Error; readError != nil {
		return fmt.Errorf("read the highest bot identifier: %w", readError)
	}

	// No bots means nothing to move, so do not burn an ID.
	if highestBotID == 0 {
		return nil
	}

	pushed := transaction.Exec(
		`SELECT setval(
			pg_get_serial_sequence('"TradingStrategies"', 'id'),
			GREATEST(?, (SELECT COALESCE(MAX(id), 0) FROM "TradingStrategies")),
			true)`, highestBotID)
	if pushed.Error != nil {
		return fmt.Errorf("push trading strategy identifiers past every bot: %w", pushed.Error)
	}

	return nil
}

// ruleRowIDsOf reads a bot's rule row IDs before any write.
func (schemaMigrator *SchemaMigrator) ruleRowIDsOf(
	transaction *gorm.DB, entity any, botID uint,
) ([]uint, error) {
	rowIDs := []uint{}

	if readError := transaction.Model(entity).
		Where(clause.Eq{Column: "trading_strategy_id", Value: botID}).
		Pluck("id", &rowIDs).Error; readError != nil {
		return nil, fmt.Errorf("read rule rows of bot %d: %w", botID, readError)
	}

	return rowIDs, nil
}

// repointRuleRows updates by explicit IDs so it can never match an unintended row.
func (schemaMigrator *SchemaMigrator) repointRuleRows(
	transaction *gorm.DB, entity any, rowIDs []uint, tradingStrategyID uint,
) error {
	for _, rowID := range rowIDs {
		if updateError := transaction.Model(entity).
			Where(clause.Eq{Column: "id", Value: rowID}).
			UpdateColumn("trading_strategy_id", tradingStrategyID).Error; updateError != nil {
			return fmt.Errorf("repoint rule row %d: %w", rowID, updateError)
		}
	}

	return nil
}

// clearOwnerlessRows deletes rows (cascading) from tables that still lack their owner column, a condition that is true exactly once; assigning an owner instead would be a guess.
func (schemaMigrator *SchemaMigrator) clearOwnerlessRows() error {
	migrator := schemaMigrator.database.Migrator()

	for _, table := range ownerlessTables {
		if !migrator.HasTable(table.entity) {
			continue
		}

		if migrator.HasColumn(table.entity, table.ownerColumn) {
			continue
		}

		if deleteError := schemaMigrator.database.
			Where("1 = 1").
			Delete(table.entity).Error; deleteError != nil {
			return fmt.Errorf("clear ownerless %s: %w", table.description, deleteError)
		}
	}

	return nil
}

// retiredStrategyScriptAdoptionRow is a row of the table that recorded adoptions before adopting made a copy; it is
// read once, while moving to copies, and the table is then dropped.
type retiredStrategyScriptAdoptionRow struct {
	UserID           uint
	StrategyScriptID uint `gorm:"column:strategy_id"`
}

func (retiredStrategyScriptAdoptionRow) TableName() string {
	return "StrategyAdoptions"
}

// marketplaceCopyKey is one owner depending on one script someone else wrote.
type marketplaceCopyKey struct {
	ownerID                  uint
	originalStrategyScriptID uint
}

// copyMarketplaceDependencies turns every old adoption, and every signal source naming someone else's script, into a
// copy the owner holds, so no bot depends on another person's script; it does nothing once there is nothing to move.
func (schemaMigrator *SchemaMigrator) copyMarketplaceDependencies() error {
	return schemaMigrator.database.Transaction(func(transaction *gorm.DB) error {
		copiedStrategyScriptIDs := map[marketplaceCopyKey]uint{}
		takenNamesByOwner := map[uint]map[string]bool{}

		copyFor := func(key marketplaceCopyKey) (uint, bool, error) {
			if copiedStrategyScriptID, alreadyCopied := copiedStrategyScriptIDs[key]; alreadyCopied {
				return copiedStrategyScriptID, true, nil
			}

			original := entities.StrategyScript{}
			found := transaction.Preload("Parameters").Preload("Publication").
				First(&original, key.originalStrategyScriptID)
			if errors.Is(found.Error, gorm.ErrRecordNotFound) {
				return 0, false, nil
			}
			if found.Error != nil {
				return 0, false, fmt.Errorf("read script to copy: %w", found.Error)
			}
			// A script its author took off the marketplace is not handed out again; its bots already halted.
			if original.Publication == nil {
				return 0, false, nil
			}

			takenNameSet, namesKnown := takenNamesByOwner[key.ownerID]
			if !namesKnown {
				takenNames := []string{}
				if plucked := transaction.Model(&entities.StrategyScript{}).
					Where(clause.Eq{Column: "owner_id", Value: key.ownerID}).
					Pluck("name", &takenNames); plucked.Error != nil {
					return 0, false, fmt.Errorf("read names to avoid: %w", plucked.Error)
				}
				takenNameSet = make(map[string]bool, len(takenNames))
				for _, takenName := range takenNames {
					takenNameSet[takenName] = true
				}
				takenNamesByOwner[key.ownerID] = takenNameSet
			}

			marketplaceCopy := domains.NewStrategyScriptMarketplaceCopyDomain(original, key.ownerID, time.Now()).
				NamedAvoiding(takenNameSet).ToEntity()
			if created := transaction.Create(&marketplaceCopy); created.Error != nil {
				return 0, false, fmt.Errorf("create marketplace copy: %w", created.Error)
			}

			copiedStrategyScriptIDs[key] = marketplaceCopy.ID
			takenNameSet[marketplaceCopy.Name] = true

			return marketplaceCopy.ID, true, nil
		}

		// The adoptions table is the mark that this database was never moved; once dropped, nothing runs again, so a
		// script withdrawn at the time and republished later is never copied to someone who did not adopt it.
		migrator := transaction.Migrator()
		if !migrator.HasTable(&retiredStrategyScriptAdoptionRow{}) {
			return nil
		}

		adoptions := []retiredStrategyScriptAdoptionRow{}
		if read := transaction.Find(&adoptions); read.Error != nil {
			return fmt.Errorf("read adoptions: %w", read.Error)
		}

		for _, adoption := range adoptions {
			if _, _, copyError := copyFor(marketplaceCopyKey{
				ownerID: adoption.UserID, originalStrategyScriptID: adoption.StrategyScriptID,
			}); copyError != nil {
				return copyError
			}
		}

		scriptOwners, scriptOwnersError := schemaMigrator.ownersOf(transaction, &entities.StrategyScript{})
		if scriptOwnersError != nil {
			return scriptOwnersError
		}
		strategyOwners, strategyOwnersError := schemaMigrator.ownersOf(transaction, &entities.TradingStrategy{})
		if strategyOwnersError != nil {
			return strategyOwnersError
		}

		signalSources := []entities.TradingStrategySignalSource{}
		if read := transaction.Find(&signalSources); read.Error != nil {
			return fmt.Errorf("read signal sources: %w", read.Error)
		}

		for _, signalSource := range signalSources {
			scriptOwnerID, scriptExists := scriptOwners[signalSource.StrategyScriptID]
			strategyOwnerID := strategyOwners[signalSource.TradingStrategyID]
			if !scriptExists || scriptOwnerID == strategyOwnerID {
				continue
			}

			copiedStrategyScriptID, copied, copyError := copyFor(marketplaceCopyKey{
				ownerID: strategyOwnerID, originalStrategyScriptID: signalSource.StrategyScriptID,
			})
			if copyError != nil {
				return copyError
			}
			if !copied {
				continue
			}

			if repointed := transaction.Model(&entities.TradingStrategySignalSource{}).
				Where(clause.Eq{Column: "id", Value: signalSource.ID}).
				Update("strategy_id", copiedStrategyScriptID); repointed.Error != nil {
				return fmt.Errorf("point signal source at its copy: %w", repointed.Error)
			}
		}

		if dropError := migrator.DropTable(&retiredStrategyScriptAdoptionRow{}); dropError != nil {
			return fmt.Errorf("drop retired adoptions: %w", dropError)
		}

		return nil
	})
}

// ownerRow is an identifier and its owner, which is all the move needs to know about scripts and strategies.
type ownerRow struct {
	ID      uint
	OwnerID uint
}

// ownersOf maps every row of a table to its owner; used for both scripts and trading strategies.
func (schemaMigrator *SchemaMigrator) ownersOf(transaction *gorm.DB, model any) (map[uint]uint, error) {
	rows := []ownerRow{}
	if read := transaction.Model(model).Select("id", "owner_id").Find(&rows); read.Error != nil {
		return nil, fmt.Errorf("read owners: %w", read.Error)
	}

	owners := make(map[uint]uint, len(rows))
	for _, row := range rows {
		owners[row.ID] = row.OwnerID
	}

	return owners, nil
}
