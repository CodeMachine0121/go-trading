package vo

// MessageDeliveryCredentialVo is the only type holding a usable bot token; it lives for one hop from domain to infrastructure and never travels back out.
type MessageDeliveryCredentialVo struct {
	// BotToken is unsealed moments before it is used.
	BotToken string
	ChatID   string
}
