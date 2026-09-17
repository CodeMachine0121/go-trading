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
	"gorm.io/gorm"
)

// rewrittenScript stands in for "the algorithm was changed" wherever a test needs to
// prove a rewrite reached the row. It is deliberately different from the script
// strategyScriptNamed carries.
const rewrittenScript = "func Calculate(candles []vo.KCandleVo) map[string]float64 { return map[string]float64{\"x\": 1} }"

// strategyScriptRowOwnerID is whoever owns every strategy script in this file unless a test says
// otherwise. A strategy script cannot exist without an owner any more, so one is planted
// before each of these runs.
const strategyScriptRowOwnerID = uint(1)

// strategyScriptNamed is a strategy script that differs from its siblings only by name, so that a
// test about names is not also a test about anything else.
func strategyScriptNamed(name string) entities.StrategyScript {
	return entities.StrategyScript{
		OwnerID:    strategyScriptRowOwnerID,
		Name:       name,
		Script:     "func Calculate(candles []vo.KCandleVo) map[string]float64 { return nil }",
		ResultType: "float",
	}
}

// newStrategyScriptTestDatabase is a cleared database with the owner these strategy scripts
// belong to already in it. Planting the person first is not scaffolding: the column
// carries a foreign key, so a strategy script owned by nobody is a row the schema refuses.
func newStrategyScriptTestDatabase(t *testing.T) *gorm.DB {
	database := newTestDatabase(t)
	require.NoError(t, database.WithContext(t.Context()).Create(&entities.User{
		ID: strategyScriptRowOwnerID, Email: "owner@example.com", PasswordProof: "a-proof",
	}).Error)

	return database
}

// aSecondOwner plants another person and answers with their identifier, for the
// cases about two people's strategy scripts not colliding.
//
// Their identifier is pinned, exactly like the first one's, and it has to be: a
// pinned row leaves the table's own counter behind it, so a row that lets the
// database choose is handed an identifier the pinned row already holds. Whether
// that clashes depends on how far the counter happens to have climbed — which is
// why it passes on a well-used database and fails on a fresh one, the one kind of
// failure that reaches a pull request instead of a laptop.
func aSecondOwner(t *testing.T, database *gorm.DB) uint {
	secondOwner := entities.User{
		ID: strategyScriptRowOwnerID + 1, Email: "other@example.com", PasswordProof: "a-proof",
	}
	require.NoError(t, database.WithContext(t.Context()).Create(&secondOwner).Error)

	return secondOwner.ID
}

func TestStrategyScriptRepositorySaveHandsBackTheStrategyScriptAsStored(t *testing.T) {
	strategyScriptRepository := persistence.NewStrategyScriptRepository(newStrategyScriptTestDatabase(t))

	savedStrategyScript, saveError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("二十根均線"))

	require.NoError(t, saveError)
	assert.Positive(t, savedStrategyScript.ID)
	assert.False(t, savedStrategyScript.CreatedAt.IsZero())
	assert.False(t, savedStrategyScript.UpdatedAt.IsZero())
	assert.Equal(t, "二十根均線", savedStrategyScript.Name)
}

func TestStrategyScriptRepositorySaveRefusesANameAlreadyHeld(t *testing.T) {
	strategyScriptRepository := persistence.NewStrategyScriptRepository(newStrategyScriptTestDatabase(t))
	_, saveError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("二十根均線"))
	require.NoError(t, saveError)

	_, conflictError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("二十根均線"))

	require.ErrorIs(t, conflictError, domains.ErrStrategyScriptNameConflict)
	assert.Contains(t, conflictError.Error(), "二十根均線")

	strategyScripts, findError := strategyScriptRepository.FindAllOwnedBy(t.Context(), strategyScriptRowOwnerID)
	require.NoError(t, findError)
	assert.Len(t, strategyScripts, 1, "拒絕的那一次不得留下任何東西，既有那一支也不得被動到")
}

func TestStrategyScriptRepositorySaveTellsNamesApartByCase(t *testing.T) {
	// A person may well use case to tell two versions apart, and deciding for them
	// which spellings count as the same name gets in the way more often than it helps.
	strategyScriptRepository := persistence.NewStrategyScriptRepository(newStrategyScriptTestDatabase(t))

	_, upperCaseError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("MA20"))
	_, lowerCaseError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("ma20"))

	require.NoError(t, upperCaseError)
	require.NoError(t, lowerCaseError)
}

func TestStrategyScriptRepositorySaveFreesANameThatWasDeleted(t *testing.T) {
	strategyScriptRepository := persistence.NewStrategyScriptRepository(newStrategyScriptTestDatabase(t))
	savedStrategyScript, saveError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("二十根均線"))
	require.NoError(t, saveError)
	require.NoError(t, strategyScriptRepository.Delete(t.Context(), savedStrategyScript.ID))

	_, reuseError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("二十根均線"))

	require.NoError(t, reuseError)
}

func TestStrategyScriptRepositorySaveDoesNotBlameTheNameForOtherClashes(t *testing.T) {
	// A restored dump can leave the identifier sequence behind the rows it restored,
	// so the next save collides on the primary key. Answering "that name is taken"
	// there would send whoever reads it hunting for a strategy script that does not exist.
	database := newStrategyScriptTestDatabase(t)
	strategyScriptRepository := persistence.NewStrategyScriptRepository(database)
	occupying := strategyScriptNamed("二十根均線")
	occupying.ID = 5000
	_, saveError := strategyScriptRepository.Save(t.Context(), occupying)
	require.NoError(t, saveError)

	colliding := strategyScriptNamed("六十根均線")
	colliding.ID = 5000

	_, clashError := strategyScriptRepository.Save(t.Context(), colliding)

	require.Error(t, clashError)
	assert.NotErrorIs(t, clashError, domains.ErrStrategyScriptNameConflict,
		"撞到的是識別碼不是名稱，不該說名稱被佔用")
	assert.NotContains(t, clashError.Error(), "六十根均線")
}

func TestStrategyScriptRepositoryUpdateHandsBackWhatThisCallStored(t *testing.T) {
	// The rewrite and the read-back share one transaction, so the values coming back
	// are this call's own rather than whatever the row happened to hold afterwards.
	strategyScriptRepository := persistence.NewStrategyScriptRepository(newStrategyScriptTestDatabase(t))
	savedStrategyScript, saveError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("二十根均線"))
	require.NoError(t, saveError)

	rewritten := strategyScriptNamed("二十根均線")
	rewritten.ID = savedStrategyScript.ID
	rewritten.Script = rewrittenScript

	updatedStrategyScript, updateError := strategyScriptRepository.Update(t.Context(), rewritten)

	require.NoError(t, updateError)
	assert.Equal(t, rewrittenScript, updatedStrategyScript.Script)

	readBack, findError := strategyScriptRepository.FindOne(t.Context(), savedStrategyScript.ID)
	require.NoError(t, findError)
	assert.Equal(t, readBack.Script, updatedStrategyScript.Script)
	assert.Equal(t, readBack.UpdatedAt.UTC(), updatedStrategyScript.UpdatedAt.UTC())
}

func TestStrategyScriptRepositoryFindOne(t *testing.T) {
	strategyScriptRepository := persistence.NewStrategyScriptRepository(newStrategyScriptTestDatabase(t))
	savedStrategyScript, saveError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("二十根均線"))
	require.NoError(t, saveError)

	t.Run("returns the strategy script carrying that identifier", func(t *testing.T) {
		foundStrategyScript, findError := strategyScriptRepository.FindOne(t.Context(), savedStrategyScript.ID)

		require.NoError(t, findError)
		assert.Equal(t, savedStrategyScript.ID, foundStrategyScript.ID)
		assert.Equal(t, "二十根均線", foundStrategyScript.Name)
	})

	t.Run("reports not found when no strategy script carries it", func(t *testing.T) {
		missingID := savedStrategyScript.ID + 999

		_, findError := strategyScriptRepository.FindOne(t.Context(), missingID)

		require.ErrorIs(t, findError, domains.ErrStrategyScriptNotFound)
		// Worded the way every other refusal is worded, and naming the identifier
		// nobody has. A reader meeting one refusal in their own language and the
		// next in the system's internal wording has to work out both came from here.
		assert.Contains(t, findError.Error(), fmt.Sprintf("找不到識別碼為 %d 的策略腳本", missingID))
	})
}

func TestStrategyScriptRepositoryFindAllOwnedByOrdersByName(t *testing.T) {
	// Named in plain letters on purpose: the point being made is that the order is
	// the collection's and not the order they went in, and letters sort the same way
	// under every collation the database might be running.
	strategyScriptRepository := persistence.NewStrategyScriptRepository(newStrategyScriptTestDatabase(t))
	// The insertion order is neither the expected order nor its reverse, so an
	// ordering taken from when a strategy script was saved cannot pass by coincidence.
	for _, name := range []string{"MA60", "RSI14", "MA20"} {
		_, saveError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed(name))
		require.NoError(t, saveError)
	}

	strategyScripts, findError := strategyScriptRepository.FindAllOwnedBy(t.Context(), strategyScriptRowOwnerID)

	require.NoError(t, findError)
	foundNames := make([]string, 0, len(strategyScripts))
	for _, strategyScript := range strategyScripts {
		foundNames = append(foundNames, strategyScript.Name)
	}
	assert.Equal(t, []string{"MA20", "MA60", "RSI14"}, foundNames)
}

func TestStrategyScriptRepositoryFindAllOwnedByOnAnEmptyCollection(t *testing.T) {
	strategyScriptRepository := persistence.NewStrategyScriptRepository(newStrategyScriptTestDatabase(t))

	strategyScripts, findError := strategyScriptRepository.FindAllOwnedBy(t.Context(), strategyScriptRowOwnerID)

	require.NoError(t, findError)
	assert.Empty(t, strategyScripts)
}

func TestStrategyScriptRepositoryUpdateRewritesTheFiveThingsAStrategyScriptRemembers(t *testing.T) {
	strategyScriptRepository := persistence.NewStrategyScriptRepository(newStrategyScriptTestDatabase(t))
	savedStrategyScript, saveError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("二十根均線"))
	require.NoError(t, saveError)

	rewritten := entities.StrategyScript{
		ID:         savedStrategyScript.ID,
		Name:       "六十根均線",
		Script:     "func Calculate(candles []vo.KCandleVo) map[string][]bool { return nil }",
		ResultType: "boolList",
	}

	updatedStrategyScript, updateError := strategyScriptRepository.Update(t.Context(), rewritten)

	require.NoError(t, updateError)
	assert.Equal(t, "六十根均線", updatedStrategyScript.Name)
	assert.Equal(t, rewritten.Script, updatedStrategyScript.Script)
	assert.Equal(t, "boolList", updatedStrategyScript.ResultType)
}

func TestStrategyScriptRepositoryUpdateLeavesTheIdentifierAndTheFirstSavedTimeAlone(t *testing.T) {
	strategyScriptRepository := persistence.NewStrategyScriptRepository(newStrategyScriptTestDatabase(t))
	savedStrategyScript, saveError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("二十根均線"))
	require.NoError(t, saveError)

	// A rewrite that tries to move both of them, to prove they are out of reach
	// rather than merely left unset by a well-behaved caller.
	rewritten := strategyScriptNamed("六十根均線")
	rewritten.ID = savedStrategyScript.ID
	rewritten.CreatedAt = savedStrategyScript.CreatedAt.Add(-48 * time.Hour)

	updatedStrategyScript, updateError := strategyScriptRepository.Update(t.Context(), rewritten)

	require.NoError(t, updateError)
	assert.Equal(t, savedStrategyScript.ID, updatedStrategyScript.ID)
	assert.WithinDuration(t, savedStrategyScript.CreatedAt, updatedStrategyScript.CreatedAt, time.Millisecond)
	assert.True(t, updatedStrategyScript.UpdatedAt.After(savedStrategyScript.UpdatedAt),
		"最後修改時間必須往前走，否則看不出這一支被動過")
}

func TestStrategyScriptRepositoryUpdateToItsOwnNameIsNotAConflict(t *testing.T) {
	strategyScriptRepository := persistence.NewStrategyScriptRepository(newStrategyScriptTestDatabase(t))
	savedStrategyScript, saveError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("二十根均線"))
	require.NoError(t, saveError)

	rewritten := strategyScriptNamed("二十根均線")
	rewritten.ID = savedStrategyScript.ID
	rewritten.Script = rewrittenScript

	updatedStrategyScript, updateError := strategyScriptRepository.Update(t.Context(), rewritten)

	require.NoError(t, updateError)
	assert.Equal(t, rewrittenScript, updatedStrategyScript.Script)
}

func TestStrategyScriptRepositoryUpdateRefusesAnotherStrategyScriptsName(t *testing.T) {
	strategyScriptRepository := persistence.NewStrategyScriptRepository(newStrategyScriptTestDatabase(t))
	_, firstError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("二十根均線"))
	require.NoError(t, firstError)
	secondStrategyScript, secondError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("六十根均線"))
	require.NoError(t, secondError)

	rewritten := strategyScriptNamed("二十根均線")
	rewritten.ID = secondStrategyScript.ID

	_, conflictError := strategyScriptRepository.Update(t.Context(), rewritten)

	require.ErrorIs(t, conflictError, domains.ErrStrategyScriptNameConflict)

	strategyScripts, findError := strategyScriptRepository.FindAllOwnedBy(t.Context(), strategyScriptRowOwnerID)
	require.NoError(t, findError)
	foundNames := make([]string, 0, len(strategyScripts))
	for _, strategyScript := range strategyScripts {
		foundNames = append(foundNames, strategyScript.Name)
	}
	assert.ElementsMatch(t, []string{"二十根均線", "六十根均線"}, foundNames,
		"拒絕的改名不得動到任何一支")
}

func TestStrategyScriptRepositoryUpdateReportsNotFound(t *testing.T) {
	strategyScriptRepository := persistence.NewStrategyScriptRepository(newStrategyScriptTestDatabase(t))

	rewritten := strategyScriptNamed("二十根均線")
	rewritten.ID = 999999

	_, updateError := strategyScriptRepository.Update(t.Context(), rewritten)

	require.ErrorIs(t, updateError, domains.ErrStrategyScriptNotFound)
	assert.Contains(t, updateError.Error(), "找不到識別碼為 999999 的策略腳本")

	strategyScripts, findError := strategyScriptRepository.FindAllOwnedBy(t.Context(), strategyScriptRowOwnerID)
	require.NoError(t, findError)
	assert.Empty(t, strategyScripts, "改一支不存在的策略腳本不得因此建出一支新的")
}

func TestStrategyScriptRepositoryDelete(t *testing.T) {
	strategyScriptRepository := persistence.NewStrategyScriptRepository(newStrategyScriptTestDatabase(t))
	deletedStrategyScript, firstError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("二十根均線"))
	require.NoError(t, firstError)
	keptStrategyScript, secondError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("六十根均線"))
	require.NoError(t, secondError)

	require.NoError(t, strategyScriptRepository.Delete(t.Context(), deletedStrategyScript.ID))

	t.Run("the deleted strategy script is gone for good", func(t *testing.T) {
		_, findError := strategyScriptRepository.FindOne(t.Context(), deletedStrategyScript.ID)

		require.ErrorIs(t, findError, domains.ErrStrategyScriptNotFound)
	})

	t.Run("it no longer appears in the collection", func(t *testing.T) {
		strategyScripts, findError := strategyScriptRepository.FindAllOwnedBy(t.Context(), strategyScriptRowOwnerID)

		require.NoError(t, findError)
		require.Len(t, strategyScripts, 1)
		assert.Equal(t, keptStrategyScript.ID, strategyScripts[0].ID)
	})

	t.Run("deleting it again reports not found", func(t *testing.T) {
		deleteError := strategyScriptRepository.Delete(t.Context(), deletedStrategyScript.ID)

		require.ErrorIs(t, deleteError, domains.ErrStrategyScriptNotFound)
	})
}

func TestStrategyScriptRepositoryDeleteReportsNotFound(t *testing.T) {
	strategyScriptRepository := persistence.NewStrategyScriptRepository(newStrategyScriptTestDatabase(t))

	deleteError := strategyScriptRepository.Delete(t.Context(), 999999)

	require.ErrorIs(t, deleteError, domains.ErrStrategyScriptNotFound)
	assert.Contains(t, deleteError.Error(), "找不到識別碼為 999999 的策略腳本")
}

func TestStrategyScriptRepositorySaysSoWhenItCannotReachTheDatabase(t *testing.T) {
	strategyScriptRepository := persistence.NewStrategyScriptRepository(closedDatabase(t))

	_, saveError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("二十根均線"))
	_, findOneError := strategyScriptRepository.FindOne(t.Context(), 1)
	_, findAllError := strategyScriptRepository.FindAllOwnedBy(t.Context(), strategyScriptRowOwnerID)
	_, updateError := strategyScriptRepository.Update(t.Context(), strategyScriptNamed("二十根均線"))
	deleteError := strategyScriptRepository.Delete(t.Context(), 1)

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
	nameField, found := reflect.TypeFor[entities.StrategyScript]().FieldByName("Name")
	require.True(t, found, "the entity has no Name field to carry the index")

	assert.Contains(t, nameField.Tag.Get("gorm"), "uniqueIndex:"+persistence.StrategyScriptNameIndex)
}

// withParameters is a strategy script carrying knobs. A knob has no identity anybody names
// — it is its name inside its strategy script — so these tests read them back by name.
func withParameters(
	strategyScript entities.StrategyScript, parameters ...entities.StrategyScriptParameter,
) entities.StrategyScript {
	strategyScript.Parameters = parameters

	return strategyScript
}

func lookbackCountKnob(name string, defaultValue float64) entities.StrategyScriptParameter {
	return entities.StrategyScriptParameter{Name: name, Kind: "lookbackCount", DefaultValue: defaultValue}
}

func numberKnob(name string, defaultValue float64) entities.StrategyScriptParameter {
	return entities.StrategyScriptParameter{Name: name, Kind: "number", DefaultValue: defaultValue}
}

func knobsByName(strategyScript entities.StrategyScript) map[string]entities.StrategyScriptParameter {
	byName := make(map[string]entities.StrategyScriptParameter, len(strategyScript.Parameters))
	for _, parameter := range strategyScript.Parameters {
		byName[parameter.Name] = parameter
	}

	return byName
}

// A strategy script read back without its knobs looks like a strategy script that has none, and
// every knob it declared would silently stop existing.
func TestStrategyScriptRepositoryKeepsTheKnobsAStrategyScriptCarries(t *testing.T) {
	strategyScriptRepository := persistence.NewStrategyScriptRepository(newStrategyScriptTestDatabase(t))

	savedStrategyScript, saveError := strategyScriptRepository.Save(t.Context(), withParameters(
		strategyScriptNamed("布林通道"),
		lookbackCountKnob("期數", 20),
		numberKnob("倍數", 2.5)))
	require.NoError(t, saveError)

	readBack, findError := strategyScriptRepository.FindOne(t.Context(), savedStrategyScript.ID)

	require.NoError(t, findError)
	knobs := knobsByName(readBack)
	require.Len(t, knobs, 2)
	assert.Equal(t, "lookbackCount", knobs["期數"].Kind)
	assert.InDelta(t, 20.0, knobs["期數"].DefaultValue, 0)
	assert.Equal(t, "number", knobs["倍數"].Kind)
	assert.InDelta(t, 2.5, knobs["倍數"].DefaultValue, 0)
}

func TestStrategyScriptRepositoryListsEveryStrategyScriptWithItsKnobs(t *testing.T) {
	strategyScriptRepository := persistence.NewStrategyScriptRepository(newStrategyScriptTestDatabase(t))
	_, saveError := strategyScriptRepository.Save(t.Context(), withParameters(
		strategyScriptNamed("布林通道"), lookbackCountKnob("期數", 20)))
	require.NoError(t, saveError)

	strategyScripts, findError := strategyScriptRepository.FindAllOwnedBy(t.Context(), strategyScriptRowOwnerID)

	require.NoError(t, findError)
	require.Len(t, strategyScripts, 1)
	require.Len(t, strategyScripts[0].Parameters, 1)
	assert.Equal(t, "期數", strategyScripts[0].Parameters[0].Name)
}

// Rewriting replaces the whole set. Leaving the old rows behind would give a
// strategy script knobs it no longer declares, and the largest look-back — which decides
// how many candles get read — would be computed from a knob nobody can see.
func TestStrategyScriptRepositoryReplacesTheWholeSetOfKnobsOnRewrite(t *testing.T) {
	strategyScriptRepository := persistence.NewStrategyScriptRepository(newStrategyScriptTestDatabase(t))
	savedStrategyScript, saveError := strategyScriptRepository.Save(t.Context(), withParameters(
		strategyScriptNamed("布林通道"), lookbackCountKnob("期數", 20), numberKnob("倍數", 2)))
	require.NoError(t, saveError)

	rewritten := withParameters(
		strategyScriptNamed("布林通道"), lookbackCountKnob("週期", 50))
	rewritten.ID = savedStrategyScript.ID

	updatedStrategyScript, updateError := strategyScriptRepository.Update(t.Context(), rewritten)

	require.NoError(t, updateError)
	knobs := knobsByName(updatedStrategyScript)
	require.Len(t, knobs, 1, "舊的兩個必須整份消失，不是被留下來")
	assert.InDelta(t, 50.0, knobs["週期"].DefaultValue, 0)
}

func TestStrategyScriptRepositoryLetsAStrategyScriptDropEveryKnobItHad(t *testing.T) {
	strategyScriptRepository := persistence.NewStrategyScriptRepository(newStrategyScriptTestDatabase(t))
	savedStrategyScript, saveError := strategyScriptRepository.Save(t.Context(), withParameters(
		strategyScriptNamed("布林通道"), lookbackCountKnob("期數", 20)))
	require.NoError(t, saveError)

	rewritten := strategyScriptNamed("布林通道")
	rewritten.ID = savedStrategyScript.ID

	updatedStrategyScript, updateError := strategyScriptRepository.Update(t.Context(), rewritten)

	require.NoError(t, updateError)
	assert.Empty(t, updatedStrategyScript.Parameters)
}
