package vo

// AssistantQueryDeclarationVo describes one capability; ArgumentSchema stays raw text because the domain only passes it through.
type AssistantQueryDeclarationVo struct {
	Name           string
	Description    string
	ArgumentSchema string
}
