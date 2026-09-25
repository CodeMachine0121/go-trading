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

// rewrittenScript differs from strategyScriptNamed's script so a test can prove a rewrite reached the row.
const rewrittenScript = "func Calculate(candles []vo.KCandleVo) map[string]float64 { return map[string]float64{\"x\": 1} }"

const strategyScriptRowOwnerID = uint(1)

func strategyScriptNamed(name string) entities.StrategyScript {
	return entities.StrategyScript{
		OwnerID:    strategyScriptRowOwnerID,
		Name:       name,
		Script:     "func Calculate(candles []vo.KCandleVo) map[string]float64 { return nil }",
		ResultType: "float",
	}
}

// newStrategyScriptTestDatabase seeds the owner that the strategy script foreign key requires.
func newStrategyScriptTestDatabase(t *testing.T) *gorm.DB {
	database := newTestDatabase(t)
	require.NoError(t, database.WithContext(t.Context()).Create(&entities.User{
		ID: strategyScriptRowOwnerID, Email: "owner@example.com", PasswordProof: "a-proof",
	}).Error)

	return database
}

// aSecondOwner pins the identifier like the first owner, since a database-assigned one could collide with the pinned row depending on the sequence.
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
	// Names are case-sensitive.
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
	// A primary-key clash (e.g. a stale sequence after a restore) must not be reported as a name conflict.
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
		// The refusal uses the same user-facing wording as others and names the missing identifier.
		assert.Contains(t, findError.Error(), fmt.Sprintf("找不到識別碼為 %d 的策略腳本", missingID))
	})
}

func TestStrategyScriptRepositoryFindAllOwnedByOrdersByName(t *testing.T) {
	// ASCII names sort the same under every collation.
	strategyScriptRepository := persistence.NewStrategyScriptRepository(newStrategyScriptTestDatabase(t))
	// Insertion order is neither expected nor reversed, so ordering by save time cannot pass.
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

	// The rewrite tries to change both, proving they are out of reach.
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

// The index name is repeated because struct tags cannot hold constants; this test needs no database, so it never skips.
func TestTheNameIndexTheRepositoryBlamesIsTheOneTheEntityDeclares(t *testing.T) {
	nameField, found := reflect.TypeFor[entities.StrategyScript]().FieldByName("Name")
	require.True(t, found, "the entity has no Name field to carry the index")

	assert.Contains(t, nameField.Tag.Get("gorm"), "uniqueIndex:"+persistence.StrategyScriptNameIndex)
}

// withParameters adds parameters, which are identified only by name within their script.
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

// A rewrite replaces all parameters; leftovers would skew the largest look-back and the candle count read.
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

func TestStrategyScriptRepositoryReadsAScriptWrittenWithoutAKindOfMarketAsASpotOne(t *testing.T) {
	// A row without a kind reads back as the column default, as scripts saved before the kind existed.
	strategyScriptRepository := persistence.NewStrategyScriptRepository(newStrategyScriptTestDatabase(t))
	withoutKind := strategyScriptNamed("舊的均線")
	withoutKind.MarketDataKind = ""

	saved, saveError := strategyScriptRepository.Save(t.Context(), withoutKind)
	require.NoError(t, saveError)

	stored, findError := strategyScriptRepository.FindOne(t.Context(), saved.ID)
	require.NoError(t, findError)
	assert.Equal(t, "kCandle", stored.MarketDataKind)
}

func TestStrategyScriptRepositoryNeverChangesTheKindOfMarketOnARewrite(t *testing.T) {
	strategyScriptRepository := persistence.NewStrategyScriptRepository(newStrategyScriptTestDatabase(t))
	contractScript := strategyScriptNamed("費率反轉")
	contractScript.MarketDataKind = "contractKCandle"
	saved, saveError := strategyScriptRepository.Save(t.Context(), contractScript)
	require.NoError(t, saveError)

	rewrite := saved
	rewrite.MarketDataKind = "kCandle"
	_, updateError := strategyScriptRepository.Update(t.Context(), rewrite)
	require.NoError(t, updateError)

	stored, findError := strategyScriptRepository.FindOne(t.Context(), saved.ID)
	require.NoError(t, findError)
	assert.Equal(t, "contractKCandle", stored.MarketDataKind)
}
