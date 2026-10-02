package dto

// JobLeadershipChangeDto says whether one renewal changed this replica's duty, so only transitions get logged.
type JobLeadershipChangeDto struct {
	Gained bool
	Lost   bool
}
