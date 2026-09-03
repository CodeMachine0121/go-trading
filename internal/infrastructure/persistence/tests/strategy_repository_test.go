package persistence_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rewrittenScript stands in for "the algorithm was changed" wherever a test needs to
// prove a rewrite reached the row. It is deliberately different from the script
// strategyNamed carries.
const rewrittenScript = "func Calculate(candles []vo.KCandleVo) map[string]float64 { return map[string]float64{\"x\": 1} }"

// strategyNamed is a strategy that differs from its siblings only by name, so that a
// test about names is not also a test about anything else.
func strategyNamed(name string) entities.Strategy {
	return entities.Strategy{
		Name:       name,
		Script:     "func Calculate(candles []vo.KCandleVo) map[string]float64 { return nil }",
		ResultType: "float",
	}
}

func TestStrategyRepositorySaveHandsBackTheStrategyAsStored(t *testing.T) {
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))

	savedStrategy, saveError := strategyRepository.Save(t.Context(), strategyNamed("二十根均線"))

	require.NoError(t, saveError)
	assert.Positive(t, savedStrategy.ID)
	assert.False(t, savedStrategy.CreatedAt.IsZero())
	assert.False(t, savedStrategy.UpdatedAt.IsZero())
	assert.Equal(t, "二十根均線", savedStrategy.Name)
}

func TestStrategyRepositorySaveRefusesANameAlreadyHeld(t *testing.T) {
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))
	_, saveError := strategyRepository.Save(t.Context(), strategyNamed("二十根均線"))
	require.NoError(t, saveError)

	_, conflictError := strategyRepository.Save(t.Context(), strategyNamed("二十根均線"))

	require.ErrorIs(t, conflictError, domains.ErrStrategyNameConflict)
	assert.Contains(t, conflictError.Error(), "二十根均線")

	strategies, findError := strategyRepository.FindAll(t.Context())
	require.NoError(t, findError)
	assert.Len(t, strategies, 1, "拒絕的那一次不得留下任何東西，既有那一支也不得被動到")
}

func TestStrategyRepositorySaveTellsNamesApartByCase(t *testing.T) {
	// A person may well use case to tell two versions apart, and deciding for them
	// which spellings count as the same name gets in the way more often than it helps.
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))

	_, upperCaseError := strategyRepository.Save(t.Context(), strategyNamed("MA20"))
	_, lowerCaseError := strategyRepository.Save(t.Context(), strategyNamed("ma20"))

	require.NoError(t, upperCaseError)
	require.NoError(t, lowerCaseError)
}

func TestStrategyRepositorySaveFreesANameThatWasDeleted(t *testing.T) {
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))
	savedStrategy, saveError := strategyRepository.Save(t.Context(), strategyNamed("二十根均線"))
	require.NoError(t, saveError)
	require.NoError(t, strategyRepository.Delete(t.Context(), savedStrategy.ID))

	_, reuseError := strategyRepository.Save(t.Context(), strategyNamed("二十根均線"))

	require.NoError(t, reuseError)
}

func TestStrategyRepositorySaveDoesNotBlameTheNameForOtherClashes(t *testing.T) {
	// A restored dump can leave the identifier sequence behind the rows it restored,
	// so the next save collides on the primary key. Answering "that name is taken"
	// there would send whoever reads it hunting for a strategy that does not exist.
	database := newTestDatabase(t)
	strategyRepository := persistence.NewStrategyRepository(database)
	occupying := strategyNamed("二十根均線")
	occupying.ID = 5000
	_, saveError := strategyRepository.Save(t.Context(), occupying)
	require.NoError(t, saveError)

	colliding := strategyNamed("六十根均線")
	colliding.ID = 5000

	_, clashError := strategyRepository.Save(t.Context(), colliding)

	require.Error(t, clashError)
	assert.NotErrorIs(t, clashError, domains.ErrStrategyNameConflict,
		"撞到的是識別碼不是名稱，不該說名稱被佔用")
	assert.NotContains(t, clashError.Error(), "六十根均線")
}

func TestStrategyRepositoryUpdateHandsBackWhatThisCallStored(t *testing.T) {
	// The rewrite and the read-back share one transaction, so the values coming back
	// are this call's own rather than whatever the row happened to hold afterwards.
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))
	savedStrategy, saveError := strategyRepository.Save(t.Context(), strategyNamed("二十根均線"))
	require.NoError(t, saveError)

	rewritten := strategyNamed("二十根均線")
	rewritten.ID = savedStrategy.ID
	rewritten.Script = rewrittenScript

	updatedStrategy, updateError := strategyRepository.Update(t.Context(), rewritten)

	require.NoError(t, updateError)
	assert.Equal(t, rewrittenScript, updatedStrategy.Script)

	readBack, findError := strategyRepository.FindOne(t.Context(), savedStrategy.ID)
	require.NoError(t, findError)
	assert.Equal(t, readBack.Script, updatedStrategy.Script)
	assert.Equal(t, readBack.UpdatedAt.UTC(), updatedStrategy.UpdatedAt.UTC())
}

func TestStrategyRepositoryFindOne(t *testing.T) {
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))
	savedStrategy, saveError := strategyRepository.Save(t.Context(), strategyNamed("二十根均線"))
	require.NoError(t, saveError)

	t.Run("returns the strategy carrying that identifier", func(t *testing.T) {
		foundStrategy, findError := strategyRepository.FindOne(t.Context(), savedStrategy.ID)

		require.NoError(t, findError)
		assert.Equal(t, savedStrategy.ID, foundStrategy.ID)
		assert.Equal(t, "二十根均線", foundStrategy.Name)
	})

	t.Run("reports not found when no strategy carries it", func(t *testing.T) {
		missingID := savedStrategy.ID + 999

		_, findError := strategyRepository.FindOne(t.Context(), missingID)

		require.ErrorIs(t, findError, domains.ErrStrategyNotFound)
		// Worded the way every other refusal is worded, and naming the identifier
		// nobody has. A reader meeting one refusal in their own language and the
		// next in the system's internal wording has to work out both came from here.
		assert.Contains(t, findError.Error(), fmt.Sprintf("找不到識別碼為 %d 的策略", missingID))
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
		_, saveError := strategyRepository.Save(t.Context(), strategyNamed(name))
		require.NoError(t, saveError)
	}

	strategies, findError := strategyRepository.FindAll(t.Context())

	require.NoError(t, findError)
	foundNames := make([]string, 0, len(strategies))
	for _, strategy := range strategies {
		foundNames = append(foundNames, strategy.Name)
	}
	assert.Equal(t, []string{"MA20", "MA60", "RSI14"}, foundNames)
}

func TestStrategyRepositoryFindAllOnAnEmptyCollection(t *testing.T) {
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))

	strategies, findError := strategyRepository.FindAll(t.Context())

	require.NoError(t, findError)
	assert.Empty(t, strategies)
}

func TestStrategyRepositoryUpdateRewritesTheFiveThingsAStrategyRemembers(t *testing.T) {
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))
	savedStrategy, saveError := strategyRepository.Save(t.Context(), strategyNamed("二十根均線"))
	require.NoError(t, saveError)

	rewritten := entities.Strategy{
		ID:         savedStrategy.ID,
		Name:       "六十根均線",
		Script:     "func Calculate(candles []vo.KCandleVo) map[string][]bool { return nil }",
		ResultType: "boolList",
	}

	updatedStrategy, updateError := strategyRepository.Update(t.Context(), rewritten)

	require.NoError(t, updateError)
	assert.Equal(t, "六十根均線", updatedStrategy.Name)
	assert.Equal(t, rewritten.Script, updatedStrategy.Script)
	assert.Equal(t, "boolList", updatedStrategy.ResultType)
}

func TestStrategyRepositoryUpdateLeavesTheIdentifierAndTheFirstSavedTimeAlone(t *testing.T) {
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))
	savedStrategy, saveError := strategyRepository.Save(t.Context(), strategyNamed("二十根均線"))
	require.NoError(t, saveError)

	// A rewrite that tries to move both of them, to prove they are out of reach
	// rather than merely left unset by a well-behaved caller.
	rewritten := strategyNamed("六十根均線")
	rewritten.ID = savedStrategy.ID
	rewritten.CreatedAt = savedStrategy.CreatedAt.Add(-48 * time.Hour)

	updatedStrategy, updateError := strategyRepository.Update(t.Context(), rewritten)

	require.NoError(t, updateError)
	assert.Equal(t, savedStrategy.ID, updatedStrategy.ID)
	assert.WithinDuration(t, savedStrategy.CreatedAt, updatedStrategy.CreatedAt, time.Millisecond)
	assert.True(t, updatedStrategy.UpdatedAt.After(savedStrategy.UpdatedAt),
		"最後修改時間必須往前走，否則看不出這一支被動過")
}

func TestStrategyRepositoryUpdateToItsOwnNameIsNotAConflict(t *testing.T) {
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))
	savedStrategy, saveError := strategyRepository.Save(t.Context(), strategyNamed("二十根均線"))
	require.NoError(t, saveError)

	rewritten := strategyNamed("二十根均線")
	rewritten.ID = savedStrategy.ID
	rewritten.Script = rewrittenScript

	updatedStrategy, updateError := strategyRepository.Update(t.Context(), rewritten)

	require.NoError(t, updateError)
	assert.Equal(t, rewrittenScript, updatedStrategy.Script)
}

func TestStrategyRepositoryUpdateRefusesAnotherStrategysName(t *testing.T) {
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))
	_, firstError := strategyRepository.Save(t.Context(), strategyNamed("二十根均線"))
	require.NoError(t, firstError)
	secondStrategy, secondError := strategyRepository.Save(t.Context(), strategyNamed("六十根均線"))
	require.NoError(t, secondError)

	rewritten := strategyNamed("二十根均線")
	rewritten.ID = secondStrategy.ID

	_, conflictError := strategyRepository.Update(t.Context(), rewritten)

	require.ErrorIs(t, conflictError, domains.ErrStrategyNameConflict)

	strategies, findError := strategyRepository.FindAll(t.Context())
	require.NoError(t, findError)
	foundNames := make([]string, 0, len(strategies))
	for _, strategy := range strategies {
		foundNames = append(foundNames, strategy.Name)
	}
	assert.ElementsMatch(t, []string{"二十根均線", "六十根均線"}, foundNames,
		"拒絕的改名不得動到任何一支")
}

func TestStrategyRepositoryUpdateReportsNotFound(t *testing.T) {
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))

	rewritten := strategyNamed("二十根均線")
	rewritten.ID = 999999

	_, updateError := strategyRepository.Update(t.Context(), rewritten)

	require.ErrorIs(t, updateError, domains.ErrStrategyNotFound)
	assert.Contains(t, updateError.Error(), "找不到識別碼為 999999 的策略")

	strategies, findError := strategyRepository.FindAll(t.Context())
	require.NoError(t, findError)
	assert.Empty(t, strategies, "改一支不存在的策略不得因此建出一支新的")
}

func TestStrategyRepositoryDelete(t *testing.T) {
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))
	deletedStrategy, firstError := strategyRepository.Save(t.Context(), strategyNamed("二十根均線"))
	require.NoError(t, firstError)
	keptStrategy, secondError := strategyRepository.Save(t.Context(), strategyNamed("六十根均線"))
	require.NoError(t, secondError)

	require.NoError(t, strategyRepository.Delete(t.Context(), deletedStrategy.ID))

	t.Run("the deleted strategy is gone for good", func(t *testing.T) {
		_, findError := strategyRepository.FindOne(t.Context(), deletedStrategy.ID)

		require.ErrorIs(t, findError, domains.ErrStrategyNotFound)
	})

	t.Run("it no longer appears in the collection", func(t *testing.T) {
		strategies, findError := strategyRepository.FindAll(t.Context())

		require.NoError(t, findError)
		require.Len(t, strategies, 1)
		assert.Equal(t, keptStrategy.ID, strategies[0].ID)
	})

	t.Run("deleting it again reports not found", func(t *testing.T) {
		deleteError := strategyRepository.Delete(t.Context(), deletedStrategy.ID)

		require.ErrorIs(t, deleteError, domains.ErrStrategyNotFound)
	})
}

func TestStrategyRepositoryDeleteReportsNotFound(t *testing.T) {
	strategyRepository := persistence.NewStrategyRepository(newTestDatabase(t))

	deleteError := strategyRepository.Delete(t.Context(), 999999)

	require.ErrorIs(t, deleteError, domains.ErrStrategyNotFound)
	assert.Contains(t, deleteError.Error(), "找不到識別碼為 999999 的策略")
}

func TestStrategyRepositorySaysSoWhenItCannotReachTheDatabase(t *testing.T) {
	strategyRepository := persistence.NewStrategyRepository(closedDatabase(t))

	_, saveError := strategyRepository.Save(t.Context(), strategyNamed("二十根均線"))
	_, findOneError := strategyRepository.FindOne(t.Context(), 1)
	_, findAllError := strategyRepository.FindAll(t.Context())
	_, updateError := strategyRepository.Update(t.Context(), strategyNamed("二十根均線"))
	deleteError := strategyRepository.Delete(t.Context(), 1)

	require.Error(t, saveError)
	require.Error(t, findOneError)
	require.Error(t, findAllError)
	require.Error(t, updateError)
	require.Error(t, deleteError)
}

// The repository names the index it blames in Go; the entity spells it in a struct
// tag, which cannot hold a constant. Nothing but this stops the two drifting, and if
// they drift a duplicate name stops being answered as a conflict and starts being
// answered as a storage failure. This test needs no database, so unlike the conflict
// tests above it cannot skip.
func TestTheNameIndexTheRepositoryBlamesIsTheOneTheEntityDeclares(t *testing.T) {
	nameField, found := reflect.TypeFor[entities.Strategy]().FieldByName("Name")
	require.True(t, found, "the entity has no Name field to carry the index")

	assert.Contains(t, nameField.Tag.Get("gorm"), "uniqueIndex:"+persistence.StrategyNameIndex)
}
