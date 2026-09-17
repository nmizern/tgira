package tg

import (
	"testing"

	"github.com/nmizern/tgira/internal/domain"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v4"
)

func react(b *Bot, task domain.Task, user *tele.User, before, after []string) {
	b.Process(tele.Update{
		ID: 950,
		MessageReaction: &tele.MessageReaction{
			Chat:        &tele.Chat{ID: chatID},
			MessageID:   int(task.CardMsgID),
			User:        user,
			OldReaction: reactions(before),
			NewReaction: reactions(after),
		},
	})
}

func reactions(emojis []string) []tele.Reaction {
	out := make([]tele.Reaction, 0, len(emojis))
	for _, e := range emojis {
		out = append(out, tele.Reaction{Type: "emoji", Emoji: e})
	}
	return out
}

func TestEyesStartWorkAndClaimTheTask(t *testing.T) {
	b, _, st, _ := newTestBot(t)
	task := newTask(t, b, 810, "разобраться с логами")

	react(b, task, ivan(), nil, []string{"👀"})

	got, err := st.Task(t.Context(), task.ID)
	require.NoError(t, err)
	require.Equal(t, domain.StatusDoing, got.Status)
	require.EqualValues(t, 200, got.AssigneeID)
}

func TestThumbsUpClosesTheTask(t *testing.T) {
	b, api, st, _ := newTestBot(t)
	task := newTask(t, b, 811, "починить деплой")

	react(b, task, author(), nil, []string{"👍"})

	got, err := st.Task(t.Context(), task.ID)
	require.NoError(t, err)
	require.Equal(t, domain.StatusDone, got.Status)
	require.NotNil(t, got.ClosedAt)
	require.Contains(t, api.last(t, "editMessageText").str("text"), "done")
}

func TestCelebrationAlsoClosesTheTask(t *testing.T) {
	b, _, st, _ := newTestBot(t)
	task := newTask(t, b, 812, "выкатить релиз")

	react(b, task, author(), nil, []string{"🎉"})

	got, err := st.Task(t.Context(), task.ID)
	require.NoError(t, err)
	require.Equal(t, domain.StatusDone, got.Status)
}

func TestThumbsDownCancelsTheTask(t *testing.T) {
	b, _, st, _ := newTestBot(t)
	task := newTask(t, b, 813, "опечатка")

	react(b, task, author(), nil, []string{"👎"})

	got, err := st.Task(t.Context(), task.ID)
	require.NoError(t, err)
	require.Equal(t, domain.StatusCancelled, got.Status)
}

// The bot cannot take somebody's reaction back, so it explains itself instead.
func TestOutsiderReactionIsRefusedWithANote(t *testing.T) {
	b, api, st, _ := newTestBot(t)
	require.NoError(t, st.UpsertUser(t.Context(), domain.User{ID: 200, Username: "ivan"}))
	task := newTask(t, b, 814, "@ivan починить логин")
	before := len(api.calls("sendMessage"))

	react(b, task, outsider(), nil, []string{"👍"})

	got, err := st.Task(t.Context(), task.ID)
	require.NoError(t, err)
	require.Equal(t, domain.StatusTodo, got.Status)

	require.Len(t, api.calls("sendMessage"), before+1)
	note := api.last(t, "sendMessage")
	require.Contains(t, note.str("text"), "TG-1")
	// the note cleans up after itself
	require.Equal(t, itoa(atoi(note.str("chat_id"))), note.str("chat_id"))
	require.NotEmpty(t, api.calls("deleteMessage"))
}

func TestTakingTheReactionBackRestoresTheStatus(t *testing.T) {
	b, _, st, _ := newTestBot(t)
	task := newTask(t, b, 815, "обновить readme")

	react(b, task, author(), nil, []string{"👍"})
	react(b, task, author(), []string{"👍"}, nil)

	got, err := st.Task(t.Context(), task.ID)
	require.NoError(t, err)
	require.Equal(t, domain.StatusTodo, got.Status)
	require.Nil(t, got.ClosedAt)
}

func TestTakingAReactionBackAfterSomebodyElseMovedItDoesNothing(t *testing.T) {
	b, _, st, _ := newTestBot(t)
	task := newTask(t, b, 816, "спорная задача")

	react(b, task, author(), nil, []string{"👍"})
	press(b, task, domain.StatusDoing, author())
	react(b, task, author(), []string{"👍"}, nil)

	got, err := st.Task(t.Context(), task.ID)
	require.NoError(t, err)
	require.Equal(t, domain.StatusDoing, got.Status)
}

func TestReactionsWithNoMeaningAreIgnored(t *testing.T) {
	b, api, st, _ := newTestBot(t)
	task := newTask(t, b, 817, "просто задача")
	before := len(api.calls("editMessageText"))

	react(b, task, author(), nil, []string{"🔥"})

	got, err := st.Task(t.Context(), task.ID)
	require.NoError(t, err)
	require.Equal(t, domain.StatusTodo, got.Status)
	require.Len(t, api.calls("editMessageText"), before)
}

func TestReactionsOnOtherMessagesAreIgnored(t *testing.T) {
	b, api, _, _ := newTestBot(t)
	task := newTask(t, b, 818, "задача")
	before := len(api.methods())

	stray := task
	stray.CardMsgID = 4242
	react(b, stray, author(), nil, []string{"👍"})

	require.Len(t, api.methods(), before)
}
