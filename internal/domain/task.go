// Package domain holds the task tracker's core types and rules.
// It knows nothing about Telegram or SQL.
package domain

import "time"

// Status is where a task currently sits.
type Status string

// The four statuses a task can be in.
const (
	StatusTodo      Status = "todo"
	StatusDoing     Status = "doing"
	StatusDone      Status = "done"
	StatusCancelled Status = "cancelled"
)

// Priority is 1 (highest) to 3 (lowest); zero means the author set none.
type Priority int

// Priority values.
const (
	PriorityNone Priority = 0
	PriorityHigh Priority = 1
	PriorityMid  Priority = 2
	PriorityLow  Priority = 3
)

// Media is an attachment carried over from the message that created the task.
type Media struct {
	Kind   string
	FileID string
}

// Empty reports whether there is no attachment.
func (m Media) Empty() bool { return m.Kind == "" || m.FileID == "" }

// Task is a single ticket on a board.
type Task struct {
	ID           int64
	BoardID      int64
	Num          int64
	Title        string
	Description  string
	RawText      string
	Status       Status
	Priority     Priority
	AuthorID     int64
	AssigneeID   int64
	AssigneeName string
	CardMsgID    int64
	SourceMsgID  int64
	Media        Media
	Tags         []string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ClosedAt     *time.Time
	DeletedAt    *time.Time
}

// Board is one forum topic the bot watches.
type Board struct {
	ID        int64
	Code      string
	Title     string
	ChatID    int64
	ThreadID  int64
	PinMsgID  int64
	CreatedAt time.Time
}

// User is a Telegram account the bot has seen.
type User struct {
	ID        int64
	Username  string
	FirstName string
	LastName  string
	DMChatID  int64
	UpdatedAt time.Time
}

// EventKind names what happened to a task.
type EventKind string

// Event kinds recorded in the task history.
const (
	EventCreated  EventKind = "created"
	EventStatus   EventKind = "status"
	EventAssign   EventKind = "assign"
	EventPriority EventKind = "priority"
	EventEdit     EventKind = "edit"
	EventDelete   EventKind = "delete"
)

// Event is one entry in a task's history.
type Event struct {
	ID        int64
	TaskID    int64
	ActorID   int64
	Kind      EventKind
	From      Status
	To        Status
	Payload   string
	CreatedAt time.Time
}
