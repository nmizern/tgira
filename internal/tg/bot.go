// Package tg connects the tracker to Telegram.
package tg

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nmizern/tgira/internal/config"
	"github.com/nmizern/tgira/internal/domain"
	"github.com/nmizern/tgira/internal/i18n"
	"github.com/nmizern/tgira/internal/render"
	"github.com/nmizern/tgira/internal/store"
	tele "gopkg.in/telebot.v4"
)

// Reactions only reach a bot that is an administrator and asks for them by name.
var allowedUpdates = []string{"message", "callback_query", "message_reaction"}

type threadKey struct {
	chat   int64
	thread int64
}

// Bot serves one tgira instance.
type Bot struct {
	api    *tele.Bot
	store  *store.Store
	cfg    config.Config
	texts  i18n.Strings
	log    *slog.Logger
	boards map[threadKey]domain.Board
	byID   map[int64]domain.Board
	ctx    context.Context
	pins   sync.Map // board id -> *pin

	now      func() time.Time
	sleep    func(time.Duration)
	after    func(time.Duration, func())
	schedule func(time.Duration, func())
}

// New dials Telegram and wires the handlers.
func New(cfg config.Config, st *store.Store, log *slog.Logger) (*Bot, error) {
	api, err := tele.NewBot(tele.Settings{
		Token:     cfg.Token,
		ParseMode: tele.ModeHTML,
		// One update at a time keeps task numbers and the pinned board
		// from racing each other.
		Synchronous: true,
		Poller: &tele.LongPoller{
			Timeout:        30 * time.Second,
			AllowedUpdates: allowedUpdates,
		},
		OnError: func(err error, _ tele.Context) {
			log.Error("handler failed", "error", err)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("connect to telegram: %w", err)
	}

	bot := newBot(cfg, st, log, api)

	// telebot's router has no branch for reaction updates, so they are taken
	// off the stream before it ever sees them.
	api.Poller = &tele.MiddlewarePoller{Poller: api.Poller, Filter: bot.intercept}
	return bot, nil
}

func newBot(cfg config.Config, st *store.Store, log *slog.Logger, api *tele.Bot) *Bot {
	b := &Bot{
		api:      api,
		store:    st,
		cfg:      cfg,
		texts:    i18n.Get(cfg.Locale),
		log:      log,
		boards:   map[threadKey]domain.Board{},
		byID:     map[int64]domain.Board{},
		ctx:      context.Background(),
		now:      time.Now,
		sleep:    time.Sleep,
		after:    func(d time.Duration, f func()) { time.AfterFunc(d, f) },
		schedule: func(d time.Duration, f func()) { time.AfterFunc(d, f) },
	}
	b.api.Handle(tele.OnText, b.onMessage)
	b.api.Handle(tele.OnMedia, b.onMessage)
	b.api.Handle(&tele.InlineButton{Unique: render.StatusAction}, b.onStatusButton)
	return b
}

// Start loads the boards and serves updates until the context is cancelled.
func (b *Bot) Start(ctx context.Context) error {
	b.ctx = ctx
	if err := b.syncBoards(ctx); err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		b.api.Stop()
	}()

	for id := range b.byID {
		if err := b.flushBoard(ctx, id); err != nil {
			b.log.Warn("could not draw board at startup", "error", err)
		}
	}

	b.log.Info("listening", "boards", len(b.boards))
	b.api.Start()
	return nil
}

func (b *Bot) syncBoards(ctx context.Context) error {
	want := make([]domain.Board, 0, len(b.cfg.Boards))
	for _, cb := range b.cfg.Boards {
		want = append(want, domain.Board{
			Code:     cb.Code,
			Title:    cb.Title,
			ChatID:   cb.ChatID,
			ThreadID: cb.ThreadID,
		})
	}

	boards, err := b.store.SyncBoards(ctx, want)
	if err != nil {
		return fmt.Errorf("sync boards: %w", err)
	}
	for _, bd := range boards {
		b.boards[threadKey{chat: bd.ChatID, thread: bd.ThreadID}] = bd
		b.byID[bd.ID] = bd
	}
	return nil
}

// intercept handles what telebot's router cannot, and reports whether the
// update should carry on to it.
func (b *Bot) intercept(u *tele.Update) bool {
	if u.MessageReaction == nil {
		return true
	}
	if err := b.onReaction(u.MessageReaction); err != nil {
		b.log.Error("reaction failed", "error", err)
	}
	return false
}

// Process runs one update through the same path the poller uses.
func (b *Bot) Process(u tele.Update) {
	if b.intercept(&u) {
		b.api.ProcessUpdate(u)
	}
}

func (b *Bot) boardFor(m *tele.Message) (domain.Board, bool) {
	if m == nil || m.Chat == nil {
		return domain.Board{}, false
	}
	bd, ok := b.boards[threadKey{chat: m.Chat.ID, thread: int64(m.ThreadID)}]
	return bd, ok
}

// people resolves ids to the names shown on cards, remembering what it looked up.
type people struct {
	ctx   context.Context
	store *store.Store
	cache map[int64]string
}

func (b *Bot) people(ctx context.Context) render.Users {
	return &people{ctx: ctx, store: b.store, cache: map[int64]string{}}
}

func (p *people) Display(id int64) string {
	if name, ok := p.cache[id]; ok {
		return name
	}

	name := fmt.Sprintf("id:%d", id)
	if u, err := p.store.User(p.ctx, id); err == nil {
		name = u.Display()
	}
	p.cache[id] = name
	return name
}
