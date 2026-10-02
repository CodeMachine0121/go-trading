package persistence_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var heartbeatAt = time.Date(2026, 10, 3, 8, 0, 0, 0, time.UTC)

func TestReplicaHeartbeatRepositoryFindsOnlyReplicasSeenRecently(t *testing.T) {
	repository := persistence.NewReplicaHeartbeatRepository(newTestDatabase(t))
	require.NoError(t, repository.Beat(t.Context(), "replica-a", heartbeatAt.Add(-time.Minute)))
	require.NoError(t, repository.Beat(t.Context(), "replica-b", heartbeatAt.Add(-time.Minute)))
	// A second beat moves the replica's last sighting rather than adding a row.
	require.NoError(t, repository.Beat(t.Context(), "replica-a", heartbeatAt))

	seen, findError := repository.FindSeenSince(t.Context(), heartbeatAt.Add(-30*time.Second))
	require.NoError(t, findError)

	assert.Equal(t, []string{"replica-a"}, seen)
}

func TestReplicaHeartbeatRepositorySaysSoWhenStorageIsGone(t *testing.T) {
	repository := persistence.NewReplicaHeartbeatRepository(closedDatabase(t))

	assert.Error(t, repository.Beat(t.Context(), "replica-a", heartbeatAt))
	_, findError := repository.FindSeenSince(t.Context(), heartbeatAt)
	assert.Error(t, findError)
}
