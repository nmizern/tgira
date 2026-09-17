package tg

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/nmizern/tgira/internal/config"
	"github.com/nmizern/tgira/internal/domain"
	"github.com/nmizern/tgira/internal/store"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v4"
)

const (
	chatID   = int64(-1001234567890)
	threadID = 42
)

var day = time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)

// pending holds board redraws until a test asks for them, so that a board
// refresh never lands in the middle of another assertion.
var pending []func()

func flushPending() {
	queued := pending
	pending = nil
	for _, f := range queued {
		f()
	}
}

func newTestBot(t *testing.T) (*Bot, *fakeAPI, *store.Store, domain.Board) {
	t.Helper()
	pending = nil

	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })
	require.NoError(t, st.Migrate(t.Context()))

	api := newFakeAPI(t)
	teleBot, err := tele.NewBot(tele.Settings{
		URL:         api.srv.URL,
		Token:       "test",
		Offline:     true,
		Synchronous: true,
		ParseMode:   tele.ModeHTML,
	})
	require.NoError(t, err)

	cfg := config.Defaults()
	cfg.Token = "test"
	cfg.Boards = []config.Board{{Code: "TG", Title: "Tasks", ChatID: chatID, ThreadID: threadID}}

	b := newBot(cfg, st, slog.New(slog.NewTextHandler(io.Discard, nil)), teleBot)
	b.now = func() time.Time { return day }
	b.sleep = func(time.Duration) {}
	b.after = func(_ time.Duration, f func()) { f() }
	b.schedule = func(_ time.Duration, f func()) { pending = append(pending, f) }
	b.ctx = t.Context()
	require.NoError(t, b.syncBoards(t.Context()))

	board, err := st.BoardByCode(t.Context(), "TG")
	require.NoError(t, err)
	return b, api, st, board
}

func message(id int, text string) *tele.Message {
	return &tele.Message{
		ID:       id,
		ThreadID: threadID,
		Chat:     &tele.Chat{ID: chatID, Type: tele.ChatSuperGroup},
		Sender:   &tele.User{ID: 100, Username: "mikita", FirstName: "Mikita"},
		Text:     text,
	}
}

func feed(b *Bot, m *tele.Message) {
	b.Process(tele.Update{ID: m.ID, Message: m})
}

func TestMessageBecomesTask(t *testing.T) {
	b, api, st, board := newTestBot(t)

	feed(b, message(700, "1 поправить редирект после логина #backend"))

	task, err := st.TaskByNum(t.Context(), board.ID, 1)
	require.NoError(t, err)
	require.Equal(t, "поправить редирект после логина", task.Title)
	require.Equal(t, domain.PriorityHigh, task.Priority)
	require.Equal(t, []string{"backend"}, task.Tags)
	require.Equal(t, domain.StatusTodo, task.Status)
	require.EqualValues(t, 100, task.AuthorID)
	require.EqualValues(t, 700, task.SourceMsgID)
	require.NotZero(t, task.CardMsgID)
	require.Equal(t, "1 поправить редирект после логина #backend", task.RawText)

	// card first, then the link to itself, then the original goes away
	require.Equal(t, []string{"sendMessage", "editMessageText", "deleteMessage"}, api.methods())

	sent := api.last(t, "sendMessage")
	require.Contains(t, sent.str("text"), "TG-1")
	require.Contains(t, sent.str("text"), "поправить редирект после логина")
	require.Equal(t, "42", sent.str("message_thread_id"))

	// only the edited card carries the deep link, since the id did not exist before
	require.NotContains(t, sent.str("text"), "https://t.me/c/")
	require.Contains(t, api.last(t, "editMessageText").str("text"), "https://t.me/c/1234567890/42/")

	require.Equal(t, "700", api.last(t, "deleteMessage").str("message_id"))
}

func TestRepliesAreNotTasks(t *testing.T) {
	b, api, st, board := newTestBot(t)

	m := message(701, "да, согласен")
	m.ReplyTo = message(700, "исходная задача")
	feed(b, m)

	require.Empty(t, api.methods())
	tasks, err := st.Tasks(t.Context(), store.Filter{BoardID: board.ID})
	require.NoError(t, err)
	require.Empty(t, tasks)
}

func TestOtherThreadsAreIgnored(t *testing.T) {
	b, api, _, _ := newTestBot(t)

	elsewhere := message(702, "болтовня в другом треде")
	elsewhere.ThreadID = 7
	feed(b, elsewhere)

	otherChat := message(703, "сообщение в другом чате")
	otherChat.Chat = &tele.Chat{ID: -999, Type: tele.ChatSuperGroup}
	feed(b, otherChat)

	require.Empty(t, api.methods())
}

func TestMessageWithoutATitleIsLeftAlone(t *testing.T) {
	b, api, st, board := newTestBot(t)

	feed(b, message(704, "@ivan"))

	tasks, err := st.Tasks(t.Context(), store.Filter{BoardID: board.ID})
	require.NoError(t, err)
	require.Empty(t, tasks)
	// nothing was created, so nothing may be deleted either
	require.Empty(t, api.methods())
}

func TestCommandsAreNotTasks(t *testing.T) {
	b, api, st, board := newTestBot(t)

	feed(b, message(705, "/tasks done"))

	tasks, err := st.Tasks(t.Context(), store.Filter{BoardID: board.ID})
	require.NoError(t, err)
	require.Empty(t, tasks)
	// it was answered as a command, not turned into work
	require.NotEmpty(t, api.calls("sendMessage"))
}

// A restarted bot re-reads updates it had already handled.
func TestTheSameMessageTwiceMakesOneTask(t *testing.T) {
	b, api, st, board := newTestBot(t)

	feed(b, message(706, "починить деплой"))
	feed(b, message(706, "починить деплой"))

	tasks, err := st.Tasks(t.Context(), store.Filter{BoardID: board.ID})
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Len(t, api.calls("sendMessage"), 1)
}

func TestKnownHandleBecomesTheAssignee(t *testing.T) {
	b, _, st, board := newTestBot(t)
	require.NoError(t, st.UpsertUser(t.Context(), domain.User{ID: 200, Username: "ivan"}))

	feed(b, message(707, "@ivan починить логин"))

	task, err := st.TaskByNum(t.Context(), board.ID, 1)
	require.NoError(t, err)
	require.EqualValues(t, 200, task.AssigneeID)
	require.Empty(t, task.AssigneeName)
}

func TestUnknownHandleIsKeptAsWritten(t *testing.T) {
	b, _, st, board := newTestBot(t)

	feed(b, message(708, "@petr починить логин"))

	task, err := st.TaskByNum(t.Context(), board.ID, 1)
	require.NoError(t, err)
	require.Zero(t, task.AssigneeID)
	require.Equal(t, "petr", task.AssigneeName)
	require.Contains(t, task.Title, "починить логин")
}

func TestAttachmentIsCopiedAndTheOriginalRemoved(t *testing.T) {
	b, api, st, board := newTestBot(t)

	withPhoto := message(709, "")
	withPhoto.Text = ""
	withPhoto.Caption = "1 падает на этом экране #ui"
	withPhoto.Photo = &tele.Photo{File: tele.File{FileID: "photo-1"}}
	feed(b, withPhoto)

	task, err := st.TaskByNum(t.Context(), board.ID, 1)
	require.NoError(t, err)
	require.Equal(t, "падает на этом экране", task.Title)
	require.Equal(t, "photo", task.Media.Kind)
	require.Equal(t, "photo-1", task.Media.FileID)

	copied := api.last(t, "copyMessage")
	require.Equal(t, "709", copied.str("message_id"))
	require.Equal(t, "падает на этом экране", copied.str("caption"))

	// the card hangs under the attachment, and the original message is gone
	require.Equal(t, []string{"copyMessage", "sendMessage", "editMessageText", "deleteMessage"}, api.methods())
	require.NotEmpty(t, api.last(t, "sendMessage").str("reply_to_message_id"))
}

// Losing a screenshot is worse than an extra message in the thread.
func TestAttachmentThatCannotBeCopiedKeepsTheOriginal(t *testing.T) {
	b, api, st, board := newTestBot(t)
	api.fail("copyMessage", "Bad Request: message can't be copied")

	withPhoto := message(710, "")
	withPhoto.Caption = "скриншот бага"
	withPhoto.Photo = &tele.Photo{File: tele.File{FileID: "photo-2"}}
	feed(b, withPhoto)

	_, err := st.TaskByNum(t.Context(), board.ID, 1)
	require.NoError(t, err)

	require.Empty(t, api.calls("deleteMessage"))
	require.Equal(t, "710", api.last(t, "sendMessage").str("reply_to_message_id"))
}
