package domain

import (
	"fmt"
	"strconv"
	"strings"
)

// Valid reports whether the status is one the tracker knows.
func (s Status) Valid() bool {
	switch s {
	case StatusTodo, StatusDoing, StatusDone, StatusCancelled:
		return true
	}
	return false
}

// Open reports whether the task still needs work.
func (s Status) Open() bool {
	return s == StatusTodo || s == StatusDoing
}

// ParseStatus reads a status written by a human.
func ParseStatus(v string) (Status, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "todo", "to do", "to-do":
		return StatusTodo, true
	case "doing", "wip", "in progress":
		return StatusDoing, true
	case "done":
		return StatusDone, true
	case "cancelled", "canceled":
		return StatusCancelled, true
	}
	return "", false
}

// Valid reports whether the priority is none or one of 1..3.
func (p Priority) Valid() bool {
	return p >= PriorityNone && p <= PriorityLow
}

// Key is how a task is referred to in chat, such as "TG-12".
func (t Task) Key(boardCode string) string {
	return boardCode + "-" + strconv.FormatInt(t.Num, 10)
}

// MessageLink builds a deep link to a message in this board's topic.
func (b Board) MessageLink(msgID int64) string {
	// Supergroup ids are -100 followed by the internal id the link needs.
	internal := strings.TrimPrefix(strconv.FormatInt(b.ChatID, 10), "-100")
	if b.ThreadID == 0 {
		return fmt.Sprintf("https://t.me/c/%s/%d", internal, msgID)
	}
	return fmt.Sprintf("https://t.me/c/%s/%d/%d", internal, b.ThreadID, msgID)
}

// Display is the short name used on cards and in listings.
func (u User) Display() string {
	if u.Username != "" {
		return "@" + u.Username
	}
	if name := strings.TrimSpace(u.FirstName + " " + u.LastName); name != "" {
		return name
	}
	return "id:" + strconv.FormatInt(u.ID, 10)
}
