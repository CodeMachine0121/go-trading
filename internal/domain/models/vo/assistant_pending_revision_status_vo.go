package vo

// AssistantPendingRevisionStatusVo is where a proposed rewrite stands; only a pending one can be confirmed or rejected.
type AssistantPendingRevisionStatusVo string

const (
	AssistantPendingRevisionPending   AssistantPendingRevisionStatusVo = "pending"
	AssistantPendingRevisionConfirmed AssistantPendingRevisionStatusVo = "confirmed"
	AssistantPendingRevisionRejected  AssistantPendingRevisionStatusVo = "rejected"
)
