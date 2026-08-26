package storage

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
	"time"
)

func (s *SQLite) ReviewCase(ctx context.Context, caseID, decision, mitigation, reviewer string, expected int) (domain.RiskAssessment, error) {
	var out domain.RiskAssessment
	err := tx(ctx, s.db, func(t *sql.Tx) error {
		var status string
		var version int
		if err := t.QueryRowContext(ctx, `SELECT status,version FROM cases WHERE id=?`, caseID).Scan(&status, &version); err != nil {
			return domain.ErrNotFound
		}
		if version != expected {
			return domain.ErrConflict
		}
		if status != string(domain.StatusPendingReview) {
			return fmt.Errorf("%w：案卷不在待复核", domain.ErrValidation)
		}
		var at string
		if err := t.QueryRowContext(ctx, `SELECT id,case_id,level,factors,decision,mitigation,reviewer,reviewed_at FROM risks WHERE case_id=?`, caseID).Scan(&out.ID, &out.CaseID, &out.Level, &out.Factors, &out.Decision, &out.Mitigation, &out.Reviewer, &at); err != nil {
			return domain.ErrNotFound
		}
		out.Decision, out.Mitigation, out.Reviewer, out.ReviewedAt = decision, mitigation, reviewer, time.Now().UTC()
		to := domain.StatusRejected
		if decision == "通过" {
			to = domain.StatusPendingRetest
		}
		if _, err := t.ExecContext(ctx, `UPDATE cases SET status=?,version=?,updated_at=? WHERE id=? AND version=?`, to, expected+1, ts(out.ReviewedAt), caseID, expected); err != nil {
			return err
		}
		if _, err := t.ExecContext(ctx, `UPDATE risks SET decision=?,mitigation=?,reviewer=?,reviewed_at=? WHERE case_id=?`, decision, mitigation, reviewer, ts(out.ReviewedAt), caseID); err != nil {
			return err
		}
		return s.eventTx(t, domain.AuditEvent{ID: domain.NewID("event"), CaseID: caseID, Action: "人工复核", Detail: decision + "（版本" + fmt.Sprint(expected) + "）", Actor: reviewer, CreatedAt: out.ReviewedAt})
	})
	return out, err
}

func (s *SQLite) SaveRisk(ctx context.Context, r domain.RiskAssessment) error {
	_, e := s.db.ExecContext(ctx, `INSERT INTO risks VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(case_id) DO UPDATE SET level=excluded.level,factors=excluded.factors,decision=excluded.decision,mitigation=excluded.mitigation,reviewer=excluded.reviewer,reviewed_at=excluded.reviewed_at`, r.ID, r.CaseID, r.Level, r.Factors, r.Decision, r.Mitigation, r.Reviewer, ts(r.ReviewedAt))
	return e
}
func (s *SQLite) GetRisk(ctx context.Context, id string) (domain.RiskAssessment, error) {
	var r domain.RiskAssessment
	var at string
	e := s.db.QueryRowContext(ctx, `SELECT id,case_id,level,factors,decision,mitigation,reviewer,reviewed_at FROM risks WHERE case_id=?`, id).Scan(&r.ID, &r.CaseID, &r.Level, &r.Factors, &r.Decision, &r.Mitigation, &r.Reviewer, &at)
	if e == sql.ErrNoRows {
		return r, domain.ErrNotFound
	}
	r.ReviewedAt = parse(at)
	return r, e
}
