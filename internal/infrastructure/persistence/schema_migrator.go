package persistence

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SchemaMigrator syncs the database schema from the entity definitions, code first.
// The loose element type is required by GORM's AutoMigrate signature and stays
// confined to this infrastructure file.
type SchemaMigrator struct {
	database *gorm.DB
}

func NewSchemaMigrator(database *gorm.DB) *SchemaMigrator {
	return &SchemaMigrator{database: database}
}

// retiredColumn is a column an entity used to have. AutoMigrate adds and widens but
// never drops, so a field removed from an entity leaves its column behind forever
// unless it is said out loud here — and a reader who finds aggregation_interval
// still sitting on StrategyScripts has every reason to believe a strategy script still
// remembers it.
type retiredColumn struct {
	entity any
	name   string
}

// retiredColumns are the columns to drop after the schema is synced. Dropping is
// idempotent: a column that is already gone is skipped, so this list may be kept
// long after every database has caught up.
var retiredColumns = []retiredColumn{
	// How coarse the K candles are and how many of them describe one run of an
	// algorithm, not the algorithm; they moved onto the calculation request.
	{entity: &entities.StrategyScript{}, name: "aggregation_interval"},
	{entity: &entities.StrategyScript{}, name: "candle_count"},
}

// retiredIndex is an index an entity used to carry. AutoMigrate adds indexes but
// never drops them, so an index that has been replaced stays behind and keeps
// enforcing a rule nobody asked for — which is worse than a leftover column: a
// column just sits there, whereas a leftover unique index refuses writes the system
// now considers perfectly fine.
type retiredIndex struct {
	entity any
	name   string
}

// retiredIndexes are the indexes to drop after the schema is synced. Dropping is
// idempotent, so this list may be kept long after every database has caught up.
var retiredIndexes = []retiredIndex{
	// A strategy script's name used to be unique across the whole system. It is now unique
	// within one owner's collection, and the old index would keep the first person
	// here holding "二十根均線" against everybody else forever.
	{entity: &entities.StrategyScript{}, name: "idx_strategies_name"},
}

// ownerlessTable is a table that gained an owner it may not be without. Rows saved
// before that column existed belong to nobody, and nobody is not a person whose
// things these are — so they go.
type ownerlessTable struct {
	entity      any
	ownerColumn string
	// description names the rows in the failure, because "clear ownerless rows"
	// tells whoever reads it nothing about which ones.
	description string
}

// ownerlessTables are the tables to empty while they still predate their owner
// column. Each condition stops being true the moment the migration after it runs,
// so this list may be kept long after every database has caught up.
var ownerlessTables = []ownerlessTable{
	// StrategyScripts became somebody's property.
	{entity: &entities.StrategyScript{}, ownerColumn: "owner_id", description: "strategy scripts"},
	// So did conversations, and for a sharper reason: the assistant acts as whoever
	// asked it, so a transcript can hold that person's own algorithms. A conversation
	// belonging to nobody would be readable by everybody.
	{entity: &entities.Conversation{}, ownerColumn: "owner_id", description: "conversations"},
}

// Migrate creates or updates the table of every registered entity, drops the columns
// no entity claims any more, and reports the resulting table names. Register every
// new entity in the slice below.
func (schemaMigrator *SchemaMigrator) Migrate() ([]string, error) {
	migratedEntities := []any{
		&entities.KCandle{},
		&entities.TradingSymbol{},
		&entities.StrategyScript{},
		&entities.StrategyScriptParameter{},
		&entities.Conversation{},
		&entities.AssistantTurn{},
		&entities.AssistantQueryRecord{},
		&entities.User{},
		&entities.Session{},
		&entities.PublishedStrategyScript{},
		&entities.StrategyScriptAdoption{},
		&entities.TelegramDelivery{},
		&entities.TradingStrategy{},
		&entities.StrategyBot{},
		&entities.TradingStrategySignalSource{},
		&entities.TradingStrategySignalSourceParameterValue{},
		&entities.TradingStrategyConditionNode{},
		&entities.StrategyBotRunRecord{},
		&entities.KCandleHistorySyncRun{},
	}

	// Renaming has to happen before the schema is synced, not after. These three
	// tables and the column that points into two of them changed names when the
	// rules moved off the bot; left to AutoMigrate, the old tables would simply sit
	// there while three empty new ones appeared beside them.
	if renameError := schemaMigrator.renameMovedRuleTables(); renameError != nil {
		return nil, renameError
	}

	// The rules move before the schema is synced, not after, and the two steps
	// below are the whole reason.
	//
	// Syncing adds the foreign keys that tie a signal source and a condition node to
	// the trading strategy they belong to. Until the move has run, those rows carry
	// bot identifiers, so those keys cannot hold and syncing fails outright — on
	// every database that has a bot in it, which is every database worth migrating.
	// Moving first means they hold the moment they are added.
	if prepareError := schemaMigrator.prepareForTheMove(); prepareError != nil {
		return nil, prepareError
	}

	if moveError := schemaMigrator.moveRulesOntoTradingStrategies(); moveError != nil {
		return nil, moveError
	}

	// Clearing has to happen before the schema is synced, not after: these tables
	// gained an owner that may not be null, and a table with rows in it cannot grow
	// such a column.
	if clearError := schemaMigrator.clearOwnerlessRows(); clearError != nil {
		return nil, clearError
	}

	migrateError := schemaMigrator.database.AutoMigrate(migratedEntities...)
	if migrateError != nil {
		return nil, fmt.Errorf("auto migrate schema: %w", migrateError)
	}

	if dropError := schemaMigrator.dropRetiredColumns(); dropError != nil {
		return nil, dropError
	}

	if dropError := schemaMigrator.dropRetiredIndexes(); dropError != nil {
		return nil, dropError
	}

	// After the retired ones are gone, so that a name being reused never runs into
	// the index it is replacing.
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

// dropRetiredColumns removes every column no entity claims any more, skipping the
// ones already gone so that running this twice is the same as running it once.
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

// AssistantTurnOneRunningPerConversationIndex is what makes "one answer at a time per
// conversation" a fact about the store rather than a check somebody won.
//
// The rule is asked before a question is accepted, but asking and appending are two
// statements: two requests arriving together both read a conversation with nothing in
// flight, both pass, and both start writing into it. What is then recorded is two
// interleaved exchanges nobody can attribute — which is exactly what the rule exists
// to prevent, and the double-send is the very thing that triggers it.
//
// It is named here because the write path has to recognise this one breaking
// specifically: it is a person asking twice, not a fault.
const AssistantTurnOneRunningPerConversationIndex = "idx_assistant_turns_one_running_per_conversation"

// createPartialIndexes adds the indexes the ORM's own tags cannot express.
//
// A unique index over part of a table has no tag: "unique" there would mean one
// exchange per conversation ever, which is the opposite of a conversation. So this is
// the second place in the codebase that writes a statement out by hand, for the same
// reason as the first — the ORM cannot say it.
//
// Both identifiers go through the ORM's own quoting. **The status cannot**, and that
// is the database's rule rather than a shortcut: PostgreSQL does not accept a
// parameter inside an index predicate, so it has to be written into the statement.
// It is read from the same constant everything else compares against — writing
// "running" out by hand here is how a rename would silently leave this index
// enforcing a state nothing uses — and it is a constant of this system, never
// anything a caller supplied.
//
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

	return nil
}

// dropRetiredIndexes removes every index no entity claims any more, skipping the
// ones already gone so that running this twice is the same as running it once.
func (schemaMigrator *SchemaMigrator) dropRetiredIndexes() error {
	migrator := schemaMigrator.database.Migrator()

	for _, index := range retiredIndexes {
		if !migrator.HasIndex(index.entity, index.name) {
			continue
		}

		// The ORM's own DropIndex is not usable here, and this is the one place in
		// the codebase that writes a statement out by hand. On this driver it
		// builds "DROP INDEX <schema>.<name>" and, on a connection that names no
		// schema, fills the first blank with a function call — which is not valid
		// there. The statement below is what it was trying to write.
		//
		// It carries no value from anywhere: the name is a constant in the list
		// above, and it goes through the ORM's own identifier quoting rather than
		// being pasted into the text.
		dropped := schemaMigrator.database.Exec("DROP INDEX IF EXISTS ?", clause.Column{Name: index.name})
		if dropped.Error != nil {
			return fmt.Errorf("drop retired index %s: %w", index.name, dropped.Error)
		}
	}

	return nil
}

// retiredConstraint is a foreign key an entity used to carry under another name.
//
// The ORM names a foreign key after the *field* that declares the association, so
// the rules moving off the bot left four behind on the tables that came with them —
// and, like an index, one left behind is never dropped on its own.
//
// Two of the four still point at StrategyBots, which is not a leftover but a live
// fault: every row written to those tables is checked against a bot identifier that
// is now a trading strategy identifier, and almost every write fails. The other two
// are harmless duplicates of constraints that now carry the right name.
type retiredConstraint struct {
	entity any
	name   string
}

// retiredConstraints are the foreign keys to drop after the schema is synced.
// Dropping is idempotent, so this list may be kept long after every database has
// caught up.
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
	// A database synced before a bot's reference lost its constraint would carry
	// this one, and it would refuse the next upgrade that has rules to move.
	{entity: &entities.StrategyBot{}, name: "fk_TradingStrategies_bots"},
	{entity: &entities.StrategyBot{}, name: "fk_StrategyBots_trading_strategy"},
}

// dropRetiredConstraints removes every foreign key no entity claims any more,
// skipping the ones already gone so that running this twice is the same as running
// it once.
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

// movedRuleTable is a table that changed names when the signal sources and the two
// condition trees moved off the bot and onto the trading strategy it now follows.
type movedRuleTable struct {
	oldName string
	entity  any
}

// movedRuleTables are those renames. Each one is skipped once it has happened, so
// running this twice is the same as running it once.
var movedRuleTables = []movedRuleTable{
	{oldName: "StrategyBotSignalSources", entity: &entities.TradingStrategySignalSource{}},
	{oldName: "StrategyBotConditionNodes", entity: &entities.TradingStrategyConditionNode{}},
	{
		oldName: "StrategyBotSignalSourceParameterValues",
		entity:  &entities.TradingStrategySignalSourceParameterValue{},
	},
}

// movedRuleIndex is an index that came along with a renamed table still carrying the
// name it was created under.
type movedRuleIndex struct {
	entity  any
	oldName string
	newName string
}

// movedRuleIndexes are those. Renaming rather than creating and dropping is what
// keeps the table indexed the whole way through.
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

// movedRuleColumn is a column that changed names when the rules moved off the bot.
type movedRuleColumn struct {
	entity  any
	oldName string
	newName string
}

// movedRuleColumns are those renames. The first two used to name the bot the rules
// hung off and now name the trading strategy they belong to — the values in them are
// still bot identifiers when this runs, and moveRulesOntoTradingStrategies is what
// corrects them. The third only changed because the model it points at was renamed;
// its values were right all along.
//
// Renaming rather than adding is the whole point: the old column is declared NOT
// NULL, so a new one beside it would leave every write failing on a column no entity
// claims.
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

// renameMovedRuleTables carries the rules across to their new names without moving a
// single row: the tables, their indexes and the column that points into them are
// renamed in place.
//
// Copying rows into new tables was the alternative and was rejected. A copy has a
// half-way state, and a migration with a half-way state is a migration that can be
// interrupted into one.
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

		// The ORM's own RenameIndex is not usable here, for the reason its DropIndex
		// is not (see dropRetiredIndexes): on this driver it qualifies the index
		// with a function call, which is not valid there. The statement below is
		// what it was trying to write.
		//
		// It carries no value from anywhere: both names are constants in the list
		// above, and they go through the ORM's own identifier quoting rather than
		// being pasted into the text.
		renamed := schemaMigrator.database.Exec("ALTER INDEX IF EXISTS ? RENAME TO ?",
			clause.Column{Name: index.oldName}, clause.Column{Name: index.newName})
		if renamed.Error != nil {
			return fmt.Errorf("rename moved rule index %s: %w", index.oldName, renamed.Error)
		}
	}

	return nil
}

// prepareForTheMove builds the two things the move writes into — the table the
// trading strategies go in, and the column on a bot that names one — without any of
// the foreign keys that cannot hold until the move has run.
//
// Syncing the schema does both of these too, and does them properly. This is only
// what has to exist *first*, so that the move has somewhere to put its answer.
func (schemaMigrator *SchemaMigrator) prepareForTheMove() error {
	// Creating the table here declares only the keys that live *on* it — the one
	// naming its owner. The keys that would refuse the move live on the child
	// tables and are declared when those are synced, which happens afterwards.
	migrator := schemaMigrator.database.Migrator()

	// Nothing to prepare for on a database that has never held a bot: syncing the
	// schema builds all of this properly a moment later, and there is nothing to
	// move into it.
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

	// Two of these still point at StrategyBots and would refuse the very writes the
	// move is about to make. Dropped here rather than with the rest, because the
	// rest can wait until the schema is synced and these cannot.
	return schemaMigrator.dropRetiredConstraints()
}

// moveRulesOntoTradingStrategies gives every bot that still has none a trading
// strategy of its own, made of exactly the rules it was already running.
//
// The name is the bot's. Two bots of one owner cannot share a name, and before this
// there were no trading strategies at all, so nothing can collide.
//
// The children are repointed **by their own identifiers**, and every bot's are
// gathered before the first trading strategy is created. Repointing them by the
// value they carry would be a trap: that value is a bot identifier, the new trading
// strategy identifiers come from their own sequence, and the two ranges overlap.
//
// A bot whose identifier is already set is skipped, which is what makes running this
// a second time do nothing at all.
func (schemaMigrator *SchemaMigrator) moveRulesOntoTradingStrategies() error {
	if !schemaMigrator.database.Migrator().HasTable(&entities.StrategyBot{}) {
		return nil
	}

	return schemaMigrator.database.Transaction(func(transaction *gorm.DB) error {
		unmovedBots := []entities.StrategyBot{}
		// The three columns are named rather than taking the whole row, and that is
		// not a saving. This runs on a connection that has already read this table
		// once and has just altered it: a statement whose result type changed under
		// a cached plan is refused outright. Three columns this step does not touch
		// keep the same result type on both sides of the change.
		if findError := transaction.Model(&entities.StrategyBot{}).
			Select("id", "owner_id", "name").
			Where(clause.Eq{Column: "trading_strategy_id", Value: 0}).
			Order("id ASC").
			Find(&unmovedBots).Error; findError != nil {
			return fmt.Errorf("find bots without a trading strategy: %w", findError)
		}

		// Every identifier this move hands out is pushed past every bot identifier
		// before the first one is used. Until a row is moved it still carries a bot
		// identifier, and (trading_strategy_id, label) is unique — so a trading
		// strategy whose identifier happens to equal a bot that has not been moved
		// yet collides with that bot's own rows, for as long as the move runs.
		//
		// Two bots sharing the label "A" is not unusual; it is the default the
		// screen offers. So this is the ordinary case, not a corner of it.
		if pushError := schemaMigrator.pushIdentifiersPastEveryBot(transaction); pushError != nil {
			return pushError
		}

		// Every bot's rows are gathered before any trading strategy is created, not
		// one bot at a time. Gathering per bot would read a table earlier bots have
		// already been written into.
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

// pushIdentifiersPastEveryBot makes the next trading strategy identifier greater
// than every bot identifier, and greater than every trading strategy identifier
// already handed out.
//
// This is the one statement in the migrator written by hand, and the ORM is the
// reason: advancing a sequence has no word in it. Nothing is pasted into the text —
// the one value it needs goes through the ORM's own binding.
func (schemaMigrator *SchemaMigrator) pushIdentifiersPastEveryBot(transaction *gorm.DB) error {
	highestBotID := uint(0)
	if readError := transaction.Model(&entities.StrategyBot{}).
		Select("COALESCE(MAX(id), 0)").Scan(&highestBotID).Error; readError != nil {
		return fmt.Errorf("read the highest bot identifier: %w", readError)
	}

	// Nothing to push past, and nothing to move either. Pushing anyway would burn
	// the first identifier on a database that has never held a bot.
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

// ruleRowIDsOf is which rows of one rule table currently hang off this bot, read
// before anything is written so that the answer cannot be disturbed by the writing.
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

// repointRuleRows moves these exact rows onto this trading strategy, one identifier
// at a time. A migration runs over a handful of rows per bot, so naming each one is
// cheaper than a condition that could ever match a row it was not meant to.
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

// clearOwnerlessRows empties every table that still predates the owner it may not
// be without. What hangs off those rows goes with them through the cascades already
// on the tables.
//
// The condition is the point: each one fires only while its table exists and has no
// owner column, which is exactly once, and never again after the migration that
// follows it. It is therefore not a script somebody has to remember to run once — it
// is a statement about a shape that stops being true the moment it has done its
// work, in the same spirit as the retired columns above.
//
// Assigning the rows to somebody instead was the alternative, and it was rejected:
// picking an owner for a test row is a guess, and a guess here would leave "every
// one of these belongs to a person" true only by accident.
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
