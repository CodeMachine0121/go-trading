package dto

import "time"

type SymbolScreeningRunDto struct {
	ID                        uint       `json:"id"`
	ProfileID                 *uint      `json:"profileId"`
	ProfileName               string     `json:"profileName"`
	Status                    string     `json:"status"`
	ProcessedCandidateCount   int        `json:"processedCandidateCount"`
	CandidateCount            int        `json:"candidateCount"`
	ProgressPercent           int        `json:"progressPercent"`
	FailureReason             string     `json:"failureReason"`
	ContributionFailureReason string     `json:"contributionFailureReason"`
	SubmittedAt               time.Time  `json:"submittedAt"`
	StartedAt                 *time.Time `json:"startedAt"`
	FinishedAt                *time.Time `json:"finishedAt"`
}

// SymbolScreeningRunDetailDto lists candidates in recommendation order and carries the settings the run used.
type SymbolScreeningRunDetailDto struct {
	SymbolScreeningRunDto
	Snapshot   SymbolScreeningSnapshotDto    `json:"snapshot"`
	Candidates []SymbolScreeningCandidateDto `json:"candidates"`
}

type SymbolScreeningRunPageDto struct {
	Runs       []SymbolScreeningRunDto `json:"runs"`
	TotalCount int64                   `json:"totalCount"`
}

type SymbolScreeningRunListQueryDto struct {
	Status string
	Limit  int
}

type SymbolScreeningRunSubmissionDto struct {
	ProfileID  uint
	SymbolList string
}

type SymbolScreeningCandidateDto struct {
	Symbol             string                     `json:"symbol"`
	Position           int                        `json:"position"`
	Status             string                     `json:"status"`
	Stage              string                     `json:"stage"`
	Reason             string                     `json:"reason"`
	Findings           SymbolScreeningFindingsDto `json:"findings"`
	RecommendationRank *int                       `json:"recommendationRank"`
}
