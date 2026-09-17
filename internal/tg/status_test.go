package tg

import (
	"testing"

	"github.com/nmizern/tgira/internal/domain"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v4"
)

func press(b *Bot, task domain.Task, to domain.Status, sender *tele.User) {
	b.Process(tele.Update{
		ID: 900,
		Callback: &tele.Callback{
			ID:      "cb-1",
			Sender:  sender,
			Message: &tele.Message{ID: int(task.CardMsgID), ThreadID: threadID, Chat: &tele.Chat{ID: chatID}},
			Data:    "\fstatus|" + itoa(int(task.ID)) + ":" + string(to),
		},
	})
}

func author() *tele.User   { return &tele.User{ID: 100, Username: "mikita"} }
func ivan() *tele.User     { return &tele.User{ID: 200, Username: "ivan"} }
func outsider() *tele.User { return &tele.User{ID: 300, Username: "petr"} }

// newTask puts a task in the thread the way a real message would.
func newTask(t *testing.T, b *Bot, srcID int, text string) domain.Task {
	t.Helper()
	feed(b, message(srcID, text))

	board, err := b.store.BoardByCode(t.Context(), "TG")
	require.NoError(t, err)
	task, err := b.store.TaskBySource(t.Context(), board.ID, int64(srcID))
	require.NoError(t, err)
	return task
}

func TestDoneButtonClosesTheTask(t *testing.T) {
	b, api, st, _ := newTestBot(t)
	task := newTask(t, b, 800, "починить деплой")

	press(b, task, domain.StatusDone, author())

	got, err := st.Task(t.Context(), task.ID)
	require.NoError(t, err)
	require.Equal(t, domain.StatusDone, got.Status)
	require.NotNil(t, got.ClosedAt)

	events, err := st.Events(t.Context(), task.ID)
	require.NoError(t, err)
	require.Equal(t, domain.EventStatus, events[len(events)-1].Kind)
	require.Equal(t, domain.StatusTodo, events[len(events)-1].From)
	require.Equal(t, domain.StatusDone, events[len(events)-1].To)

	require.Contains(t, api.last(t, "editMessageText").str("text"), "done")
	require.NotEmpty(t, api.calls("answerCallbackQuery"))
}

func TestTakeButtonClaimsAnUnownedTask(t *testing.T) {
	b, _, st, _ := newTestBot(t)
	task := newTask(t, b, 801, "разобраться с логами")

	press(b, task, domain.StatusDoing, ivan())

	got, err := st.Task(t.Context(), task.ID)
	require.NoError(t, err)
	require.Equal(t, domain.StatusDoing, got.Status)
	require.EqualValues(t, 200, got.AssigneeID)
}

func TestStrangerCannotMoveSomebodyElsesTask(t *testing.T) {
	b, api, st, _ := newTestBot(t)
	require.NoError(t, st.UpsertUser(t.Context(), domain.User{ID: 200, Username: "ivan"}))
	task := newTask(t, b, 802, "@ivan починить логин")

	press(b, task, domain.StatusDone, outsider())

	got, err := st.Task(t.Context(), task.ID)
	require.NoError(t, err)
	require.Equal(t, domain.StatusTodo, got.Status)

	answer := api.last(t, "answerCallbackQuery")
	require.Contains(t, answer.str("text"), "TG-1")
	require.Equal(t, true, answer.Params["show_alert"])
}

func TestReopenBringsTheTaskBack(t *testing.T) {
	b, _, st, _ := newTestBot(t)
	task := newTask(t, b, 803, "обновить readme")

	press(b, task, domain.StatusDone, author())
	closed, err := st.Task(t.Context(), task.ID)
	require.NoError(t, err)
	require.NotNil(t, closed.ClosedAt)

	press(b, task, domain.StatusTodo, author())
	got, err := st.Task(t.Context(), task.ID)
	require.NoError(t, err)
	require.Equal(t, domain.StatusTodo, got.Status)
	require.Nil(t, got.ClosedAt)
}

func TestPressingAButtonOnADeletedTaskIsHarmless(t *testing.T) {
	b, api, st, _ := newTestBot(t)
	task := newTask(t, b, 804, "ошибочная задача")
	require.NoError(t, st.SoftDelete(t.Context(), task.ID, day))

	press(b, task, domain.StatusDone, author())

	require.Contains(t, api.last(t, "answerCallbackQuery").str("text"), "gone")
}

func TestPressingTheStatusItAlreadyHasChangesNothing(t *testing.T) {
	b, api, st, _ := newTestBot(t)
	task := newTask(t, b, 805, "уже в очереди")
	before := len(api.calls("editMessageText"))

	press(b, task, domain.StatusTodo, author())

	events, err := st.Events(t.Context(), task.ID)
	require.NoError(t, err)
	require.Len(t, events, 1) // only "created"
	require.Len(t, api.calls("editMessageText"), before)
}
