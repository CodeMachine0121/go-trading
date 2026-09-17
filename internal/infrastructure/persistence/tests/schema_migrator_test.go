package persistence_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// retiredStrategyScriptColumns are the columns a strategy script used to carry. They described one
// run of an algorithm rather than the algorithm, and moved onto the calculation that
// runs a strategy script.
var retiredStrategyScriptColumns = []string{"aggregation_interval", "candle_count"}

func TestSchemaMigratorDropsColumnsNoEntityClaimsAnyMore(t *testing.T) {
	// Syncing the schema only ever adds and widens, so a column left behind by a
	// removed field would sit on the table forever — and a reader who finds
	// aggregation_interval still there has every reason to believe a strategy script still
	// remembers it.
	database := newTestDatabase(t)
	migrator := database.Migrator()

	// Putting the columns back has to be said in raw SQL: syncing the schema works
	// from the entity, and the entity no longer has these fields to name. Raw SQL
	// belongs to the test alone — this is the one place that needs to describe a
	// database as it was, not as the code says it should be.
	for _, retiredColumn := range retiredStrategyScriptColumns {
		require.NoError(t, database.Exec(
			`ALTER TABLE "Strategies" ADD COLUMN IF NOT EXISTS "`+retiredColumn+`" text`).Error,
			"這個測試得先把欄位種回去，才有東西可以被刪掉")
		require.True(t, migrator.HasColumn(&entities.StrategyScript{}, retiredColumn))
	}

	_, migrateError := persistence.NewSchemaMigrator(database).Migrate()

	require.NoError(t, migrateError)
	for _, retiredColumn := range retiredStrategyScriptColumns {
		assert.False(t, migrator.HasColumn(&entities.StrategyScript{}, retiredColumn),
			"%s 應該已經被刪掉", retiredColumn)
	}
}

func TestSchemaMigratorRunsTwiceWithTheSameResult(t *testing.T) {
	// The columns are already gone by the time this runs, so dropping has nothing to
	// do — and having nothing to do must not be a failure, or the second start of
	// the server would never get past migration.
	database := newTestDatabase(t)

	firstTables, firstError := persistence.NewSchemaMigrator(database).Migrate()
	require.NoError(t, firstError)

	secondTables, secondError := persistence.NewSchemaMigrator(database).Migrate()

	require.NoError(t, secondError)
	assert.Equal(t, firstTables, secondTables)
	assert.Contains(t, secondTables, "Strategies")
}

func TestSchemaMigratorLeavesTheAlgorithmAloneWhileDroppingThePlan(t *testing.T) {
	// Dropping is aimed at two named columns and nothing else. What a strategy script
	// actually is — its name, its script, the kind of value it produces and when it
	// was first saved — has to come through untouched, or the migration would be
	// quietly destroying the thing it was meant to leave alone.
	database := newStrategyScriptTestDatabase(t)
	strategyScriptRepository := persistence.NewStrategyScriptRepository(database)
	savedStrategyScript, saveError := strategyScriptRepository.Save(t.Context(), entities.StrategyScript{
		OwnerID:    strategyScriptRowOwnerID,
		Name:       "二十根均線",
		Script:     "func Calculate(candles []vo.KCandleVo) map[string][]float64 { return nil }",
		ResultType: "floatList",
	})
	require.NoError(t, saveError)

	for _, retiredColumn := range retiredStrategyScriptColumns {
		require.NoError(t, database.Exec(
			`ALTER TABLE "Strategies" ADD COLUMN IF NOT EXISTS "`+retiredColumn+`" text`).Error)
	}

	_, migrateError := persistence.NewSchemaMigrator(database).Migrate()

	require.NoError(t, migrateError)
	readBack, findError := strategyScriptRepository.FindOne(t.Context(), savedStrategyScript.ID)
	require.NoError(t, findError)
	assert.Equal(t, savedStrategyScript.Name, readBack.Name)
	assert.Equal(t, savedStrategyScript.Script, readBack.Script)
	assert.Equal(t, savedStrategyScript.ResultType, readBack.ResultType)
	assert.WithinDuration(t, savedStrategyScript.CreatedAt, readBack.CreatedAt, time.Millisecond)
}

func TestSchemaMigratorClearsStrategyScriptsSavedBeforeAnybodyOwnedThem(t *testing.T) {
	// A strategy script cannot exist without an owner any more, and a table with rows in it
	// cannot grow a column that may not be null. The rows saved before ownership are
	// therefore cleared — assigning them to somebody would be a guess, and a guess
	// here would leave "every strategy script has an owner" true only by accident.
	//
	// The condition that clears them is "the table exists and has no owner column",
	// which stops being true the moment the migration that follows it runs. Putting
	// the column back is how this test reaches that state again.
	database := newStrategyScriptTestDatabase(t)
	strategyScriptRepository := persistence.NewStrategyScriptRepository(database)
	_, saveError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("存在既有資料裡的"))
	require.NoError(t, saveError)
	require.NoError(t, database.Exec(`ALTER TABLE "Strategies" DROP COLUMN "owner_id"`).Error)

	_, migrateError := persistence.NewSchemaMigrator(database).Migrate()

	require.NoError(t, migrateError)
	strategyScripts, findError := strategyScriptRepository.FindAllOwnedBy(t.Context(), strategyScriptRowOwnerID)
	require.NoError(t, findError)
	assert.Empty(t, strategyScripts)
}

func TestSchemaMigratorRunTwiceLeavesOwnedStrategyScriptsAlone(t *testing.T) {
	// The clearing must not fire again once the column is there, or every restart
	// would wipe everybody's work.
	database := newStrategyScriptTestDatabase(t)
	strategyScriptRepository := persistence.NewStrategyScriptRepository(database)
	_, saveError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("留下來的"))
	require.NoError(t, saveError)

	_, migrateError := persistence.NewSchemaMigrator(database).Migrate()

	require.NoError(t, migrateError)
	strategyScripts, findError := strategyScriptRepository.FindAllOwnedBy(t.Context(), strategyScriptRowOwnerID)
	require.NoError(t, findError)
	require.Len(t, strategyScripts, 1)
	assert.Equal(t, "留下來的", strategyScripts[0].Name)
}

func TestSchemaMigratorDropsTheIndexThatMadeANameUniqueEverywhere(t *testing.T) {
	// AutoMigrate adds indexes and never drops them, and a leftover unique index is
	// worse than a leftover column: it goes on enforcing a rule the system no longer
	// holds. This one held the first person to save a name against everybody else.
	database := newStrategyScriptTestDatabase(t)
	strategyScriptRepository := persistence.NewStrategyScriptRepository(database)
	secondOwnerID := aSecondOwner(t, database)
	require.NoError(t, database.Exec(
		`CREATE UNIQUE INDEX "idx_strategies_name" ON "Strategies" ("name")`).Error)

	_, migrateError := persistence.NewSchemaMigrator(database).Migrate()

	require.NoError(t, migrateError)
	_, firstError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("二十根均線"))
	require.NoError(t, firstError)
	secondPersons := strategyScriptNamed("二十根均線")
	secondPersons.OwnerID = secondOwnerID
	_, secondError := strategyScriptRepository.Save(t.Context(), secondPersons)
	require.NoError(t, secondError, "the two of them may hold the same name")
}

func TestSchemaMigratorBuildsAStrategyScriptTableThatIsNotThereYet(t *testing.T) {
	// The very first migration meets a database with no StrategyScripts table at all.
	// Clearing the rows saved before ownership must not go looking for a table
	// nobody has built yet.
	database := newStrategyScriptTestDatabase(t)
	require.NoError(t, database.Exec(`DROP TABLE "Strategies" CASCADE`).Error)

	migratedTables, migrateError := persistence.NewSchemaMigrator(database).Migrate()

	require.NoError(t, migrateError)
	assert.Contains(t, migratedTables, "Strategies")
	strategyScripts, findError := persistence.NewStrategyScriptRepository(database).
		FindAllOwnedBy(t.Context(), strategyScriptRowOwnerID)
	require.NoError(t, findError)
	assert.Empty(t, strategyScripts)
}

// asTheShapeBeforeTheRulesMoved puts the database back the way it was when a bot
// carried its own signal sources and its own two condition trees.
//
// It is raw SQL for the reason the retired columns above are: syncing the schema
// works from the entities, and the entities no longer describe this shape. Raw SQL
// belongs to the test alone — this is the one place that has to describe a database
// as it was rather than as the code says it should be.
func asTheShapeBeforeTheRulesMoved(t *testing.T, database *gorm.DB) {
	t.Helper()

	for _, statement := range []string{
		`ALTER TABLE "TradingStrategySignalSources" RENAME TO "StrategyBotSignalSources"`,
		`ALTER TABLE "TradingStrategyConditionNodes" RENAME TO "StrategyBotConditionNodes"`,
		`ALTER TABLE "TradingStrategySignalSourceParameterValues"
		   RENAME TO "StrategyBotSignalSourceParameterValues"`,
		`ALTER TABLE "StrategyBotSignalSources" RENAME COLUMN "trading_strategy_id" TO "strategy_bot_id"`,
		`ALTER TABLE "StrategyBotConditionNodes" RENAME COLUMN "trading_strategy_id" TO "strategy_bot_id"`,
		`ALTER TABLE "StrategyBotSignalSourceParameterValues"
		   RENAME COLUMN "trading_strategy_signal_source_id" TO "strategy_bot_signal_source_id"`,
		`ALTER INDEX "idx_trading_strategy_signal_sources_strategy"
		   RENAME TO "idx_strategy_bot_signal_sources_bot"`,
		`ALTER INDEX "idx_trading_strategy_signal_sources_strategy_label"
		   RENAME TO "idx_strategy_bot_signal_sources_bot_label"`,
		`ALTER INDEX "idx_trading_strategy_signal_sources_script"
		   RENAME TO "idx_strategy_bot_signal_sources_strategy"`,
		`ALTER INDEX "idx_trading_strategy_condition_nodes_strategy"
		   RENAME TO "idx_strategy_bot_condition_nodes_bot"`,
		`ALTER INDEX "idx_trading_strategy_condition_nodes_parent"
		   RENAME TO "idx_strategy_bot_condition_nodes_parent"`,
		`ALTER INDEX "idx_trading_strategy_source_parameter_values_source"
		   RENAME TO "idx_strategy_bot_source_parameter_values_source"`,
		`ALTER TABLE "StrategyBots" DROP COLUMN "trading_strategy_id"`,
		`DROP TABLE "TradingStrategies" CASCADE`,
	} {
		require.NoError(t, database.Exec(statement).Error,
			"這個測試得先把資料庫變回搬家以前的樣子，才有東西可以被搬")
	}
}

// aBotOfTheOldShape plants one bot carrying its own rules, exactly as one was stored
// before they moved, and hands back its identifier.
func aBotOfTheOldShape(t *testing.T, database *gorm.DB, ownerID uint, name string) uint {
	t.Helper()

	botID := uint(0)
	require.NoError(t, database.Raw(
		`INSERT INTO "StrategyBots"
		   ("owner_id","name","symbol","trigger_interval_minutes","run_state","next_run_at",
		    "last_sent_signal","halt_reason","conflicting","created_at","updated_at")
		 VALUES (?,?,'BTCUSDT',5,'running',now(),'','',false,now(),now()) RETURNING id`,
		ownerID, name).Scan(&botID).Error)
	require.NotZero(t, botID)

	require.NoError(t, database.Exec(
		`INSERT INTO "StrategyBotSignalSources"
		   ("strategy_bot_id","label","strategy_id","aggregation_interval")
		 VALUES (?,'A',9,'1h')`, botID).Error)
	require.NoError(t, database.Exec(
		`INSERT INTO "StrategyBotConditionNodes"
		   ("strategy_bot_id","side","position","operator","source_label","expected_signal")
		 VALUES (?,'buy',0,'','A','buy'), (?,'sell',0,'','A','sell')`, botID, botID).Error)

	return botID
}

// A bot that was already running keeps every set of rules it was running, and keeps
// running. Somebody who left a bot watching overnight must find it watching the same
// thing in the morning.
func TestSchemaMigratorGivesEveryExistingBotItsOwnTradingStrategy(t *testing.T) {
	database := newTestDatabase(t)
	require.NoError(t, database.Create(&entities.User{
		ID: 1, Email: "moved-rules@example.com", PasswordProof: "a-proof",
	}).Error)

	asTheShapeBeforeTheRulesMoved(t, database)
	botID := aBotOfTheOldShape(t, database, 1, "我的機器人")

	_, migrateError := persistence.NewSchemaMigrator(database).Migrate()
	require.NoError(t, migrateError)

	movedBot := entities.StrategyBot{}
	require.NoError(t, database.Preload("TradingStrategy").First(&movedBot, botID).Error)

	// The rules are now a thing of their own, named after the bot that was running
	// them, and belonging to the same person.
	require.NotZero(t, movedBot.TradingStrategyID)
	assert.Equal(t, "我的機器人", movedBot.TradingStrategy.Name)
	assert.Equal(t, uint(1), movedBot.TradingStrategy.OwnerID)
	// Still running, still due, still the same bot.
	assert.Equal(t, "running", movedBot.RunState)
	assert.Equal(t, "我的機器人", movedBot.Name)

	movedTradingStrategy := entities.TradingStrategy{}
	require.NoError(t, database.
		Preload("SignalSources").Preload("ConditionNodes").
		First(&movedTradingStrategy, movedBot.TradingStrategyID).Error)

	// Not one of them is lost, and none of them is a copy: these are the very rows
	// the bot was running.
	require.Len(t, movedTradingStrategy.SignalSources, 1)
	assert.Equal(t, "A", movedTradingStrategy.SignalSources[0].Label)
	assert.Equal(t, uint(9), movedTradingStrategy.SignalSources[0].StrategyScriptID)
	require.Len(t, movedTradingStrategy.ConditionNodes, 2)
}

// Two bots must not end up sharing one set of rules, and neither must end up with
// the other's. The identifiers a fresh sequence hands out overlap with the bot
// identifiers the rows are still carrying, so this is where that would show.
func TestSchemaMigratorKeepsEachBotsRulesToItself(t *testing.T) {
	database := newTestDatabase(t)
	require.NoError(t, database.Create(&entities.User{
		ID: 1, Email: "two-bots@example.com", PasswordProof: "a-proof",
	}).Error)

	asTheShapeBeforeTheRulesMoved(t, database)
	firstBotID := aBotOfTheOldShape(t, database, 1, "第一台")
	secondBotID := aBotOfTheOldShape(t, database, 1, "第二台")

	_, migrateError := persistence.NewSchemaMigrator(database).Migrate()
	require.NoError(t, migrateError)

	firstBot := entities.StrategyBot{}
	require.NoError(t, database.Preload("TradingStrategy").First(&firstBot, firstBotID).Error)
	secondBot := entities.StrategyBot{}
	require.NoError(t, database.Preload("TradingStrategy").First(&secondBot, secondBotID).Error)

	assert.Equal(t, "第一台", firstBot.TradingStrategy.Name)
	assert.Equal(t, "第二台", secondBot.TradingStrategy.Name)
	assert.NotEqual(t, firstBot.TradingStrategyID, secondBot.TradingStrategyID)

	for _, bot := range []entities.StrategyBot{firstBot, secondBot} {
		sourceCount := int64(0)
		require.NoError(t, database.Model(&entities.TradingStrategySignalSource{}).
			Where(clause.Eq{Column: "trading_strategy_id", Value: bot.TradingStrategyID}).
			Count(&sourceCount).Error)
		assert.Equal(t, int64(1), sourceCount, "%s 的信號來源應該還在它自己身上", bot.Name)
	}
}

// Migrating again must not hand out a second set of rules to a bot that already has
// one — a server restarts, and a migration that is not safe to repeat is a migration
// that breaks on the second start.
func TestSchemaMigratorMovesTheRulesOnlyOnce(t *testing.T) {
	database := newTestDatabase(t)
	require.NoError(t, database.Create(&entities.User{
		ID: 1, Email: "twice@example.com", PasswordProof: "a-proof",
	}).Error)

	asTheShapeBeforeTheRulesMoved(t, database)
	botID := aBotOfTheOldShape(t, database, 1, "我的機器人")

	_, firstError := persistence.NewSchemaMigrator(database).Migrate()
	require.NoError(t, firstError)

	firstBot := entities.StrategyBot{}
	require.NoError(t, database.First(&firstBot, botID).Error)

	_, secondError := persistence.NewSchemaMigrator(database).Migrate()
	require.NoError(t, secondError)

	secondBot := entities.StrategyBot{}
	require.NoError(t, database.First(&secondBot, botID).Error)
	assert.Equal(t, firstBot.TradingStrategyID, secondBot.TradingStrategyID)

	tradingStrategyCount := int64(0)
	require.NoError(t, database.Model(&entities.TradingStrategy{}).Count(&tradingStrategyCount).Error)
	assert.Equal(t, int64(1), tradingStrategyCount)
}
