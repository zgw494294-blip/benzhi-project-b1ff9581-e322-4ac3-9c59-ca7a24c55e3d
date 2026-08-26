package storage

import (
	"context"
	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
)

func (s *SQLite) Events(ctx context.Context, id string) ([]domain.AuditEvent, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT id,case_id,action,detail,actor,created_at FROM events WHERE case_id=? ORDER BY created_at`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.AuditEvent{}
	for rows.Next() {
		var x domain.AuditEvent
		var at string
		if e = rows.Scan(&x.ID, &x.CaseID, &x.Action, &x.Detail, &x.Actor, &at); e != nil {
			return nil, e
		}
		x.CreatedAt = parse(at)
		out = append(out, x)
	}
	return out, rows.Err()
}
