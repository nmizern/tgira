package render

import (
	"strconv"
	"strings"

	"github.com/nmizern/tgira/internal/domain"
	"github.com/nmizern/tgira/internal/i18n"
)

const historyTime = "2006-01-02 15:04"

// List answers a listing command: one line per task, status spelled out.
func List(b domain.Board, tasks []domain.Task, u Users, l i18n.Strings) string {
	if len(tasks) == 0 {
		return esc(l.T(i18n.KeyNoTasks))
	}

	lines := make([]string, 0, len(tasks))
	for _, t := range tasks {
		line := marker(t) + " " + key(b, t) + " · <i>" + esc(l.Status(t.Status)) + "</i> · " + esc(shorten(t.Title))
		if name := assignee(t, u); name != "" {
			line += " · " + esc(name)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// Show is a card plus everything that ever happened to the task.
func Show(b domain.Board, t domain.Task, events []domain.Event, u Users, l i18n.Strings) string {
	out := Card(b, t, u, l)
	if len(events) == 0 {
		return out
	}

	lines := make([]string, 0, len(events))
	for _, e := range events {
		lines = append(lines, "<code>"+e.CreatedAt.Format(historyTime)+"</code> "+
			esc(u.Display(e.ActorID))+" · "+esc(describe(e, l)))
	}
	return out + "\n\n" + strings.Join(lines, "\n")
}

func describe(e domain.Event, l i18n.Strings) string {
	switch e.Kind {
	case domain.EventStatus:
		return l.Status(e.To)
	case domain.EventCreated:
		return l.T(i18n.KeyEventCreated)
	case domain.EventAssign:
		return l.T(i18n.KeyEventAssigned)
	case domain.EventPriority:
		return l.T(i18n.KeyEventPriority)
	case domain.EventEdit:
		return l.T(i18n.KeyEventEdited)
	case domain.EventDelete:
		return l.T(i18n.KeyEventDeleted)
	}
	return string(e.Kind)
}

// Where tells a person the ids they need to put in the config.
func Where(chatID int64, threadID int, l i18n.Strings) string {
	return l.T(i18n.KeyWhereAmI) + "\n" +
		"<code>chat_id: " + itoa64(chatID) + "</code>\n" +
		"<code>thread_id: " + itoa64(int64(threadID)) + "</code>"
}

func itoa64(v int64) string {
	return strconv.FormatInt(v, 10)
}

// AssigneeOf is the name a command should echo back after a reassignment.
func AssigneeOf(t domain.Task, u Users) string {
	if name := assignee(t, u); name != "" {
		return name
	}
	return "—"
}
