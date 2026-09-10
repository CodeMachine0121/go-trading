package _interface

import "context"

//go:generate go tool mockgen -source=i_assistant_query.go -destination=mocks/mock_i_assistant_query.go -package=mocks

// IAssistantQuery is one thing the assistant is allowed to do.
//
// What the assistant can do is the set of these it is given and nothing else. That is
// why it cannot delete a strategy: no such implementation exists, so there is nothing
// to reach for and no guard clause anybody could remove by accident. Adding a
// capability is adding an implementation and registering it; nothing that orchestrates
// or talks to the assistant changes.
//
// Every implementation obeys the rules that already govern the use case underneath
// it, unrelaxed. A refusal from those rules is handed back to the assistant as the
// reason rather than raised as a failure: it asked for something it may not have,
// and asking differently is within its power.
type IAssistantQuery interface {
	// Name is how the assistant asks for this capability. It is unique among the
	// capabilities offered.
	Name() string
	// Description tells the assistant what this capability is for and when to reach
	// for it.
	Description() string
	// ArgumentSchema describes the arguments as the text of a schema. Nothing inside
	// the domain reads it — it is handed to the assistant untouched — so it stays
	// text rather than forcing a loosely typed structure through the domain to
	// describe something the domain has no opinion about.
	ArgumentSchema() string
	// Run carries this capability out on behalf of one person and hands back what
	// the assistant should read. An error is the reason it was refused, which the
	// assistant reads and may act on; it does not end the answer being written.
	//
	// Who it acts for is a parameter rather than something read off the context,
	// so that forgetting to pass it on is a compile error instead of a capability
	// quietly acting as nobody. Several capabilities have no use for it — the
	// market is not anybody's property — and they ignore it by name.
	Run(executionContext context.Context, viewerID uint, arguments string) (string, error)
}
