package dto

import "time"

// RewriteTargetDto is a strategy script or trading strategy as a rewrite of it would find it, checked but not written.
type RewriteTargetDto struct {
	ID        uint
	Name      string
	UpdatedAt time.Time
	// BotReferenceCount counts every bot using it, running or not.
	BotReferenceCount int
}
