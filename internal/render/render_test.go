package render

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nmizern/tgira/internal/domain"
	"github.com/nmizern/tgira/internal/i18n"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "rewrite golden files")

type users map[int64]string

func (u users) Display(id int64) string {
	if v, ok := u[id]; ok {
		return v
	}
	return fmt.Sprintf("id:%d", id)
}

var (
	people = users{100: "@mikita", 200: "@ivan"}
	board  = domain.Board{ID: 1, Code: "TG", Title: "Tasks", ChatID: -1001234567890, ThreadID: 42}
	en     = i18n.Get("en")
)

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".txt")
	if *update {
		require.NoError(t, os.WriteFile(path, []byte(got), 0o600))
		return
	}
	want, err := os.ReadFile(path)
	require.NoErrorf(t, err, "run: go test ./internal/render -update")
	require.Equal(t, string(want), got)
}

func task(num int64, title string) domain.Task {
	return domain.Task{
		ID:        num,
		BoardID:   board.ID,
		Num:       num,
		Title:     title,
		Status:    domain.StatusTodo,
		AuthorID:  100,
		CardMsgID: 1000 + num,
		CreatedAt: time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC),
	}
}

func TestCard(t *testing.T) {
	full := task(12, "Поправить редирект после логина через Google")
	full.Status = domain.StatusDoing
	full.Priority = domain.PriorityHigh
	full.AssigneeID = 200
	full.Tags = []string{"backend", "auth"}
	full.Description = "Валится только в Safari"

	done := task(9, "Прикрутить метрики")
	done.Status = domain.StatusDone
	done.AssigneeID = 200

	cancelled := task(8, "Опечатка")
	cancelled.Status = domain.StatusCancelled

	unposted := task(13, "Ещё не отправлена")
	unposted.CardMsgID = 0

	escaped := task(14, "Fix <script> & \"quotes\"")
	escaped.Description = "a < b & c"
	escaped.Tags = []string{"a&b"}

	lowPriority := task(15, "Обновить README")
	lowPriority.Priority = domain.PriorityLow

	// assigned to somebody the bot has not met yet
	byHandle := task(16, "Написать миграцию")
	byHandle.AssigneeName = "ivan"

	cases := map[string]domain.Task{
		"card_doing_full":   full,
		"card_todo_plain":   task(7, "Разобраться с логами nginx"),
		"card_done":         done,
		"card_cancelled":    cancelled,
		"card_no_message":   unposted,
		"card_escaped":      escaped,
		"card_low_priority": lowPriority,
		"card_by_handle":    byHandle,
	}

	for name, tk := range cases {
		t.Run(name, func(t *testing.T) {
			golden(t, name, Card(board, tk, people, en))
		})
	}
}

func TestButtons(t *testing.T) {
	todo := task(1, "a")
	doing := task(2, "b")
	doing.Status = domain.StatusDoing
	done := task(3, "c")
	done.Status = domain.StatusDone

	require.Equal(t, []Button{
		{Text: "👀 Take", Unique: StatusAction, Data: "1:doing"},
		{Text: "✅ Done", Unique: StatusAction, Data: "1:done"},
	}, Buttons(todo, en))

	require.Equal(t, []Button{
		{Text: "✅ Done", Unique: StatusAction, Data: "2:done"},
		{Text: "↩ To do", Unique: StatusAction, Data: "2:todo"},
	}, Buttons(doing, en))

	require.Equal(t, []Button{
		{Text: "↩ Reopen", Unique: StatusAction, Data: "3:todo"},
	}, Buttons(done, en))
}

func TestBoardText(t *testing.T) {
	doing := task(12, "Редирект после логина")
	doing.Status = domain.StatusDoing
	doing.Priority = domain.PriorityHigh
	doing.AssigneeID = 200

	doingLow := task(9, "Метрики")
	doingLow.Status = domain.StatusDoing
	doingLow.Priority = domain.PriorityLow
	doingLow.AssigneeID = 200

	todoMid := task(14, "Починить деплой")
	todoMid.Priority = domain.PriorityMid
	todoMid.AssigneeID = 100

	todoNone := task(13, "Обновить README")

	// closed tasks never reach the board
	closed := task(5, "Уже готово")
	closed.Status = domain.StatusDone

	golden(t, "board_full", BoardText(board, []domain.Task{doing, doingLow, todoMid, todoNone, closed}, people, en))
	golden(t, "board_empty", BoardText(board, nil, people, en))

	var many []domain.Task
	for i := int64(1); i <= 200; i++ {
		many = append(many, task(i, fmt.Sprintf("Задача номер %d с довольно длинным названием", i)))
	}
	long := BoardText(board, many, people, en)
	require.LessOrEqual(t, len([]rune(long)), 4096)
	require.Contains(t, long, "and")
}
