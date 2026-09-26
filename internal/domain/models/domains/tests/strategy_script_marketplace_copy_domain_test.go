package domains_test

import (
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/stretchr/testify/assert"
)

func TestStrategyScriptMarketplaceCopyDomainNamesACopySoItClashesWithNothing(t *testing.T) {
	testCases := []struct {
		name         string
		takenNames   []string
		expectedName string
	}{
		{name: "a free name is kept", takenNames: []string{"均線"}, expectedName: "動能"},
		{name: "a taken name is marked", takenNames: []string{"動能"}, expectedName: "動能（市集）"},
		{name: "a taken mark is numbered", takenNames: []string{"動能", "動能（市集）"}, expectedName: "動能（市集 2）"},
		{name: "numbering goes on", takenNames: []string{"動能", "動能（市集）", "動能（市集 2）"}, expectedName: "動能（市集 3）"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			takenNames := map[string]bool{}
			for _, takenName := range testCase.takenNames {
				takenNames[takenName] = true
			}

			copied := domains.NewStrategyScriptMarketplaceCopyDomain(
				entities.StrategyScript{Name: "動能"}, 2, time.Now()).NamedAvoiding(takenNames).ToEntity()

			assert.Equal(t, testCase.expectedName, copied.Name)
		})
	}
}

func TestStrategyScriptAccessDomainTreatsAMarketplaceCopyAsItsAuthorsWords(t *testing.T) {
	testCases := []struct {
		name                     string
		isAdoptedFromMarketplace bool
		expectedAuthoredByViewer bool
	}{
		{name: "the owner's own work is theirs", isAdoptedFromMarketplace: false, expectedAuthoredByViewer: true},
		{name: "a copy the owner adopted is not", isAdoptedFromMarketplace: true, expectedAuthoredByViewer: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			strategyScript := entities.StrategyScript{ID: 8, OwnerID: 1, IsAdoptedFromMarketplace: testCase.isAdoptedFromMarketplace}

			runnable, runnableError := domains.NewStrategyScriptAccessDomain(strategyScript, 1, false).ToRunnableDto()

			assert.NoError(t, runnableError)
			assert.Equal(t, testCase.expectedAuthoredByViewer, runnable.AuthoredByViewer)
		})
	}
}

func TestStrategyScriptToDtoNeverCarriesACopysAlgorithm(t *testing.T) {
	copied := entities.StrategyScript{ID: 8, Name: "動能", Script: "func Calculate() {}", IsAdoptedFromMarketplace: true}
	own := entities.StrategyScript{ID: 9, Name: "均線", Script: "func Calculate() {}"}

	assert.Empty(t, copied.ToDto().Script)
	assert.True(t, copied.ToDto().IsAdoptedFromMarketplace)
	assert.Equal(t, "func Calculate() {}", own.ToDto().Script)
	assert.False(t, own.ToDto().IsAdoptedFromMarketplace)
}

func TestStrategyScriptMarketplaceCopyDomainShortensALongNameSoTheMarkFits(t *testing.T) {
	longName := strings.Repeat("長", 128)

	copied := domains.NewStrategyScriptMarketplaceCopyDomain(
		entities.StrategyScript{Name: longName}, 2, time.Now()).
		NamedAvoiding(map[string]bool{longName: true}).ToEntity()

	assert.Equal(t, 128, len([]rune(copied.Name)))
	assert.True(t, strings.HasSuffix(copied.Name, "（市集）"))
}
