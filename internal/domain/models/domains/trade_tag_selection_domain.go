package domains

import "github.com/CodeMachine0121/go-trading/internal/domain/models/entities"

// TradeTagSelectionDomain picks the tags a person asked for out of what storage found, answering not found for any that is missing or somebody else's.
type TradeTagSelectionDomain struct {
	tagsByID map[uint]entities.TradeTag
}

func NewTradeTagSelectionDomain(foundTags []entities.TradeTag, ownerID uint) TradeTagSelectionDomain {
	tagsByID := map[uint]entities.TradeTag{}
	for _, tag := range foundTags {
		if tag.OwnerID == ownerID {
			tagsByID[tag.ID] = tag
		}
	}

	return TradeTagSelectionDomain{tagsByID: tagsByID}
}

// Select keeps the order asked for, so the tags read back the way the person listed them.
func (selectionDomain TradeTagSelectionDomain) Select(tagIDs []uint) ([]entities.TradeTag, error) {
	selectedTags := make([]entities.TradeTag, 0, len(tagIDs))
	for _, tagID := range tagIDs {
		tag, isOwned := selectionDomain.tagsByID[tagID]
		if !isOwned {
			return nil, TradeTagNotFound(tagID)
		}
		selectedTags = append(selectedTags, tag)
	}

	return selectedTags, nil
}
