package dto

import "time"

// TelegramDeliveryDto is a person's Telegram delivery setting as it is handed back.
//
// There is no field for the bot token, and that absence is the design rather than an
// omission. A route that can return the token is a route that can leak it, and the
// only way to be certain no such route is ever written is for the shape every route
// answers with to have nowhere to put one. What comes back is enough to recognise
// which token is stored and nothing like enough to use it.
//
// Configured says whether there is a setting at all. Having never set one up is an
// ordinary state, not a failure, so it is answered as a value here rather than as an
// error somebody has to catch.
type TelegramDeliveryDto struct {
	Configured   bool      `json:"configured"`
	ChatID       string    `json:"chatId"`
	BotTokenTail string    `json:"botTokenTail"`
	ConfiguredAt time.Time `json:"configuredAt"`
}
