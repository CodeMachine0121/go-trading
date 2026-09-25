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

var retiredStrategyScriptColumns = []string{"aggregation_interval", "candle_count"}

func TestSchemaMigratorDropsColumnsNoEntityClaimsAnyMore(t *testing.T) {
	// Schema sync only adds and widens, so removed fields' columns must be dropped explicitly.
	database := newTestDatabase(t)
	migrator := database.Migrator()

	// Raw SQL restores the old columns because the entity no longer names them.
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

// The contract-replay columns (account kind, suggested borrowing) describe a capability the system no longer has.
func TestSchemaMigratorDropsTheColumnsThatOutlivedContractReplays(t *testing.T) {
	testCases := []struct {
		name        string
		table       string
		column      string
		columnType  string
		hasColumnOf func() (any, string)
	}{
		{
			name:       "which kind of account a set of rules was written for",
			table:      "TradingStrategies",
			column:     "trading_mode",
			columnType: "text",
			hasColumnOf: func() (any, string) {
				return &entities.TradingStrategy{}, "trading_mode"
			},
		},
		{
			name:       "how much a bot suggested borrowing",
			table:      "StrategyBots",
			column:     "position_plan_leverage",
			columnType: "numeric",
			hasColumnOf: func() (any, string) {
				return &entities.StrategyBot{}, "position_plan_leverage"
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			database := newTestDatabase(t)
			migrator := database.Migrator()

			// Raw SQL restores the old column because the entity no longer names it.
			require.NoError(t, database.Exec(
				`ALTER TABLE "`+testCase.table+`" ADD COLUMN IF NOT EXISTS "`+
					testCase.column+`" `+testCase.columnType).Error)
			entity, column := testCase.hasColumnOf()
			require.True(t, migrator.HasColumn(entity, column))

			_, migrateError := persistence.NewSchemaMigrator(database).Migrate()

			require.NoError(t, migrateError)
			assert.False(t, migrator.HasColumn(entity, column),
				"%s 應該已經被刪掉", testCase.column)
		})
	}
}

func TestSchemaMigratorRunsTwiceWithTheSameResult(t *testing.T) {
	// Dropping already-dropped columns must succeed so a second startup passes migration.
	database := newTestDatabase(t)

	firstTables, firstError := persistence.NewSchemaMigrator(database).Migrate()
	require.NoError(t, firstError)

	secondTables, secondError := persistence.NewSchemaMigrator(database).Migrate()

	require.NoError(t, secondError)
	assert.Equal(t, firstTables, secondTables)
	assert.Contains(t, secondTables, "Strategies")
}

func TestSchemaMigratorLeavesTheAlgorithmAloneWhileDroppingThePlan(t *testing.T) {
	// Only the two retired columns are dropped; the strategy script's real data must survive.
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
	// Pre-ownership rows are cleared (assigning an owner would be a guess) when the table exists without an owner column; restoring the column recreates that state.
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
	// Once the owner column exists the clearing must not run again, or every restart would wipe data.
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
	// AutoMigrate never drops indexes, so the obsolete global unique name index must be dropped explicitly.
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
	// The first migration must not fail on a missing StrategyScripts table.
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

// asTheShapeBeforeTheRulesMoved restores, via raw SQL, the schema where a bot carried its own signal sources and condition trees.
func asTheShapeBeforeTheRulesMoved(t *testing.T, database *gorm.DB) {
	t.Helper()

	for _, statement := range []string{
		// The foreign keys point back at the bot, as in the old shape.
		`ALTER TABLE "TradingStrategySignalSources"
		   DROP CONSTRAINT IF EXISTS "fk_TradingStrategies_signal_sources"`,
		`ALTER TABLE "TradingStrategyConditionNodes"
		   DROP CONSTRAINT IF EXISTS "fk_TradingStrategies_condition_nodes"`,
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
		`ALTER TABLE "StrategyBotSignalSources"
		   ADD CONSTRAINT "fk_StrategyBots_signal_sources"
		   FOREIGN KEY ("strategy_bot_id") REFERENCES "StrategyBots"("id") ON DELETE CASCADE`,
		`ALTER TABLE "StrategyBotConditionNodes"
		   ADD CONSTRAINT "fk_StrategyBots_condition_nodes"
		   FOREIGN KEY ("strategy_bot_id") REFERENCES "StrategyBots"("id") ON DELETE CASCADE`,
	} {
		require.NoError(t, database.Exec(statement).Error,
			"這個測試得先把資料庫變回搬家以前的樣子，才有東西可以被搬")
	}
}

// asAStranger also drops the trading strategies table; kept separate because one test depends on that table's identifier sequence.
func asAStranger(t *testing.T, database *gorm.DB) {
	t.Helper()

	asTheShapeBeforeTheRulesMoved(t, database)
	require.NoError(t, database.Exec(`DROP TABLE "TradingStrategies" CASCADE`).Error)
}

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

// Running bots keep their rules and keep running after migration.
func TestSchemaMigratorGivesEveryExistingBotItsOwnTradingStrategy(t *testing.T) {
	database := newTestDatabase(t)
	require.NoError(t, database.Create(&entities.User{
		ID: 1, Email: "moved-rules@example.com", PasswordProof: "a-proof",
	}).Error)

	asAStranger(t, database)
	botID := aBotOfTheOldShape(t, database, 1, "我的機器人")

	_, migrateError := persistence.NewSchemaMigrator(database).Migrate()
	require.NoError(t, migrateError)

	movedBot := entities.StrategyBot{}
	require.NoError(t, database.Preload("TradingStrategy").First(&movedBot, botID).Error)

	// The rules become a trading strategy named after the bot and owned by the same person.
	require.NotZero(t, movedBot.TradingStrategyID)
	assert.Equal(t, "我的機器人", movedBot.TradingStrategy.Name)
	assert.Equal(t, uint(1), movedBot.TradingStrategy.OwnerID)
	assert.Equal(t, "running", movedBot.RunState)
	assert.Equal(t, "我的機器人", movedBot.Name)

	movedTradingStrategy := entities.TradingStrategy{}
	require.NoError(t, database.
		Preload("SignalSources").Preload("ConditionNodes").
		First(&movedTradingStrategy, movedBot.TradingStrategyID).Error)

	// The original rule rows are moved, not copied.
	require.Len(t, movedTradingStrategy.SignalSources, 1)
	assert.Equal(t, "A", movedTradingStrategy.SignalSources[0].Label)
	assert.Equal(t, uint(9), movedTradingStrategy.SignalSources[0].StrategyScriptID)
	require.Len(t, movedTradingStrategy.ConditionNodes, 2)
}

// A fresh sequence's identifiers overlap old bot identifiers still on the rule rows, which is where a mix-up would show.
func TestSchemaMigratorKeepsEachBotsRulesToItself(t *testing.T) {
	database := newTestDatabase(t)
	require.NoError(t, database.Create(&entities.User{
		ID: 1, Email: "two-bots@example.com", PasswordProof: "a-proof",
	}).Error)

	asAStranger(t, database)
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

// The migration must be safe to repeat on restart.
func TestSchemaMigratorMovesTheRulesOnlyOnce(t *testing.T) {
	database := newTestDatabase(t)
	require.NoError(t, database.Create(&entities.User{
		ID: 1, Email: "twice@example.com", PasswordProof: "a-proof",
	}).Error)

	asAStranger(t, database)
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

// A failed attempt advances the sequence (Postgres sequences do not roll back), so a new identifier can equal a later bot's and must not let that bot sweep up an earlier one's rules.
func TestSchemaMigratorKeepsEachBotsRulesToItselfAfterAFailedAttempt(t *testing.T) {
	database := newTestDatabase(t)
	require.NoError(t, database.Create(&entities.User{
		ID: 1, Email: "retry@example.com", PasswordProof: "a-proof",
	}).Error)

	asTheShapeBeforeTheRulesMoved(t, database)
	firstBotID := aBotOfTheOldShape(t, database, 1, "第一台")
	secondBotID := aBotOfTheOldShape(t, database, 1, "第二台")
	require.Less(t, firstBotID, secondBotID)

	// Simulates the sequence position a rolled-back first attempt can leave.
	withTheNextIdentifierBeing(t, database, secondBotID)

	_, migrateError := persistence.NewSchemaMigrator(database).Migrate()
	require.NoError(t, migrateError)

	for _, botID := range []uint{firstBotID, secondBotID} {
		bot := entities.StrategyBot{}
		require.NoError(t, database.First(&bot, botID).Error)

		sourceCount := int64(0)
		require.NoError(t, database.Model(&entities.TradingStrategySignalSource{}).
			Where(clause.Eq{Column: "trading_strategy_id", Value: bot.TradingStrategyID}).
			Count(&sourceCount).Error)
		assert.Equal(t, int64(1), sourceCount, "%s 的信號來源應該還在它自己身上", bot.Name)

		nodeCount := int64(0)
		require.NoError(t, database.Model(&entities.TradingStrategyConditionNode{}).
			Where(clause.Eq{Column: "trading_strategy_id", Value: bot.TradingStrategyID}).
			Count(&nodeCount).Error)
		assert.Equal(t, int64(2), nodeCount, "%s 的兩棵條件樹應該還在它自己身上", bot.Name)
		assert.Greater(t, bot.TradingStrategyID, secondBotID)
	}
}

// withTheNextIdentifierBeing moves the table's sequence onto an unmoved bot's identifier, via raw SQL since the ORM cannot.
func withTheNextIdentifierBeing(t *testing.T, database *gorm.DB, nextIdentifier uint) {
	t.Helper()

	require.NoError(t, database.Exec(
		`SELECT setval(pg_get_serial_sequence('"TradingStrategies"', 'id'), ?, false)`,
		nextIdentifier).Error)
}
