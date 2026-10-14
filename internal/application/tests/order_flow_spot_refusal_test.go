package application_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const spotRefusesOrderFlow = "訂單流價位寬度只提供給合約行情種類的策略腳本"

// carryingWithOrderFlow is an unsaved algorithm that declares an order flow price step.
func carryingWithOrderFlow(t *testing.T) domains.RunSubjectDomain {
	t.Helper()

	runSubjectDomain, subjectError := domains.NewRunSubjectDomain(0, "the script", "", []dto.StrategyScriptParameterWriteDto{
		{Name: "寬", Kind: "orderFlowPriceStep", DefaultValue: 10},
	})
	require.NoError(t, subjectError)

	return runSubjectDomain
}

func TestASpotCalculationOfAnAlgorithmDeclaringOrderFlowIsRefusedBeforeReadingAnything(t *testing.T) {
	fixture := newIndicatorUnderTest(t)

	_, err := fixture.indicatorCalculationApplication.CalculateIndicator(
		t.Context(), indicatorViewerID, carryingWithOrderFlow(t), indicatorRequest(2))

	require.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
	assert.Contains(t, err.Error(), spotRefusesOrderFlow)
}

func TestASpotBacktestOfAnAlgorithmDeclaringOrderFlowIsRefusedBeforeItIsQueued(t *testing.T) {
	fixture := newBacktestUnderTest(t)

	_, err := fixture.backtestApplication.SubmitBacktest(
		t.Context(), backtestViewerID, carryingWithOrderFlow(t), backtestRequestDto())

	require.ErrorIs(t, err, domains.ErrBacktestValidation)
	assert.Contains(t, err.Error(), spotRefusesOrderFlow)
}

func TestASpotReplayDeclaringOrderFlowIsRefusedBeforeReadingAnything(t *testing.T) {
	fixture := newBacktestUnderTest(t)
	requestDto := replayRequestDto()
	requestDto.Parameters = []dto.StrategyScriptParameterWriteDto{{Name: "寬", Kind: "orderFlowPriceStep", DefaultValue: 10}}

	_, err := fixture.backtestService.RunBacktest(t.Context(), requestDto, nil)

	require.ErrorIs(t, err, domains.ErrBacktestValidation)
	assert.Contains(t, err.Error(), spotRefusesOrderFlow)
}
