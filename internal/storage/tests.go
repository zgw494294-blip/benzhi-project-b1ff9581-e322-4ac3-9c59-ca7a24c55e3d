package storage

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
)

func (s *SQLite) AddTest(ctx context.Context, x domain.TestResult) error {
	return tx(ctx, s.db, func(t *sql.Tx) error {
		if e := domain.ValidateTest(x); e != nil {
			return e
		}
		var old int
		var status string
		if e := t.QueryRowContext(ctx, `SELECT version,status FROM cases WHERE id=?`, x.CaseID).Scan(&old, &status); e != nil {
			if e == sql.ErrNoRows {
				return domain.ErrNotFound
			}
			return e
		}
		if status != string(domain.StatusPendingTest) && status != string(domain.StatusPendingReview) {
			return fmt.Errorf("%w：案卷当前不可录入实验", domain.ErrValidation)
		}
		if _, e := t.ExecContext(ctx, `INSERT INTO tests(id,case_id,test_type,performed_at,operator,pathogen,load,method,result,notes) VALUES(?,?,?,?,?,?,?,?,?,?)`, x.ID, x.CaseID, x.TestType, ts(x.PerformedAt), x.Operator, x.Pathogen, x.Load, x.Method, x.Result, x.Notes); e != nil {
			return e
		}
		c := domain.Case{ID: x.CaseID, Status: domain.CaseStatus(status), Version: old, UpdatedAt: x.PerformedAt}
		if status == string(domain.StatusPendingTest) {
			if err := c.Advance(domain.StatusPendingReview); err != nil {
				return err
			}
		} else {
			c.Version++
			c.UpdatedAt = x.PerformedAt
		}
		if _, e := t.ExecContext(ctx, `UPDATE cases SET status=?,version=?,updated_at=?,pending_review_since=CASE WHEN ?=? THEN COALESCE(pending_review_since,?) ELSE pending_review_since END WHERE id=? AND version=?`, c.Status, c.Version, ts(c.UpdatedAt), status, string(domain.StatusPendingTest), ts(x.PerformedAt), c.ID, old); e != nil {
			return e
		}
		level, factors, mitigation := "低风险", "", "常规养护并季度复查"
		var sampleCount, maxSeq int
		if err := t.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(MAX(sequence),0) FROM samples WHERE case_id=?`, x.CaseID).Scan(&sampleCount, &maxSeq); err != nil {
			return err
		}
		sampleOK := sampleCount > 0 && sampleCount == maxSeq
		rows, err := t.QueryContext(ctx, `SELECT id,test_type,pathogen,method,load FROM tests WHERE case_id=?`, x.CaseID)
		if err != nil {
			return err
		}
		max := 0
		for rows.Next() {
			var id, testType, pathogen, method, load string
			if err := rows.Scan(&id, &testType, &pathogen, &method, &load); err != nil {
				rows.Close()
				return err
			}
			if !domain.TestTypeKnown(testType) || !domain.MethodKnown(method) {
				rows.Close()
				return fmt.Errorf("%w：已有实验目录数据无效", domain.ErrValidation)
			}
			if _, ok := domain.FindPathogen(pathogen); !ok {
				rows.Close()
				return fmt.Errorf("%w：已有病原数据无效", domain.ErrValidation)
			}
			test := domain.TestResult{ID: id, Pathogen: pathogen, Method: method, Load: load}
			l, _, m := domain.AssessRisk(test, sampleOK)
			if n := domain.RiskOrder(l); n > max {
				max, level, mitigation = n, l, m
			}
			factors += pathogen + "/" + method + "/" + domain.NormalizeLoad(load) + "（" + id + "）;"
		}
		rows.Close()
		if factors == "" {
			factors = "暂无实验结果"
		}
		if sampleOK {
			factors += "样本链完整"
		} else {
			factors += "样本链不完整"
		}
		_, err = t.ExecContext(ctx, `INSERT INTO risks(id,case_id,level,factors,decision,mitigation,reviewer,reviewed_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(case_id) DO UPDATE SET level=excluded.level,factors=excluded.factors,mitigation=excluded.mitigation,reviewed_at=excluded.reviewed_at`, domain.NewID("risk"), x.CaseID, level, factors, "待人工复核", mitigation, "", ts(x.PerformedAt))
		if err != nil {
			return err
		}
		return s.eventTx(t, domain.AuditEvent{ID: domain.NewID("event"), CaseID: x.CaseID, Action: "实验录入", Detail: x.Pathogen + " / " + x.Load, Actor: x.Operator, CreatedAt: x.PerformedAt})
	})
}
func (s *SQLite) ListTests(ctx context.Context, id string) ([]domain.TestResult, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT id,case_id,test_type,performed_at,operator,pathogen,load,method,result,notes,invalidated_at,invalidated_by,invalidation_reason,replaces_test_id,correction_request_id FROM tests WHERE case_id=? ORDER BY performed_at`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.TestResult{}
	for rows.Next() {
		var x domain.TestResult
		var p string
		var invalidated, invalidatedBy, invalidationReason, replacesTestID, correctionRequestID sql.NullString
		if e = rows.Scan(&x.ID, &x.CaseID, &x.TestType, &p, &x.Operator, &x.Pathogen, &x.Load, &x.Method, &x.Result, &x.Notes, &invalidated, &invalidatedBy, &invalidationReason, &replacesTestID, &correctionRequestID); e != nil {
			return nil, e
		}
		x.PerformedAt = parse(p)
		x.InvalidatedBy = invalidatedBy.String
		x.InvalidationReason = invalidationReason.String
		x.ReplacesTestID = replacesTestID.String
		x.CorrectionRequestID = correctionRequestID.String
		if invalidated.Valid {
			t := parse(invalidated.String)
			x.InvalidatedAt = &t
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
