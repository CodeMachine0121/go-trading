package vo

// MessageDeliveryCredentialVo is what a delivery channel needs to send one message:
// who to speak as, and where to speak.
//
// It is the only type in this system that holds a usable bot token, and it exists
// for one hop — the domain builds it, infrastructure spends it, and nothing converts
// it into anything that travels back out. Keeping that hop inside a named type is
// what makes it possible to say where a whole token may appear: here, and nowhere
// else.
type MessageDeliveryCredentialVo struct {
	// BotToken is the token in the form it is used in, opened moments before it is
	// spent.
	BotToken string
	// ChatID is where the message goes.
	ChatID string
}
