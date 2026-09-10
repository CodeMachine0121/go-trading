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

// publishedAtNoon and publishedAtDusk are two moments far enough apart that an
// ordering by them cannot be a coincidence.
var (
	publishedAtNoon = time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	publishedAtDusk = time.Date(2026, 9, 10, 18, 0, 0, 0, time.UTC)
)

func TestStrategiesOfTwoPeopleMayShareAName(t *testing.T) {
	// A name is what its owner recognises a strategy by, and nobody recognises a
	// stranger's. One shared pool would mean the first person here takes the good
	// names away from everybody else for good.
	database := newStrategyTestDatabase(t)
	strategyRepository := persistence.NewStrategyRepository(database)
	secondOwnerID := aSecondOwner(t, database)

	_, firstError := strategyRepository.Save(t.Context(), strategyNamed("二十根均線"))
	secondPersons := strategyNamed("二十根均線")
	secondPersons.OwnerID = secondOwnerID
	_, secondError := strategyRepository.Save(t.Context(), secondPersons)

	require.NoError(t, firstError)
	require.NoError(t, secondError)
}

func TestFindAllOwnedByAnswersOnlyThatPersonsStrategies(t *testing.T) {
	database := newStrategyTestDatabase(t)
	strategyRepository := persistence.NewStrategyRepository(database)
	secondOwnerID := aSecondOwner(t, database)
	_, firstError := strategyRepository.Save(t.Context(), strategyNamed("我的"))
	require.NoError(t, firstError)
	theirs := strategyNamed("他的")
	theirs.OwnerID = secondOwnerID
	_, secondError := strategyRepository.Save(t.Context(), theirs)
	require.NoError(t, secondError)

	strategies, findError := strategyRepository.FindAllOwnedBy(t.Context(), strategyRowOwnerID)

	require.NoError(t, findError)
	require.Len(t, strategies, 1)
	assert.Equal(t, "我的", strategies[0].Name)
}

func TestPublishStrategyTwiceKeepsOneRowAndTheFirstMoment(t *testing.T) {
	// Publishing states the state to end in, not an event. Saying it twice says the
	// same thing as saying it once, and the strategy has been out there since the
	// first time — pressing the button again does not change when that started.
	database := newStrategyTestDatabase(t)
	strategyRepository := persistence.NewStrategyRepository(database)
	publishedStrategyRepository := persistence.NewPublishedStrategyRepository(database)
	savedStrategy, saveError := strategyRepository.Save(t.Context(), strategyNamed("二十根均線"))
	require.NoError(t, saveError)

	require.NoError(t, publishedStrategyRepository.Publish(t.Context(), savedStrategy.ID, publishedAtNoon))
	require.NoError(t, publishedStrategyRepository.Publish(t.Context(), savedStrategy.ID, publishedAtDusk))

	publications, findError := strategyRepository.FindAllPublished(t.Context())
	require.NoError(t, findError)
	require.Len(t, publications, 1)
	assert.Equal(t, publishedAtNoon, publications[0].PublishedAt.UTC())
}

func TestWithdrawStrategyThatWasNeverPublishedIsNotAFailure(t *testing.T) {
	database := newStrategyTestDatabase(t)
	strategyRepository := persistence.NewStrategyRepository(database)
	savedStrategy, saveError := strategyRepository.Save(t.Context(), strategyNamed("二十根均線"))
	require.NoError(t, saveError)

	withdrawError := persistence.NewPublishedStrategyRepository(database).
		Withdraw(t.Context(), savedStrategy.ID)

	require.NoError(t, withdrawError, "what was asked for already holds")
}

func TestFindOnePublicationAnswersWhenItIsOnTheMarketplace(t *testing.T) {
	database := newStrategyTestDatabase(t)
	strategyID := aPublishedStrategy(t, database)

	publication, findError := persistence.NewPublishedStrategyRepository(database).
		FindOne(t.Context(), strategyID)

	require.NoError(t, findError)
	assert.Equal(t, strategyID, publication.StrategyID)
	assert.Equal(t, publishedAtNoon, publication.PublishedAt.UTC())
}

func TestFindOnePublicationSaysWhenThereIsNone(t *testing.T) {
	database := newStrategyTestDatabase(t)
	strategyRepository := persistence.NewStrategyRepository(database)
	savedStrategy, saveError := strategyRepository.Save(t.Context(), strategyNamed("二十根均線"))
	require.NoError(t, saveError)

	_, findError := persistence.NewPublishedStrategyRepository(database).
		FindOne(t.Context(), savedStrategy.ID)

	require.ErrorIs(t, findError, domains.ErrStrategyNotPublished)
}

func TestFindAllPublishedOrdersByTheNewestPublicationFirst(t *testing.T) {
	database := newStrategyTestDatabase(t)
	strategyRepository := persistence.NewStrategyRepository(database)
	publishedStrategyRepository := persistence.NewPublishedStrategyRepository(database)
	earlier, earlierError := strategyRepository.Save(t.Context(), strategyNamed("先發佈的"))
	require.NoError(t, earlierError)
	later, laterError := strategyRepository.Save(t.Context(), strategyNamed("後發佈的"))
	require.NoError(t, laterError)
	require.NoError(t, publishedStrategyRepository.Publish(t.Context(), earlier.ID, publishedAtNoon))
	require.NoError(t, publishedStrategyRepository.Publish(t.Context(), later.ID, publishedAtDusk))

	publications, findError := strategyRepository.FindAllPublished(t.Context())

	require.NoError(t, findError)
	require.Len(t, publications, 2)
	assert.Equal(t, "後發佈的", publications[0].Strategy.Name)
	assert.Equal(t, "先發佈的", publications[1].Strategy.Name)
}

func TestFindAllPublishedCarriesTheStrategyItsKnobsAndItsOwner(t *testing.T) {
	// A listing of identifiers would send the reader round again per row, and the
	// marketplace is the one page where every row needs all of it.
	database := newStrategyTestDatabase(t)
	strategyRepository := persistence.NewStrategyRepository(database)
	withKnob := strategyNamed("布林通道")
	withKnob.Parameters = []entities.StrategyParameter{
		{Name: "期數", Kind: "lookbackCount", DefaultValue: 20},
	}
	savedStrategy, saveError := strategyRepository.Save(t.Context(), withKnob)
	require.NoError(t, saveError)
	require.NoError(t, persistence.NewPublishedStrategyRepository(database).
		Publish(t.Context(), savedStrategy.ID, publishedAtNoon))

	publications, findError := strategyRepository.FindAllPublished(t.Context())

	require.NoError(t, findError)
	require.Len(t, publications, 1)
	assert.Equal(t, "布林通道", publications[0].Strategy.Name)
	require.Len(t, publications[0].Strategy.Parameters, 1)
	assert.Equal(t, "期數", publications[0].Strategy.Parameters[0].Name)
	assert.Equal(t, "owner@example.com", publications[0].Strategy.Owner.Email)
}

func TestAdoptStrategyTwiceKeepsOneRow(t *testing.T) {
	database := newStrategyTestDatabase(t)
	adopterID := aSecondOwner(t, database)
	strategyID := aPublishedStrategy(t, database)
	strategyAdoptionRepository := persistence.NewStrategyAdoptionRepository(database)

	require.NoError(t, strategyAdoptionRepository.Adopt(t.Context(), adopterID, strategyID, publishedAtNoon))
	require.NoError(t, strategyAdoptionRepository.Adopt(t.Context(), adopterID, strategyID, publishedAtDusk))

	adopted, findError := persistence.NewStrategyRepository(database).
		FindAllAdoptedBy(t.Context(), adopterID)
	require.NoError(t, findError)
	assert.Len(t, adopted, 1)
}

func TestAdoptSomethingNotOnTheMarketplaceIsRefusedAsNotFound(t *testing.T) {
	// There is no publication for the shelf entry to hang from, and the schema is
	// what finds out — asking first and writing afterwards would let a withdrawal
	// land in between and leave a shelf pointing at nothing.
	database := newStrategyTestDatabase(t)
	strategyRepository := persistence.NewStrategyRepository(database)
	adopterID := aSecondOwner(t, database)
	unpublished, saveError := strategyRepository.Save(t.Context(), strategyNamed("沒發佈的"))
	require.NoError(t, saveError)

	adoptError := persistence.NewStrategyAdoptionRepository(database).
		Adopt(t.Context(), adopterID, unpublished.ID, publishedAtNoon)

	require.ErrorIs(t, adoptError, domains.ErrStrategyNotFound)
}

func TestAbandonSomethingNeverAdoptedIsNotAFailure(t *testing.T) {
	database := newStrategyTestDatabase(t)
	adopterID := aSecondOwner(t, database)
	strategyID := aPublishedStrategy(t, database)

	abandonError := persistence.NewStrategyAdoptionRepository(database).
		Abandon(t.Context(), adopterID, strategyID)

	require.NoError(t, abandonError)
}

func TestWithdrawingAStrategyClearsEverybodysAdoptionOfIt(t *testing.T) {
	// This is the whole implementation of that rule: the adoptions hang off the
	// publication with a cascade, so no line of Go performs it and none can forget.
	database := newStrategyTestDatabase(t)
	adopterID := aSecondOwner(t, database)
	strategyID := aPublishedStrategy(t, database)
	require.NoError(t, persistence.NewStrategyAdoptionRepository(database).
		Adopt(t.Context(), adopterID, strategyID, publishedAtNoon))

	require.NoError(t, persistence.NewPublishedStrategyRepository(database).
		Withdraw(t.Context(), strategyID))

	adopted, findError := persistence.NewStrategyRepository(database).
		FindAllAdoptedBy(t.Context(), adopterID)
	require.NoError(t, findError)
	assert.Empty(t, adopted)
}

func TestDeletingAStrategyTakesItsPublicationAndEveryAdoptionWithIt(t *testing.T) {
	database := newStrategyTestDatabase(t)
	strategyRepository := persistence.NewStrategyRepository(database)
	adopterID := aSecondOwner(t, database)
	strategyID := aPublishedStrategy(t, database)
	require.NoError(t, persistence.NewStrategyAdoptionRepository(database).
		Adopt(t.Context(), adopterID, strategyID, publishedAtNoon))

	require.NoError(t, strategyRepository.Delete(t.Context(), strategyID))

	publications, publicationError := strategyRepository.FindAllPublished(t.Context())
	require.NoError(t, publicationError)
	assert.Empty(t, publications)
	adopted, adoptedError := strategyRepository.FindAllAdoptedBy(t.Context(), adopterID)
	require.NoError(t, adoptedError)
	assert.Empty(t, adopted)
}

func TestFindAllAdoptedByOrdersByTheStrategysName(t *testing.T) {
	database := newStrategyTestDatabase(t)
	strategyRepository := persistence.NewStrategyRepository(database)
	publishedStrategyRepository := persistence.NewPublishedStrategyRepository(database)
	strategyAdoptionRepository := persistence.NewStrategyAdoptionRepository(database)
	adopterID := aSecondOwner(t, database)

	// The names are plain letters on purpose: how a database orders Chinese
	// characters depends on its collation, and this case is about the column the
	// ordering names, not about anybody's collation.
	for _, name := range []string{"beta", "alpha"} {
		savedStrategy, saveError := strategyRepository.Save(t.Context(), strategyNamed(name))
		require.NoError(t, saveError)
		require.NoError(t, publishedStrategyRepository.Publish(t.Context(), savedStrategy.ID, publishedAtNoon))
		require.NoError(t, strategyAdoptionRepository.Adopt(
			t.Context(), adopterID, savedStrategy.ID, publishedAtNoon))
	}

	adopted, findError := strategyRepository.FindAllAdoptedBy(t.Context(), adopterID)

	require.NoError(t, findError)
	require.Len(t, adopted, 2)
	assert.Equal(t, "alpha", adopted[0].Strategy.Name)
	assert.Equal(t, "beta", adopted[1].Strategy.Name)
}

func TestFindAllAdoptedBySeesOnlyThatPersonsShelf(t *testing.T) {
	database := newStrategyTestDatabase(t)
	adopterID := aSecondOwner(t, database)
	strategyID := aPublishedStrategy(t, database)
	require.NoError(t, persistence.NewStrategyAdoptionRepository(database).
		Adopt(t.Context(), adopterID, strategyID, publishedAtNoon))

	adopted, findError := persistence.NewStrategyRepository(database).
		FindAllAdoptedBy(t.Context(), strategyRowOwnerID)

	require.NoError(t, findError)
	assert.Empty(t, adopted, "one person adopting something puts nothing on anybody else's shelf")
}

// aPublishedStrategy saves a strategy for the usual owner, puts it on the
// marketplace, and answers with its identifier.
func aPublishedStrategy(t *testing.T, database *gorm.DB) uint {
	t.Helper()

	savedStrategy, saveError := persistence.NewStrategyRepository(database).
		Save(t.Context(), strategyNamed("二十根均線"))
	require.NoError(t, saveError)
	require.NoError(t, persistence.NewPublishedStrategyRepository(database).
		Publish(t.Context(), savedStrategy.ID, publishedAtNoon))

	return savedStrategy.ID
}

func TestEveryMarketplaceOperationReportsAnUnusableStore(t *testing.T) {
	// A store that will not answer must never come back as a business answer. Read
	// as "not published" or "not on anybody's shelf", an outage would refuse people
	// in words they cannot tell from a real refusal, and they would go looking for
	// a strategy that is sitting right there.
	database := newStrategyTestDatabase(t)
	strategyRepository := persistence.NewStrategyRepository(database)
	publishedStrategyRepository := persistence.NewPublishedStrategyRepository(database)
	strategyAdoptionRepository := persistence.NewStrategyAdoptionRepository(database)

	sqlDatabase, connectionError := database.DB()
	require.NoError(t, connectionError)
	require.NoError(t, sqlDatabase.Close())

	t.Run("publish", func(t *testing.T) {
		err := publishedStrategyRepository.Publish(t.Context(), 7, publishedAtNoon)
		assert.Error(t, err)
		assert.NotErrorIs(t, err, domains.ErrStrategyNotFound)
	})

	t.Run("withdraw", func(t *testing.T) {
		err := publishedStrategyRepository.Withdraw(t.Context(), 7)
		assert.Error(t, err)
		assert.NotErrorIs(t, err, domains.ErrStrategyNotFound)
	})

	t.Run("read one publication", func(t *testing.T) {
		_, err := publishedStrategyRepository.FindOne(t.Context(), 7)
		assert.Error(t, err)
		assert.NotErrorIs(t, err, domains.ErrStrategyNotPublished,
			"an outage is not the same answer as 'this one is not shared'")
	})

	t.Run("adopt", func(t *testing.T) {
		err := strategyAdoptionRepository.Adopt(t.Context(), 2, 7, publishedAtNoon)
		assert.Error(t, err)
		assert.NotErrorIs(t, err, domains.ErrStrategyNotFound)
	})

	t.Run("abandon", func(t *testing.T) {
		err := strategyAdoptionRepository.Abandon(t.Context(), 2, 7)
		assert.Error(t, err)
		assert.NotErrorIs(t, err, domains.ErrStrategyNotFound)
	})

	t.Run("browse the marketplace", func(t *testing.T) {
		_, err := strategyRepository.FindAllPublished(t.Context())
		assert.Error(t, err)
	})

	t.Run("read a shelf", func(t *testing.T) {
		_, err := strategyRepository.FindAllAdoptedBy(t.Context(), 2)
		assert.Error(t, err)
	})

	t.Run("read one person's strategies", func(t *testing.T) {
		_, err := strategyRepository.FindAllOwnedBy(t.Context(), strategyRowOwnerID)
		assert.Error(t, err)
	})
}

func TestRewritingAStrategyDoesNotChangeWhoItBelongsTo(t *testing.T) {
	// A strategy never changes hands, and the write path is where that could
	// quietly stop being true: the owner is not on the list of columns a rewrite
	// may touch, so a rewrite carrying somebody else's identifier reaches nothing.
	database := newStrategyTestDatabase(t)
	strategyRepository := persistence.NewStrategyRepository(database)
	secondOwnerID := aSecondOwner(t, database)
	savedStrategy, saveError := strategyRepository.Save(t.Context(), strategyNamed("二十根均線"))
	require.NoError(t, saveError)

	handedOver := savedStrategy
	handedOver.OwnerID = secondOwnerID
	handedOver.Script = rewrittenScript
	_, updateError := strategyRepository.Update(t.Context(), handedOver)

	require.NoError(t, updateError)
	stillMine, findError := strategyRepository.FindAllOwnedBy(t.Context(), strategyRowOwnerID)
	require.NoError(t, findError)
	require.Len(t, stillMine, 1, "它還在原來那個人的名下")
	assert.Equal(t, rewrittenScript, stillMine[0].Script, "改得動的是算式，不是主人")
	theirs, theirError := strategyRepository.FindAllOwnedBy(t.Context(), secondOwnerID)
	require.NoError(t, theirError)
	assert.Empty(t, theirs)
}

func TestRepublishingDoesNotBringBackAnybodysAdoption(t *testing.T) {
	// The owner took it back, and taking it back is not an agreement that everyone
	// gets it again the moment they change their mind. Each person decides afresh.
	database := newStrategyTestDatabase(t)
	adopterID := aSecondOwner(t, database)
	strategyID := aPublishedStrategy(t, database)
	require.NoError(t, persistence.NewStrategyAdoptionRepository(database).
		Adopt(t.Context(), adopterID, strategyID, publishedAtNoon))
	publishedStrategyRepository := persistence.NewPublishedStrategyRepository(database)
	require.NoError(t, publishedStrategyRepository.Withdraw(t.Context(), strategyID))

	require.NoError(t, publishedStrategyRepository.Publish(t.Context(), strategyID, publishedAtDusk))

	adopted, findError := persistence.NewStrategyRepository(database).
		FindAllAdoptedBy(t.Context(), adopterID)
	require.NoError(t, findError)
	assert.Empty(t, adopted, "要再加入一次")
}

func TestRewritingAPublishedStrategyLeavesItPublishedAndAdopted(t *testing.T) {
	// Publishing hands out the use of an algorithm, not a frozen copy. An owner who
	// fixes a mistake should not also have to remember to publish again, and
	// whoever is using it should get the fix.
	database := newStrategyTestDatabase(t)
	strategyRepository := persistence.NewStrategyRepository(database)
	adopterID := aSecondOwner(t, database)
	strategyID := aPublishedStrategy(t, database)
	require.NoError(t, persistence.NewStrategyAdoptionRepository(database).
		Adopt(t.Context(), adopterID, strategyID, publishedAtNoon))

	rewritten := strategyNamed("改過名字的")
	rewritten.ID = strategyID
	rewritten.Description = "改過的說明"
	rewritten.Script = rewrittenScript
	_, updateError := strategyRepository.Update(t.Context(), rewritten)
	require.NoError(t, updateError)

	adopted, findError := strategyRepository.FindAllAdoptedBy(t.Context(), adopterID)
	require.NoError(t, findError)
	require.Len(t, adopted, 1, "改一支策略不會把它從別人的書架上拿走")
	assert.Equal(t, "改過名字的", adopted[0].ToDto().Name, "採用的是那一支策略，不是它當時的名字")
	assert.Equal(t, "改過的說明", adopted[0].ToDto().Description)
	assert.Equal(t, rewrittenScript, adopted[0].Strategy.Script,
		"下一次執行拿到的是改過之後的")

	onTheShelf, publishedError := strategyRepository.FindAllPublished(t.Context())
	require.NoError(t, publishedError)
	require.Len(t, onTheShelf, 1, "它仍然在市集上")
	assert.Equal(t, "改過的說明", onTheShelf[0].ToDto().Description)
}
