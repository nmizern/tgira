package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/nmizern/tgira/internal/domain"
	"github.com/stretchr/testify/require"
)

var day = time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)

func openStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })
	require.NoError(t, st.Migrate(t.Context()))
	return st
}

func seedBoard(t *testing.T, st *Store) domain.Board {
	t.Helper()
	boards, err := st.SyncBoards(t.Context(), []domain.Board{
		{Code: "TG", Title: "Tasks", ChatID: -1001, ThreadID: 42},
	})
	require.NoError(t, err)
	require.Len(t, boards, 1)
	return boards[0]
}

func newTask(b domain.Board, srcMsgID int64, title string) domain.Task {
	return domain.Task{
		BoardID:     b.ID,
		Title:       title,
		Status:      domain.StatusTodo,
		AuthorID:    100,
		SourceMsgID: srcMsgID,
		CreatedAt:   day,
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	st := openStore(t)
	require.NoError(t, st.Migrate(t.Context()))

	var version int
	require.NoError(t, st.db.QueryRowContext(t.Context(),
		`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&version))
	require.Equal(t, 1, version)
}

func TestSyncBoardsInsertsThenUpdates(t *testing.T) {
	st := openStore(t)
	ctx := t.Context()

	first, err := st.SyncBoards(ctx, []domain.Board{{Code: "TG", Title: "Tasks", ChatID: -1001, ThreadID: 42}})
	require.NoError(t, err)
	require.NotZero(t, first[0].ID)

	// the topic was moved and renamed: same code, new coordinates, same row
	second, err := st.SyncBoards(ctx, []domain.Board{{Code: "TG", Title: "Board", ChatID: -1002, ThreadID: 7}})
	require.NoError(t, err)
	require.Equal(t, first[0].ID, second[0].ID)
	require.Equal(t, "Board", second[0].Title)
	require.EqualValues(t, -1002, second[0].ChatID)

	all, err := st.Boards(ctx)
	require.NoError(t, err)
	require.Len(t, all, 1)
}

func TestBoardLookups(t *testing.T) {
	st := openStore(t)
	b := seedBoard(t, st)

	byChat, err := st.BoardByChat(t.Context(), b.ChatID, b.ThreadID)
	require.NoError(t, err)
	require.Equal(t, b.ID, byChat.ID)

	byCode, err := st.BoardByCode(t.Context(), "TG")
	require.NoError(t, err)
	require.Equal(t, b.ID, byCode.ID)

	_, err = st.BoardByChat(t.Context(), -999, 0)
	require.ErrorIs(t, err, ErrNotFound)

	require.NoError(t, st.SetPin(t.Context(), b.ID, 555))
	pinned, err := st.BoardByCode(t.Context(), "TG")
	require.NoError(t, err)
	require.EqualValues(t, 555, pinned.PinMsgID)
}

func TestCreateTaskNumbersPerBoard(t *testing.T) {
	st := openStore(t)
	ctx := t.Context()
	boards, err := st.SyncBoards(ctx, []domain.Board{
		{Code: "TG", Title: "Tasks", ChatID: -1001, ThreadID: 42},
		{Code: "OPS", Title: "Ops", ChatID: -1001, ThreadID: 43},
	})
	require.NoError(t, err)

	for i := int64(1); i <= 3; i++ {
		got, err := st.CreateTask(ctx, newTask(boards[0], i, "task"))
		require.NoError(t, err)
		require.Equal(t, i, got.Num)
	}

	other, err := st.CreateTask(ctx, newTask(boards[1], 99, "other board"))
	require.NoError(t, err)
	require.EqualValues(t, 1, other.Num)
}

// A restarted bot can receive the same update twice; that must not double the task.
func TestCreateTaskIsIdempotentBySourceMessage(t *testing.T) {
	st := openStore(t)
	b := seedBoard(t, st)

	first, err := st.CreateTask(t.Context(), newTask(b, 10, "one"))
	require.NoError(t, err)
	second, err := st.CreateTask(t.Context(), newTask(b, 10, "one again"))
	require.NoError(t, err)

	require.Equal(t, first.ID, second.ID)
	require.Equal(t, "one", second.Title)

	events, err := st.Events(t.Context(), first.ID)
	require.NoError(t, err)
	require.Len(t, events, 1)
}

func TestCreateTaskStoresTagsAndCreatedEvent(t *testing.T) {
	st := openStore(t)
	b := seedBoard(t, st)

	task := newTask(b, 1, "fix login")
	task.Tags = []string{"backend", "auth"}
	task.Priority = domain.PriorityHigh
	task.Media = domain.Media{Kind: "photo", FileID: "abc"}

	created, err := st.CreateTask(t.Context(), task)
	require.NoError(t, err)

	got, err := st.TaskByNum(t.Context(), b.ID, created.Num)
	require.NoError(t, err)
	require.Equal(t, []string{"auth", "backend"}, got.Tags)
	require.Equal(t, domain.PriorityHigh, got.Priority)
	require.Equal(t, "photo", got.Media.Kind)
	require.Equal(t, day, got.CreatedAt)

	events, err := st.Events(t.Context(), created.ID)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, domain.EventCreated, events[0].Kind)
	require.EqualValues(t, 100, events[0].ActorID)
	require.Equal(t, domain.StatusTodo, events[0].To)
}

func TestTaskLookups(t *testing.T) {
	st := openStore(t)
	b := seedBoard(t, st)
	created, err := st.CreateTask(t.Context(), newTask(b, 77, "lookup me"))
	require.NoError(t, err)

	require.NoError(t, st.SetCardMsg(t.Context(), created.ID, 1234))

	byCard, err := st.TaskByCard(t.Context(), b.ID, 1234)
	require.NoError(t, err)
	require.Equal(t, created.ID, byCard.ID)

	bySource, err := st.TaskBySource(t.Context(), b.ID, 77)
	require.NoError(t, err)
	require.Equal(t, created.ID, bySource.ID)

	_, err = st.TaskByNum(t.Context(), b.ID, 999)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestUpdateStatusTracksClosedAt(t *testing.T) {
	st := openStore(t)
	b := seedBoard(t, st)
	created, err := st.CreateTask(t.Context(), newTask(b, 1, "close me"))
	require.NoError(t, err)

	closed := day.Add(time.Hour)
	require.NoError(t, st.UpdateStatus(t.Context(), created.ID, domain.StatusDone, closed))
	got, err := st.TaskByNum(t.Context(), b.ID, created.Num)
	require.NoError(t, err)
	require.Equal(t, domain.StatusDone, got.Status)
	require.NotNil(t, got.ClosedAt)
	require.Equal(t, closed, *got.ClosedAt)

	// reopening must clear the closing timestamp
	require.NoError(t, st.UpdateStatus(t.Context(), created.ID, domain.StatusTodo, closed.Add(time.Hour)))
	got, err = st.TaskByNum(t.Context(), b.ID, created.Num)
	require.NoError(t, err)
	require.Nil(t, got.ClosedAt)
}

func TestUpdateFields(t *testing.T) {
	st := openStore(t)
	b := seedBoard(t, st)
	created, err := st.CreateTask(t.Context(), newTask(b, 1, "old title"))
	require.NoError(t, err)

	require.NoError(t, st.UpdateAssignee(t.Context(), created.ID, 200))
	require.NoError(t, st.UpdatePriority(t.Context(), created.ID, domain.PriorityMid))
	require.NoError(t, st.UpdateText(t.Context(), created.ID, "new title", "details", []string{"api"}))

	got, err := st.TaskByNum(t.Context(), b.ID, created.Num)
	require.NoError(t, err)
	require.EqualValues(t, 200, got.AssigneeID)
	require.Equal(t, domain.PriorityMid, got.Priority)
	require.Equal(t, "new title", got.Title)
	require.Equal(t, "details", got.Description)
	require.Equal(t, []string{"api"}, got.Tags)
}

func TestSoftDeleteHidesTask(t *testing.T) {
	st := openStore(t)
	b := seedBoard(t, st)
	created, err := st.CreateTask(t.Context(), newTask(b, 1, "mistake"))
	require.NoError(t, err)

	require.NoError(t, st.SoftDelete(t.Context(), created.ID, day))

	_, err = st.TaskByNum(t.Context(), b.ID, created.Num)
	require.ErrorIs(t, err, ErrNotFound)

	list, err := st.Tasks(t.Context(), Filter{BoardID: b.ID})
	require.NoError(t, err)
	require.Empty(t, list)
}

func TestTasksFilterAndOrder(t *testing.T) {
	st := openStore(t)
	ctx := t.Context()
	b := seedBoard(t, st)

	mk := func(src int64, title string, p domain.Priority, s domain.Status, assignee int64, tags []string, age time.Duration) domain.Task {
		task := newTask(b, src, title)
		task.Priority = p
		task.Status = s
		task.AssigneeID = assignee
		task.Tags = tags
		task.CreatedAt = day.Add(age)
		return task
	}

	for _, task := range []domain.Task{
		mk(1, "no priority old", domain.PriorityNone, domain.StatusTodo, 0, nil, 0),
		mk(2, "low", domain.PriorityLow, domain.StatusTodo, 200, []string{"api"}, time.Minute),
		mk(3, "high", domain.PriorityHigh, domain.StatusDoing, 200, []string{"api", "ui"}, 2*time.Minute),
		mk(4, "mid", domain.PriorityMid, domain.StatusDone, 0, nil, 3*time.Minute),
	} {
		_, err := st.CreateTask(ctx, task)
		require.NoError(t, err)
	}

	titles := func(f Filter) []string {
		list, err := st.Tasks(ctx, f)
		require.NoError(t, err)
		out := make([]string, 0, len(list))
		for _, task := range list {
			out = append(out, task.Title)
		}
		return out
	}

	// priority first, unprioritised last, then oldest first
	require.Equal(t,
		[]string{"high", "mid", "low", "no priority old"},
		titles(Filter{BoardID: b.ID}))

	require.Equal(t,
		[]string{"high", "low", "no priority old"},
		titles(Filter{BoardID: b.ID, Statuses: []domain.Status{domain.StatusTodo, domain.StatusDoing}}))

	require.Equal(t, []string{"high", "low"}, titles(Filter{BoardID: b.ID, AssigneeID: 200}))
	require.Equal(t, []string{"high"}, titles(Filter{BoardID: b.ID, Tag: "ui"}))
	require.Equal(t, []string{"high", "mid"}, titles(Filter{BoardID: b.ID, Limit: 2}))
}

func TestUpsertUserKeepsDMChat(t *testing.T) {
	st := openStore(t)
	ctx := t.Context()

	require.NoError(t, st.UpsertUser(ctx, domain.User{ID: 1, Username: "ivan", FirstName: "Ivan"}))
	require.NoError(t, st.SetDMChat(ctx, 1, 555))
	// a later group message must not wipe the private chat we learned earlier
	require.NoError(t, st.UpsertUser(ctx, domain.User{ID: 1, Username: "ivan2", FirstName: "Ivan"}))

	got, err := st.User(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "ivan2", got.Username)
	require.EqualValues(t, 555, got.DMChatID)
}

func TestUserByUsernameIgnoresCase(t *testing.T) {
	st := openStore(t)
	require.NoError(t, st.UpsertUser(t.Context(), domain.User{ID: 1, Username: "Ivan"}))

	got, err := st.UserByUsername(t.Context(), "ivan")
	require.NoError(t, err)
	require.EqualValues(t, 1, got.ID)

	_, err = st.UserByUsername(t.Context(), "petr")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestLastStatusEvent(t *testing.T) {
	st := openStore(t)
	ctx := t.Context()
	b := seedBoard(t, st)
	task, err := st.CreateTask(ctx, newTask(b, 1, "history"))
	require.NoError(t, err)

	require.NoError(t, st.AddEvent(ctx, domain.Event{
		TaskID: task.ID, ActorID: 100, Kind: domain.EventStatus,
		From: domain.StatusTodo, To: domain.StatusDoing, CreatedAt: day,
	}))
	require.NoError(t, st.AddEvent(ctx, domain.Event{
		TaskID: task.ID, ActorID: 200, Kind: domain.EventStatus,
		From: domain.StatusDoing, To: domain.StatusDone, CreatedAt: day.Add(time.Minute),
	}))

	got, err := st.LastStatusEvent(ctx, task.ID, 200, domain.StatusDone)
	require.NoError(t, err)
	require.Equal(t, domain.StatusDoing, got.From)

	// the same status set by somebody else is not this actor's event to undo
	_, err = st.LastStatusEvent(ctx, task.ID, 100, domain.StatusDone)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestStatsCountsCreatedAndClosed(t *testing.T) {
	st := openStore(t)
	ctx := t.Context()
	b := seedBoard(t, st)

	first, err := st.CreateTask(ctx, newTask(b, 1, "a"))
	require.NoError(t, err)
	second := newTask(b, 2, "b")
	second.AuthorID = 200
	_, err = st.CreateTask(ctx, second)
	require.NoError(t, err)

	require.NoError(t, st.AddEvent(ctx, domain.Event{
		TaskID: first.ID, ActorID: 200, Kind: domain.EventStatus,
		From: domain.StatusTodo, To: domain.StatusDone, CreatedAt: day.Add(time.Hour),
	}))
	// an old event must stay outside the window
	require.NoError(t, st.AddEvent(ctx, domain.Event{
		TaskID: first.ID, ActorID: 100, Kind: domain.EventStatus,
		From: domain.StatusTodo, To: domain.StatusDone, CreatedAt: day.Add(-48 * time.Hour),
	}))

	stats, err := st.Stats(ctx, b.ID, day.Add(-time.Hour))
	require.NoError(t, err)

	byUser := map[int64]Stat{}
	for _, s := range stats {
		byUser[s.UserID] = s
	}
	require.Equal(t, Stat{UserID: 100, Created: 1, Closed: 0}, byUser[100])
	require.Equal(t, Stat{UserID: 200, Created: 1, Closed: 1}, byUser[200])
}

// A task can name somebody the bot has never seen; it must find its owner
// the moment that person writes anything.
func TestUpsertUserAdoptsTasksAssignedByHandle(t *testing.T) {
	st := openStore(t)
	ctx := t.Context()
	b := seedBoard(t, st)

	waiting := newTask(b, 1, "ждёт ивана")
	waiting.AssigneeName = "Ivan"
	created, err := st.CreateTask(ctx, waiting)
	require.NoError(t, err)

	other := newTask(b, 2, "чужая")
	other.AssigneeName = "petr"
	_, err = st.CreateTask(ctx, other)
	require.NoError(t, err)

	require.NoError(t, st.UpsertUser(ctx, domain.User{ID: 500, Username: "ivan"}))

	got, err := st.TaskByNum(ctx, b.ID, created.Num)
	require.NoError(t, err)
	require.EqualValues(t, 500, got.AssigneeID)
	require.Empty(t, got.AssigneeName)

	untouched, err := st.TaskByNum(ctx, b.ID, 2)
	require.NoError(t, err)
	require.Zero(t, untouched.AssigneeID)
	require.Equal(t, "petr", untouched.AssigneeName)
}
