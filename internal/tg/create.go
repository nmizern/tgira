package tg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/nmizern/tgira/internal/domain"
	"github.com/nmizern/tgira/internal/parse"
	"github.com/nmizern/tgira/internal/render"
	"github.com/nmizern/tgira/internal/store"
	tele "gopkg.in/telebot.v4"
)

// captionLimit is what Telegram allows under an attachment.
const captionLimit = 1024

func (b *Bot) onMessage(c tele.Context) error {
	m := c.Message()

	board, ok := b.boardFor(m)
	if !ok {
		return nil
	}
	// Replies are the conversation under a card, not new work.
	if m.ReplyTo != nil {
		return nil
	}
	if m.Sender == nil || m.Sender.IsBot {
		return nil
	}
	// An unknown command is still a command, not a task.
	if strings.HasPrefix(strings.TrimSpace(m.Text), "/") {
		return nil
	}

	return b.createTask(b.ctx, board, m)
}

func (b *Bot) createTask(ctx context.Context, board domain.Board, m *tele.Message) error {
	text, entities := messageText(m)
	draft := parse.Parse(text, convertEntities(entities), parse.Options{
		PriorityPosition: b.cfg.Parse.PriorityPosition,
	})
	if draft.Title == "" {
		return nil
	}

	author := userOf(m.Sender)
	if err := b.store.UpsertUser(ctx, author); err != nil {
		return err
	}

	task := domain.Task{
		BoardID:      board.ID,
		Title:        draft.Title,
		Description:  draft.Description,
		RawText:      text,
		Status:       domain.StatusTodo,
		Priority:     draft.Priority,
		AuthorID:     author.ID,
		AssigneeID:   draft.AssigneeID,
		AssigneeName: draft.AssigneeUsername,
		SourceMsgID:  int64(m.ID),
		Media:        mediaOf(m),
		Tags:         draft.Tags,
		CreatedAt:    b.now(),
	}
	if err := b.resolveAssignee(ctx, &task); err != nil {
		return err
	}

	task, err := b.store.CreateTask(ctx, task)
	if err != nil {
		return err
	}
	if task.CardMsgID != 0 {
		// The same message came round twice; its card is already in the thread.
		return nil
	}

	card, keepSource, err := b.postCard(ctx, board, task, m)
	if err != nil {
		return fmt.Errorf("post card: %w", err)
	}

	task.CardMsgID = int64(card.ID)
	if err := b.store.SetCardMsg(ctx, task.ID, task.CardMsgID); err != nil {
		return err
	}

	// The number links to the card, whose id only exists now.
	if err := b.updateCard(ctx, board, task); err != nil {
		b.log.Warn("could not link card to itself", "task", task.Key(board.Code), "error", err)
	}

	if keepSource {
		return nil
	}
	return b.deleteMessage(m.Chat.ID, int64(m.ID))
}

// resolveAssignee turns a typed handle into an account when the bot knows it.
func (b *Bot) resolveAssignee(ctx context.Context, task *domain.Task) error {
	if task.AssigneeID != 0 || task.AssigneeName == "" {
		return nil
	}

	u, err := b.store.UserByUsername(ctx, task.AssigneeName)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	task.AssigneeID, task.AssigneeName = u.ID, ""
	return nil
}

// postCard puts the card in the thread and reports whether the original
// message has to stay where it is.
func (b *Bot) postCard(ctx context.Context, board domain.Board, task domain.Task, src *tele.Message) (*tele.Message, bool, error) {
	text := render.Card(board, task, b.people(ctx), b.texts)

	if task.Media.Empty() {
		card, err := b.sendCard(board, task, text, nil)
		return card, false, err
	}

	// Copy the attachment across so deleting the original loses nothing.
	copied, err := b.copyMedia(board, src, shortCaption(task.Title))
	if err != nil {
		// An extra message in the thread beats a lost screenshot.
		b.log.Warn("could not copy attachment, keeping the original",
			"task", task.Key(board.Code), "error", err)
		card, sendErr := b.sendCard(board, task, text, src)
		return card, true, sendErr
	}

	card, err := b.sendCard(board, task, text, copied)
	return card, false, err
}

func (b *Bot) sendCard(board domain.Board, task domain.Task, text string, replyTo *tele.Message) (*tele.Message, error) {
	opts := b.sendOptions(board, task)
	opts.ReplyTo = replyTo

	var card *tele.Message
	err := b.call("sendMessage", func() error {
		msg, err := b.api.Send(tele.ChatID(board.ChatID), text, opts)
		card = msg
		return err
	})
	return card, err
}

// copyMedia re-sends the attachment under the bot's own name. telebot's Copy
// cannot set a caption, so the method is called directly.
func (b *Bot) copyMedia(board domain.Board, src *tele.Message, caption string) (*tele.Message, error) {
	params := map[string]string{
		"chat_id":      strconv.FormatInt(board.ChatID, 10),
		"from_chat_id": strconv.FormatInt(src.Chat.ID, 10),
		"message_id":   strconv.Itoa(src.ID),
		"caption":      caption,
		"parse_mode":   string(tele.ModeHTML),
	}
	if board.ThreadID != 0 {
		params["message_thread_id"] = strconv.FormatInt(board.ThreadID, 10)
	}

	var copied *tele.Message
	err := b.call("copyMessage", func() error {
		data, err := b.api.Raw("copyMessage", params)
		if err != nil {
			return err
		}

		// copyMessage answers with a bare message id, not a whole message.
		var resp struct {
			Result struct {
				MessageID int `json:"message_id"`
			} `json:"result"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("decode copyMessage: %w", err)
		}

		copied = &tele.Message{ID: resp.Result.MessageID, Chat: &tele.Chat{ID: board.ChatID}}
		return nil
	})
	return copied, err
}

// updateCard rewrites a card in place.
func (b *Bot) updateCard(ctx context.Context, board domain.Board, task domain.Task) error {
	text := render.Card(board, task, b.people(ctx), b.texts)

	return b.call("editMessageText", func() error {
		_, err := b.api.Edit(msgRef(board.ChatID, task.CardMsgID), text, &tele.SendOptions{
			ParseMode:             tele.ModeHTML,
			DisableWebPagePreview: true,
			ReplyMarkup:           b.markup(task),
		})
		if errors.Is(err, tele.ErrSameMessageContent) || errors.Is(err, tele.ErrMessageNotModified) {
			return nil
		}
		return err
	})
}

func shortCaption(title string) string {
	runes := []rune(title)
	if len(runes) > captionLimit {
		title = string(runes[:captionLimit-1]) + "…"
	}
	return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(
		title, "&", "&amp;"), "<", "&lt;"), ">", "&gt;")
}

func messageText(m *tele.Message) (string, tele.Entities) {
	if m.Text != "" {
		return m.Text, m.Entities
	}
	return m.Caption, m.CaptionEntities
}

func convertEntities(entities tele.Entities) []parse.Entity {
	out := make([]parse.Entity, 0, len(entities))
	for _, e := range entities {
		converted := parse.Entity{Type: string(e.Type), Offset: e.Offset, Length: e.Length}
		if e.User != nil {
			converted.UserID = e.User.ID
		}
		out = append(out, converted)
	}
	return out
}

func userOf(u *tele.User) domain.User {
	if u == nil {
		return domain.User{}
	}
	return domain.User{
		ID:        u.ID,
		Username:  u.Username,
		FirstName: u.FirstName,
		LastName:  u.LastName,
	}
}

func mediaOf(m *tele.Message) domain.Media {
	switch {
	case m.Photo != nil:
		return domain.Media{Kind: "photo", FileID: m.Photo.FileID}
	case m.Document != nil:
		return domain.Media{Kind: "document", FileID: m.Document.FileID}
	case m.Video != nil:
		return domain.Media{Kind: "video", FileID: m.Video.FileID}
	case m.Animation != nil:
		return domain.Media{Kind: "animation", FileID: m.Animation.FileID}
	case m.Voice != nil:
		return domain.Media{Kind: "voice", FileID: m.Voice.FileID}
	case m.Audio != nil:
		return domain.Media{Kind: "audio", FileID: m.Audio.FileID}
	}
	return domain.Media{}
}
