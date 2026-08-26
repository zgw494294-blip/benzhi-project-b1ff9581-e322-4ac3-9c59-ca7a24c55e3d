package storage

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
	"time"
)

func (s *SQLite) RetestCase(ctx context.Context, caseID, result, operator, notes, key string, expected int) (domain.Case, error) {
	var c domain.Case
	err := tx(ctx, s.db, func(t *sql.Tx) error {
		var status string
		if err := t.QueryRowContext(ctx, `SELECT status FROM cases WHERE id=?`, caseID).Scan(&status); err != nil {
			return domain.ErrNotFound
		}
		if key != "" {
			var existing string
			if err := t.QueryRowContext(ctx, `SELECT result FROM retests WHERE idempotency_key=?`, key).Scan(&existing); err == nil {
				if err := t.QueryRowContext(ctx, `SELECT id,tree_code,species,location,owner,status,version,created_at,updated_at FROM cases WHERE id=?`, caseID).Scan(&c.ID, &c.TreeCode, &c.Species, &c.Location, &c.Owner, &c.Status, &c.Version, new(string), new(string)); err != nil {
					return domain.ErrNotFound
				}
				return nil
			}
		}
		if err := t.QueryRowContext(ctx, `SELECT id,tree_code,species,location,owner,status,version,created_at,updated_at FROM cases WHERE id=?`, caseID).Scan(&c.ID, &c.TreeCode, &c.Species, &c.Location, &c.Owner, &c.Status, &c.Version, new(string), new(string)); err != nil {
			return domain.ErrNotFound
		}
		if c.Version != expected {
			return domain.ErrConflict
		}
		if c.Status != domain.StatusPendingRetest {
			return fmt.Errorf("%w：案卷不在待复检", domain.ErrValidation)
		}
		now := time.Now().UTC()
		to := domain.StatusReleased
		if domain.IsFailingResult(result) {
			to = domain.StatusRejected
		}
		c.Status = to
		c.Version++
		c.UpdatedAt = now
		if _, err := t.ExecContext(ctx, `UPDATE cases SET status=?,version=?,updated_at=? WHERE id=? AND version=?`, to, c.Version, ts(now), caseID, expected); err != nil {
			return err
		}
		retestID := domain.NewID("retest")
		if _, err := t.ExecContext(ctx, `INSERT INTO retests(id,case_id,idempotency_key,result,operator,notes,performed_at) VALUES(?,?,?,?,?,?,?)`, retestID, caseID, key, result, operator, notes, ts(now)); err != nil {
			return err
		}
		if _, err := t.ExecContext(ctx, `INSERT INTO tests(id,case_id,test_type,performed_at,operator,pathogen,load,method,result,notes) VALUES(?,?,?,?,?,?,?,?,?,?)`, retestID, caseID, "复检", ts(now), operator, "复检", "", "", result, notes); err != nil {
			return err
		}
		return s.eventTx(t, domain.AuditEvent{ID: domain.NewID("event"), CaseID: caseID, Action: "复检", Detail: result, Actor: operator, CreatedAt: now})
	})
	return c, err
}
