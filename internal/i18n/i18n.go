// Package i18n holds every string the bot shows to people.
package i18n

import (
	"fmt"

	"github.com/nmizern/tgira/internal/domain"
)

// Message keys. Using constants keeps typos out of the render layer.
const (
	KeyBoardOpen   = "board.open"
	KeyBoardEmpty  = "board.empty"
	KeyBoardMore   = "board.more"
	KeyBtnTake     = "btn.take"
	KeyBtnDone     = "btn.done"
	KeyBtnTodo     = "btn.todo"
	KeyBtnReopen   = "btn.reopen"
	KeyNoTasks     = "list.empty"
	KeyNotAllowed  = "error.not_allowed"
	KeyTaskGone    = "error.task_gone"
	KeyUnknownTask = "error.unknown_task"
)

// Strings is the set of texts for one language.
type Strings interface {
	Status(domain.Status) string
	T(key string, args ...any) string
}

type table struct {
	statuses map[domain.Status]string
	texts    map[string]string
}

func (t table) Status(s domain.Status) string {
	if v, ok := t.statuses[s]; ok {
		return v
	}
	return string(s)
}

func (t table) T(key string, args ...any) string {
	v, ok := t.texts[key]
	if !ok {
		return key
	}
	if len(args) == 0 {
		return v
	}
	return fmt.Sprintf(v, args...)
}

// Get returns the strings for a locale, falling back to English.
func Get(locale string) Strings {
	if locale == "ru" {
		return russian
	}
	return english
}

var english = table{
	statuses: map[domain.Status]string{
		domain.StatusTodo:      "to do",
		domain.StatusDoing:     "doing",
		domain.StatusDone:      "done",
		domain.StatusCancelled: "cancelled",
	},
	texts: map[string]string{
		KeyBoardOpen:   "%d open",
		KeyBoardEmpty:  "Nothing open. Write a message here to add a task.",
		KeyBoardMore:   "… and %d more",
		KeyBtnTake:     "👀 Take",
		KeyBtnDone:     "✅ Done",
		KeyBtnTodo:     "↩ To do",
		KeyBtnReopen:   "↩ Reopen",
		KeyNoTasks:     "Nothing found.",
		KeyNotAllowed:  "Only the author or the assignee can change %s.",
		KeyTaskGone:    "That task is gone.",
		KeyUnknownTask: "No task %s on this board.",
	},
}

var russian = table{
	statuses: map[domain.Status]string{
		domain.StatusTodo:      "в очереди",
		domain.StatusDoing:     "в работе",
		domain.StatusDone:      "готово",
		domain.StatusCancelled: "отменено",
	},
	texts: map[string]string{
		KeyBoardOpen:   "открытых: %d",
		KeyBoardEmpty:  "Открытых задач нет. Напишите сюда сообщение, чтобы добавить.",
		KeyBoardMore:   "… и ещё %d",
		KeyBtnTake:     "👀 Беру",
		KeyBtnDone:     "✅ Готово",
		KeyBtnTodo:     "↩ В очередь",
		KeyBtnReopen:   "↩ Вернуть",
		KeyNoTasks:     "Ничего не нашлось.",
		KeyNotAllowed:  "Менять %s может только автор или исполнитель.",
		KeyTaskGone:    "Этой задачи больше нет.",
		KeyUnknownTask: "На этой доске нет задачи %s.",
	},
}
