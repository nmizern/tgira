package tg

import (
	"bytes"
	"context"
	"strings"
	"time"

	"github.com/nmizern/tgira/internal/domain"
	"github.com/nmizern/tgira/internal/export"
	"github.com/nmizern/tgira/internal/i18n"
	"github.com/nmizern/tgira/internal/render"
	"github.com/nmizern/tgira/internal/store"
	tele "gopkg.in/telebot.v4"
)

func (b *Bot) cmdExport(c tele.Context) error {
	m := c.Message()

	format, rest, _ := strings.Cut(strings.TrimSpace(m.Payload), " ")
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "json" {
		return b.answer(m, b.texts.T(i18n.KeyBadArguments, "/export csv|json [CODE]"))
	}

	board, ok := b.pickBoard(m, strings.TrimSpace(rest))
	if !ok {
		return b.answer(m, b.texts.T(i18n.KeyNoBoardHere))
	}
	if !b.isMember(board, m.Sender.ID) {
		return b.answer(m, b.texts.T(i18n.KeyNotAMember))
	}

	data, err := b.exportData(b.ctx, board)
	if err != nil {
		return err
	}

	if format == "json" {
		var body bytes.Buffer
		if err := export.JSON(&body, data); err != nil {
			return err
		}
		if err := b.sendDocument(m.Chat.ID, m.ThreadID, "tgira-"+board.Code+".json", body.Bytes()); err != nil {
			return err
		}
	} else {
		var tasks, events bytes.Buffer
		if err := export.TasksCSV(&tasks, data); err != nil {
			return err
		}
		if err := export.EventsCSV(&events, data); err != nil {
			return err
		}
		if err := b.sendDocument(m.Chat.ID, m.ThreadID, "tasks-"+board.Code+".csv", tasks.Bytes()); err != nil {
			return err
		}
		if err := b.sendDocument(m.Chat.ID, m.ThreadID, "events-"+board.Code+".csv", events.Bytes()); err != nil {
			return err
		}
	}

	return b.answer(m, b.texts.T(i18n.KeyExportReady, board.Title, len(data.Tasks)))
}

func (b *Bot) cmdStats(c tele.Context) error {
	m := c.Message()

	period := strings.ToLower(strings.TrimSpace(m.Payload))
	if period == "" {
		period = "7d"
	}
	since, ok := periodStart(b.now(), period)
	if !ok {
		return b.answer(m, b.texts.T(i18n.KeyBadArguments, "/stats 7d|30d|all"))
	}

	board, ok := b.pickBoard(m, "")
	if !ok {
		return b.answer(m, b.texts.T(i18n.KeyNoBoardHere))
	}
	if !b.isMember(board, m.Sender.ID) {
		return b.answer(m, b.texts.T(i18n.KeyNotAMember))
	}

	stats, err := b.store.Stats(b.ctx, board.ID, since)
	if err != nil {
		return err
	}

	names := b.people(b.ctx)
	rows := make([]render.StatRow, 0, len(stats))
	for _, s := range stats {
		rows = append(rows, render.StatRow{
			Name:    names.Display(s.UserID),
			Created: s.Created,
			Closed:  s.Closed,
		})
	}
	return b.answer(m, render.Stats(board, period, rows, b.texts))
}

func (b *Bot) exportData(ctx context.Context, board domain.Board) (export.Data, error) {
	tasks, err := b.store.Tasks(ctx, store.Filter{BoardID: board.ID})
	if err != nil {
		return export.Data{}, err
	}

	events := map[int64][]domain.Event{}
	all, err := b.store.BoardEvents(ctx, board.ID)
	if err != nil {
		return export.Data{}, err
	}
	for _, e := range all {
		events[e.TaskID] = append(events[e.TaskID], e)
	}

	users, err := b.store.Users(ctx)
	if err != nil {
		return export.Data{}, err
	}

	return export.Data{Board: board, Tasks: tasks, Events: events, Users: users}, nil
}

// pickBoard finds the board a private-chat command is about: by code, by the
// topic it was typed in, or the only one there is.
func (b *Bot) pickBoard(m *tele.Message, code string) (domain.Board, bool) {
	if code != "" {
		for _, board := range b.byID {
			if strings.EqualFold(board.Code, code) {
				return board, true
			}
		}
		return domain.Board{}, false
	}

	if board, ok := b.commandBoard(m); ok {
		return board, true
	}
	if len(b.byID) == 1 {
		for _, board := range b.byID {
			return board, true
		}
	}
	return domain.Board{}, false
}

// isMember keeps a board's data inside the chat it belongs to.
func (b *Bot) isMember(board domain.Board, userID int64) bool {
	member, err := b.api.ChatMemberOf(tele.ChatID(board.ChatID), &tele.User{ID: userID})
	if err != nil {
		b.log.Warn("could not check membership", "board", board.Code, "error", err)
		return false
	}
	return member.Role != tele.Left && member.Role != tele.Kicked
}

func (b *Bot) sendDocument(chatID int64, threadID int, name string, body []byte) error {
	return b.call("sendDocument", func() error {
		doc := &tele.Document{
			File:     tele.FromReader(bytes.NewReader(body)),
			FileName: name,
			MIME:     mimeOf(name),
		}
		_, err := b.api.Send(tele.ChatID(chatID), doc, &tele.SendOptions{
			ThreadID:            threadID,
			DisableNotification: true,
		})
		return err
	})
}

func mimeOf(name string) string {
	if strings.HasSuffix(name, ".json") {
		return "application/json"
	}
	return "text/csv"
}

func periodStart(now time.Time, period string) (time.Time, bool) {
	switch period {
	case "all":
		return time.Time{}, true
	case "7d":
		return now.AddDate(0, 0, -7), true
	case "30d":
		return now.AddDate(0, 0, -30), true
	}
	return time.Time{}, false
}
