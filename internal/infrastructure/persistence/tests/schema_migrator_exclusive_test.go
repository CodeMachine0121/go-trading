package persistence_test

import (
	"sync"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaMigratorLetsOnlyOneMigrationRunAtATime(t *testing.T) {
	database := newTestDatabase(t)
	firstMigrator := persistence.NewSchemaMigrator(database)
	secondMigrator := persistence.NewSchemaMigrator(database)
	firstStarted := make(chan struct{})
	letFirstFinish := make(chan struct{})
	order := []string{}
	orderMutex := sync.Mutex{}
	record := func(step string) {
		orderMutex.Lock()
		defer orderMutex.Unlock()
		order = append(order, step)
	}
	finished := sync.WaitGroup{}
	finished.Add(2)

	go func() {
		defer finished.Done()
		assert.NoError(t, firstMigrator.Exclusively(t.Context(), func() error {
			close(firstStarted)
			<-letFirstFinish
			record("first ends")

			return nil
		}))
	}()
	<-firstStarted
	go func() {
		defer finished.Done()
		assert.NoError(t, secondMigrator.Exclusively(t.Context(), func() error {
			record("second starts")

			return nil
		}))
	}()
	time.Sleep(200 * time.Millisecond)
	close(letFirstFinish)
	finished.Wait()

	require.Len(t, order, 2)
	assert.Equal(t, []string{"first ends", "second starts"}, order)
}
