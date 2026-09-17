package export

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/nmizern/tgira/internal/domain"
	"github.com/stretchr/testify/require"
)

var at = time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)

func sample() Data {
	closed := at.Add(time.Hour)
	done := domain.Task{
		ID: 2, BoardID: 1, Num: 2, Title: "Метрики", Status: domain.StatusDone,
		AuthorID: 100, AssigneeID: 200, CardMsgID: 1002,
		CreatedAt: at, UpdatedAt: closed, ClosedAt: &closed,
	}
	open := domain.Task{
		ID: 1, BoardID: 1, Num: 1, Title: "Редирект, и запятая тоже",
		Description: "падает в Safari", Status: domain.StatusDoing,
		Priority: domain.PriorityHigh, AuthorID: 100, AssigneeName: "petr",
		Tags: []string{"backend", "auth"}, CardMsgID: 1001,
		CreatedAt: at, UpdatedAt: at,
	}

	return Data{
		Board: domain.Board{ID: 1, Code: "TG", Title: "Tasks", ChatID: -1001234567890, ThreadID: 42},
		Tasks: []domain.Task{open, done},
		Events: map[int64][]domain.Event{
			1: {{TaskID: 1, ActorID: 100, Kind: domain.EventCreated, To: domain.StatusTodo, CreatedAt: at}},
			2: {
				{TaskID: 2, ActorID: 100, Kind: domain.EventCreated, To: domain.StatusTodo, CreatedAt: at},
				{TaskID: 2, ActorID: 200, Kind: domain.EventStatus, From: domain.StatusTodo, To: domain.StatusDone, CreatedAt: closed},
			},
		},
		Users: map[int64]domain.User{
			100: {ID: 100, Username: "mikita"},
			200: {ID: 200, Username: "ivan"},
		},
	}
}

func TestTasksCSV(t *testing.T) {
	var out strings.Builder
	require.NoError(t, TasksCSV(&out, sample()))

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	require.Len(t, lines, 3)
	require.Equal(t,
		"key,board,num,title,description,status,priority,author,assignee,tags,created_at,updated_at,closed_at,link",
		strings.TrimSpace(lines[0]))

	// a comma in the title must not break the row apart
	require.Contains(t, lines[1], `"Редирект, и запятая тоже"`)
	require.Contains(t, lines[1], "TG-1")
	require.Contains(t, lines[1], "doing")
	require.Contains(t, lines[1], "@petr")
	require.Contains(t, lines[1], `"backend,auth"`)
	require.Contains(t, lines[1], "2026-09-17T12:00:00Z")
	require.Contains(t, lines[1], "https://t.me/c/1234567890/42/1001")

	require.Contains(t, lines[2], "TG-2")
	require.Contains(t, lines[2], "@ivan")
	require.Contains(t, lines[2], "2026-09-17T13:00:00Z")
}

func TestEventsCSV(t *testing.T) {
	var out strings.Builder
	require.NoError(t, EventsCSV(&out, sample()))

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	require.Len(t, lines, 4)
	require.Equal(t, "key,actor,kind,from_status,to_status,payload,created_at", strings.TrimSpace(lines[0]))
	require.Contains(t, lines[1], "TG-1,@mikita,created")
	require.Contains(t, lines[3], "TG-2,@ivan,status,todo,done")
}

func TestJSONCarriesTheHistoryInsideEachTask(t *testing.T) {
	var out strings.Builder
	require.NoError(t, JSON(&out, sample()))

	var doc struct {
		Board string `json:"board"`
		Tasks []struct {
			Key      string   `json:"key"`
			Assignee string   `json:"assignee"`
			Tags     []string `json:"tags"`
			Events   []struct {
				Kind string `json:"kind"`
			} `json:"events"`
		} `json:"tasks"`
	}
	require.NoError(t, json.Unmarshal([]byte(out.String()), &doc))

	require.Equal(t, "TG", doc.Board)
	require.Len(t, doc.Tasks, 2)
	require.Equal(t, "TG-1", doc.Tasks[0].Key)
	require.Equal(t, "@petr", doc.Tasks[0].Assignee)
	require.Equal(t, []string{"backend", "auth"}, doc.Tasks[0].Tags)
	require.Len(t, doc.Tasks[1].Events, 2)
}

func TestEmptyBoardStillProducesAHeader(t *testing.T) {
	var out strings.Builder
	require.NoError(t, TasksCSV(&out, Data{Board: domain.Board{Code: "TG"}}))
	require.Equal(t, 1, len(strings.Split(strings.TrimSpace(out.String()), "\n")))
}
