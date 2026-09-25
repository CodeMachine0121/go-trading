package vo

// AssistantRevisionSubjectKindVo is what kind of thing an assistant's rewrite is about.
type AssistantRevisionSubjectKindVo string

const (
	AssistantRevisionSubjectStrategyScript  AssistantRevisionSubjectKindVo = "strategyScript"
	AssistantRevisionSubjectTradingStrategy AssistantRevisionSubjectKindVo = "tradingStrategy"
)
