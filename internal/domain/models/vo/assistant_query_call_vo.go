package vo

// AssistantQueryCallVo is the assistant asking for one capability to be run:
// immutable plain data, no behavior.
//
// CallID is the assistant's own handle for this request. It is carried back
// untouched with the result, because an assistant that made several requests at once
// has no other way to tell which answer belongs to which.
type AssistantQueryCallVo struct {
	CallID    string
	Name      string
	Arguments string
}
