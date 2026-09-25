package vo

// LiveStreamCapacityVo caps how many live streams may be open at once, per requester and across the service.
type LiveStreamCapacityVo struct {
	PerRequester int
	Total        int
}
