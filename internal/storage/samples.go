package storage

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
)

func (s *SQLite) AddSample(ctx context.Context, x domain.SampleChain, expected int) error {
	return tx(ctx, s.db, func(t *sql.Tx) error {
		var v int
		var status string
		if e := t.QueryRowContext(ctx, `SELECT version,status FROM cases WHERE id=?`, x.CaseID).Scan(&v, &status); e != nil {
			return e
		}
		if v != expected {
			return domain.ErrConflict
		}
		if status != string(domain.StatusPendingSample) && status != string(domain.StatusPendingTest) {
			return fmt.Errorf("%w：案卷当前不可采样", domain.ErrValidation)
		}
		var n int
		if err := t.QueryRowContext(ctx, `SELECT COUNT(*) FROM samples WHERE case_id=?`, x.CaseID).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			var prev string
			if err := t.QueryRowContext(ctx, `SELECT handoff_at FROM samples WHERE case_id=? ORDER BY sequence DESC LIMIT 1`, x.CaseID).Scan(&prev); err == nil && x.CollectedAt.After(parse(prev)) {
				return fmt.Errorf("%w：采集时间晚于前一条交接", domain.ErrValidation)
			}
		}
		if e := domain.ValidateSample(x, n); e != nil {
			return e
		}
		if _, e := t.ExecContext(ctx, `INSERT INTO samples VALUES(?,?,?,?,?,?,?,?,?,?)`, x.ID, x.CaseID, x.SampleCode, ts(x.CollectedAt), x.Collector, x.SealCode, ts(x.HandoffAt), x.Receiver, x.Condition, x.Sequence); e != nil {
			return e
		}
		if _, e := t.ExecContext(ctx, `UPDATE cases SET status=?,version=?,updated_at=? WHERE id=? AND version=?`, domain.StatusPendingTest, v+1, ts(x.HandoffAt), x.CaseID, expected); e != nil {
			return e
		}
		return s.eventTx(t, domain.AuditEvent{ID: domain.NewID("event"), CaseID: x.CaseID, Action: "样本交接", Detail: x.SampleCode + " 已完成封签交接", Actor: x.Receiver, CreatedAt: x.HandoffAt})
	})
}
func (s *SQLite) sampleCount(ctx context.Context, id string) int {
	var n int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM samples WHERE case_id=?`, id).Scan(&n)
	return n
}
func (s *SQLite) ListSamples(ctx context.Context, id string) ([]domain.SampleChain, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT id,case_id,sample_code,collected_at,collector,seal_code,handoff_at,receiver,condition,sequence FROM samples WHERE case_id=? ORDER BY sequence`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.SampleChain{}
	for rows.Next() {
		var x domain.SampleChain
		var a, b string
		if e = rows.Scan(&x.ID, &x.CaseID, &x.SampleCode, &a, &x.Collector, &x.SealCode, &b, &x.Receiver, &x.Condition, &x.Sequence); e != nil {
			return nil, e
		}
		x.CollectedAt = parse(a)
		x.HandoffAt = parse(b)
		out = append(out, x)
	}
	return out, rows.Err()
}
