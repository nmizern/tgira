package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/nmizern/tgira/internal/domain"
)

const boardColumns = `id, code, chat_id, thread_id, title, pin_msg_id, created_at`

// SyncBoards makes the database match the boards declared in the config and
// returns them with their ids filled in, in config order.
func (s *Store) SyncBoards(ctx context.Context, want []domain.Board) ([]domain.Board, error) {
	now := formatTime(time.Now())
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		for _, b := range want {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO boards (code, chat_id, thread_id, title, created_at)
				VALUES (?, ?, ?, ?, ?)
				ON CONFLICT (code) DO UPDATE SET
					chat_id   = excluded.chat_id,
					thread_id = excluded.thread_id,
					title     = excluded.title`,
				b.Code, b.ChatID, b.ThreadID, b.Title, now)
			if err != nil {
				return fmt.Errorf("sync board %s: %w", b.Code, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	out := make([]domain.Board, 0, len(want))
	for _, b := range want {
		got, err := s.BoardByCode(ctx, b.Code)
		if err != nil {
			return nil, err
		}
		out = append(out, got)
	}
	return out, nil
}

// Boards returns every known board.
func (s *Store) Boards(ctx context.Context) ([]domain.Board, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+boardColumns+` FROM boards ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list boards: %w", err)
	}
	defer rows.Close()

	var out []domain.Board
	for rows.Next() {
		b, err := scanBoard(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// BoardByChat finds the board watching a given chat topic.
func (s *Store) BoardByChat(ctx context.Context, chatID, threadID int64) (domain.Board, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+boardColumns+` FROM boards WHERE chat_id = ? AND thread_id = ?`, chatID, threadID)
	return scanBoard(row)
}

// BoardByID loads a board by its internal id.
func (s *Store) BoardByID(ctx context.Context, id int64) (domain.Board, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+boardColumns+` FROM boards WHERE id = ?`, id)
	return scanBoard(row)
}

// BoardByCode finds a board by its short code, such as "TG".
func (s *Store) BoardByCode(ctx context.Context, code string) (domain.Board, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+boardColumns+` FROM boards WHERE code = ?`, code)
	return scanBoard(row)
}

// SetPin remembers which message holds the live board.
func (s *Store) SetPin(ctx context.Context, boardID, msgID int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE boards SET pin_msg_id = ? WHERE id = ?`, msgID, boardID)
	if err != nil {
		return fmt.Errorf("set pin: %w", err)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanBoard(row scanner) (domain.Board, error) {
	var (
		b         domain.Board
		createdAt string
	)
	err := row.Scan(&b.ID, &b.Code, &b.ChatID, &b.ThreadID, &b.Title, &b.PinMsgID, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Board{}, ErrNotFound
	}
	if err != nil {
		return domain.Board{}, fmt.Errorf("scan board: %w", err)
	}
	if b.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.Board{}, err
	}
	return b, nil
}
