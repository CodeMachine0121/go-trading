package domains

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

const tradeTagNameMaximumLength = 64

type TradeTagDomain struct {
	kind vo.TradeTagKindVo
	name string
}

func NewTradeTagDomain(writeDto dto.TradeTagWriteDto) (TradeTagDomain, error) {
	kind := vo.TradeTagKindVo(strings.TrimSpace(writeDto.Kind))
	if kind != vo.TradeTagKindMistake && kind != vo.TradeTagKindSetup {
		return TradeTagDomain{}, fmt.Errorf(
			"%w: 標籤種類只有失誤（mistake）與型態（setup）", ErrTradeTagValidation)
	}

	name := strings.TrimSpace(writeDto.Name)
	if name == "" {
		return TradeTagDomain{}, fmt.Errorf("%w: 標籤名稱不得為空白", ErrTradeTagValidation)
	}
	if utf8.RuneCountInString(name) > tradeTagNameMaximumLength {
		return TradeTagDomain{}, fmt.Errorf(
			"%w: 標籤名稱不得超過 %d 個字", ErrTradeTagValidation, tradeTagNameMaximumLength)
	}

	return TradeTagDomain{kind: kind, name: name}, nil
}

func (tradeTagDomain TradeTagDomain) Name() string {
	return tradeTagDomain.name
}

func (tradeTagDomain TradeTagDomain) ToEntity(ownerID uint) entities.TradeTag {
	return entities.TradeTag{OwnerID: ownerID, Kind: string(tradeTagDomain.kind), Name: tradeTagDomain.name}
}
