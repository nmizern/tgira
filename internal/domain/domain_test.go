package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStatusOpen(t *testing.T) {
	require.True(t, StatusTodo.Open())
	require.True(t, StatusDoing.Open())
	require.False(t, StatusDone.Open())
	require.False(t, StatusCancelled.Open())
}

func TestParseStatus(t *testing.T) {
	cases := map[string]Status{
		"todo":      StatusTodo,
		"to do":     StatusTodo,
		"TODO":      StatusTodo,
		"doing":     StatusDoing,
		"done":      StatusDone,
		"cancelled": StatusCancelled,
		"canceled":  StatusCancelled,
	}
	for in, want := range cases {
		got, ok := ParseStatus(in)
		require.Truef(t, ok, "input %q", in)
		require.Equal(t, want, got)
	}

	_, ok := ParseStatus("later")
	require.False(t, ok)
}

func TestPriorityValid(t *testing.T) {
	require.True(t, PriorityNone.Valid())
	require.True(t, PriorityHigh.Valid())
	require.True(t, PriorityLow.Valid())
	require.False(t, Priority(4).Valid())
	require.False(t, Priority(-1).Valid())
}

func TestTaskKey(t *testing.T) {
	require.Equal(t, "TG-12", Task{Num: 12}.Key("TG"))
}

func TestBoardMessageLink(t *testing.T) {
	topic := Board{ChatID: -1001234567890, ThreadID: 42}
	require.Equal(t, "https://t.me/c/1234567890/42/1337", topic.MessageLink(1337))

	// the General topic has no thread segment
	general := Board{ChatID: -1001234567890}
	require.Equal(t, "https://t.me/c/1234567890/1337", general.MessageLink(1337))
}

func TestUserDisplay(t *testing.T) {
	require.Equal(t, "@ivan", User{ID: 1, Username: "ivan", FirstName: "Ivan"}.Display())
	require.Equal(t, "Ivan Petrov", User{ID: 1, FirstName: "Ivan", LastName: "Petrov"}.Display())
	require.Equal(t, "Ivan", User{ID: 1, FirstName: "Ivan"}.Display())
	require.Equal(t, "id:7", User{ID: 7}.Display())
}

func TestAccess(t *testing.T) {
	const (
		author   = int64(1)
		assignee = int64(2)
		stranger = int64(3)
	)

	assigned := Task{AuthorID: author, AssigneeID: assignee}
	unassigned := Task{AuthorID: author}

	// anyone may pick up a task nobody owns
	require.True(t, CanChangeStatus(unassigned, stranger))

	require.True(t, CanChangeStatus(assigned, author))
	require.True(t, CanChangeStatus(assigned, assignee))
	require.False(t, CanChangeStatus(assigned, stranger))

	require.True(t, CanEdit(assigned, assignee))
	require.False(t, CanEdit(assigned, stranger))

	// deleting is the author's call alone
	require.True(t, CanDelete(assigned, author))
	require.False(t, CanDelete(assigned, assignee))
	require.False(t, CanDelete(unassigned, stranger))
}
