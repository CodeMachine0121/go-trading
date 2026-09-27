package dto

type TradeTagDto struct {
	ID   uint   `json:"id"`
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type TradeTagWriteDto struct {
	Kind string
	Name string
}
