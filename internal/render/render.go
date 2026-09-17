// Package render turns tasks into the HTML the bot sends to Telegram.
package render

import (
	"strings"

	"github.com/nmizern/tgira/internal/domain"
)

// Users resolves a Telegram id to the name shown on cards.
type Users interface {
	Display(id int64) string
}

// Status markers. The leading circle carries the priority while a task is
// open, and the outcome once it is closed.
const (
	markerDone      = "✅"
	markerCancelled = "⚫"
)

var priorityMarker = map[domain.Priority]string{
	domain.PriorityNone: "⚪",
	domain.PriorityHigh: "🔴",
	domain.PriorityMid:  "🟠",
	domain.PriorityLow:  "🟡",
}

func marker(t domain.Task) string {
	switch t.Status {
	case domain.StatusDone:
		return markerDone
	case domain.StatusCancelled:
		return markerCancelled
	}
	if m, ok := priorityMarker[t.Priority]; ok {
		return m
	}
	return priorityMarker[domain.PriorityNone]
}

// key renders the task number, linked to its card when one exists.
func key(b domain.Board, t domain.Task) string {
	text := esc(t.Key(b.Code))
	if t.CardMsgID == 0 {
		return text
	}
	return `<a href="` + b.MessageLink(t.CardMsgID) + `">` + text + `</a>`
}

func tags(t domain.Task) string {
	if len(t.Tags) == 0 {
		return ""
	}
	out := make([]string, 0, len(t.Tags))
	for _, tag := range t.Tags {
		out = append(out, "#"+esc(tag))
	}
	return strings.Join(out, " ")
}

func join(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, " · ")
}

// esc escapes exactly what Telegram's HTML mode requires, and nothing else:
// the standard library also turns quotes into numeric entities, which is
// needless noise inside message text.
func esc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	return strings.ReplaceAll(s, ">", "&gt;")
}

// assignee is the person shown on a card, by account when the bot knows it and
// by the handle the author typed until then.
func assignee(t domain.Task, u Users) string {
	if t.AssigneeID != 0 {
		return u.Display(t.AssigneeID)
	}
	return t.AssigneeHandle()
}
