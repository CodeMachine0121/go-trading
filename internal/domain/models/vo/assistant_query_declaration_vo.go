package vo

// AssistantQueryDeclarationVo describes one capability to the assistant: immutable
// plain data, no behavior.
//
// ArgumentSchema is carried as the text of a schema rather than as a parsed
// structure, and that is deliberate. Nothing inside the domain reads it — it is
// handed to the assistant untouched — so parsing it here would buy nothing and would
// force a loosely typed map through the whole domain to describe something the domain
// has no opinion about.
type AssistantQueryDeclarationVo struct {
	Name           string
	Description    string
	ArgumentSchema string
}
