package tg

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nmizern/tgira/internal/domain"
	"github.com/nmizern/tgira/internal/render"
	tele "gopkg.in/telebot.v4"
)

const (
	retryAttempts = 4
	retryBackoff  = 500 * time.Millisecond
	retryCeiling  = 30 * time.Second
)

// call runs a Telegram request, waiting out rate limits and brief failures.
func (b *Bot) call(what string, fn func() error) error {
	wait := retryBackoff
	var err error

	for attempt := 0; attempt < retryAttempts; attempt++ {
		if err = fn(); err == nil {
			return nil
		}

		var flood tele.FloodError
		switch {
		case errors.As(err, &flood):
			b.log.Warn("rate limited", "call", what, "retry_after", flood.RetryAfter)
			b.sleep(time.Duration(flood.RetryAfter) * time.Second)
		case retriable(err):
			b.sleep(wait)
			if wait *= 2; wait > retryCeiling {
				wait = retryCeiling
			}
		default:
			return err
		}
	}
	return err
}

// retriable reports whether trying the same call again could work. Telegram's
// own 4xx answers will not change; anything else might.
func retriable(err error) bool {
	_, code := apiFailure(err)
	if code == 0 {
		return true // network trouble rather than an answer from Telegram
	}
	return code >= 500
}

var codeInText = regexp.MustCompile(`\((\d{3})\)$`)

// apiFailure pulls the description and the status code out of a Telegram
// error. telebot only builds a typed error for descriptions it recognises;
// every other one arrives as plain text like
// "telegram: Bad Request: message to edit not found (400)".
func apiFailure(err error) (string, int) {
	var apiErr *tele.Error
	if errors.As(err, &apiErr) {
		return strings.ToLower(apiErr.Description), apiErr.Code
	}

	text := strings.ToLower(err.Error())
	if match := codeInText.FindStringSubmatch(text); match != nil {
		code, convErr := strconv.Atoi(match[1])
		if convErr == nil {
			return text, code
		}
	}
	return text, 0
}

func msgRef(chatID, msgID int64) tele.StoredMessage {
	return tele.StoredMessage{MessageID: strconv.FormatInt(msgID, 10), ChatID: chatID}
}

func (b *Bot) sendOptions(board domain.Board, task domain.Task) *tele.SendOptions {
	return &tele.SendOptions{
		ThreadID:              int(board.ThreadID),
		ParseMode:             tele.ModeHTML,
		DisableWebPagePreview: true,
		ReplyMarkup:           b.markup(task),
	}
}

func (b *Bot) markup(task domain.Task) *tele.ReplyMarkup {
	if !b.cfg.UI.Buttons {
		return nil
	}

	buttons := render.Buttons(task, b.texts)
	row := make([]tele.InlineButton, 0, len(buttons))
	for _, btn := range buttons {
		row = append(row, tele.InlineButton{Unique: btn.Unique, Text: btn.Text, Data: btn.Data})
	}
	return &tele.ReplyMarkup{InlineKeyboard: [][]tele.InlineButton{row}}
}

func (b *Bot) deleteMessage(chatID, msgID int64) error {
	return b.call("deleteMessage", func() error {
		err := b.api.Delete(msgRef(chatID, msgID))
		if errors.Is(err, tele.ErrNotFoundToDelete) {
			return nil
		}
		return err
	})
}

// notify says something short in the board's thread and takes it back again,
// so the topic does not fill up with service messages.
func (b *Bot) notify(board domain.Board, text string) {
	var sent *tele.Message
	err := b.call("sendMessage", func() error {
		msg, err := b.api.Send(tele.ChatID(board.ChatID), text, &tele.SendOptions{
			ThreadID:              int(board.ThreadID),
			ParseMode:             tele.ModeHTML,
			DisableWebPagePreview: true,
			DisableNotification:   true,
		})
		sent = msg
		return err
	})
	if err != nil {
		b.log.Warn("could not send note", "error", err)
		return
	}

	ttl := b.cfg.UI.EphemeralTTL()
	if ttl <= 0 || sent == nil {
		return
	}
	b.after(ttl, func() {
		if err := b.deleteMessage(board.ChatID, int64(sent.ID)); err != nil {
			b.log.Warn("could not remove note", "error", err)
		}
	})
}

// answer replies to a command. In a group both the question and the answer
// clear themselves away, so the topic stays a list of tasks; in a private
// chat they stay, because there is nothing to keep tidy.
func (b *Bot) answer(m *tele.Message, text string) error {
	sent, err := b.say0(m, text)
	if err != nil {
		return err
	}

	ttl := b.cfg.UI.EphemeralTTL()
	if ttl <= 0 || m.Private() {
		return nil
	}

	b.after(ttl, func() {
		if err := b.deleteMessage(m.Chat.ID, int64(m.ID)); err != nil {
			b.log.Debug("could not remove command", "error", err)
		}
		if sent != nil {
			if err := b.deleteMessage(m.Chat.ID, int64(sent.ID)); err != nil {
				b.log.Debug("could not remove answer", "error", err)
			}
		}
	})
	return nil
}

// say replies and leaves the reply where it is.
func (b *Bot) say(m *tele.Message, text string) error {
	_, err := b.say0(m, text)
	return err
}

func (b *Bot) say0(m *tele.Message, text string) (*tele.Message, error) {
	var sent *tele.Message
	err := b.call("sendMessage", func() error {
		msg, sendErr := b.api.Send(tele.ChatID(m.Chat.ID), text, &tele.SendOptions{
			ThreadID:              m.ThreadID,
			ParseMode:             tele.ModeHTML,
			DisableWebPagePreview: true,
			DisableNotification:   true,
		})
		sent = msg
		return sendErr
	})
	return sent, err
}
