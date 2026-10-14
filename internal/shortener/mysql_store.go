package shortener

import (
	"context"
	"database/sql"
	"errors"

	"github.com/go-sql-driver/mysql"
)

// MySQLStore saves links in MySQL using database/sql. All queries use ? placeholders,
// so user input is never put directly into SQL.
type MySQLStore struct {
	db *sql.DB
}

// NewMySQLStore wraps an open *sql.DB.
func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func (s *MySQLStore) Create(ctx context.Context, code, targetURL string) (Link, error) {
	res, err := s.db.ExecContext(ctx, "INSERT INTO links (code, target_url) VALUES (?, ?)", code, targetURL)
	if err != nil {
		var me *mysql.MySQLError
		if errors.As(err, &me) && me.Number == 1062 { // 1062 = duplicate key
			return Link{}, ErrCodeTaken
		}
		return Link{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Link{}, err
	}
	return s.getBy(ctx, "SELECT id, code, target_url, clicks, created_at FROM links WHERE id = ?", id)
}

func (s *MySQLStore) GetByCode(ctx context.Context, code string) (Link, error) {
	return s.getBy(ctx, "SELECT id, code, target_url, clicks, created_at FROM links WHERE code = ?", code)
}

func (s *MySQLStore) IncrementClicks(ctx context.Context, code string) error {
	// Doing "+ 1" inside SQL makes it atomic, so two clicks at the same time are both counted.
	res, err := s.db.ExecContext(ctx, "UPDATE links SET clicks = clicks + 1 WHERE code = ?", code)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *MySQLStore) Recent(ctx context.Context, limit int) ([]Link, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, code, target_url, clicks, created_at FROM links ORDER BY id DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var links []Link
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.ID, &l.Code, &l.TargetURL, &l.Clicks, &l.CreatedAt); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

func (s *MySQLStore) getBy(ctx context.Context, query string, arg any) (Link, error) {
	var l Link
	err := s.db.QueryRowContext(ctx, query, arg).Scan(&l.ID, &l.Code, &l.TargetURL, &l.Clicks, &l.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Link{}, ErrNotFound
	}
	return l, err
}
