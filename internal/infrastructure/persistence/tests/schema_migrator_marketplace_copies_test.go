package persistence_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// legacyAdoptionRow is the adoptions table as it stood before adopting made a copy.
type legacyAdoptionRow struct {
	ID               uint `gorm:"primaryKey"`
	UserID           uint `gorm:"not null"`
	StrategyScriptID uint `gorm:"column:strategy_id;not null"`
	AdoptedAt        time.Time
}

func (legacyAdoptionRow) TableName() string {
	return "StrategyAdoptions"
}

const (
	legacyPublisherID = strategyScriptRowOwnerID
	legacyAdopterID   = strategyScriptRowOwnerID + 1
)

// legacyMarketplace is the state an old database was left in: the publisher's 動能 with a knob, and the adopter.
type legacyMarketplace struct {
	database         *gorm.DB
	originalScriptID uint
}

func newLegacyMarketplace(t *testing.T) legacyMarketplace {
	database := newStrategyScriptTestDatabase(t)
	aSecondOwner(t, database)
	require.NoError(t, database.Migrator().CreateTable(&legacyAdoptionRow{}))
	t.Cleanup(func() { _ = database.Migrator().DropTable(&legacyAdoptionRow{}) })

	original := strategyScriptNamed("動能")
	original.OwnerID = legacyPublisherID
	original.Parameters = []entities.StrategyScriptParameter{{Name: "lookback", Kind: "lookbackCount", DefaultValue: 20}}
	require.NoError(t, database.Create(&original).Error)
	require.NoError(t, persistence.NewPublishedStrategyScriptRepository(database).
		Publish(t.Context(), original.ID, time.Now()))

	return legacyMarketplace{database: database, originalScriptID: original.ID}
}

// followedBy gives the adopter a trading strategy whose one source names this script.
func (legacy legacyMarketplace) followedBy(t *testing.T, strategyScriptID uint) uint {
	tradingStrategy := entities.TradingStrategy{
		OwnerID: legacyAdopterID, Name: "跟著別人的",
		SignalSources: []entities.TradingStrategySignalSource{
			{Label: "A", StrategyScriptID: strategyScriptID, AggregationInterval: "1h"},
		},
	}
	require.NoError(t, legacy.database.Create(&tradingStrategy).Error)

	return tradingStrategy.SignalSources[0].ID
}

func (legacy legacyMarketplace) sourceScriptOf(t *testing.T, signalSourceID uint) uint {
	signalSource := entities.TradingStrategySignalSource{}
	require.NoError(t, legacy.database.First(&signalSource, signalSourceID).Error)

	return signalSource.StrategyScriptID
}

func (legacy legacyMarketplace) adopted(t *testing.T) {
	require.NoError(t, legacy.database.Create(&legacyAdoptionRow{
		UserID: legacyAdopterID, StrategyScriptID: legacy.originalScriptID, AdoptedAt: time.Now(),
	}).Error)
}

// followedByAStrategy gives the adopter a trading strategy whose one source names the publisher's script.
func (legacy legacyMarketplace) followedByAStrategy(t *testing.T) uint {
	tradingStrategy := entities.TradingStrategy{
		OwnerID: legacyAdopterID, Name: "跟著動能",
		SignalSources: []entities.TradingStrategySignalSource{
			{Label: "A", StrategyScriptID: legacy.originalScriptID, AggregationInterval: "1h"},
		},
	}
	require.NoError(t, legacy.database.Create(&tradingStrategy).Error)

	return tradingStrategy.SignalSources[0].ID
}

func (legacy legacyMarketplace) adoptersScripts(t *testing.T) []entities.StrategyScript {
	strategyScripts := []entities.StrategyScript{}
	require.NoError(t, legacy.database.Preload("Parameters").
		Where(clause.Eq{Column: "owner_id", Value: legacyAdopterID}).Order("name").Find(&strategyScripts).Error)

	return strategyScripts
}

func (legacy legacyMarketplace) migrate(t *testing.T) {
	_, migrateError := persistence.NewSchemaMigrator(legacy.database).Migrate()
	require.NoError(t, migrateError)
}

func TestMigrateTurnsAnOldAdoptionIntoACopyAndDropsTheAdoptions(t *testing.T) {
	legacy := newLegacyMarketplace(t)
	legacy.adopted(t)

	legacy.migrate(t)

	copies := legacy.adoptersScripts(t)
	require.Len(t, copies, 1)
	assert.Equal(t, "動能", copies[0].Name)
	assert.True(t, copies[0].IsAdoptedFromMarketplace)
	assert.Equal(t, strategyScriptNamed("動能").Script, copies[0].Script)
	require.Len(t, copies[0].Parameters, 1)
	assert.Equal(t, "lookback", copies[0].Parameters[0].Name)
	assert.False(t, legacy.database.Migrator().HasTable(&legacyAdoptionRow{}))
}

func TestMigratePointsASignalSourceNamingSomeoneElsesScriptAtTheOwnersCopy(t *testing.T) {
	legacy := newLegacyMarketplace(t)
	signalSourceID := legacy.followedByAStrategy(t)

	legacy.migrate(t)

	copies := legacy.adoptersScripts(t)
	require.Len(t, copies, 1, "沒加入但直接指名的也替他建一份")
	assert.Equal(t, copies[0].ID, legacy.sourceScriptOf(t, signalSourceID))
	original := entities.StrategyScript{}
	require.NoError(t, legacy.database.First(&original, legacy.originalScriptID).Error)
	assert.Equal(t, original.Script, copies[0].Script, "機器人跑的算式與更新前相同")
}

func TestMigrateLeavesASourceWhoseOriginalIsGoneOrWithdrawn(t *testing.T) {
	t.Run("deleted", func(t *testing.T) {
		legacy := newLegacyMarketplace(t)
		signalSourceID := legacy.followedBy(t, 999999)

		legacy.migrate(t)

		assert.Empty(t, legacy.adoptersScripts(t))
		assert.Equal(t, uint(999999), legacy.sourceScriptOf(t, signalSourceID))
	})

	t.Run("withdrawn", func(t *testing.T) {
		// The author took it off the marketplace; handing out a copy now would undo that.
		legacy := newLegacyMarketplace(t)
		require.NoError(t, persistence.NewPublishedStrategyScriptRepository(legacy.database).
			Withdraw(t.Context(), legacy.originalScriptID))
		signalSourceID := legacy.followedBy(t, legacy.originalScriptID)

		legacy.migrate(t)

		assert.Empty(t, legacy.adoptersScripts(t))
		assert.Equal(t, legacy.originalScriptID, legacy.sourceScriptOf(t, signalSourceID))
	})
}

func TestMigrateMakesOneCopyForAnAdoptionAndASourceOfTheSameScript(t *testing.T) {
	legacy := newLegacyMarketplace(t)
	legacy.adopted(t)
	signalSourceID := legacy.followedByAStrategy(t)

	legacy.migrate(t)

	copies := legacy.adoptersScripts(t)
	require.Len(t, copies, 1)
	signalSource := entities.TradingStrategySignalSource{}
	require.NoError(t, legacy.database.First(&signalSource, signalSourceID).Error)
	assert.Equal(t, copies[0].ID, signalSource.StrategyScriptID)
}

func TestMigrateNamesACopySoItClashesWithNoneOfTheOwnersScripts(t *testing.T) {
	testCases := []struct {
		name          string
		ownNames      []string
		expectedNames []string
	}{
		{name: "a clash is marked", ownNames: []string{"動能"}, expectedNames: []string{"動能", "動能（市集）"}},
		{
			name: "a clash with the mark is numbered", ownNames: []string{"動能", "動能（市集）"},
			expectedNames: []string{"動能", "動能（市集 2）", "動能（市集）"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			legacy := newLegacyMarketplace(t)
			for _, ownName := range testCase.ownNames {
				own := strategyScriptNamed(ownName)
				own.OwnerID = legacyAdopterID
				require.NoError(t, legacy.database.Create(&own).Error)
			}
			legacy.adopted(t)

			legacy.migrate(t)

			names := []string{}
			for _, strategyScript := range legacy.adoptersScripts(t) {
				names = append(names, strategyScript.Name)
			}
			assert.ElementsMatch(t, testCase.expectedNames, names)
		})
	}
}

func TestMigrateAgainMovesNothingMore(t *testing.T) {
	legacy := newLegacyMarketplace(t)
	legacy.adopted(t)
	signalSourceID := legacy.followedByAStrategy(t)
	legacy.migrate(t)

	legacy.migrate(t)

	copies := legacy.adoptersScripts(t)
	require.Len(t, copies, 1)
	signalSource := entities.TradingStrategySignalSource{}
	require.NoError(t, legacy.database.First(&signalSource, signalSourceID).Error)
	assert.Equal(t, copies[0].ID, signalSource.StrategyScriptID)
}

func TestMigrateLeavesASourceNamingItsOwnersOwnScriptAlone(t *testing.T) {
	legacy := newLegacyMarketplace(t)
	own := strategyScriptNamed("均線")
	own.OwnerID = legacyAdopterID
	require.NoError(t, legacy.database.Create(&own).Error)
	tradingStrategy := entities.TradingStrategy{
		OwnerID: legacyAdopterID, Name: "自己的",
		SignalSources: []entities.TradingStrategySignalSource{{Label: "A", StrategyScriptID: own.ID, AggregationInterval: "1h"}},
	}
	require.NoError(t, legacy.database.Create(&tradingStrategy).Error)

	legacy.migrate(t)

	assert.Len(t, legacy.adoptersScripts(t), 1)
	signalSource := entities.TradingStrategySignalSource{}
	require.NoError(t, legacy.database.First(&signalSource, tradingStrategy.SignalSources[0].ID).Error)
	assert.Equal(t, own.ID, signalSource.StrategyScriptID)
}

func TestMigrateRunsOnceSoALaterRepublishHandsOutNothing(t *testing.T) {
	// Left alone because it was withdrawn; republishing afterwards must not copy it to someone who never adopted it.
	legacy := newLegacyMarketplace(t)
	publications := persistence.NewPublishedStrategyScriptRepository(legacy.database)
	require.NoError(t, publications.Withdraw(t.Context(), legacy.originalScriptID))
	signalSourceID := legacy.followedBy(t, legacy.originalScriptID)
	legacy.migrate(t)

	require.NoError(t, publications.Publish(t.Context(), legacy.originalScriptID, time.Now()))
	legacy.migrate(t)

	assert.Empty(t, legacy.adoptersScripts(t))
	assert.Equal(t, legacy.originalScriptID, legacy.sourceScriptOf(t, signalSourceID))
}
