package storage

import (
	"context"
	"database/sql"
)

func (s *SQLite) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *SQLite) CountCases(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cases`).Scan(&count)
	return count, err
}

func (s *SQLite) CountEvents(ctx context.Context, caseID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events WHERE case_id=?`, caseID).Scan(&count)
	return count, err
}

func (s *SQLite) DeleteCase(ctx context.Context, caseID string) error {
	return tx(ctx, s.db, func(t *sql.Tx) error {
		for _, table := range []string{"credentials", "risks", "tests", "samples", "events", "cases"} {
			if _, err := t.ExecContext(ctx, `DELETE FROM `+table+` WHERE case_id=? OR id=?`, caseID, caseID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *SQLite) Vacuum(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `VACUUM`)
	return err
}

func (s *SQLite) Integrity(ctx context.Context) error {
	var value string
	if err := s.db.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&value); err != nil {
		return err
	}
	if value != "ok" {
		return sql.ErrNoRows
	}
	return nil
}
