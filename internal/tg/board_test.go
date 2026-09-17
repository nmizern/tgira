package tg

import (
	"testing"

	"github.com/nmizern/tgira/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestBoardIsCreatedAndPinned(t *testing.T) {
	b, api, st, board := newTestBot(t)
	newTask(t, b, 820, "1 починить деплой")

	flushPending()

	require.Contains(t, api.methods(), "pinChatMessage")
	pinned := api.last(t, "pinChatMessage")
	drawn := api.last(t, "sendMessage")
	require.Equal(t, drawn.str("chat_id"), pinned.str("chat_id"))
	require.Contains(t, drawn.str("text"), "📋")
	require.Contains(t, drawn.str("text"), "TG-1")
	require.Contains(t, drawn.str("text"), "1 open")

	stored, err := st.BoardByID(t.Context(), board.ID)
	require.NoError(t, err)
	require.NotZero(t, stored.PinMsgID)
}

func TestExistingBoardIsEditedNotResent(t *testing.T) {
	b, api, _, _ := newTestBot(t)
	newTask(t, b, 821, "первая")
	flushPending()
	sends := len(api.calls("sendMessage"))

	newTask(t, b, 822, "вторая")
	flushPending()

	// the new card is sent, the board is only edited
	require.Len(t, api.calls("sendMessage"), sends+1)
	require.Contains(t, api.last(t, "editMessageText").str("text"), "2 open")
}

// A burst of changes must cost one redraw, not one per change.
func TestBurstOfChangesRedrawsTheBoardOnce(t *testing.T) {
	b, api, _, board := newTestBot(t)

	for i := 0; i < 10; i++ {
		b.touchBoard(board.ID)
	}
	require.Len(t, pending, 1)

	before := len(api.calls("sendMessage"))
	flushPending()
	require.Len(t, api.calls("sendMessage"), before+1)
}

func TestBoardIsRebuiltWhenSomebodyDeletesIt(t *testing.T) {
	b, api, st, board := newTestBot(t)
	newTask(t, b, 823, "задача")
	flushPending()

	first, err := st.BoardByID(t.Context(), board.ID)
	require.NoError(t, err)
	require.NotZero(t, first.PinMsgID)

	api.fail("editMessageText", "Bad Request: message to edit not found")
	newTask(t, b, 824, "вторая задача")
	flushPending()

	second, err := st.BoardByID(t.Context(), board.ID)
	require.NoError(t, err)
	require.NotEqual(t, first.PinMsgID, second.PinMsgID)
	require.Len(t, api.calls("pinChatMessage"), 2)
}

func TestUnchangedBoardIsLeftAlone(t *testing.T) {
	b, api, _, board := newTestBot(t)
	newTask(t, b, 825, "задача")
	flushPending()
	edits := len(api.calls("editMessageText"))

	b.touchBoard(board.ID)
	flushPending()

	require.Len(t, api.calls("editMessageText"), edits)
}

func TestClosedTasksLeaveTheBoard(t *testing.T) {
	b, api, _, _ := newTestBot(t)
	task := newTask(t, b, 826, "закроем её")
	flushPending()

	press(b, task, domain.StatusDone, author())
	flushPending()

	drawn := api.last(t, "editMessageText").str("text")
	require.NotContains(t, drawn, "закроем её")
	require.Contains(t, drawn, "Nothing open")
}
