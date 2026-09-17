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

	KeyEventCreated  = "event.created"
	KeyEventAssigned = "event.assigned"
	KeyEventPriority = "event.priority"
	KeyEventEdited   = "event.edited"
	KeyEventDeleted  = "event.deleted"

	KeyWhereAmI     = "cmd.whereami"
	KeyHelp         = "cmd.help"
	KeyNoBoardHere  = "error.no_board"
	KeyBadArguments = "error.bad_arguments"
	KeyOnlyAuthor   = "error.only_author"
	KeyRemoved      = "cmd.removed"
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

		KeyEventCreated:  "created",
		KeyEventAssigned: "assigned",
		KeyEventPriority: "priority changed",
		KeyEventEdited:   "edited",
		KeyEventDeleted:  "removed",

		KeyWhereAmI:     "Put these into the board config:",
		KeyNoBoardHere:  "This topic is not a tgira board.",
		KeyBadArguments: "Usage: %s",
		KeyOnlyAuthor:   "Only the author can remove %s.",
		KeyRemoved:      "%s is gone.",
		KeyHelp: "Write anything in this topic and it becomes a task.\n" +
			"A lone 1, 2 or 3 at either end sets the priority, @name assigns it, #word tags it.\n\n" +
			"👀 take it · 👍 done · 👎 cancelled — as a reaction on the card or as a button.\n\n" +
			"/tasks, /my, /show TG-1, /take TG-1, /assign TG-1 @name, /pri TG-1 2, /edit TG-1 text, /rm TG-1, /board, /whereami",
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

		KeyEventCreated:  "создана",
		KeyEventAssigned: "назначена",
		KeyEventPriority: "приоритет изменён",
		KeyEventEdited:   "отредактирована",
		KeyEventDeleted:  "удалена",

		KeyWhereAmI:     "Впишите это в конфиг доски:",
		KeyNoBoardHere:  "Этот тред не является доской tgira.",
		KeyBadArguments: "Как пользоваться: %s",
		KeyOnlyAuthor:   "Удалить %s может только автор.",
		KeyRemoved:      "%s удалена.",
		KeyHelp: "Напишите что угодно в этот тред — появится задача.\n" +
			"Отдельная 1, 2 или 3 в начале или конце задаёт приоритет, @имя назначает исполнителя, #слово ставит тег.\n\n" +
			"👀 беру · 👍 готово · 👎 отменено — реакцией на карточке или кнопкой.\n\n" +
			"/tasks, /my, /show TG-1, /take TG-1, /assign TG-1 @имя, /pri TG-1 2, /edit TG-1 текст, /rm TG-1, /board, /whereami",
	},
}
