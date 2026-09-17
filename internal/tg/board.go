package tg

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/nmizern/tgira/internal/domain"
	"github.com/nmizern/tgira/internal/render"
	"github.com/nmizern/tgira/internal/store"
	tele "gopkg.in/telebot.v4"
)

// pin remembers what the board message currently says, so an unchanged board
// is never rewritten.
type pin struct {
	mu      sync.Mutex
	pending bool
	text    string
}

func (b *Bot) pin(boardID int64) *pin {
	p, _ := b.pins.LoadOrStore(boardID, &pin{})
	return p.(*pin)
}

// touchBoard asks for a redraw. Calls that arrive during the debounce window
// are folded into the one that is already on its way.
func (b *Bot) touchBoard(boardID int64) {
	if !b.cfg.UI.PinBoard {
		return
	}

	p := b.pin(boardID)
	p.mu.Lock()
	if p.pending {
		p.mu.Unlock()
		return
	}
	p.pending = true
	p.mu.Unlock()

	b.schedule(b.cfg.UI.BoardDebounce(), func() {
		p.mu.Lock()
		p.pending = false
		p.mu.Unlock()

		if err := b.flushBoard(b.ctx, boardID); err != nil {
			b.log.Warn("could not refresh board", "board", boardID, "error", err)
		}
	})
}

// flushBoard rewrites the pinned message, creating it when there is none.
func (b *Bot) flushBoard(ctx context.Context, boardID int64) error {
	if !b.cfg.UI.PinBoard {
		return nil
	}

	board, err := b.store.BoardByID(ctx, boardID)
	if err != nil {
		return err
	}

	tasks, err := b.store.Tasks(ctx, store.Filter{
		BoardID:  boardID,
		Statuses: []domain.Status{domain.StatusDoing, domain.StatusTodo},
	})
	if err != nil {
		return err
	}

	text := render.BoardText(board, tasks, b.people(ctx), b.texts)
	p := b.pin(boardID)

	p.mu.Lock()
	unchanged := p.text == text && board.PinMsgID != 0
	p.mu.Unlock()
	if unchanged {
		return nil
	}

	if board.PinMsgID != 0 {
		err := b.call("editMessageText", func() error {
			_, err := b.api.Edit(msgRef(board.ChatID, board.PinMsgID), text, boardOptions())
			if errors.Is(err, tele.ErrSameMessageContent) || errors.Is(err, tele.ErrMessageNotModified) {
				return nil
			}
			return err
		})
		if err == nil {
			b.remember(p, text)
			return nil
		}
		if !pinIsGone(err) {
			return err
		}
		// Somebody deleted the board message; put a new one in its place.
		b.log.Info("board message is gone, creating a new one", "board", board.Code)
	}

	return b.createPin(ctx, board, text, p)
}

func (b *Bot) createPin(ctx context.Context, board domain.Board, text string, p *pin) error {
	var msg *tele.Message
	err := b.call("sendMessage", func() error {
		opts := boardOptions()
		opts.ThreadID = int(board.ThreadID)
		sent, err := b.api.Send(tele.ChatID(board.ChatID), text, opts)
		msg = sent
		return err
	})
	if err != nil || msg == nil {
		return err
	}

	if err := b.call("pinChatMessage", func() error {
		return b.api.Pin(msgRef(board.ChatID, int64(msg.ID)), tele.Silent)
	}); err != nil {
		// A board that is not pinned is still a board.
		b.log.Warn("could not pin the board", "board", board.Code, "error", err)
	}

	if err := b.store.SetPin(ctx, board.ID, int64(msg.ID)); err != nil {
		return err
	}
	board.PinMsgID = int64(msg.ID)
	b.byID[board.ID] = board
	b.boards[threadKey{chat: board.ChatID, thread: board.ThreadID}] = board

	b.remember(p, text)
	return nil
}

func (b *Bot) remember(p *pin, text string) {
	p.mu.Lock()
	p.text = text
	p.mu.Unlock()
}

func boardOptions() *tele.SendOptions {
	return &tele.SendOptions{
		ParseMode:             tele.ModeHTML,
		DisableWebPagePreview: true,
		DisableNotification:   true,
	}
}

// pinIsGone reports whether the board message no longer exists, which happens
// when somebody deletes it by hand.
func pinIsGone(err error) bool {
	desc, _ := apiFailure(err)
	return strings.Contains(desc, "message to edit not found") ||
		strings.Contains(desc, "message can't be edited") ||
		strings.Contains(desc, "message to pin not found")
}
