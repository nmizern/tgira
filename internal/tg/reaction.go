package tg

import (
	"context"
	"errors"

	"github.com/nmizern/tgira/internal/domain"
	"github.com/nmizern/tgira/internal/i18n"
	"github.com/nmizern/tgira/internal/store"
	tele "gopkg.in/telebot.v4"
)

// Telegram allows a fixed set of reaction emoji, and neither a check mark nor
// a cross is in it, so these are the closest gestures that exist.
var reactionStatus = map[string]domain.Status{
	"👀": domain.StatusDoing,
	"👍": domain.StatusDone,
	"🎉": domain.StatusDone,
	"💯": domain.StatusDone,
	"👎": domain.StatusCancelled,
}

func (b *Bot) onReaction(r *tele.MessageReaction) error {
	// Anonymous admins react on behalf of a chat, which nobody owns.
	if r.User == nil || r.Chat == nil {
		return nil
	}

	ctx := b.ctx
	board, task, err := b.taskByCard(ctx, r.Chat.ID, int64(r.MessageID))
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	added, removed := reactionDelta(r.OldReaction, r.NewReaction)

	if to, ok := statusFor(added); ok {
		return b.reactionSets(ctx, board, task, r.User, to)
	}
	if to, ok := statusFor(removed); ok {
		return b.reactionUndoes(ctx, board, task, r.User, to)
	}
	return nil
}

func (b *Bot) reactionSets(ctx context.Context, board domain.Board, task domain.Task, user *tele.User, to domain.Status) error {
	if err := b.store.UpsertUser(ctx, userOf(user)); err != nil {
		return err
	}
	// Adding the account may have just handed this task to them.
	task, err := b.store.Task(ctx, task.ID)
	if err != nil {
		return err
	}

	if !domain.CanChangeStatus(task, user.ID) {
		// A reaction set by somebody else cannot be removed by the bot, so the
		// only way to explain the refusal is to say it out loud.
		b.notify(board, b.texts.T(i18n.KeyNotAllowed, task.Key(board.Code)))
		return nil
	}

	_, err = b.applyStatus(ctx, board, task, user.ID, to)
	return err
}

func (b *Bot) reactionUndoes(ctx context.Context, board domain.Board, task domain.Task, user *tele.User, to domain.Status) error {
	// Only the move this very reaction caused can be taken back.
	if task.Status != to {
		return nil
	}

	event, err := b.store.LastStatusEvent(ctx, task.ID, user.ID, to)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !event.From.Valid() {
		return nil
	}

	_, err = b.applyStatus(ctx, board, task, user.ID, event.From)
	return err
}

// taskByCard finds which board's card a message is, given only the chat.
// Reaction updates carry no thread id.
func (b *Bot) taskByCard(ctx context.Context, chatID, msgID int64) (domain.Board, domain.Task, error) {
	for _, board := range b.byID {
		if board.ChatID != chatID {
			continue
		}

		task, err := b.store.TaskByCard(ctx, board.ID, msgID)
		if err == nil {
			return board, task, nil
		}
		if !errors.Is(err, store.ErrNotFound) {
			return domain.Board{}, domain.Task{}, err
		}
	}
	return domain.Board{}, domain.Task{}, store.ErrNotFound
}

func reactionDelta(before, after []tele.Reaction) (added, removed []string) {
	was, is := emojiSet(before), emojiSet(after)

	for emoji := range is {
		if !was[emoji] {
			added = append(added, emoji)
		}
	}
	for emoji := range was {
		if !is[emoji] {
			removed = append(removed, emoji)
		}
	}
	return added, removed
}

func emojiSet(list []tele.Reaction) map[string]bool {
	out := make(map[string]bool, len(list))
	for _, r := range list {
		if r.Type == "emoji" && r.Emoji != "" {
			out[r.Emoji] = true
		}
	}
	return out
}

func statusFor(emojis []string) (domain.Status, bool) {
	for _, emoji := range emojis {
		if status, ok := reactionStatus[emoji]; ok {
			return status, true
		}
	}
	return "", false
}
