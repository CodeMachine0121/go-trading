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
		name             string
		strategyScriptID uint
		ownScript        string
		refused          bool
	}{
		{name: "naming a saved strategy script", strategyScriptID: 7, ownScript: "", refused: false},
		{name: "carrying an algorithm nobody saved", strategyScriptID: 0, ownScript: "func Calculate() {}", refused: false},
		// Both at once can disagree about what actually ran, and picking a winner
		// makes the loser vanish without a word.
		{name: "both at once", strategyScriptID: 7, ownScript: "func Calculate() {}", refused: true},
		{name: "neither", strategyScriptID: 0, ownScript: "", refused: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, subjectError := domains.NewRunSubjectDomain(
				testCase.strategyScriptID, testCase.ownScript, "", nil)

			if testCase.refused {
				require.ErrorIs(t, subjectError, domains.ErrRunSubjectAmbiguous)
				return
			}

			require.NoError(t, subjectError)
		})
	}
}

func TestRunSubjectDomainSaysWhichStrategyScriptWasNamed(t *testing.T) {
	runSubjectDomain, subjectError := domains.NewRunSubjectDomain(7, "", "", nil)
	require.NoError(t, subjectError)

	strategyScriptID, namesAStrategyScript := runSubjectDomain.NamedStrategyScriptID()

	assert.True(t, namesAStrategyScript)
	assert.Equal(t, uint(7), strategyScriptID)
}

func TestRunSubjectDomainHandsOverAnUnsavedAlgorithmToRun(t *testing.T) {
	// The kind of value and the knobs come from the caller, because there is no
	// strategy script to have declared them.
	runSubjectDomain, subjectError := domains.NewRunSubjectDomain(
		0, "func Calculate() {}", "floatList",
		[]dto.StrategyScriptParameterWriteDto{{Name: "期數", Kind: "lookbackCount", DefaultValue: 20}})
	require.NoError(t, subjectError)

	_, namesAStrategyScript := runSubjectDomain.NamedStrategyScriptID()
	runnableStrategyScriptDto := runSubjectDomain.ToRunnableDto()

	assert.False(t, namesAStrategyScript)
	assert.Equal(t, "func Calculate() {}", runnableStrategyScriptDto.Script)
	assert.Equal(t, "floatList", runnableStrategyScriptDto.ResultType)
	require.Len(t, runnableStrategyScriptDto.Parameters, 1)
	assert.Equal(t, "期數", runnableStrategyScriptDto.Parameters[0].Name)
}
