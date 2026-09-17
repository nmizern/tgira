package render

import (
	"strings"

	"github.com/nmizern/tgira/internal/domain"
	"github.com/nmizern/tgira/internal/i18n"
)

// Card is the message that represents a task in the thread.
func Card(b domain.Board, t domain.Task, u Users, l i18n.Strings) string {
	title := esc(t.Title)
	if !t.Status.Open() {
		title = "<s>" + title + "</s>"
	}

	lines := []string{
		marker(t) + " " + key(b, t) + " · <i>" + esc(l.Status(t.Status)) + "</i>",
		"<b>" + title + "</b>",
	}
	if t.Description != "" {
		lines = append(lines, esc(t.Description))
	}

	var who string
	if name := assignee(t, u); name != "" {
		who = "👤 " + esc(name)
	}
	if meta := join(who, tags(t)); meta != "" {
		lines = append(lines, meta)
	}
	return strings.Join(lines, "\n")
}
