package domains

import (
	"errors"
	"fmt"
)

var ErrTradeTagValidation = errors.New("trade tag validation failed")

// ErrTradeTagNotFound also covers tags owned by someone else.
var ErrTradeTagNotFound = errors.New("trade tag not found")

func TradeTagNotFound(id uint) error {
	return fmt.Errorf("%w: 找不到識別碼為 %d 的標籤", ErrTradeTagNotFound, id)
}

// ErrTradeTagNameConflict is scoped to one owner and one kind of tag.
var ErrTradeTagNameConflict = errors.New("trade tag name already in use")

// ErrTradeTagInUse keeps a tag that statistics still count from disappearing under them.
var ErrTradeTagInUse = errors.New("trade tag in use")

var ErrTradeJournalSettingValidation = errors.New("trade journal setting validation failed")
