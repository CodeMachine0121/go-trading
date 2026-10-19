package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/stretchr/testify/assert"
)

func TestSymbolScreeningCandidateVerdicts(t *testing.T) {
	historyDays := 400
	findings := dto.SymbolScreeningFindingsDto{HistoryDays: &historyDays}
	waiting := domains.NewSymbolScreeningCandidateDomain(entities.SymbolScreeningCandidate{Symbol: "LINKUSDT", Status: "waiting"})
	testCases := []struct {
		name           string
		verdict        domains.SymbolScreeningCandidateDomain
		expectedStatus string
		expectedStage  string
		expectedReason string
	}{
		{name: "being screened", verdict: waiting.Processing(), expectedStatus: "processing"},
		{name: "passed", verdict: waiting.Passed(findings), expectedStatus: "passed"},
		{name: "rejected at the basic condition", verdict: waiting.Rejected("basicCondition", "目前沒有在交易", findings),
			expectedStatus: "rejected", expectedStage: "basicCondition", expectedReason: "目前沒有在交易"},
		{name: "unfinished keeps the figures it had", verdict: waiting.Unfinished("行情取不到", findings),
			expectedStatus: "unfinished", expectedReason: "行情取不到"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			candidate := testCase.verdict.ToEntity()

			assert.Equal(t, testCase.expectedStatus, candidate.Status)
			assert.Equal(t, testCase.expectedStage, candidate.Stage)
			assert.Equal(t, testCase.expectedReason, candidate.Reason)
			assert.Equal(t, "LINKUSDT", candidate.Symbol)
		})
	}
	assert.Equal(t, &historyDays, waiting.Unfinished("行情取不到", findings).ToEntity().Findings.HistoryDays)
}
