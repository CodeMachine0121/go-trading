package dto

import (
	"encoding/json"
	"time"
)

// AssistantPendingRevisionDto is a proposed rewrite as its owner reviews it.
type AssistantPendingRevisionDto struct {
	ID          uint   `json:"id"`
	SubjectKind string `json:"subjectKind"`
	SubjectID   uint   `json:"subjectId"`
	SubjectName string `json:"subjectName"`
	// Content is the whole rewrite as the assistant sent it, in the same shape its capability takes.
	Content    json.RawMessage `json:"content"`
	Status     string          `json:"status"`
	ProposedAt time.Time       `json:"proposedAt"`
	// SubjectUpdatedAt is kept off the response; it only decides whether the proposal went stale.
	SubjectUpdatedAt time.Time `json:"-"`
}
