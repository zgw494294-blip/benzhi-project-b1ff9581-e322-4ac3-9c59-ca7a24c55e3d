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
		// 幂等重放与冲突判定必须在状态/版本校验之前完成，否则首次提交已推进状态后，
		// 客户端以相同幂等键与版本重放会在状态校验处被拒绝，无法返回首次结果。
		if key != "" {
			var prevCaseID, prevResult, prevOperator, prevNotes sql.NullString
			if qErr := t.QueryRowContext(ctx, `SELECT case_id,result,operator,notes FROM retests WHERE idempotency_key=?`, key).Scan(&prevCaseID, &prevResult, &prevOperator, &prevNotes); qErr == nil {
				if prevCaseID.String != caseID || prevResult.String != result || prevOperator.String != operator || prevNotes.String != notes {
					return domain.ErrConflict
				}
				if err := t.QueryRowContext(ctx, `SELECT id,tree_code,species,location,owner,status,version,created_at,updated_at FROM cases WHERE id=?`, caseID).Scan(&c.ID, &c.TreeCode, &c.Species, &c.Location, &c.Owner, &c.Status, &c.Version, new(string), new(string)); err != nil {
					return domain.ErrNotFound
				}
				return nil
			} else if qErr != sql.ErrNoRows {
				return qErr
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
