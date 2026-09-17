package tg

import (
	"strings"
	"testing"

	"github.com/nmizern/tgira/internal/domain"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v4"
)

func privately(b *Bot, id int, text string, sender *tele.User) {
	b.Process(tele.Update{ID: id, Message: &tele.Message{
		ID:     id,
		Chat:   &tele.Chat{ID: sender.ID, Type: tele.ChatPrivate},
		Sender: sender,
		Text:   text,
	}})
}

func TestStartRemembersWhereToWriteBack(t *testing.T) {
	b, _, st, _ := newTestBot(t)

	privately(b, 870, "/start", ivan())

	u, err := st.User(t.Context(), 200)
	require.NoError(t, err)
	require.EqualValues(t, 200, u.DMChatID)
}

func TestExportSendsTwoCsvFiles(t *testing.T) {
	b, api, _, _ := newTestBot(t)
	newTask(t, b, 871, "1 задача с #тегом")

	privately(b, 872, "/export csv", author())

	docs := api.calls("sendDocument")
	require.Len(t, docs, 2)
	require.Equal(t, "tasks-TG.csv", docs[0].str("document_filename"))
	require.Equal(t, "events-TG.csv", docs[1].str("document_filename"))

	tasks := docs[0].str("document_body")
	require.True(t, strings.HasPrefix(tasks, "key,board,num,title"))
	require.Contains(t, tasks, "TG-1")
	require.Contains(t, tasks, "задача с")
	require.Contains(t, docs[1].str("document_body"), "created")
}

func TestExportCanProduceJSON(t *testing.T) {
	b, api, _, _ := newTestBot(t)
	newTask(t, b, 873, "задача")

	privately(b, 874, "/export json", author())

	docs := api.calls("sendDocument")
	require.Len(t, docs, 1)
	require.Equal(t, "tgira-TG.json", docs[0].str("document_filename"))
	require.Contains(t, docs[0].str("document_body"), `"key": "TG-1"`)
}

func TestExportRefusesAnOutsider(t *testing.T) {
	b, api, _, _ := newTestBot(t)
	newTask(t, b, 875, "задача")
	api.setRole("left")

	privately(b, 876, "/export csv", outsider())

	require.Empty(t, api.calls("sendDocument"))
	require.Contains(t, api.last(t, "sendMessage").str("text"), "not in the chat")
}

func TestExportRejectsAnUnknownFormat(t *testing.T) {
	b, api, _, _ := newTestBot(t)

	privately(b, 877, "/export xlsx", author())

	require.Empty(t, api.calls("sendDocument"))
	require.Contains(t, api.last(t, "sendMessage").str("text"), "/export")
}

func TestStatsCountsWhatEachPersonDid(t *testing.T) {
	b, api, st, _ := newTestBot(t)
	require.NoError(t, st.UpsertUser(t.Context(), domain.User{ID: 200, Username: "ivan"}))
	task := newTask(t, b, 878, "закрыть эту")
	press(b, task, domain.StatusDone, author())

	privately(b, 879, "/stats 7d", author())

	shown := api.last(t, "sendMessage").str("text")
	require.Contains(t, shown, "@mikita")
	require.Contains(t, shown, "created 1")
	require.Contains(t, shown, "closed 1")
}

func TestStatsRejectsAnUnknownPeriod(t *testing.T) {
	b, api, _, _ := newTestBot(t)

	privately(b, 880, "/stats вчера", author())

	require.Contains(t, api.last(t, "sendMessage").str("text"), "/stats")
}
