package persistence_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// strategyNamed is a strategy that differs from its siblings only by name, so that a
// test about names is not also a test about anything else.
func strategyNamed(name string) entities.Strategy {
	return entities.Strategy{
		Name:                name,
		Script:              "func Calculate(candles []vo.KCandleVo) map[string]float64 { return nil }",
		ResultType:          "float",
		AggregationInterval: "5m",
		CandleCount:         20,
	}
}

func TestStrategyRepositorySaveHandsBackTheStrategyAsStored(t *testing.T) {
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))

	savedStrategy, saveError := strategyRepository.Save(strategyNamed("二十根均線"))

	require.NoError(t, saveError)
	assert.Positive(t, savedStrategy.ID)
	assert.False(t, savedStrategy.CreatedAt.IsZero())
	assert.False(t, savedStrategy.UpdatedAt.IsZero())
	assert.Equal(t, "二十根均線", savedStrategy.Name)
}

func TestStrategyRepositorySaveRefusesANameAlreadyHeld(t *testing.T) {
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))
	_, saveError := strategyRepository.Save(strategyNamed("二十根均線"))
	require.NoError(t, saveError)

	_, conflictError := strategyRepository.Save(strategyNamed("二十根均線"))

	require.ErrorIs(t, conflictError, domains.ErrStrategyNameConflict)
	assert.Contains(t, conflictError.Error(), "二十根均線")
}

func TestStrategyRepositorySaveTellsNamesApartByCase(t *testing.T) {
	// A person may well use case to tell two versions apart, and deciding for them
	// which spellings count as the same name gets in the way more often than it helps.
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))

	_, upperCaseError := strategyRepository.Save(strategyNamed("MA20"))
	_, lowerCaseError := strategyRepository.Save(strategyNamed("ma20"))

	require.NoError(t, upperCaseError)
	require.NoError(t, lowerCaseError)
}

func TestStrategyRepositorySaveFreesANameThatWasDeleted(t *testing.T) {
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))
	savedStrategy, saveError := strategyRepository.Save(strategyNamed("二十根均線"))
	require.NoError(t, saveError)
	require.NoError(t, strategyRepository.Delete(savedStrategy.ID))

	_, reuseError := strategyRepository.Save(strategyNamed("二十根均線"))

	require.NoError(t, reuseError)
}

func TestStrategyRepositoryFindOne(t *testing.T) {
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))
	savedStrategy, saveError := strategyRepository.Save(strategyNamed("二十根均線"))
	require.NoError(t, saveError)

	t.Run("returns the strategy carrying that identifier", func(t *testing.T) {
		foundStrategy, findError := strategyRepository.FindOne(savedStrategy.ID)

		require.NoError(t, findError)
		assert.Equal(t, savedStrategy.ID, foundStrategy.ID)
		assert.Equal(t, "二十根均線", foundStrategy.Name)
	})

	t.Run("reports not found when no strategy carries it", func(t *testing.T) {
		_, findError := strategyRepository.FindOne(savedStrategy.ID + 999)

		require.ErrorIs(t, findError, domains.ErrStrategyNotFound)
	})
}

func TestStrategyRepositoryFindAllOrdersByName(t *testing.T) {
	// Named in plain letters on purpose: the point being made is that the order is
	// the collection's and not the order they went in, and letters sort the same way
	// under every collation the database might be running.
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))
	// The insertion order is neither the expected order nor its reverse, so an
	// ordering taken from when a strategy was saved cannot pass by coincidence.
	for _, name := range []string{"MA60", "RSI14", "MA20"} {
		_, saveError := strategyRepository.Save(strategyNamed(name))
		require.NoError(t, saveError)
	}

	strategies, findError := strategyRepository.FindAll()

	require.NoError(t, findError)
	foundNames := make([]string, 0, len(strategies))
	for _, strategy := range strategies {
		foundNames = append(foundNames, strategy.Name)
	}
	assert.Equal(t, []string{"MA20", "MA60", "RSI14"}, foundNames)
}

func TestStrategyRepositoryFindAllOnAnEmptyCollection(t *testing.T) {
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))

	strategies, findError := strategyRepository.FindAll()

	require.NoError(t, findError)
	assert.Empty(t, strategies)
}

func TestStrategyRepositoryUpdateRewritesTheFiveThingsAStrategyRemembers(t *testing.T) {
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))
	savedStrategy, saveError := strategyRepository.Save(strategyNamed("二十根均線"))
	require.NoError(t, saveError)

	rewritten := entities.Strategy{
		ID:                  savedStrategy.ID,
		Name:                "六十根均線",
		Script:              "func Calculate(candles []vo.KCandleVo) map[string][]bool { return nil }",
		ResultType:          "boolList",
		AggregationInterval: "1h",
		CandleCount:         60,
	}

	updatedStrategy, updateError := strategyRepository.Update(rewritten)

	require.NoError(t, updateError)
	assert.Equal(t, "六十根均線", updatedStrategy.Name)
	assert.Equal(t, rewritten.Script, updatedStrategy.Script)
	assert.Equal(t, "boolList", updatedStrategy.ResultType)
	assert.Equal(t, "1h", updatedStrategy.AggregationInterval)
	assert.Equal(t, 60, updatedStrategy.CandleCount)
}

func TestStrategyRepositoryUpdateLeavesTheIdentifierAndTheFirstSavedTimeAlone(t *testing.T) {
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))
	savedStrategy, saveError := strategyRepository.Save(strategyNamed("二十根均線"))
	require.NoError(t, saveError)

	// A rewrite that tries to move both of them, to prove they are out of reach
	// rather than merely left unset by a well-behaved caller.
	rewritten := strategyNamed("六十根均線")
	rewritten.ID = savedStrategy.ID
	rewritten.CreatedAt = savedStrategy.CreatedAt.Add(-48 * time.Hour)

	updatedStrategy, updateError := strategyRepository.Update(rewritten)

	require.NoError(t, updateError)
	assert.Equal(t, savedStrategy.ID, updatedStrategy.ID)
	assert.WithinDuration(t, savedStrategy.CreatedAt, updatedStrategy.CreatedAt, time.Millisecond)
	assert.False(t, updatedStrategy.UpdatedAt.Before(updatedStrategy.CreatedAt))
}

func TestStrategyRepositoryUpdateToItsOwnNameIsNotAConflict(t *testing.T) {
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))
	savedStrategy, saveError := strategyRepository.Save(strategyNamed("二十根均線"))
	require.NoError(t, saveError)

	rewritten := strategyNamed("二十根均線")
	rewritten.ID = savedStrategy.ID
	rewritten.CandleCount = 60

	updatedStrategy, updateError := strategyRepository.Update(rewritten)

	require.NoError(t, updateError)
	assert.Equal(t, 60, updatedStrategy.CandleCount)
}

func TestStrategyRepositoryUpdateRefusesAnotherStrategysName(t *testing.T) {
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))
	_, firstError := strategyRepository.Save(strategyNamed("二十根均線"))
	require.NoError(t, firstError)
	secondStrategy, secondError := strategyRepository.Save(strategyNamed("六十根均線"))
	require.NoError(t, secondError)

	rewritten := strategyNamed("二十根均線")
	rewritten.ID = secondStrategy.ID

	_, conflictError := strategyRepository.Update(rewritten)

	require.ErrorIs(t, conflictError, domains.ErrStrategyNameConflict)
}

func TestStrategyRepositoryUpdateReportsNotFound(t *testing.T) {
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))

	rewritten := strategyNamed("二十根均線")
	rewritten.ID = 999999

	_, updateError := strategyRepository.Update(rewritten)

	require.ErrorIs(t, updateError, domains.ErrStrategyNotFound)
}

func TestStrategyRepositoryDelete(t *testing.T) {
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))
	deletedStrategy, firstError := strategyRepository.Save(strategyNamed("二十根均線"))
	require.NoError(t, firstError)
	keptStrategy, secondError := strategyRepository.Save(strategyNamed("六十根均線"))
	require.NoError(t, secondError)

	require.NoError(t, strategyRepository.Delete(deletedStrategy.ID))

	t.Run("the deleted strategy is gone for good", func(t *testing.T) {
		_, findError := strategyRepository.FindOne(deletedStrategy.ID)

		require.ErrorIs(t, findError, domains.ErrStrategyNotFound)
	})

	t.Run("it no longer appears in the collection", func(t *testing.T) {
		strategies, findError := strategyRepository.FindAll()

		require.NoError(t, findError)
		require.Len(t, strategies, 1)
		assert.Equal(t, keptStrategy.ID, strategies[0].ID)
	})

	t.Run("deleting it again reports not found", func(t *testing.T) {
		deleteError := strategyRepository.Delete(deletedStrategy.ID)

		require.ErrorIs(t, deleteError, domains.ErrStrategyNotFound)
	})
}

func TestStrategyRepositoryDeleteReportsNotFound(t *testing.T) {
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))

	deleteError := strategyRepository.Delete(999999)

	require.ErrorIs(t, deleteError, domains.ErrStrategyNotFound)
}

func TestStrategyRepositorySaysSoWhenItCannotReachTheDatabase(t *testing.T) {
	strategyRepository := persistence.NewStrategyRepository(closedDatabase(t))

	_, saveError := strategyRepository.Save(strategyNamed("二十根均線"))
	_, findOneError := strategyRepository.FindOne(1)
	_, findAllError := strategyRepository.FindAll()
	_, updateError := strategyRepository.Update(strategyNamed("二十根均線"))
	deleteError := strategyRepository.Delete(1)

	require.Error(t, saveError)
	require.Error(t, findOneError)
	require.Error(t, findAllError)
	require.Error(t, updateError)
	require.Error(t, deleteError)
}
