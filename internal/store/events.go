package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/nmizern/tgira/internal/domain"
)

const eventColumns = `id, task_id, actor_id, kind, from_status, to_status, payload, created_at`

// Stat is one person's numbers over a period.
type Stat struct {
	UserID  int64
	Created int
	Closed  int
}

// AddEvent appends an entry to a task's history.
func (s *Store) AddEvent(ctx context.Context, e domain.Event) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		return insertEvent(ctx, tx, e)
	})
}

func insertEvent(ctx context.Context, tx *sql.Tx, e domain.Event) error {
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO task_events (task_id, actor_id, kind, from_status, to_status, payload, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.TaskID, e.ActorID, string(e.Kind), string(e.From), string(e.To), e.Payload, formatTime(e.CreatedAt))
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

// Events returns a task's history, oldest first.
func (s *Store) Events(ctx context.Context, taskID int64) ([]domain.Event, error) {
	return s.events(ctx, `SELECT `+eventColumns+` FROM task_events WHERE task_id = ? ORDER BY id`, taskID)
}

// BoardEvents returns the history of every task on a board, oldest first.
func (s *Store) BoardEvents(ctx context.Context, boardID int64) ([]domain.Event, error) {
	return s.events(ctx, `
		SELECT `+eventColumns+` FROM task_events
		WHERE task_id IN (SELECT id FROM tasks WHERE board_id = ?)
		ORDER BY id`, boardID)
}

// LastStatusEvent finds the most recent time this actor moved the task to this
// status. Undoing a reaction uses it to learn what to roll back to.
func (s *Store) LastStatusEvent(ctx context.Context, taskID, actorID int64, to domain.Status) (domain.Event, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+eventColumns+` FROM task_events
		WHERE task_id = ? AND actor_id = ? AND kind = ? AND to_status = ?
		ORDER BY id DESC LIMIT 1`,
		taskID, actorID, string(domain.EventStatus), string(to))
	return scanEvent(row)
}

// Stats counts what each person created and closed since a moment in time.
func (s *Store) Stats(ctx context.Context, boardID int64, since time.Time) ([]Stat, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT actor_id,
			SUM(CASE WHEN kind = ? THEN 1 ELSE 0 END),
			SUM(CASE WHEN kind = ? AND to_status = ? THEN 1 ELSE 0 END)
		FROM task_events
		WHERE task_id IN (SELECT id FROM tasks WHERE board_id = ?) AND created_at >= ?
		GROUP BY actor_id
		ORDER BY 3 DESC, 2 DESC, actor_id`,
		string(domain.EventCreated), string(domain.EventStatus), string(domain.StatusDone),
		boardID, formatTime(since))
	if err != nil {
		return nil, fmt.Errorf("stats: %w", err)
	}
	defer rows.Close()

	var out []Stat
	for rows.Next() {
		var st Stat
		if err := rows.Scan(&st.UserID, &st.Created, &st.Closed); err != nil {
			return nil, fmt.Errorf("scan stat: %w", err)
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *Store) events(ctx context.Context, query string, args ...any) ([]domain.Event, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	var out []domain.Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func scanEvent(row scanner) (domain.Event, error) {
	var (
		e              domain.Event
		kind, from, to string
		createdAt      string
	)
	err := row.Scan(&e.ID, &e.TaskID, &e.ActorID, &kind, &from, &to, &e.Payload, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Event{}, ErrNotFound
	}
	if err != nil {
		return domain.Event{}, fmt.Errorf("scan event: %w", err)
	}

	e.Kind = domain.EventKind(kind)
	e.From = domain.Status(from)
	e.To = domain.Status(to)
	if e.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.Event{}, err
	}
	return e, nil
}
