package storage

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
)

func (s *SQLite) CreateCase(ctx context.Context, c domain.Case, actor string) error {
	return tx(ctx, s.db, func(t *sql.Tx) error {
		var status string
		err := t.QueryRowContext(ctx, `SELECT status FROM cases WHERE tree_code=? ORDER BY created_at DESC LIMIT 1`, c.TreeCode).Scan(&status)
		if err == nil && !domain.IsTerminal(domain.CaseStatus(status)) {
			return fmt.Errorf("%w：树木编号已有未终态案卷", domain.ErrValidation)
		}
		if actor == "" || !domain.ValidateActor(actor) {
			return fmt.Errorf("%w：登记人无效", domain.ErrValidation)
		}
		_, e := t.ExecContext(ctx, `INSERT INTO cases(id,tree_code,species,location,owner,status,version,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`, c.ID, c.TreeCode, c.Species, c.Location, c.Owner, c.Status, c.Version, ts(c.CreatedAt), ts(c.UpdatedAt))
		if e != nil {
			return e
		}
		return s.eventTx(t, domain.AuditEvent{ID: domain.NewID("event"), CaseID: c.ID, Action: "建案", Detail: "冻结树木与采样基线", Actor: actor, CreatedAt: c.CreatedAt})
	})
}
func (s *SQLite) GetCase(ctx context.Context, id string) (domain.Case, error) {
	var c domain.Case
	var created, updated string
	var pending sql.NullString
	e := s.db.QueryRowContext(ctx, `SELECT id,tree_code,species,location,owner,status,version,created_at,updated_at,pending_review_since FROM cases WHERE id=?`, id).Scan(&c.ID, &c.TreeCode, &c.Species, &c.Location, &c.Owner, &c.Status, &c.Version, &created, &updated, &pending)
	if e == sql.ErrNoRows {
		return c, domain.ErrNotFound
	}
	if e != nil {
		return c, e
	}
	c.CreatedAt = parse(created)
	c.UpdatedAt = parse(updated)
	if pending.Valid {
		c.PendingReviewSince = parse(pending.String)
	}
	return c, nil
}
func (s *SQLite) UpdateCase(ctx context.Context, c domain.Case, expected int) error {
	return tx(ctx, s.db, func(t *sql.Tx) error {
		r, e := t.ExecContext(ctx, `UPDATE cases SET status=?,version=?,updated_at=? WHERE id=? AND version=?`, c.Status, c.Version, ts(c.UpdatedAt), c.ID, expected)
		if e != nil {
			return e
		}
		n, _ := r.RowsAffected()
		if n == 0 {
			return domain.ErrConflict
		}
		return nil
	})
}
