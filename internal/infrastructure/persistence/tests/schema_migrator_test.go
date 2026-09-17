package persistence_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
