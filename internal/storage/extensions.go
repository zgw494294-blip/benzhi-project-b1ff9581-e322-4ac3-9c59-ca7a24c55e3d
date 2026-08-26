package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
)

func (s *SQLite) RecordHandoffException(ctx context.Context, x domain.HandoffException) error {
	if x.ID == "" {
		x.ID = domain.NewID("handoff-exception")
	}
	if x.OccurredAt.IsZero() {
		x.OccurredAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO handoff_exceptions(id,case_id,sample_code,collected_at,collector,seal_code,handoff_at,receiver,condition,sequence,reason,occurred_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, x.ID, x.CaseID, x.SampleCode, ts(x.CollectedAt), x.Collector, x.SealCode, ts(x.HandoffAt), x.Receiver, x.Condition, x.Sequence, x.Reason, ts(x.OccurredAt))
	return err
}

func (s *SQLite) ListHandoffExceptions(ctx context.Context, caseID string) ([]domain.HandoffException, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,case_id,sample_code,collected_at,collector,seal_code,handoff_at,receiver,condition,sequence,reason,occurred_at,correction,new_seal_code,resolved_sample_id,closed_at FROM handoff_exceptions WHERE case_id=? ORDER BY occurred_at`, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.HandoffException
	for rows.Next() {
		var x domain.HandoffException
		var a, b, c, d sql.NullString
		if err := rows.Scan(&x.ID, &x.CaseID, &x.SampleCode, &a, &x.Collector, &x.SealCode, &b, &x.Receiver, &x.Condition, &x.Sequence, &x.Reason, &c, &x.Correction, &x.NewSealCode, &x.ResolvedSampleID, &d); err != nil {
			return nil, err
		}
		x.CollectedAt = parse(a.String)
		x.HandoffAt = parse(b.String)
		x.OccurredAt = parse(c.String)
		if d.Valid {
			t := parse(d.String)
			x.ClosedAt = &t
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *SQLite) Rehandoff(ctx context.Context, exceptionID, newSeal, correction, actor string, expected int) (domain.SampleChain, error) {
	var out domain.SampleChain
	err := tx(ctx, s.db, func(t *sql.Tx) error {
		var x domain.HandoffException
		var a, b, c, d sql.NullString
		if err := t.QueryRowContext(ctx, `SELECT id,case_id,sample_code,collected_at,collector,seal_code,handoff_at,receiver,condition,sequence,reason,occurred_at,closed_at FROM handoff_exceptions WHERE id=?`, exceptionID).Scan(&x.ID, &x.CaseID, &x.SampleCode, &a, &x.Collector, &x.SealCode, &b, &x.Receiver, &x.Condition, &x.Sequence, &x.Reason, &c, &d); err != nil {
			return domain.ErrNotFound
		}
		if d.Valid {
			return fmt.Errorf("%w：异常记录已闭环", domain.ErrValidation)
		}
		if strings.TrimSpace(newSeal) == "" || strings.TrimSpace(newSeal) == x.SealCode || strings.TrimSpace(correction) == "" || !domain.ValidateActor(actor) {
			return fmt.Errorf("%w：重新封签必须使用新封签并填写纠正说明", domain.ErrValidation)
		}
		var status string
		var v int
		if err := t.QueryRowContext(ctx, `SELECT version,status FROM cases WHERE id=?`, x.CaseID).Scan(&v, &status); err != nil {
			return domain.ErrNotFound
		}
		if v != expected {
			return domain.ErrConflict
		}
		if status != string(domain.StatusPendingSample) && status != string(domain.StatusPendingTest) {
			return fmt.Errorf("%w：案卷当前不可重新交接", domain.ErrValidation)
		}
		var cnt int
		_ = t.QueryRowContext(ctx, `SELECT COUNT(*) FROM samples WHERE case_id=?`, x.CaseID).Scan(&cnt)
		out = domain.SampleChain{ID: domain.NewID("sample"), CaseID: x.CaseID, SampleCode: x.SampleCode, Collector: x.Collector, SealCode: strings.TrimSpace(newSeal), Receiver: x.Receiver, Condition: "封签完整", CollectedAt: parse(a.String), HandoffAt: time.Now().UTC(), Sequence: cnt + 1}
		if err := domain.ValidateSample(out, cnt); err != nil {
			return err
		}
		if _, err := t.ExecContext(ctx, `INSERT INTO samples VALUES(?,?,?,?,?,?,?,?,?,?)`, out.ID, out.CaseID, out.SampleCode, ts(out.CollectedAt), out.Collector, out.SealCode, ts(out.HandoffAt), out.Receiver, out.Condition, out.Sequence); err != nil {
			return err
		}
		if _, err := t.ExecContext(ctx, `UPDATE cases SET status=?,version=?,updated_at=? WHERE id=? AND version=?`, domain.StatusPendingTest, v+1, ts(out.HandoffAt), x.CaseID, expected); err != nil {
			return err
		}
		if _, err := t.ExecContext(ctx, `UPDATE handoff_exceptions SET correction=?,new_seal_code=?,resolved_sample_id=?,closed_at=? WHERE id=? AND closed_at IS NULL`, correction, newSeal, out.ID, ts(out.HandoffAt), exceptionID); err != nil {
			return err
		}
		if err := s.eventTx(t, domain.AuditEvent{ID: domain.NewID("event"), CaseID: x.CaseID, Action: "重新封签", Detail: exceptionID + " / " + correction, Actor: actor, CreatedAt: out.HandoffAt}); err != nil {
			return err
		}
		return s.eventTx(t, domain.AuditEvent{ID: domain.NewID("event"), CaseID: x.CaseID, Action: "样本交接", Detail: out.SampleCode + " 已完成重新封签交接", Actor: out.Receiver, CreatedAt: out.HandoffAt})
	})
	return out, err
}

func (s *SQLite) InvalidateAndReplaceTest(ctx context.Context, caseID, originalID string, replacement domain.TestResult, corrector, reason, requestID string, expected int) (domain.TestResult, error) {
	var out domain.TestResult
	err := tx(ctx, s.db, func(t *sql.Tx) error {
		if err := t.QueryRowContext(ctx, `SELECT replacement_test_id FROM corrections WHERE request_id=?`, requestID).Scan(&out.ID); err == nil {
			return t.QueryRowContext(ctx, `SELECT id,case_id,test_type,performed_at,operator,pathogen,load,method,result,notes,invalidated_at,invalidated_by,invalidation_reason,replaces_test_id,correction_request_id FROM tests WHERE id=?`, out.ID).Scan(&out.ID, &out.CaseID, &out.TestType, new(string), &out.Operator, &out.Pathogen, &out.Load, &out.Method, &out.Result, &out.Notes, new(sql.NullString), &out.InvalidatedBy, &out.InvalidationReason, &out.ReplacesTestID, &out.CorrectionRequestID)
		}
		var status string
		var version int
		if err := t.QueryRowContext(ctx, `SELECT status,version FROM cases WHERE id=?`, caseID).Scan(&status, &version); err != nil {
			return domain.ErrNotFound
		}
		if version != expected {
			return domain.ErrConflict
		}
		if status != string(domain.StatusPendingReview) {
			return fmt.Errorf("%w：仅待复核案卷允许实验更正", domain.ErrValidation)
		}
		var original domain.TestResult
		var p, ia sql.NullString
		if err := t.QueryRowContext(ctx, `SELECT id,case_id,test_type,performed_at,operator,pathogen,load,method,result,notes,invalidated_at FROM tests WHERE id=? AND case_id=?`, originalID, caseID).Scan(&original.ID, &original.CaseID, &original.TestType, &p, &original.Operator, &original.Pathogen, &original.Load, &original.Method, &original.Result, &original.Notes, &ia); err != nil {
			return domain.ErrNotFound
		}
		if ia.Valid {
			return fmt.Errorf("%w：原实验已失效", domain.ErrValidation)
		}
		original.PerformedAt = parse(p.String)
		var handoff string
		if err := t.QueryRowContext(ctx, `SELECT handoff_at FROM samples WHERE case_id=? ORDER BY sequence DESC LIMIT 1`, caseID).Scan(&handoff); err != nil {
			return err
		}
		if err := domain.ValidateCorrection(original, replacement, parse(handoff), time.Now().UTC(), corrector, reason, requestID); err != nil {
			return err
		}
		out = replacement
		if out.ID == "" {
			out.ID = domain.NewID("test")
		}
		now := time.Now().UTC()
		if _, err := t.ExecContext(ctx, `UPDATE tests SET invalidated_at=?,invalidated_by=?,invalidation_reason=? WHERE id=?`, ts(now), corrector, reason, originalID); err != nil {
			return err
		}
		if _, err := t.ExecContext(ctx, `INSERT INTO tests(id,case_id,test_type,performed_at,operator,pathogen,load,method,result,notes,replaces_test_id,correction_request_id) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, out.ID, caseID, out.TestType, ts(out.PerformedAt), out.Operator, out.Pathogen, out.Load, out.Method, out.Result, out.Notes, originalID, requestID); err != nil {
			return err
		}
		if _, err := t.ExecContext(ctx, `INSERT INTO corrections(request_id,replacement_test_id,case_id,created_at) VALUES(?,?,?,?)`, requestID, out.ID, caseID, ts(now)); err != nil {
			return err
		}
		var cnt, maxseq int
		_ = t.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(MAX(sequence),0) FROM samples WHERE case_id=?`, caseID).Scan(&cnt, &maxseq)
		level, factors, mitigation := domain.HighestRisk(nil, cnt > 0 && cnt == maxseq)
		rows, _ := t.QueryContext(ctx, `SELECT id,pathogen,method,load FROM tests WHERE case_id=? AND invalidated_at IS NULL`, caseID)
		var tests []domain.TestResult
		for rows.Next() {
			var z domain.TestResult
			rows.Scan(&z.ID, &z.Pathogen, &z.Method, &z.Load)
			tests = append(tests, z)
		}
		rows.Close()
		level, factors, mitigation = domain.HighestRisk(tests, cnt > 0 && cnt == maxseq)
		if _, err := t.ExecContext(ctx, `UPDATE risks SET level=?,factors=?,mitigation=?,decision='待人工复核',reviewer='',reviewed_at=? WHERE case_id=?`, level, factors, mitigation, ts(now), caseID); err != nil {
			return err
		}
		if _, err := t.ExecContext(ctx, `UPDATE cases SET version=?,updated_at=? WHERE id=? AND version=?`, version+1, ts(now), caseID, expected); err != nil {
			return err
		}
		return s.eventTx(t, domain.AuditEvent{ID: domain.NewID("event"), CaseID: caseID, Action: "实验更正", Detail: originalID + " -> " + out.ID + " / " + reason, Actor: corrector, CreatedAt: now})
	})
	return out, err
}

func (s *SQLite) AddTreatmentItem(ctx context.Context, item domain.TreatmentItem, expected int) error {
	return tx(ctx, s.db, func(t *sql.Tx) error {
		var v int
		var status string
		if err := t.QueryRowContext(ctx, `SELECT version,status FROM cases WHERE id=?`, item.CaseID).Scan(&v, &status); err != nil {
			return domain.ErrNotFound
		}
		if v != expected {
			return domain.ErrConflict
		}
		if status != string(domain.StatusPendingRetest) {
			return fmt.Errorf("%w：仅复核通过案卷允许新增处置项", domain.ErrValidation)
		}
		if err := domain.ValidateTreatmentItem(item, time.Now().UTC()); err != nil {
			return err
		}
		if _, err := t.ExecContext(ctx, `INSERT INTO treatment_items(id,case_id,content,assignee,planned_at,required,created_at) VALUES(?,?,?,?,?,?,?)`, item.ID, item.CaseID, item.Content, item.Assignee, ts(item.PlannedAt), boolInt(item.Required), ts(item.CreatedAt)); err != nil {
			return err
		}
		_, err := t.ExecContext(ctx, `UPDATE cases SET version=?,updated_at=? WHERE id=? AND version=?`, v+1, ts(item.CreatedAt), item.CaseID, expected)
		if err != nil {
			return err
		}
		return s.eventTx(t, domain.AuditEvent{ID: domain.NewID("event"), CaseID: item.CaseID, Action: "新增处置项", Detail: item.Content, Actor: item.Assignee, CreatedAt: item.CreatedAt})
	})
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
func (s *SQLite) ListTreatmentItems(ctx context.Context, caseID string) ([]domain.TreatmentItem, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT id,case_id,content,assignee,planned_at,required,completed_at,evidence,created_at FROM treatment_items WHERE case_id=? ORDER BY created_at`, caseID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.TreatmentItem
	for rows.Next() {
		var x domain.TreatmentItem
		var p, c, cr sql.NullString
		var req int
		if e = rows.Scan(&x.ID, &x.CaseID, &x.Content, &x.Assignee, &p, &req, &c, &x.Evidence, &cr); e != nil {
			return nil, e
		}
		x.PlannedAt = parse(p.String)
		x.Required = req == 1
		x.CreatedAt = parse(cr.String)
		if c.Valid {
			t := parse(c.String)
			x.CompletedAt = &t
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *SQLite) CompleteTreatmentItem(ctx context.Context, caseID, itemID, assignee, evidence string, completed time.Time, expected int) error {
	return tx(ctx, s.db, func(t *sql.Tx) error {
		var v int
		var status string
		if e := t.QueryRowContext(ctx, `SELECT version,status FROM cases WHERE id=?`, caseID).Scan(&v, &status); e != nil {
			return domain.ErrNotFound
		}
		if v != expected {
			return domain.ErrConflict
		}
		var item domain.TreatmentItem
		var p, c, cr sql.NullString
		var req int
		if e := t.QueryRowContext(ctx, `SELECT id,case_id,content,assignee,planned_at,required,completed_at,evidence,created_at FROM treatment_items WHERE id=? AND case_id=?`, itemID, caseID).Scan(&item.ID, &item.CaseID, &item.Content, &item.Assignee, &p, &req, &c, &item.Evidence, &cr); e != nil {
			return domain.ErrNotFound
		}
		item.PlannedAt = parse(p.String)
		item.Required = req == 1
		item.CreatedAt = parse(cr.String)
		if c.Valid {
			t := parse(c.String)
			item.CompletedAt = &t
		}
		if e := domain.ValidateTreatmentCompletion(item, assignee, evidence, completed, time.Now().UTC()); e != nil {
			return e
		}
		if _, e := t.ExecContext(ctx, `UPDATE treatment_items SET completed_at=?,evidence=? WHERE id=? AND completed_at IS NULL`, ts(completed), evidence, itemID); e != nil {
			return e
		}
		if _, e := t.ExecContext(ctx, `UPDATE cases SET version=?,updated_at=? WHERE id=? AND version=?`, v+1, ts(completed), caseID, expected); e != nil {
			return e
		}
		return s.eventTx(t, domain.AuditEvent{ID: domain.NewID("event"), CaseID: caseID, Action: "处置项完成", Detail: item.Content, Actor: assignee, CreatedAt: completed})
	})
}
func (s *SQLite) RetestReadiness(ctx context.Context, caseID string) ([]string, error) {
	items, e := s.ListTreatmentItems(ctx, caseID)
	if e != nil {
		return nil, e
	}
	var missing []string
	for _, x := range items {
		if x.Required && x.CompletedAt == nil {
			missing = append(missing, x.Content)
		}
	}
	return missing, nil
}
