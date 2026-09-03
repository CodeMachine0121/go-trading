package persistence_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// retiredStrategyColumns are the columns a strategy used to carry. They described one
// run of an algorithm rather than the algorithm, and moved onto the calculation that
// runs a strategy.
var retiredStrategyColumns = []string{"aggregation_interval", "candle_count"}

func TestSchemaMigratorDropsColumnsNoEntityClaimsAnyMore(t *testing.T) {
	// Syncing the schema only ever adds and widens, so a column left behind by a
	// removed field would sit on the table forever — and a reader who finds
	// aggregation_interval still there has every reason to believe a strategy still
	// remembers it.
	database := newTestDatabase(t)
	migrator := database.Migrator()

	// Putting the columns back has to be said in raw SQL: syncing the schema works
	// from the entity, and the entity no longer has these fields to name. Raw SQL
	// belongs to the test alone — this is the one place that needs to describe a
	// database as it was, not as the code says it should be.
	for _, retiredColumn := range retiredStrategyColumns {
		require.NoError(t, database.Exec(
			`ALTER TABLE "Strategies" ADD COLUMN IF NOT EXISTS "`+retiredColumn+`" text`).Error,
			"這個測試得先把欄位種回去，才有東西可以被刪掉")
		require.True(t, migrator.HasColumn(&entities.Strategy{}, retiredColumn))
	}

	_, migrateError := persistence.NewSchemaMigrator(database).Migrate()

	require.NoError(t, migrateError)
	for _, retiredColumn := range retiredStrategyColumns {
		assert.False(t, migrator.HasColumn(&entities.Strategy{}, retiredColumn),
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
	// Dropping is aimed at two named columns and nothing else. What a strategy
	// actually is — its name, its script, the kind of value it produces and when it
	// was first saved — has to come through untouched, or the migration would be
	// quietly destroying the thing it was meant to leave alone.
	database := newTestDatabase(t)
	strategyRepository := persistence.NewStrategyRepository(database)
	savedStrategy, saveError := strategyRepository.Save(t.Context(), entities.Strategy{
		Name:       "二十根均線",
		Script:     "func Calculate(candles []vo.KCandleVo) map[string][]float64 { return nil }",
		ResultType: "floatList",
	})
	require.NoError(t, saveError)

	for _, retiredColumn := range retiredStrategyColumns {
		require.NoError(t, database.Exec(
			`ALTER TABLE "Strategies" ADD COLUMN IF NOT EXISTS "`+retiredColumn+`" text`).Error)
	}

	_, migrateError := persistence.NewSchemaMigrator(database).Migrate()

	require.NoError(t, migrateError)
	readBack, findError := strategyRepository.FindOne(t.Context(), savedStrategy.ID)
	require.NoError(t, findError)
	assert.Equal(t, savedStrategy.Name, readBack.Name)
	assert.Equal(t, savedStrategy.Script, readBack.Script)
	assert.Equal(t, savedStrategy.ResultType, readBack.ResultType)
	assert.WithinDuration(t, savedStrategy.CreatedAt, readBack.CreatedAt, time.Millisecond)
}
