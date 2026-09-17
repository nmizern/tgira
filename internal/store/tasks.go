package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nmizern/tgira/internal/domain"
)

const taskColumns = `id, board_id, num, title, description, raw_text, status, priority,
	author_id, assignee_id, assignee_name, card_msg_id, source_msg_id, media_kind, media_file_id,
	created_at, updated_at, closed_at, deleted_at`

// Filter narrows a task listing. BoardID is required.
type Filter struct {
	BoardID    int64
	Statuses   []domain.Status
	AssigneeID int64
	Tag        string
	Limit      int
}

// CreateTask stores a new task and its "created" event, assigning the next
// number on the board. Creating the same source message twice returns the
// task that already exists.
func (s *Store) CreateTask(ctx context.Context, t domain.Task) (domain.Task, error) {
	existing, err := s.TaskBySource(ctx, t.BoardID, t.SourceMsgID)
	switch {
	case err == nil:
		return existing, nil
	case !errors.Is(err, ErrNotFound):
		return domain.Task{}, err
	}

	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	t.CreatedAt = t.CreatedAt.UTC().Truncate(time.Second)
	t.UpdatedAt = t.CreatedAt
	if t.Status == "" {
		t.Status = domain.StatusTodo
	}

	err = s.inTx(ctx, func(tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(num), 0) + 1 FROM tasks WHERE board_id = ?`, t.BoardID).Scan(&t.Num); err != nil {
			return fmt.Errorf("next task number: %w", err)
		}

		res, err := tx.ExecContext(ctx, `
			INSERT INTO tasks (board_id, num, title, description, raw_text, status, priority,
				author_id, assignee_id, assignee_name, card_msg_id, source_msg_id, media_kind, media_file_id,
				created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			t.BoardID, t.Num, t.Title, t.Description, t.RawText, string(t.Status), int(t.Priority),
			t.AuthorID, t.AssigneeID, t.AssigneeName, t.CardMsgID, t.SourceMsgID, t.Media.Kind, t.Media.FileID,
			formatTime(t.CreatedAt), formatTime(t.UpdatedAt))
		if err != nil {
			return fmt.Errorf("insert task: %w", err)
		}
		if t.ID, err = res.LastInsertId(); err != nil {
			return fmt.Errorf("insert task: %w", err)
		}

		if err := replaceTags(ctx, tx, t.ID, t.Tags); err != nil {
			return err
		}

		return insertEvent(ctx, tx, domain.Event{
			TaskID:    t.ID,
			ActorID:   t.AuthorID,
			Kind:      domain.EventCreated,
			To:        t.Status,
			CreatedAt: t.CreatedAt,
		})
	})
	if err != nil {
		return domain.Task{}, err
	}
	return t, nil
}

// Task loads one task by its internal id.
func (s *Store) Task(ctx context.Context, id int64) (domain.Task, error) {
	return s.task(ctx, `id = ? AND deleted_at IS NULL`, id)
}

// TaskByCard finds the task a card message represents.
func (s *Store) TaskByCard(ctx context.Context, boardID, cardMsgID int64) (domain.Task, error) {
	return s.task(ctx, `board_id = ? AND card_msg_id = ? AND deleted_at IS NULL`, boardID, cardMsgID)
}

// TaskByNum finds a task by its number on the board.
func (s *Store) TaskByNum(ctx context.Context, boardID, num int64) (domain.Task, error) {
	return s.task(ctx, `board_id = ? AND num = ? AND deleted_at IS NULL`, boardID, num)
}

// TaskBySource finds the task created from a given message.
func (s *Store) TaskBySource(ctx context.Context, boardID, sourceMsgID int64) (domain.Task, error) {
	return s.task(ctx, `board_id = ? AND source_msg_id = ?`, boardID, sourceMsgID)
}

func (s *Store) task(ctx context.Context, where string, args ...any) (domain.Task, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+taskColumns+` FROM tasks WHERE `+where, args...)
	t, err := scanTask(row)
	if err != nil {
		return domain.Task{}, err
	}
	if t.Tags, err = s.tags(ctx, t.ID); err != nil {
		return domain.Task{}, err
	}
	return t, nil
}

// SetCardMsg records the message that now carries the task's card.
func (s *Store) SetCardMsg(ctx context.Context, taskID, msgID int64) error {
	return s.exec(ctx, `UPDATE tasks SET card_msg_id = ?, updated_at = ? WHERE id = ?`,
		msgID, formatTime(time.Now()), taskID)
}

// UpdateStatus moves a task and keeps closed_at in step with it.
func (s *Store) UpdateStatus(ctx context.Context, taskID int64, st domain.Status, at time.Time) error {
	var closedAt any
	if st == domain.StatusDone || st == domain.StatusCancelled {
		closedAt = formatTime(at)
	}
	return s.exec(ctx, `UPDATE tasks SET status = ?, closed_at = ?, updated_at = ? WHERE id = ?`,
		string(st), closedAt, formatTime(at), taskID)
}

// UpdateAssignee sets the assignee; zero means nobody.
func (s *Store) UpdateAssignee(ctx context.Context, taskID, userID int64) error {
	return s.exec(ctx, `UPDATE tasks SET assignee_id = ?, assignee_name = '', updated_at = ? WHERE id = ?`,
		userID, formatTime(time.Now()), taskID)
}

// UpdateAssigneeHandle remembers a handle the bot cannot resolve to an
// account yet. The person claims the task the moment they write anything.
func (s *Store) UpdateAssigneeHandle(ctx context.Context, taskID int64, handle string) error {
	return s.exec(ctx, `UPDATE tasks SET assignee_id = 0, assignee_name = ?, updated_at = ? WHERE id = ?`,
		handle, formatTime(time.Now()), taskID)
}

// UpdatePriority sets the priority; zero means none.
func (s *Store) UpdatePriority(ctx context.Context, taskID int64, p domain.Priority) error {
	return s.exec(ctx, `UPDATE tasks SET priority = ?, updated_at = ? WHERE id = ?`,
		int(p), formatTime(time.Now()), taskID)
}

// UpdateText replaces the title, description and tags of a task.
func (s *Store) UpdateText(ctx context.Context, taskID int64, title, description string, tags []string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`UPDATE tasks SET title = ?, description = ?, updated_at = ? WHERE id = ?`,
			title, description, formatTime(time.Now()), taskID)
		if err != nil {
			return fmt.Errorf("update text: %w", err)
		}
		return replaceTags(ctx, tx, taskID, tags)
	})
}

// SoftDelete hides a task without losing its history.
func (s *Store) SoftDelete(ctx context.Context, taskID int64, at time.Time) error {
	return s.exec(ctx, `UPDATE tasks SET deleted_at = ?, updated_at = ? WHERE id = ?`,
		formatTime(at), formatTime(at), taskID)
}

// Tasks lists tasks of a board, most important first.
func (s *Store) Tasks(ctx context.Context, f Filter) ([]domain.Task, error) {
	query := `SELECT ` + taskColumns + ` FROM tasks WHERE board_id = ? AND deleted_at IS NULL`
	args := []any{f.BoardID}

	if len(f.Statuses) > 0 {
		marks := make([]string, len(f.Statuses))
		for i, st := range f.Statuses {
			marks[i] = "?"
			args = append(args, string(st))
		}
		query += ` AND status IN (` + strings.Join(marks, ", ") + `)`
	}
	if f.AssigneeID != 0 {
		query += ` AND assignee_id = ?`
		args = append(args, f.AssigneeID)
	}
	if f.Tag != "" {
		query += ` AND EXISTS (SELECT 1 FROM task_tags WHERE task_id = tasks.id AND tag = ?)`
		args = append(args, f.Tag)
	}

	// Priority 0 means "none", which sorts after the real priorities.
	query += ` ORDER BY CASE WHEN priority = 0 THEN 4 ELSE priority END, created_at, id`
	if f.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, f.Limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var out []domain.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range out {
		if out[i].Tags, err = s.tags(ctx, out[i].ID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Store) tags(ctx context.Context, taskID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT tag FROM task_tags WHERE task_id = ? ORDER BY tag`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		out = append(out, tag)
	}
	return out, rows.Err()
}

func (s *Store) exec(ctx context.Context, query string, args ...any) error {
	if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("exec: %w", err)
	}
	return nil
}

func replaceTags(ctx context.Context, tx *sql.Tx, taskID int64, tags []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM task_tags WHERE task_id = ?`, taskID); err != nil {
		return fmt.Errorf("clear tags: %w", err)
	}
	for _, tag := range tags {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO task_tags (task_id, tag) VALUES (?, ?)`, taskID, tag); err != nil {
			return fmt.Errorf("insert tag %q: %w", tag, err)
		}
	}
	return nil
}

func scanTask(row scanner) (domain.Task, error) {
	var (
		t                    domain.Task
		status               string
		priority             int
		createdAt, updatedAt string
		closedAt, deletedAt  sql.NullString
	)
	err := row.Scan(&t.ID, &t.BoardID, &t.Num, &t.Title, &t.Description, &t.RawText, &status, &priority,
		&t.AuthorID, &t.AssigneeID, &t.AssigneeName, &t.CardMsgID, &t.SourceMsgID, &t.Media.Kind, &t.Media.FileID,
		&createdAt, &updatedAt, &closedAt, &deletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Task{}, ErrNotFound
	}
	if err != nil {
		return domain.Task{}, fmt.Errorf("scan task: %w", err)
	}

	t.Status = domain.Status(status)
	t.Priority = domain.Priority(priority)
	if t.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.Task{}, err
	}
	if t.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.Task{}, err
	}
	if t.ClosedAt, err = parseNullTime(closedAt); err != nil {
		return domain.Task{}, err
	}
	if t.DeletedAt, err = parseNullTime(deletedAt); err != nil {
		return domain.Task{}, err
	}
	return t, nil
}
