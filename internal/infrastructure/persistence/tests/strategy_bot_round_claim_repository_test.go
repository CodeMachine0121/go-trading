package persistence_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var claimNow = time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)

// aDueRunningBot stores a running bot that came due a minute before claimNow.
func aDueRunningBot(t *testing.T, repository *persistence.StrategyBotRepository, name string) entities.StrategyBot {
	t.Helper()

	bot, saveError := repository.Save(t.Context(), aBotRow(name))
	require.NoError(t, saveError)
	bot.RunState = string(vo.StrategyBotRunning)
	bot.NextRunAt = claimNow.Add(-time.Minute)
	require.NoError(t, repository.UpdateRunState(t.Context(), bot))

	return bot
}

func claimedByOf(t *testing.T, database *gorm.DB, id uint) string {
	t.Helper()

	stored := entities.StrategyBot{}
	require.NoError(t, database.WithContext(t.Context()).First(&stored, id).Error)

	return stored.RoundClaimedBy
}

func namesOf(bots []entities.StrategyBot) []string {
	names := make([]string, 0, len(bots))
	for _, bot := range bots {
		names = append(names, bot.Name)
	}

	return names
}

func TestStrategyBotRepositoryClaimDueClaimsWhatItReturns(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)
	botX := aDueRunningBot(t, repository, "X")
	botY := aDueRunningBot(t, repository, "Y")

	claimed, claimError := repository.ClaimDue(t.Context(), claimNow, 10, "replica-a", claimNow.Add(2*time.Minute))
	require.NoError(t, claimError)

	assert.ElementsMatch(t, []string{"X", "Y"}, namesOf(claimed))
	assert.Equal(t, "replica-a", claimedByOf(t, database, botX.ID))
	assert.Equal(t, "replica-a", claimedByOf(t, database, botY.ID))
}

func TestStrategyBotRepositoryClaimDueRespectsAnotherReplicasClaim(t *testing.T) {
	testCases := []struct {
		name          string
		secondClaimAt time.Time
		expectedNames []string
		expectedOwner string
	}{
		{
			name: "a live claim keeps the bot from the second replica", secondClaimAt: claimNow.Add(time.Minute),
			expectedNames: []string{}, expectedOwner: "replica-a",
		},
		{
			name: "an expired claim lets the second replica take the bot", secondClaimAt: claimNow.Add(3 * time.Minute),
			expectedNames: []string{"X"}, expectedOwner: "replica-b",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			database := newStrategyBotTestDatabase(t)
			repository := persistence.NewStrategyBotRepository(database)
			botX := aDueRunningBot(t, repository, "X")
			_, firstError := repository.ClaimDue(t.Context(), claimNow, 10, "replica-a", claimNow.Add(2*time.Minute))
			require.NoError(t, firstError)

			claimed, claimError := repository.ClaimDue(
				t.Context(), testCase.secondClaimAt, 10, "replica-b", testCase.secondClaimAt.Add(2*time.Minute))
			require.NoError(t, claimError)

			assert.Equal(t, testCase.expectedNames, namesOf(claimed))
			assert.Equal(t, testCase.expectedOwner, claimedByOf(t, database, botX.ID))
		})
	}
}

func TestStrategyBotRepositoryRacingReplicasNeverClaimTheSameBot(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)
	botNames := []string{"A", "B", "C", "D", "E", "F", "G", "H"}
	for _, name := range botNames {
		aDueRunningBot(t, repository, name)
	}
	replicaNames := []string{"replica-a", "replica-b", "replica-c", "replica-d"}
	claimedByReplica := make([][]string, len(replicaNames))
	startTogether := sync.WaitGroup{}
	startTogether.Add(1)
	finished := sync.WaitGroup{}

	for index, replicaName := range replicaNames {
		finished.Add(1)
		go func() {
			defer finished.Done()
			startTogether.Wait()
			claimed, claimError := repository.ClaimDue(
				t.Context(), claimNow, 3, replicaName, claimNow.Add(2*time.Minute))
			if claimError == nil {
				claimedByReplica[index] = namesOf(claimed)
			}
		}()
	}
	startTogether.Done()
	finished.Wait()

	everyClaim := []string{}
	for _, names := range claimedByReplica {
		everyClaim = append(everyClaim, names...)
	}
	assert.ElementsMatch(t, botNames, everyClaim, "every due bot is claimed exactly once across replicas")
}

func TestStrategyBotRepositoryClaimOne(t *testing.T) {
	testCases := []struct {
		name          string
		runState      vo.StrategyBotRunStateVo
		heldByA       bool
		expectedClaim bool
	}{
		{name: "a bot another replica is running cannot be claimed", runState: vo.StrategyBotRunning,
			heldByA: true, expectedClaim: false},
		{name: "a bot nobody is running is claimed", runState: vo.StrategyBotRunning, expectedClaim: true},
		{name: "a stopped bot can still be claimed for a round by hand", runState: vo.StrategyBotStopped,
			expectedClaim: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			repository := persistence.NewStrategyBotRepository(newStrategyBotTestDatabase(t))
			bot, saveError := repository.Save(t.Context(), aBotRow("X"))
			require.NoError(t, saveError)
			bot.RunState = string(testCase.runState)
			require.NoError(t, repository.UpdateRunState(t.Context(), bot))
			if testCase.heldByA {
				_, _ = repository.ClaimOne(t.Context(), bot.ID, "replica-a", claimNow, claimNow.Add(2*time.Minute))
			}

			claimed, claimError := repository.ClaimOne(
				t.Context(), bot.ID, "replica-b", claimNow.Add(time.Minute), claimNow.Add(3*time.Minute))
			require.NoError(t, claimError)

			assert.Equal(t, testCase.expectedClaim, claimed)
		})
	}
}

func TestStrategyBotRepositoryReleaseRoundClaim(t *testing.T) {
	testCases := []struct {
		name          string
		releasedBy    string
		expectedOwner string
	}{
		{name: "the holder frees the bot", releasedBy: "replica-a", expectedOwner: ""},
		{name: "someone else cannot free a claim that is not theirs", releasedBy: "replica-b",
			expectedOwner: "replica-a"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			database := newStrategyBotTestDatabase(t)
			repository := persistence.NewStrategyBotRepository(database)
			botX := aDueRunningBot(t, repository, "X")
			_, _ = repository.ClaimDue(t.Context(), claimNow, 10, "replica-a", claimNow.Add(2*time.Minute))

			require.NoError(t, repository.ReleaseRoundClaim(t.Context(), botX.ID, testCase.releasedBy))

			assert.Equal(t, testCase.expectedOwner, claimedByOf(t, database, botX.ID))
		})
	}
}

func TestStrategyBotRepositoryAReleasedBotIsClaimableAgainAtOnce(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)
	aDueRunningBot(t, repository, "X")
	first, _ := repository.ClaimDue(t.Context(), claimNow, 10, "replica-a", claimNow.Add(2*time.Minute))
	require.NoError(t, repository.ReleaseRoundClaim(t.Context(), first[0].ID, "replica-a"))

	claimed, claimError := repository.ClaimDue(t.Context(), claimNow.Add(time.Second), 10, "replica-b",
		claimNow.Add(2*time.Minute))
	require.NoError(t, claimError)

	assert.Equal(t, []string{"X"}, namesOf(claimed))
}

func TestStrategyBotRepositoryForgetSentSignal(t *testing.T) {
	testCases := []struct {
		name           string
		forgotten      vo.SignalVo
		expectedSignal string
	}{
		{name: "the signal a message carried is forgotten", forgotten: vo.SignalBuy, expectedSignal: ""},
		{name: "a newer signal is kept", forgotten: vo.SignalSell, expectedSignal: string(vo.SignalBuy)},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			database := newStrategyBotTestDatabase(t)
			repository := persistence.NewStrategyBotRepository(database)
			bot := aDueRunningBot(t, repository, "X")
			bot.LastSentSignal = string(vo.SignalBuy)
			require.NoError(t, repository.UpdateRunState(t.Context(), bot))

			require.NoError(t, repository.ForgetSentSignal(t.Context(), bot.ID, string(testCase.forgotten)))

			stored, findError := repository.FindOne(t.Context(), bot.ID)
			require.NoError(t, findError)
			assert.Equal(t, testCase.expectedSignal, stored.LastSentSignal)
		})
	}
}

func TestStrategyBotRepositoryFindOneLockedReadsTheBotInsideATransaction(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)
	bot := aDueRunningBot(t, repository, "X")

	readError := persistence.NewTransactionRepository(database).Atomically(t.Context(),
		func(transactionContext context.Context) error {
			locked, findError := repository.FindOneLocked(transactionContext, bot.ID)
			require.NoError(t, findError)
			assert.Equal(t, "X", locked.Name)

			_, missingError := repository.FindOneLocked(transactionContext, bot.ID+1000)
			assert.ErrorIs(t, missingError, domains.ErrStrategyBotNotFound)

			return nil
		})

	require.NoError(t, readError)
}
