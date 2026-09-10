package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunSubjectDomainTakesOneOrTheOther(t *testing.T) {
	testCases := []struct {
		name       string
		strategyID uint
		ownScript  string
		refused    bool
	}{
		{name: "naming a saved strategy", strategyID: 7, ownScript: "", refused: false},
		{name: "carrying an algorithm nobody saved", strategyID: 0, ownScript: "func Calculate() {}", refused: false},
		// Both at once can disagree about what actually ran, and picking a winner
		// makes the loser vanish without a word.
		{name: "both at once", strategyID: 7, ownScript: "func Calculate() {}", refused: true},
		{name: "neither", strategyID: 0, ownScript: "", refused: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, subjectError := domains.NewRunSubjectDomain(
				testCase.strategyID, testCase.ownScript, "", nil)

			if testCase.refused {
				require.ErrorIs(t, subjectError, domains.ErrRunSubjectAmbiguous)
				return
			}

			require.NoError(t, subjectError)
		})
	}
}

func TestRunSubjectDomainSaysWhichStrategyWasNamed(t *testing.T) {
	runSubjectDomain, subjectError := domains.NewRunSubjectDomain(7, "", "", nil)
	require.NoError(t, subjectError)

	strategyID, namesAStrategy := runSubjectDomain.NamedStrategyID()

	assert.True(t, namesAStrategy)
	assert.Equal(t, uint(7), strategyID)
}

func TestRunSubjectDomainHandsOverAnUnsavedAlgorithmToRun(t *testing.T) {
	// The kind of value and the knobs come from the caller, because there is no
	// strategy to have declared them.
	runSubjectDomain, subjectError := domains.NewRunSubjectDomain(
		0, "func Calculate() {}", "floatList",
		[]dto.StrategyParameterWriteDto{{Name: "期數", Kind: "lookbackCount", DefaultValue: 20}})
	require.NoError(t, subjectError)

	_, namesAStrategy := runSubjectDomain.NamedStrategyID()
	runnableStrategyDto := runSubjectDomain.ToRunnableDto()

	assert.False(t, namesAStrategy)
	assert.Equal(t, "func Calculate() {}", runnableStrategyDto.Script)
	assert.Equal(t, "floatList", runnableStrategyDto.ResultType)
	require.Len(t, runnableStrategyDto.Parameters, 1)
	assert.Equal(t, "期數", runnableStrategyDto.Parameters[0].Name)
}
