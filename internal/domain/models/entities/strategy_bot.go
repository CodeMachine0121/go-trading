package entities

import (
	"cmp"
	"slices"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// StrategyBot is one standing bot: a few strategies' signals, two conditions that
// turn them into one conclusion, and how often to ask.
//
// It is a plain data model: fields, persistence mapping and shape conversion only,
// no business rules.
//
// The name carries a unique index spanning the owner, for the reason a strategy's
// does: a name is what its owner recognises a bot by, nobody recognises a
// stranger's, and an index — not a read-then-write check — is what actually makes it
// unique.
//
// RunState, NextRunAt, LastSentSignal, HaltReason and Conflicting are stored rather
// than held in memory. That is the difference between a bot that survives a restart
// and one that quietly stops the next time the process comes back up, with its owner
// still believing it is watching.
type StrategyBot struct {
	ID uint `gorm:"primaryKey"`
	// OwnerID is settled when the bot is created and never changes: it is not on
	// the list of columns a rewrite may touch, so "a bot cannot change hands" is
	// something the write path cannot express rather than something it remembers
	// not to do.
	OwnerID uint   `gorm:"not null;index:idx_strategy_bots_owner;uniqueIndex:idx_strategy_bots_owner_name"`
	Name    string `gorm:"size:128;not null;uniqueIndex:idx_strategy_bots_owner_name"`
	Symbol  string `gorm:"size:64;not null"`
	// TriggerIntervalMinutes is counted in minutes because that is the unit a
	// person thinks in here, and because the finest candle is one minute — asking
	// more often than that only fetches the same candle again.
	TriggerIntervalMinutes int `gorm:"not null"`
	// RunState is indexed together with NextRunAt because the scan asks exactly one
	// question of this table — which bots are running and due — and that pair is it.
	RunState string `gorm:"size:16;not null;index:idx_strategy_bots_run_state_next_run_at,priority:1"`
	// NextRunAt is when this bot is next entitled to a round. Starting sets it to
	// now, so pressing play acts immediately instead of after a whole interval; a
	// finished round sets it to now plus the interval, which is also why missed
	// rounds are never made up — the next one is measured from the round that
	// actually happened, not from the one that should have.
	NextRunAt time.Time `gorm:"type:timestamptz;index:idx_strategy_bots_run_state_next_run_at,priority:2"`
	// LastSentSignal is the last signal that reached Telegram. Cleared on start, so
	// the first conclusion after pressing play always goes out: somebody who
	// pressed play and heard nothing all night cannot tell a quiet market from a
	// broken bot.
	LastSentSignal string `gorm:"size:16;not null;default:''"`
	// HaltReason is empty unless the system stopped this bot itself.
	HaltReason string `gorm:"size:32;not null;default:''"`
	// Conflicting records that the last round found both conditions holding. It is
	// not a halt and it clears itself on the next round that does not conflict.
	Conflicting bool      `gorm:"not null;default:false"`
	CreatedAt   time.Time `gorm:"type:timestamptz;not null"`
	UpdatedAt   time.Time `gorm:"type:timestamptz;not null"`

	Owner User `gorm:"foreignKey:OwnerID;constraint:OnDelete:CASCADE"`
	// SignalSources and ConditionNodes belong to this bot and to nothing else: they
	// are never read, created or deleted on their own, which is why they have no
	// repository of their own and travel with the bot that owns them. Deleting a
	// bot takes them with it through these cascades — no Go code performs that,
	// which is why no Go code can forget to.
	SignalSources  []StrategyBotSignalSource  `gorm:"foreignKey:StrategyBotID;constraint:OnDelete:CASCADE"`
	ConditionNodes []StrategyBotConditionNode `gorm:"foreignKey:StrategyBotID;constraint:OnDelete:CASCADE"`
	// RunRecords is declared for the cascade and for nothing else: a history is read
	// on its own, through its own repository, and never travels with the bot. What
	// it buys here is that deleting a bot takes its rounds with it — otherwise they
	// stay in the table for ever, belonging to something nobody can reach.
	RunRecords []StrategyBotRunRecord `gorm:"foreignKey:StrategyBotID;constraint:OnDelete:CASCADE"`
}

// TableName pins the table to StrategyBots instead of GORM's default.
func (strategyBot StrategyBot) TableName() string {
	return "StrategyBots"
}

// ToDto converts this record into the shape the domain hands outwards, nesting the
// two condition trees back out of the flat rows they are stored as.
//
// This is the one place the tree is reassembled. Handing out the flat rows would
// push that job onto every reader — including the screen somebody builds conditions
// on — and two reassemblers eventually disagree about the same stored rows.
func (strategyBot StrategyBot) ToDto() dto.StrategyBotDto {
	return dto.StrategyBotDto{
		ID:                     strategyBot.ID,
		OwnerID:                strategyBot.OwnerID,
		Name:                   strategyBot.Name,
		Symbol:                 strategyBot.Symbol,
		TriggerIntervalMinutes: strategyBot.TriggerIntervalMinutes,
		NextRunAt:              strategyBot.NextRunAt.UTC(),
		SignalSources:          strategyBot.signalSourceDtos(),
		BuyCondition:           strategyBot.conditionDto(vo.StrategyBotConditionSideBuy),
		SellCondition:          strategyBot.conditionDto(vo.StrategyBotConditionSideSell),
		RunState:               strategyBot.RunState,
		LastSentSignal:         strategyBot.LastSentSignal,
		HaltReason:             strategyBot.HaltReason,
		Conflicting:            strategyBot.Conflicting,
		CreatedAt:              strategyBot.CreatedAt.UTC(),
		UpdatedAt:              strategyBot.UpdatedAt.UTC(),
	}
}

// signalSourceDtos hands out this bot's sources, always as a list rather than
// sometimes nothing.
func (strategyBot StrategyBot) signalSourceDtos() []dto.StrategyBotSignalSourceDto {
	signalSourceDtos := make([]dto.StrategyBotSignalSourceDto, 0, len(strategyBot.SignalSources))
	for _, signalSource := range strategyBot.SignalSources {
		signalSourceDtos = append(signalSourceDtos, signalSource.ToDto())
	}

	return signalSourceDtos
}

// conditionDto rebuilds one of the two trees from the flat rows.
//
// It indexes the children by parent once and then descends, rather than scanning the
// whole slice at every node: a tree of 32 nodes scanned per node is a thousand
// comparisons for something read on every list of every bot.
func (strategyBot StrategyBot) conditionDto(side vo.StrategyBotConditionSideVo) dto.StrategyBotConditionDto {
	childrenByParent := map[uint][]StrategyBotConditionNode{}
	root := StrategyBotConditionNode{}
	hasRoot := false

	for _, node := range strategyBot.ConditionNodes {
		if vo.StrategyBotConditionSideVo(node.Side) != side {
			continue
		}

		if node.ParentID == nil {
			root = node
			hasRoot = true

			continue
		}

		childrenByParent[*node.ParentID] = append(childrenByParent[*node.ParentID], node)
	}

	if !hasRoot {
		return dto.StrategyBotConditionDto{}
	}

	return root.toDto(childrenByParent)
}

// toDto turns this node and everything under it into the nested shape. It is on the
// node because it reads the node's own fields; the map of children is the only thing
// it cannot get from itself.
func (strategyBotConditionNode StrategyBotConditionNode) toDto(
	childrenByParent map[uint][]StrategyBotConditionNode,
) dto.StrategyBotConditionDto {
	if !strategyBotConditionNode.IsGroup() {
		return dto.StrategyBotConditionDto{
			SourceLabel: strategyBotConditionNode.SourceLabel,
			Signal:      strategyBotConditionNode.ExpectedSignal,
		}
	}

	// Siblings are ordered here rather than trusted to arrive ordered. The order
	// changes no answer — and and or do not care — but it is what somebody sees
	// when they open the condition again, and a tree that rearranges itself
	// between two reads looks like it was edited by somebody else.
	children := childrenByParent[strategyBotConditionNode.ID]
	slices.SortFunc(children, func(left StrategyBotConditionNode, right StrategyBotConditionNode) int {
		return cmp.Compare(left.Position, right.Position)
	})

	conditionDtos := make([]dto.StrategyBotConditionDto, 0, len(children))
	for _, child := range children {
		conditionDtos = append(conditionDtos, child.toDto(childrenByParent))
	}

	return dto.StrategyBotConditionDto{
		Operator:   strategyBotConditionNode.Operator,
		Conditions: conditionDtos,
	}
}
