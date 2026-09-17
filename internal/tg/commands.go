package tg

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/nmizern/tgira/internal/domain"
	"github.com/nmizern/tgira/internal/i18n"
	"github.com/nmizern/tgira/internal/parse"
	"github.com/nmizern/tgira/internal/render"
	"github.com/nmizern/tgira/internal/store"
	tele "gopkg.in/telebot.v4"
)

var openStatuses = []domain.Status{domain.StatusDoing, domain.StatusTodo}

func (b *Bot) registerCommands() {
	b.api.Handle("/whereami", b.cmdWhereAmI)
	b.api.Handle("/help", b.cmdHelp)
	b.api.Handle("/start", b.cmdHelp)
	b.api.Handle("/board", b.cmdBoard)
	b.api.Handle("/tasks", b.cmdTasks)
	b.api.Handle("/my", b.cmdMy)
	b.api.Handle("/show", b.cmdShow)
	b.api.Handle("/take", b.cmdTake)
	b.api.Handle("/assign", b.cmdAssign)
	b.api.Handle("/pri", b.cmdPriority)
	b.api.Handle("/edit", b.cmdEdit)
	b.api.Handle("/rm", b.cmdRemove)
}

// whereami answers anywhere, because its whole point is to help set the
// config up before any board exists.
func (b *Bot) cmdWhereAmI(c tele.Context) error {
	m := c.Message()
	return b.answer(m, render.Where(m.Chat.ID, m.ThreadID, b.texts))
}

func (b *Bot) cmdHelp(c tele.Context) error {
	return b.answer(c.Message(), b.texts.T(i18n.KeyHelp))
}

func (b *Bot) cmdBoard(c tele.Context) error {
	m := c.Message()
	board, ok := b.commandBoard(m)
	if !ok {
		return b.answer(m, b.texts.T(i18n.KeyNoBoardHere))
	}

	// Forget the old message so a fresh board is posted and pinned.
	if err := b.store.SetPin(b.ctx, board.ID, 0); err != nil {
		return err
	}
	b.pin(board.ID).text = ""
	return b.flushBoard(b.ctx, board.ID)
}

func (b *Bot) cmdTasks(c tele.Context) error {
	m := c.Message()
	board, ok := b.commandBoard(m)
	if !ok {
		return b.answer(m, b.texts.T(i18n.KeyNoBoardHere))
	}

	filter, err := b.listFilter(b.ctx, board, strings.TrimSpace(m.Payload))
	if err != nil {
		return b.answer(m, b.texts.T(i18n.KeyBadArguments, "/tasks [todo|doing|done|cancelled|all|@name|#tag]"))
	}

	tasks, err := b.store.Tasks(b.ctx, filter)
	if err != nil {
		return err
	}
	return b.answer(m, render.List(board, tasks, b.people(b.ctx), b.texts))
}

func (b *Bot) cmdMy(c tele.Context) error {
	m := c.Message()
	board, ok := b.commandBoard(m)
	if !ok {
		return b.answer(m, b.texts.T(i18n.KeyNoBoardHere))
	}

	tasks, err := b.store.Tasks(b.ctx, store.Filter{
		BoardID:    board.ID,
		Statuses:   openStatuses,
		AssigneeID: m.Sender.ID,
	})
	if err != nil {
		return err
	}
	return b.answer(m, render.List(board, tasks, b.people(b.ctx), b.texts))
}

func (b *Bot) cmdShow(c tele.Context) error {
	m := c.Message()
	board, task, err := b.resolve(m, m.Payload)
	if err != nil {
		return b.reportRef(m, err)
	}

	events, err := b.store.Events(b.ctx, task.ID)
	if err != nil {
		return err
	}
	return b.answer(m, render.Show(board, task, events, b.people(b.ctx), b.texts))
}

func (b *Bot) cmdTake(c tele.Context) error {
	m := c.Message()
	board, task, err := b.resolve(m, m.Payload)
	if err != nil {
		return b.reportRef(m, err)
	}
	if !domain.CanChangeStatus(task, m.Sender.ID) {
		return b.answer(m, b.texts.T(i18n.KeyNotAllowed, task.Key(board.Code)))
	}

	if err := b.store.UpsertUser(b.ctx, userOf(m.Sender)); err != nil {
		return err
	}
	if err := b.assign(b.ctx, task, m.Sender.ID, ""); err != nil {
		return err
	}
	task.AssigneeID, task.AssigneeName = m.Sender.ID, ""

	moved, err := b.applyStatus(b.ctx, board, task, m.Sender.ID, domain.StatusDoing)
	if err != nil {
		return err
	}
	if moved.Status != domain.StatusDoing {
		// already in progress, but the card still has to show the new owner
		if err := b.updateCard(b.ctx, board, task); err != nil {
			return err
		}
		b.touchBoard(board.ID)
	}
	return b.answer(m, task.Key(board.Code)+" · "+b.texts.Status(domain.StatusDoing))
}

func (b *Bot) cmdAssign(c tele.Context) error {
	m := c.Message()
	ref, rest, _ := strings.Cut(strings.TrimSpace(m.Payload), " ")
	handle := strings.TrimPrefix(strings.TrimSpace(rest), "@")
	if handle == "" {
		return b.answer(m, b.texts.T(i18n.KeyBadArguments, "/assign TG-1 @name"))
	}

	board, task, err := b.resolve(m, ref)
	if err != nil {
		return b.reportRef(m, err)
	}
	if !domain.CanEdit(task, m.Sender.ID) {
		return b.answer(m, b.texts.T(i18n.KeyNotAllowed, task.Key(board.Code)))
	}

	var assigneeID int64
	if u, err := b.store.UserByUsername(b.ctx, handle); err == nil {
		assigneeID = u.ID
		handle = ""
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}

	if err := b.assign(b.ctx, task, assigneeID, handle); err != nil {
		return err
	}
	task.AssigneeID, task.AssigneeName = assigneeID, handle

	if err := b.updateCard(b.ctx, board, task); err != nil {
		return err
	}
	b.touchBoard(board.ID)
	return b.answer(m, task.Key(board.Code)+" · "+render.AssigneeOf(task, b.people(b.ctx)))
}

func (b *Bot) cmdPriority(c tele.Context) error {
	m := c.Message()
	ref, rest, _ := strings.Cut(strings.TrimSpace(m.Payload), " ")

	priority, ok := parsePriority(strings.TrimSpace(rest))
	if !ok {
		return b.answer(m, b.texts.T(i18n.KeyBadArguments, "/pri TG-1 1|2|3|-"))
	}

	board, task, err := b.resolve(m, ref)
	if err != nil {
		return b.reportRef(m, err)
	}
	if !domain.CanEdit(task, m.Sender.ID) {
		return b.answer(m, b.texts.T(i18n.KeyNotAllowed, task.Key(board.Code)))
	}

	if err := b.store.UpdatePriority(b.ctx, task.ID, priority); err != nil {
		return err
	}
	if err := b.store.AddEvent(b.ctx, domain.Event{
		TaskID:    task.ID,
		ActorID:   m.Sender.ID,
		Kind:      domain.EventPriority,
		Payload:   strconv.Itoa(int(priority)),
		CreatedAt: b.now(),
	}); err != nil {
		return err
	}
	task.Priority = priority

	if err := b.updateCard(b.ctx, board, task); err != nil {
		return err
	}
	b.touchBoard(board.ID)
	return b.answer(m, task.Key(board.Code)+" · P"+strconv.Itoa(int(priority)))
}

func (b *Bot) cmdEdit(c tele.Context) error {
	m := c.Message()
	ref, rest, _ := strings.Cut(strings.TrimSpace(m.Payload), " ")
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return b.answer(m, b.texts.T(i18n.KeyBadArguments, "/edit TG-1 new text"))
	}

	board, task, err := b.resolve(m, ref)
	if err != nil {
		return b.reportRef(m, err)
	}
	if !domain.CanEdit(task, m.Sender.ID) {
		return b.answer(m, b.texts.T(i18n.KeyNotAllowed, task.Key(board.Code)))
	}

	draft := parse.Parse(rest, shiftEntities(m.Text, rest, m.Entities), parse.Options{
		PriorityPosition: b.cfg.Parse.PriorityPosition,
	})
	if draft.Title == "" {
		return b.answer(m, b.texts.T(i18n.KeyBadArguments, "/edit TG-1 new text"))
	}

	if err := b.store.UpdateText(b.ctx, task.ID, draft.Title, draft.Description, draft.Tags); err != nil {
		return err
	}
	if err := b.store.UpdatePriority(b.ctx, task.ID, draft.Priority); err != nil {
		return err
	}
	if err := b.store.AddEvent(b.ctx, domain.Event{
		TaskID:    task.ID,
		ActorID:   m.Sender.ID,
		Kind:      domain.EventEdit,
		CreatedAt: b.now(),
	}); err != nil {
		return err
	}

	task.Title, task.Description, task.Tags, task.Priority = draft.Title, draft.Description, draft.Tags, draft.Priority
	if err := b.updateCard(b.ctx, board, task); err != nil {
		return err
	}
	b.touchBoard(board.ID)
	return b.answer(m, task.Key(board.Code)+" · "+b.texts.T(i18n.KeyEventEdited))
}

func (b *Bot) cmdRemove(c tele.Context) error {
	m := c.Message()
	board, task, err := b.resolve(m, m.Payload)
	if err != nil {
		return b.reportRef(m, err)
	}
	if !domain.CanDelete(task, m.Sender.ID) {
		return b.answer(m, b.texts.T(i18n.KeyOnlyAuthor, task.Key(board.Code)))
	}

	if err := b.store.AddEvent(b.ctx, domain.Event{
		TaskID:    task.ID,
		ActorID:   m.Sender.ID,
		Kind:      domain.EventDelete,
		CreatedAt: b.now(),
	}); err != nil {
		return err
	}
	if err := b.store.SoftDelete(b.ctx, task.ID, b.now()); err != nil {
		return err
	}

	if task.CardMsgID != 0 {
		if err := b.deleteMessage(board.ChatID, task.CardMsgID); err != nil {
			b.log.Warn("could not remove card", "task", task.Key(board.Code), "error", err)
		}
	}
	b.touchBoard(board.ID)
	return b.answer(m, b.texts.T(i18n.KeyRemoved, task.Key(board.Code)))
}

func (b *Bot) assign(ctx context.Context, task domain.Task, userID int64, handle string) error {
	var err error
	if userID != 0 {
		err = b.store.UpdateAssignee(ctx, task.ID, userID)
	} else {
		err = b.store.UpdateAssigneeHandle(ctx, task.ID, handle)
	}
	if err != nil {
		return err
	}

	payload := handle
	if userID != 0 {
		payload = strconv.FormatInt(userID, 10)
	}
	return b.store.AddEvent(ctx, domain.Event{
		TaskID:    task.ID,
		ActorID:   task.AuthorID,
		Kind:      domain.EventAssign,
		Payload:   payload,
		CreatedAt: b.now(),
	})
}

// commandBoard finds which board a command is about. A command typed in
// another topic still has an obvious target when the chat holds one board.
func (b *Bot) commandBoard(m *tele.Message) (domain.Board, bool) {
	if board, ok := b.boardFor(m); ok {
		return board, true
	}

	var (
		found domain.Board
		count int
	)
	for _, board := range b.byID {
		if board.ChatID == m.Chat.ID {
			found = board
			count++
		}
	}
	return found, count == 1
}

var errNoBoard = errors.New("no board here")

func (b *Bot) resolve(m *tele.Message, ref string) (domain.Board, domain.Task, error) {
	board, ok := b.commandBoard(m)
	if !ok {
		return domain.Board{}, domain.Task{}, errNoBoard
	}

	num, ok := parseRef(ref, board.Code)
	if !ok {
		return board, domain.Task{}, store.ErrNotFound
	}

	task, err := b.store.TaskByNum(b.ctx, board.ID, num)
	return board, task, err
}

func (b *Bot) reportRef(m *tele.Message, err error) error {
	if errors.Is(err, errNoBoard) {
		return b.answer(m, b.texts.T(i18n.KeyNoBoardHere))
	}
	if errors.Is(err, store.ErrNotFound) {
		return b.answer(m, b.texts.T(i18n.KeyUnknownTask, strings.TrimSpace(m.Payload)))
	}
	return err
}

func (b *Bot) listFilter(ctx context.Context, board domain.Board, arg string) (store.Filter, error) {
	filter := store.Filter{BoardID: board.ID, Statuses: openStatuses}

	switch {
	case arg == "":
	case arg == "all":
		filter.Statuses = nil
	case strings.HasPrefix(arg, "#"):
		filter.Tag = strings.ToLower(strings.TrimPrefix(arg, "#"))
	case strings.HasPrefix(arg, "@"):
		u, err := b.store.UserByUsername(ctx, strings.TrimPrefix(arg, "@"))
		if err != nil {
			return filter, err
		}
		filter.AssigneeID = u.ID
	default:
		status, ok := domain.ParseStatus(arg)
		if !ok {
			return filter, store.ErrNotFound
		}
		filter.Statuses = []domain.Status{status}
	}
	return filter, nil
}

// parseRef reads TG-12, tg-12, #12 or plain 12.
func parseRef(arg, code string) (int64, bool) {
	arg = strings.TrimPrefix(strings.TrimSpace(arg), "#")
	if i := strings.LastIndex(arg, "-"); i >= 0 {
		if !strings.EqualFold(arg[:i], code) {
			return 0, false
		}
		arg = arg[i+1:]
	}

	num, err := strconv.ParseInt(arg, 10, 64)
	if err != nil || num <= 0 {
		return 0, false
	}
	return num, true
}

func parsePriority(arg string) (domain.Priority, bool) {
	if arg == "-" || arg == "0" {
		return domain.PriorityNone, true
	}

	n, err := strconv.Atoi(arg)
	if err != nil {
		return 0, false
	}

	p := domain.Priority(n)
	if !p.Valid() || p == domain.PriorityNone {
		return 0, false
	}
	return p, true
}

// shiftEntities re-bases the entities of a command onto its payload, so the
// parser sees the same offsets it would in a plain message.
func shiftEntities(full, payload string, entities tele.Entities) []parse.Entity {
	at := strings.Index(full, payload)
	if at < 0 {
		return nil
	}

	start := parse.Len16(full[:at])
	end := start + parse.Len16(payload)

	out := make([]parse.Entity, 0, len(entities))
	for _, e := range entities {
		if e.Offset < start || e.Offset+e.Length > end {
			continue
		}
		shifted := parse.Entity{Type: string(e.Type), Offset: e.Offset - start, Length: e.Length}
		if e.User != nil {
			shifted.UserID = e.User.ID
		}
		out = append(out, shifted)
	}
	return out
}
