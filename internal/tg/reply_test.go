package tg

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v4"
)

// telebot only builds a typed error for descriptions it knows, so the plain
// text form has to be understood just as well.
func TestRetriableUnderstandsBothErrorShapes(t *testing.T) {
	require.False(t, retriable(errors.New("telegram: Bad Request: message to edit not found (400)")))
	require.True(t, retriable(errors.New("telegram: Internal Server Error (500)")))
	require.True(t, retriable(errors.New("connection reset by peer")))

	require.False(t, retriable(tele.NewError(400, "Bad Request: chat not found")))
	require.True(t, retriable(tele.NewError(502, "Bad Gateway")))
}

func TestPinIsGone(t *testing.T) {
	require.True(t, pinIsGone(errors.New("telegram: Bad Request: message to edit not found (400)")))
	require.True(t, pinIsGone(tele.NewError(400, "Bad Request: message can't be edited")))
	require.False(t, pinIsGone(errors.New("telegram: Bad Request: chat not found (400)")))
	require.False(t, pinIsGone(errors.New("connection reset by peer")))
}

func TestFailingCallIsNotRepeatedForever(t *testing.T) {
	b, api, _, _ := newTestBot(t)
	api.fail("deleteMessage", "Bad Request: chat not found")

	require.Error(t, b.deleteMessage(chatID, 1))
	// a refusal from Telegram will not turn into a yes, so it is asked once
	require.Len(t, api.calls("deleteMessage"), 1)
}
