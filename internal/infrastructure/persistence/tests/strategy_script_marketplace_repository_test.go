package persistence_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var (
	publishedAtNoon = time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	publishedAtDusk = time.Date(2026, 9, 10, 18, 0, 0, 0, time.UTC)
)

func TestStrategyScriptsOfTwoPeopleMayShareAName(t *testing.T) {
	// Script names are unique per owner, not globally.
	database := newStrategyScriptTestDatabase(t)
	strategyScriptRepository := persistence.NewStrategyScriptRepository(database)
	secondOwnerID := aSecondOwner(t, database)

	_, firstError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("二十根均線"))
	secondPersons := strategyScriptNamed("二十根均線")
	secondPersons.OwnerID = secondOwnerID
	_, secondError := strategyScriptRepository.Save(t.Context(), secondPersons)

	require.NoError(t, firstError)
	require.NoError(t, secondError)
}

func TestFindAllOwnedByAnswersOnlyThatPersonsStrategyScripts(t *testing.T) {
	database := newStrategyScriptTestDatabase(t)
	strategyScriptRepository := persistence.NewStrategyScriptRepository(database)
	secondOwnerID := aSecondOwner(t, database)
	_, firstError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("我的"))
	require.NoError(t, firstError)
	theirs := strategyScriptNamed("他的")
	theirs.OwnerID = secondOwnerID
	_, secondError := strategyScriptRepository.Save(t.Context(), theirs)
	require.NoError(t, secondError)

	strategyScripts, findError := strategyScriptRepository.FindAllOwnedBy(t.Context(), strategyScriptRowOwnerID)

	require.NoError(t, findError)
	require.Len(t, strategyScripts, 1)
	assert.Equal(t, "我的", strategyScripts[0].Name)
}

func TestPublishStrategyScriptTwiceKeepsOneRowAndTheFirstMoment(t *testing.T) {
	// Publishing twice is idempotent and keeps the original publication time.
	database := newStrategyScriptTestDatabase(t)
	strategyScriptRepository := persistence.NewStrategyScriptRepository(database)
	publishedStrategyScriptRepository := persistence.NewPublishedStrategyScriptRepository(database)
	savedStrategyScript, saveError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("二十根均線"))
	require.NoError(t, saveError)

	require.NoError(t, publishedStrategyScriptRepository.Publish(t.Context(), savedStrategyScript.ID, publishedAtNoon))
	require.NoError(t, publishedStrategyScriptRepository.Publish(t.Context(), savedStrategyScript.ID, publishedAtDusk))

	publications, findError := strategyScriptRepository.FindAllPublished(t.Context())
	require.NoError(t, findError)
	require.Len(t, publications, 1)
	assert.Equal(t, publishedAtNoon, publications[0].PublishedAt.UTC())
}

func TestWithdrawStrategyScriptThatWasNeverPublishedIsNotAFailure(t *testing.T) {
	database := newStrategyScriptTestDatabase(t)
	strategyScriptRepository := persistence.NewStrategyScriptRepository(database)
	savedStrategyScript, saveError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("二十根均線"))
	require.NoError(t, saveError)

	withdrawError := persistence.NewPublishedStrategyScriptRepository(database).
		Withdraw(t.Context(), savedStrategyScript.ID)

	require.NoError(t, withdrawError, "what was asked for already holds")
}

func TestFindOnePublicationAnswersWhenItIsOnTheMarketplace(t *testing.T) {
	database := newStrategyScriptTestDatabase(t)
	strategyScriptID := aPublishedStrategyScript(t, database)

	publication, findError := persistence.NewPublishedStrategyScriptRepository(database).
		FindOne(t.Context(), strategyScriptID)

	require.NoError(t, findError)
	assert.Equal(t, strategyScriptID, publication.StrategyScriptID)
	assert.Equal(t, publishedAtNoon, publication.PublishedAt.UTC())
}

func TestFindOnePublicationSaysWhenThereIsNone(t *testing.T) {
	database := newStrategyScriptTestDatabase(t)
	strategyScriptRepository := persistence.NewStrategyScriptRepository(database)
	savedStrategyScript, saveError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("二十根均線"))
	require.NoError(t, saveError)

	_, findError := persistence.NewPublishedStrategyScriptRepository(database).
		FindOne(t.Context(), savedStrategyScript.ID)

	require.ErrorIs(t, findError, domains.ErrStrategyScriptNotPublished)
}

func TestFindAllPublishedOrdersByTheNewestPublicationFirst(t *testing.T) {
	database := newStrategyScriptTestDatabase(t)
	strategyScriptRepository := persistence.NewStrategyScriptRepository(database)
	publishedStrategyScriptRepository := persistence.NewPublishedStrategyScriptRepository(database)
	earlier, earlierError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("先發佈的"))
	require.NoError(t, earlierError)
	later, laterError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("後發佈的"))
	require.NoError(t, laterError)
	require.NoError(t, publishedStrategyScriptRepository.Publish(t.Context(), earlier.ID, publishedAtNoon))
	require.NoError(t, publishedStrategyScriptRepository.Publish(t.Context(), later.ID, publishedAtDusk))

	publications, findError := strategyScriptRepository.FindAllPublished(t.Context())

	require.NoError(t, findError)
	require.Len(t, publications, 2)
	assert.Equal(t, "後發佈的", publications[0].StrategyScript.Name)
	assert.Equal(t, "先發佈的", publications[1].StrategyScript.Name)
}

func TestFindAllPublishedCarriesTheStrategyScriptItsKnobsAndItsOwner(t *testing.T) {
	database := newStrategyScriptTestDatabase(t)
	strategyScriptRepository := persistence.NewStrategyScriptRepository(database)
	withKnob := strategyScriptNamed("布林通道")
	withKnob.Parameters = []entities.StrategyScriptParameter{
		{Name: "期數", Kind: "lookbackCount", DefaultValue: 20},
	}
	savedStrategyScript, saveError := strategyScriptRepository.Save(t.Context(), withKnob)
	require.NoError(t, saveError)
	require.NoError(t, persistence.NewPublishedStrategyScriptRepository(database).
		Publish(t.Context(), savedStrategyScript.ID, publishedAtNoon))

	publications, findError := strategyScriptRepository.FindAllPublished(t.Context())

	require.NoError(t, findError)
	require.Len(t, publications, 1)
	assert.Equal(t, "布林通道", publications[0].StrategyScript.Name)
	require.Len(t, publications[0].StrategyScript.Parameters, 1)
	assert.Equal(t, "期數", publications[0].StrategyScript.Parameters[0].Name)
	assert.Equal(t, "owner@example.com", publications[0].StrategyScript.Owner.Email)
}

func TestAdoptStrategyScriptTwiceKeepsOneRow(t *testing.T) {
	database := newStrategyScriptTestDatabase(t)
	adopterID := aSecondOwner(t, database)
	strategyScriptID := aPublishedStrategyScript(t, database)
	strategyScriptAdoptionRepository := persistence.NewStrategyScriptAdoptionRepository(database)

	require.NoError(t, strategyScriptAdoptionRepository.Adopt(t.Context(), adopterID, strategyScriptID, publishedAtNoon))
	require.NoError(t, strategyScriptAdoptionRepository.Adopt(t.Context(), adopterID, strategyScriptID, publishedAtDusk))

	adopted, findError := persistence.NewStrategyScriptRepository(database).
		FindAllAdoptedBy(t.Context(), adopterID)
	require.NoError(t, findError)
	assert.Len(t, adopted, 1)
}

func TestAdoptSomethingNotOnTheMarketplaceIsRefusedAsNotFound(t *testing.T) {
	// The foreign key, not a prior read, rejects adopting an unpublished script, avoiding a race with withdrawal.
	database := newStrategyScriptTestDatabase(t)
	strategyScriptRepository := persistence.NewStrategyScriptRepository(database)
	adopterID := aSecondOwner(t, database)
	unpublished, saveError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("沒發佈的"))
	require.NoError(t, saveError)

	adoptError := persistence.NewStrategyScriptAdoptionRepository(database).
		Adopt(t.Context(), adopterID, unpublished.ID, publishedAtNoon)

	require.ErrorIs(t, adoptError, domains.ErrStrategyScriptNotFound)
}

func TestAbandonSomethingNeverAdoptedIsNotAFailure(t *testing.T) {
	database := newStrategyScriptTestDatabase(t)
	adopterID := aSecondOwner(t, database)
	strategyScriptID := aPublishedStrategyScript(t, database)

	abandonError := persistence.NewStrategyScriptAdoptionRepository(database).
		Abandon(t.Context(), adopterID, strategyScriptID)

	require.NoError(t, abandonError)
}

func TestWithdrawingAStrategyScriptClearsEverybodysAdoptionOfIt(t *testing.T) {
	// Withdrawal removes adoptions through a schema cascade, not Go code.
	database := newStrategyScriptTestDatabase(t)
	adopterID := aSecondOwner(t, database)
	strategyScriptID := aPublishedStrategyScript(t, database)
	require.NoError(t, persistence.NewStrategyScriptAdoptionRepository(database).
		Adopt(t.Context(), adopterID, strategyScriptID, publishedAtNoon))

	require.NoError(t, persistence.NewPublishedStrategyScriptRepository(database).
		Withdraw(t.Context(), strategyScriptID))

	adopted, findError := persistence.NewStrategyScriptRepository(database).
		FindAllAdoptedBy(t.Context(), adopterID)
	require.NoError(t, findError)
	assert.Empty(t, adopted)
}

func TestDeletingAStrategyScriptTakesItsPublicationAndEveryAdoptionWithIt(t *testing.T) {
	database := newStrategyScriptTestDatabase(t)
	strategyScriptRepository := persistence.NewStrategyScriptRepository(database)
	adopterID := aSecondOwner(t, database)
	strategyScriptID := aPublishedStrategyScript(t, database)
	require.NoError(t, persistence.NewStrategyScriptAdoptionRepository(database).
		Adopt(t.Context(), adopterID, strategyScriptID, publishedAtNoon))

	require.NoError(t, strategyScriptRepository.Delete(t.Context(), strategyScriptID))

	publications, publicationError := strategyScriptRepository.FindAllPublished(t.Context())
	require.NoError(t, publicationError)
	assert.Empty(t, publications)
	adopted, adoptedError := strategyScriptRepository.FindAllAdoptedBy(t.Context(), adopterID)
	require.NoError(t, adoptedError)
	assert.Empty(t, adopted)
}

func TestFindAllAdoptedByOrdersByTheStrategyScriptsName(t *testing.T) {
	database := newStrategyScriptTestDatabase(t)
	strategyScriptRepository := persistence.NewStrategyScriptRepository(database)
	publishedStrategyScriptRepository := persistence.NewPublishedStrategyScriptRepository(database)
	strategyScriptAdoptionRepository := persistence.NewStrategyScriptAdoptionRepository(database)
	adopterID := aSecondOwner(t, database)

	// ASCII names avoid depending on the database's collation for Chinese.
	for _, name := range []string{"beta", "alpha"} {
		savedStrategyScript, saveError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed(name))
		require.NoError(t, saveError)
		require.NoError(t, publishedStrategyScriptRepository.Publish(t.Context(), savedStrategyScript.ID, publishedAtNoon))
		require.NoError(t, strategyScriptAdoptionRepository.Adopt(
			t.Context(), adopterID, savedStrategyScript.ID, publishedAtNoon))
	}

	adopted, findError := strategyScriptRepository.FindAllAdoptedBy(t.Context(), adopterID)

	require.NoError(t, findError)
	require.Len(t, adopted, 2)
	assert.Equal(t, "alpha", adopted[0].StrategyScript.Name)
	assert.Equal(t, "beta", adopted[1].StrategyScript.Name)
}

func TestFindAllAdoptedBySeesOnlyThatPersonsShelf(t *testing.T) {
	database := newStrategyScriptTestDatabase(t)
	adopterID := aSecondOwner(t, database)
	strategyScriptID := aPublishedStrategyScript(t, database)
	require.NoError(t, persistence.NewStrategyScriptAdoptionRepository(database).
		Adopt(t.Context(), adopterID, strategyScriptID, publishedAtNoon))

	adopted, findError := persistence.NewStrategyScriptRepository(database).
		FindAllAdoptedBy(t.Context(), strategyScriptRowOwnerID)

	require.NoError(t, findError)
	assert.Empty(t, adopted, "one person adopting something puts nothing on anybody else's shelf")
}

func aPublishedStrategyScript(t *testing.T, database *gorm.DB) uint {
	t.Helper()

	savedStrategyScript, saveError := persistence.NewStrategyScriptRepository(database).
		Save(t.Context(), strategyScriptNamed("二十根均線"))
	require.NoError(t, saveError)
	require.NoError(t, persistence.NewPublishedStrategyScriptRepository(database).
		Publish(t.Context(), savedStrategyScript.ID, publishedAtNoon))

	return savedStrategyScript.ID
}

func TestEveryMarketplaceOperationReportsAnUnusableStore(t *testing.T) {
	// A storage failure must never be reported as a business answer like "not published".
	database := newStrategyScriptTestDatabase(t)
	strategyScriptRepository := persistence.NewStrategyScriptRepository(database)
	publishedStrategyScriptRepository := persistence.NewPublishedStrategyScriptRepository(database)
	strategyScriptAdoptionRepository := persistence.NewStrategyScriptAdoptionRepository(database)

	sqlDatabase, connectionError := database.DB()
	require.NoError(t, connectionError)
	require.NoError(t, sqlDatabase.Close())

	t.Run("publish", func(t *testing.T) {
		err := publishedStrategyScriptRepository.Publish(t.Context(), 7, publishedAtNoon)
		assert.Error(t, err)
		assert.NotErrorIs(t, err, domains.ErrStrategyScriptNotFound)
	})

	t.Run("withdraw", func(t *testing.T) {
		err := publishedStrategyScriptRepository.Withdraw(t.Context(), 7)
		assert.Error(t, err)
		assert.NotErrorIs(t, err, domains.ErrStrategyScriptNotFound)
	})

	t.Run("read one publication", func(t *testing.T) {
		_, err := publishedStrategyScriptRepository.FindOne(t.Context(), 7)
		assert.Error(t, err)
		assert.NotErrorIs(t, err, domains.ErrStrategyScriptNotPublished,
			"an outage is not the same answer as 'this one is not shared'")
	})

	t.Run("adopt", func(t *testing.T) {
		err := strategyScriptAdoptionRepository.Adopt(t.Context(), 2, 7, publishedAtNoon)
		assert.Error(t, err)
		assert.NotErrorIs(t, err, domains.ErrStrategyScriptNotFound)
	})

	t.Run("abandon", func(t *testing.T) {
		err := strategyScriptAdoptionRepository.Abandon(t.Context(), 2, 7)
		assert.Error(t, err)
		assert.NotErrorIs(t, err, domains.ErrStrategyScriptNotFound)
	})

	t.Run("browse the marketplace", func(t *testing.T) {
		_, err := strategyScriptRepository.FindAllPublished(t.Context())
		assert.Error(t, err)
	})

	t.Run("read a shelf", func(t *testing.T) {
		_, err := strategyScriptRepository.FindAllAdoptedBy(t.Context(), 2)
		assert.Error(t, err)
	})

	t.Run("read one person's strategy scripts", func(t *testing.T) {
		_, err := strategyScriptRepository.FindAllOwnedBy(t.Context(), strategyScriptRowOwnerID)
		assert.Error(t, err)
	})
}

func TestRewritingAStrategyScriptDoesNotChangeWhoItBelongsTo(t *testing.T) {
	// The owner is not a writable column, so a rewrite carrying another owner changes nothing.
	database := newStrategyScriptTestDatabase(t)
	strategyScriptRepository := persistence.NewStrategyScriptRepository(database)
	secondOwnerID := aSecondOwner(t, database)
	savedStrategyScript, saveError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("二十根均線"))
	require.NoError(t, saveError)

	handedOver := savedStrategyScript
	handedOver.OwnerID = secondOwnerID
	handedOver.Script = rewrittenScript
	_, updateError := strategyScriptRepository.Update(t.Context(), handedOver)

	require.NoError(t, updateError)
	stillMine, findError := strategyScriptRepository.FindAllOwnedBy(t.Context(), strategyScriptRowOwnerID)
	require.NoError(t, findError)
	require.Len(t, stillMine, 1, "它還在原來那個人的名下")
	assert.Equal(t, rewrittenScript, stillMine[0].Script, "改得動的是算式，不是主人")
	theirs, theirError := strategyScriptRepository.FindAllOwnedBy(t.Context(), secondOwnerID)
	require.NoError(t, theirError)
	assert.Empty(t, theirs)
}

func TestRepublishingDoesNotBringBackAnybodysAdoption(t *testing.T) {
	// Republishing after withdrawal does not restore earlier adoptions.
	database := newStrategyScriptTestDatabase(t)
	adopterID := aSecondOwner(t, database)
	strategyScriptID := aPublishedStrategyScript(t, database)
	require.NoError(t, persistence.NewStrategyScriptAdoptionRepository(database).
		Adopt(t.Context(), adopterID, strategyScriptID, publishedAtNoon))
	publishedStrategyScriptRepository := persistence.NewPublishedStrategyScriptRepository(database)
	require.NoError(t, publishedStrategyScriptRepository.Withdraw(t.Context(), strategyScriptID))

	require.NoError(t, publishedStrategyScriptRepository.Publish(t.Context(), strategyScriptID, publishedAtDusk))

	adopted, findError := persistence.NewStrategyScriptRepository(database).
		FindAllAdoptedBy(t.Context(), adopterID)
	require.NoError(t, findError)
	assert.Empty(t, adopted, "要再加入一次")
}

func TestRewritingAPublishedStrategyScriptLeavesItPublishedAndAdopted(t *testing.T) {
	// Adopters see the owner's latest version, not a frozen copy.
	database := newStrategyScriptTestDatabase(t)
	strategyScriptRepository := persistence.NewStrategyScriptRepository(database)
	adopterID := aSecondOwner(t, database)
	strategyScriptID := aPublishedStrategyScript(t, database)
	require.NoError(t, persistence.NewStrategyScriptAdoptionRepository(database).
		Adopt(t.Context(), adopterID, strategyScriptID, publishedAtNoon))

	rewritten := strategyScriptNamed("改過名字的")
	rewritten.ID = strategyScriptID
	rewritten.Description = "改過的說明"
	rewritten.Script = rewrittenScript
	_, updateError := strategyScriptRepository.Update(t.Context(), rewritten)
	require.NoError(t, updateError)

	adopted, findError := strategyScriptRepository.FindAllAdoptedBy(t.Context(), adopterID)
	require.NoError(t, findError)
	require.Len(t, adopted, 1, "改一支策略腳本不會把它從別人的書架上拿走")
	assert.Equal(t, "改過名字的", adopted[0].ToDto().Name, "採用的是那一支策略腳本，不是它當時的名字")
	assert.Equal(t, "改過的說明", adopted[0].ToDto().Description)
	assert.Equal(t, rewrittenScript, adopted[0].StrategyScript.Script,
		"下一次執行拿到的是改過之後的")

	onTheShelf, publishedError := strategyScriptRepository.FindAllPublished(t.Context())
	require.NoError(t, publishedError)
	require.Len(t, onTheShelf, 1, "它仍然在市集上")
	assert.Equal(t, "改過的說明", onTheShelf[0].ToDto().Description)
}

func TestAnOwnersOwnStrategyScriptsSayWhetherTheyAreOnTheMarketplace(t *testing.T) {
	// This decides whether the owner sees publish or withdraw.
	database := newStrategyScriptTestDatabase(t)
	strategyScriptRepository := persistence.NewStrategyScriptRepository(database)
	published := aPublishedStrategyScript(t, database)
	kept, saveError := strategyScriptRepository.Save(t.Context(), strategyScriptNamed("沒發佈的"))
	require.NoError(t, saveError)

	strategyScripts, findError := strategyScriptRepository.FindAllOwnedBy(t.Context(), strategyScriptRowOwnerID)

	require.NoError(t, findError)
	require.Len(t, strategyScripts, 2)
	byIdentifier := map[uint]bool{}
	for _, strategyScript := range strategyScripts {
		byIdentifier[strategyScript.ID] = strategyScript.ToDto().Published
	}
	assert.True(t, byIdentifier[published])
	assert.False(t, byIdentifier[kept.ID])
}
