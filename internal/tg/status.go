package tg

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/nmizern/tgira/internal/domain"
	"github.com/nmizern/tgira/internal/i18n"
	"github.com/nmizern/tgira/internal/store"
	tele "gopkg.in/telebot.v4"
)

func (b *Bot) onStatusButton(c tele.Context) error {
	cb := c.Callback()
	if cb == nil || cb.Sender == nil {
		return nil
	}

	taskID, to, ok := parseStatusData(cb.Data)
	if !ok {
		return c.Respond(&tele.CallbackResponse{Text: b.texts.T(i18n.KeyTaskGone), ShowAlert: true})
	}

	ctx := b.ctx
	task, err := b.store.Task(ctx, taskID)
	if errors.Is(err, store.ErrNotFound) {
		return c.Respond(&tele.CallbackResponse{Text: b.texts.T(i18n.KeyTaskGone), ShowAlert: true})
	}
	if err != nil {
		return err
	}

	board, ok := b.byID[task.BoardID]
	if !ok {
		return c.Respond(&tele.CallbackResponse{Text: b.texts.T(i18n.KeyTaskGone), ShowAlert: true})
	}

	if err := b.store.UpsertUser(ctx, userOf(cb.Sender)); err != nil {
		return err
	}
	// The task may have just been adopted by the person pressing the button.
	if task, err = b.store.Task(ctx, task.ID); err != nil {
		return err
	}

	if !domain.CanChangeStatus(task, cb.Sender.ID) {
		return c.Respond(&tele.CallbackResponse{
			Text:      b.texts.T(i18n.KeyNotAllowed, task.Key(board.Code)),
			ShowAlert: true,
		})
	}

	moved, err := b.applyStatus(ctx, board, task, cb.Sender.ID, to)
	if err != nil {
		return err
	}
	return c.Respond(&tele.CallbackResponse{Text: moved.Key(board.Code) + " · " + b.texts.Status(moved.Status)})
}

// applyStatus moves a task, claims it when nobody owns it yet, records the
// history and redraws the card. Buttons and reactions both come through here.
func (b *Bot) applyStatus(ctx context.Context, board domain.Board, task domain.Task, actorID int64, to domain.Status) (domain.Task, error) {
	if task.Status == to {
		return task, nil
	}

	at := b.now()

	// Starting work on a task nobody owns makes it yours.
	if to == domain.StatusDoing && !task.Assigned() {
		if err := b.store.UpdateAssignee(ctx, task.ID, actorID); err != nil {
			return task, err
		}
		if err := b.store.AddEvent(ctx, domain.Event{
			TaskID:    task.ID,
			ActorID:   actorID,
			Kind:      domain.EventAssign,
			Payload:   strconv.FormatInt(actorID, 10),
			CreatedAt: at,
		}); err != nil {
			return task, err
		}
		task.AssigneeID, task.AssigneeName = actorID, ""
	}

	if err := b.store.UpdateStatus(ctx, task.ID, to, at); err != nil {
		return task, err
	}
	if err := b.store.AddEvent(ctx, domain.Event{
		TaskID:    task.ID,
		ActorID:   actorID,
		Kind:      domain.EventStatus,
		From:      task.Status,
		To:        to,
		CreatedAt: at,
	}); err != nil {
		return task, err
	}
	task.Status = to

	if err := b.updateCard(ctx, board, task); err != nil {
		return task, fmt.Errorf("redraw card: %w", err)
	}
	return task, nil
}

// parseStatusData reads the "<task id>:<status>" payload of a status button.
func parseStatusData(data string) (int64, domain.Status, bool) {
	id, rest, found := strings.Cut(strings.TrimSpace(data), ":")
	if !found {
		return 0, "", false
	}

	taskID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return 0, "", false
	}

	status := domain.Status(rest)
	if !status.Valid() {
		return 0, "", false
	}
	return taskID, status, true
}
