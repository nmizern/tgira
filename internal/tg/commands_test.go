package tg

import (
	"testing"

	"github.com/nmizern/tgira/internal/domain"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v4"
)

func command(b *Bot, id int, text string, sender *tele.User) {
	m := message(id, text)
	m.Sender = sender
	feed(b, m)
}

func TestWhereAmITellsTheIdsNeededForTheConfig(t *testing.T) {
	b, api, _, _ := newTestBot(t)

	command(b, 830, "/whereami", author())

	answer := api.last(t, "sendMessage").str("text")
	require.Contains(t, answer, "chat_id: -1001234567890")
	require.Contains(t, answer, "thread_id: 42")
}

func TestTasksListsOpenWorkByDefault(t *testing.T) {
	b, api, _, _ := newTestBot(t)
	open := newTask(t, b, 831, "1 открытая")
	closed := newTask(t, b, 832, "закрытая")
	press(b, closed, domain.StatusDone, author())

	command(b, 833, "/tasks", author())

	listed := api.last(t, "sendMessage").str("text")
	require.Contains(t, listed, open.Key("TG"))
	require.NotContains(t, listed, "закрытая")
}

func TestTasksUnderstandsItsFilters(t *testing.T) {
	b, api, st, _ := newTestBot(t)
	require.NoError(t, st.UpsertUser(t.Context(), domain.User{ID: 200, Username: "ivan"}))
	newTask(t, b, 834, "@ivan с тегом #backend")
	other := newTask(t, b, 835, "чужая задача")
	press(b, other, domain.StatusDone, author())

	command(b, 836, "/tasks done", author())
	require.Contains(t, api.last(t, "sendMessage").str("text"), "чужая задача")

	command(b, 837, "/tasks @ivan", author())
	require.Contains(t, api.last(t, "sendMessage").str("text"), "с тегом")

	command(b, 838, "/tasks #backend", author())
	require.Contains(t, api.last(t, "sendMessage").str("text"), "с тегом")

	command(b, 839, "/tasks all", author())
	listed := api.last(t, "sendMessage").str("text")
	require.Contains(t, listed, "с тегом")
	require.Contains(t, listed, "чужая задача")

	command(b, 840, "/tasks завтра", author())
	require.Contains(t, api.last(t, "sendMessage").str("text"), "/tasks")
}

func TestMyListsOnlyMyWork(t *testing.T) {
	b, api, st, _ := newTestBot(t)
	require.NoError(t, st.UpsertUser(t.Context(), domain.User{ID: 200, Username: "ivan"}))
	newTask(t, b, 841, "@ivan моя задача")
	newTask(t, b, 842, "ничья задача")

	command(b, 843, "/my", ivan())

	listed := api.last(t, "sendMessage").str("text")
	require.Contains(t, listed, "моя задача")
	require.NotContains(t, listed, "ничья задача")
}

func TestShowPrintsTheHistory(t *testing.T) {
	b, api, _, _ := newTestBot(t)
	task := newTask(t, b, 844, "посмотреть историю")
	press(b, task, domain.StatusDoing, author())

	command(b, 845, "/show TG-1", author())

	shown := api.last(t, "sendMessage").str("text")
	require.Contains(t, shown, "посмотреть историю")
	require.Contains(t, shown, "created")
	require.Contains(t, shown, "doing")
}

func TestTakeClaimsAndStartsTheTask(t *testing.T) {
	b, _, st, _ := newTestBot(t)
	task := newTask(t, b, 846, "ничья работа")

	command(b, 847, "/take TG-1", ivan())

	got, err := st.Task(t.Context(), task.ID)
	require.NoError(t, err)
	require.EqualValues(t, 200, got.AssigneeID)
	require.Equal(t, domain.StatusDoing, got.Status)
}

func TestAssignWorksForKnownAndUnknownHandles(t *testing.T) {
	b, _, st, _ := newTestBot(t)
	require.NoError(t, st.UpsertUser(t.Context(), domain.User{ID: 200, Username: "ivan"}))
	task := newTask(t, b, 848, "кому-то отдать")

	command(b, 849, "/assign TG-1 @ivan", author())
	got, err := st.Task(t.Context(), task.ID)
	require.NoError(t, err)
	require.EqualValues(t, 200, got.AssigneeID)

	// a handle the bot has never seen is kept as written
	command(b, 850, "/assign TG-1 @petr", author())
	got, err = st.Task(t.Context(), task.ID)
	require.NoError(t, err)
	require.Zero(t, got.AssigneeID)
	require.Equal(t, "petr", got.AssigneeName)
}

func TestPriorityIsChangedAndCleared(t *testing.T) {
	b, api, st, _ := newTestBot(t)
	task := newTask(t, b, 851, "без приоритета")

	command(b, 852, "/pri TG-1 2", author())
	got, err := st.Task(t.Context(), task.ID)
	require.NoError(t, err)
	require.Equal(t, domain.PriorityMid, got.Priority)

	command(b, 853, "/pri TG-1 -", author())
	got, err = st.Task(t.Context(), task.ID)
	require.NoError(t, err)
	require.Equal(t, domain.PriorityNone, got.Priority)

	command(b, 854, "/pri TG-1 9", author())
	require.Contains(t, api.last(t, "sendMessage").str("text"), "/pri")
}

func TestEditRewritesEverythingTheTextCarries(t *testing.T) {
	b, _, st, _ := newTestBot(t)
	task := newTask(t, b, 855, "старый текст")

	command(b, 856, "/edit TG-1 1 новый текст #api", author())

	got, err := st.Task(t.Context(), task.ID)
	require.NoError(t, err)
	require.Equal(t, "новый текст", got.Title)
	require.Equal(t, domain.PriorityHigh, got.Priority)
	require.Equal(t, []string{"api"}, got.Tags)
}

func TestRemoveIsTheAuthorsCallAlone(t *testing.T) {
	b, api, st, board := newTestBot(t)
	task := newTask(t, b, 857, "ошибочная задача")

	command(b, 858, "/rm TG-1", ivan())
	_, err := st.Task(t.Context(), task.ID)
	require.NoError(t, err)
	require.Contains(t, api.last(t, "sendMessage").str("text"), "author")

	command(b, 859, "/rm TG-1", author())
	_, err = st.Task(t.Context(), task.ID)
	require.Error(t, err)

	// the card goes away with the task
	removed := false
	for _, call := range api.calls("deleteMessage") {
		if call.str("message_id") == itoa(int(task.CardMsgID)) {
			removed = true
		}
	}
	require.True(t, removed)

	tasks, err := st.Tasks(t.Context(), storeFilterOpen(board.ID))
	require.NoError(t, err)
	require.Empty(t, tasks)
}

func TestAnUnknownTaskIsReportedPlainly(t *testing.T) {
	b, api, _, _ := newTestBot(t)
	newTask(t, b, 860, "единственная")

	command(b, 861, "/show TG-99", author())

	require.Contains(t, api.last(t, "sendMessage").str("text"), "TG-99")
}

func TestBoardCommandPinsAFreshMessage(t *testing.T) {
	b, api, st, board := newTestBot(t)
	newTask(t, b, 862, "задача")
	flushPending()
	first, err := st.BoardByID(t.Context(), board.ID)
	require.NoError(t, err)

	command(b, 863, "/board", author())

	second, err := st.BoardByID(t.Context(), board.ID)
	require.NoError(t, err)
	require.NotEqual(t, first.PinMsgID, second.PinMsgID)
	require.Len(t, api.calls("pinChatMessage"), 2)
}

// Commands and their answers must not pile up in the topic.
func TestCommandAndAnswerCleanUpAfterThemselves(t *testing.T) {
	b, api, _, _ := newTestBot(t)

	command(b, 864, "/help", author())

	removed := make([]string, 0, 2)
	for _, call := range api.calls("deleteMessage") {
		removed = append(removed, call.str("message_id"))
	}
	require.Len(t, removed, 2)
	require.Contains(t, removed, "864", "the command itself is removed")
	require.Contains(t, removed, itoa(api.lastMessageID()), "so is the answer")
}

// A private chat has nothing to tidy, so nothing is removed there.
func TestPrivateAnswersStay(t *testing.T) {
	b, api, _, _ := newTestBot(t)

	m := message(865, "/help")
	m.ThreadID = 0
	m.Chat = &tele.Chat{ID: 100, Type: tele.ChatPrivate}
	feed(b, m)

	require.NotEmpty(t, api.calls("sendMessage"))
	require.Empty(t, api.calls("deleteMessage"))
}

// whereami is read while editing the config, so its answer must not vanish,
// and it is the one command that works before any board exists.
func TestWhereAmIAnswerStays(t *testing.T) {
	b, api, _, _ := newTestBot(t)

	command(b, 890, "/whereami", author())

	require.NotEmpty(t, api.calls("sendMessage"))
	require.Empty(t, api.calls("deleteMessage"))
}

func TestWhereAmIWorksWithoutAnyBoard(t *testing.T) {
	b, api, _, _ := newTestBot(t)
	b.boards = nil
	b.byID = nil

	command(b, 891, "/whereami", author())

	require.Contains(t, api.last(t, "sendMessage").str("text"), "chat_id: -1001234567890")
}
