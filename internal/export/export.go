// Package export turns a board's history into files that open anywhere.
package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/nmizern/tgira/internal/domain"
)

const timeLayout = time.RFC3339

// Data is everything one export covers.
type Data struct {
	Board  domain.Board
	Tasks  []domain.Task
	Events map[int64][]domain.Event
	Users  map[int64]domain.User
}

var taskHeader = []string{
	"key", "board", "num", "title", "description", "status", "priority",
	"author", "assignee", "tags", "created_at", "updated_at", "closed_at", "link",
}

var eventHeader = []string{
	"key", "actor", "kind", "from_status", "to_status", "payload", "created_at",
}

// TasksCSV writes one row per task.
func TasksCSV(w io.Writer, d Data) error {
	out := csv.NewWriter(w)
	if err := out.Write(taskHeader); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	for _, t := range d.Tasks {
		row := []string{
			t.Key(d.Board.Code),
			d.Board.Code,
			strconv.FormatInt(t.Num, 10),
			t.Title,
			t.Description,
			string(t.Status),
			priority(t.Priority),
			d.name(t.AuthorID),
			d.assignee(t),
			strings.Join(t.Tags, ","),
			stamp(&t.CreatedAt),
			stamp(&t.UpdatedAt),
			stamp(t.ClosedAt),
			link(d.Board, t),
		}
		if err := out.Write(row); err != nil {
			return fmt.Errorf("write task %s: %w", t.Key(d.Board.Code), err)
		}
	}

	out.Flush()
	return out.Error()
}

// EventsCSV writes the whole history, oldest first.
func EventsCSV(w io.Writer, d Data) error {
	out := csv.NewWriter(w)
	if err := out.Write(eventHeader); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	for _, t := range d.Tasks {
		for _, e := range d.Events[t.ID] {
			row := []string{
				t.Key(d.Board.Code),
				d.name(e.ActorID),
				string(e.Kind),
				string(e.From),
				string(e.To),
				e.Payload,
				stamp(&e.CreatedAt),
			}
			if err := out.Write(row); err != nil {
				return fmt.Errorf("write event of %s: %w", t.Key(d.Board.Code), err)
			}
		}
	}

	out.Flush()
	return out.Error()
}

type jsonEvent struct {
	Actor     string `json:"actor"`
	Kind      string `json:"kind"`
	From      string `json:"from_status,omitempty"`
	To        string `json:"to_status,omitempty"`
	Payload   string `json:"payload,omitempty"`
	CreatedAt string `json:"created_at"`
}

type jsonTask struct {
	Key         string      `json:"key"`
	Num         int64       `json:"num"`
	Title       string      `json:"title"`
	Description string      `json:"description,omitempty"`
	Status      string      `json:"status"`
	Priority    int         `json:"priority,omitempty"`
	Author      string      `json:"author"`
	Assignee    string      `json:"assignee,omitempty"`
	Tags        []string    `json:"tags,omitempty"`
	CreatedAt   string      `json:"created_at"`
	UpdatedAt   string      `json:"updated_at"`
	ClosedAt    string      `json:"closed_at,omitempty"`
	Link        string      `json:"link"`
	Events      []jsonEvent `json:"events"`
}

type jsonExport struct {
	Board string     `json:"board"`
	Title string     `json:"title"`
	Tasks []jsonTask `json:"tasks"`
}

// JSON writes one document with the history nested inside every task.
func JSON(w io.Writer, d Data) error {
	doc := jsonExport{
		Board: d.Board.Code,
		Title: d.Board.Title,
		Tasks: make([]jsonTask, 0, len(d.Tasks)),
	}

	for _, t := range d.Tasks {
		events := make([]jsonEvent, 0, len(d.Events[t.ID]))
		for _, e := range d.Events[t.ID] {
			events = append(events, jsonEvent{
				Actor:     d.name(e.ActorID),
				Kind:      string(e.Kind),
				From:      string(e.From),
				To:        string(e.To),
				Payload:   e.Payload,
				CreatedAt: stamp(&e.CreatedAt),
			})
		}

		doc.Tasks = append(doc.Tasks, jsonTask{
			Key:         t.Key(d.Board.Code),
			Num:         t.Num,
			Title:       t.Title,
			Description: t.Description,
			Status:      string(t.Status),
			Priority:    int(t.Priority),
			Author:      d.name(t.AuthorID),
			Assignee:    d.assignee(t),
			Tags:        t.Tags,
			CreatedAt:   stamp(&t.CreatedAt),
			UpdatedAt:   stamp(&t.UpdatedAt),
			ClosedAt:    stamp(t.ClosedAt),
			Link:        link(d.Board, t),
			Events:      events,
		})
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(doc); err != nil {
		return fmt.Errorf("write json: %w", err)
	}
	return nil
}

func (d Data) name(id int64) string {
	if id == 0 {
		return ""
	}
	if u, ok := d.Users[id]; ok {
		return u.Display()
	}
	return "id:" + strconv.FormatInt(id, 10)
}

func (d Data) assignee(t domain.Task) string {
	if t.AssigneeID != 0 {
		return d.name(t.AssigneeID)
	}
	return t.AssigneeHandle()
}

func priority(p domain.Priority) string {
	if p == domain.PriorityNone {
		return ""
	}
	return strconv.Itoa(int(p))
}

func stamp(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.UTC().Format(timeLayout)
}

func link(b domain.Board, t domain.Task) string {
	if t.CardMsgID == 0 {
		return ""
	}
	return b.MessageLink(t.CardMsgID)
}
