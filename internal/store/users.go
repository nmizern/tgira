package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/nmizern/tgira/internal/domain"
)

const userColumns = `id, username, first_name, last_name, dm_chat_id, updated_at`

// UpsertUser remembers a Telegram account. It never clears the private chat
// id, which is learned separately and only once.
func (s *Store) UpsertUser(ctx context.Context, u domain.User) error {
	return s.exec(ctx, `
		INSERT INTO users (id, username, first_name, last_name, dm_chat_id, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			username   = excluded.username,
			first_name = excluded.first_name,
			last_name  = excluded.last_name,
			updated_at = excluded.updated_at`,
		u.ID, u.Username, u.FirstName, u.LastName, u.DMChatID, formatTime(time.Now()))
}

// SetDMChat records the private chat the bot can write to.
func (s *Store) SetDMChat(ctx context.Context, userID, chatID int64) error {
	return s.exec(ctx, `UPDATE users SET dm_chat_id = ? WHERE id = ?`, chatID, userID)
}

// User looks an account up by its Telegram id.
func (s *Store) User(ctx context.Context, id int64) (domain.User, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = ?`, id)
	return scanUser(row)
}

// UserByUsername looks an account up by @name, ignoring case as Telegram does.
func (s *Store) UserByUsername(ctx context.Context, username string) (domain.User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE username <> '' AND username = ? COLLATE NOCASE`, username)
	return scanUser(row)
}

// Users returns every account the bot has seen, keyed by id.
func (s *Store) Users(ctx context.Context) (map[int64]domain.User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+userColumns+` FROM users`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	out := map[int64]domain.User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out[u.ID] = u
	}
	return out, rows.Err()
}

func scanUser(row scanner) (domain.User, error) {
	var (
		u         domain.User
		updatedAt string
	)
	err := row.Scan(&u.ID, &u.Username, &u.FirstName, &u.LastName, &u.DMChatID, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("scan user: %w", err)
	}
	if u.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.User{}, err
	}
	return u, nil
}
