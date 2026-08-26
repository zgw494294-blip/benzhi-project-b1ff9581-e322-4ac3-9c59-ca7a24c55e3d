package storage

import (
	"context"
	"database/sql"
	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
	"time"
)

func (s *SQLite) SummaryCounts(ctx context.Context) (map[string]int, error) {
	out := map[string]int{}
	for _, item := range []struct{ k, q string }{{"cases", "SELECT COUNT(*) FROM cases"}, {"samples", "SELECT COUNT(*) FROM samples"}, {"tests", "SELECT COUNT(*) FROM tests"}, {"risks", "SELECT COUNT(*) FROM risks"}, {"credentials", "SELECT COUNT(*) FROM credentials"}, {"events", "SELECT COUNT(*) FROM events"}} {
		var n int
		if err := s.db.QueryRowContext(ctx, item.q).Scan(&n); err != nil {
			return nil, err
		}
		out[item.k] = n
	}
	return out, nil
}

func (s *SQLite) SearchCases(ctx context.Context, status, risk, tree string, from, to time.Time, cursor string, limit int) ([]domain.Case, string, error) {
	q := `SELECT c.id,c.tree_code,c.species,c.location,c.owner,c.status,c.version,c.created_at,c.updated_at FROM cases c LEFT JOIN risks r ON r.case_id=c.id WHERE 1=1`
	args := []any{}
	if status != "" {
		q += " AND c.status=?"
		args = append(args, status)
	}
	if risk != "" {
		q += " AND r.level=?"
		args = append(args, risk)
	}
	if tree != "" {
		q += " AND c.tree_code LIKE ?"
		args = append(args, tree+"%")
	}
	if !from.IsZero() {
		q += " AND c.updated_at>=?"
		args = append(args, ts(from))
	}
	if !to.IsZero() {
		q += " AND c.updated_at<=?"
		args = append(args, ts(to))
	}
	if cursor != "" {
		q += " AND c.updated_at<?"
		args = append(args, cursor)
	}
	q += " ORDER BY c.updated_at DESC LIMIT ?"
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	out := []domain.Case{}
	for rows.Next() {
		var c domain.Case
		var a, b string
		if err := rows.Scan(&c.ID, &c.TreeCode, &c.Species, &c.Location, &c.Owner, &c.Status, &c.Version, &a, &b); err != nil {
			return nil, "", err
		}
		c.CreatedAt = parse(a)
		c.UpdatedAt = parse(b)
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(out) > limit {
		next = out[limit-1].UpdatedAt.Format(time.RFC3339Nano)
		out = out[:limit]
	}
	return out, next, nil
}

type Counts struct {
	Cases       int
	Samples     int
	Tests       int
	Risks       int
	Credentials int
	Events      int
}

func (s *SQLite) Counts(ctx context.Context) (Counts, error) {
	var c Counts
	queries := []struct {
		target *int
		query  string
	}{{&c.Cases, `SELECT COUNT(*) FROM cases`}, {&c.Samples, `SELECT COUNT(*) FROM samples`}, {&c.Tests, `SELECT COUNT(*) FROM tests`}, {&c.Risks, `SELECT COUNT(*) FROM risks`}, {&c.Credentials, `SELECT COUNT(*) FROM credentials`}, {&c.Events, `SELECT COUNT(*) FROM events`}}
	for _, item := range queries {
		if err := s.db.QueryRowContext(ctx, item.query).Scan(item.target); err != nil {
			return c, err
		}
	}
	return c, nil
}
func (s *SQLite) FindCasesByStatus(ctx context.Context, status domain.CaseStatus) ([]domain.Case, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,tree_code,species,location,owner,status,version,created_at,updated_at FROM cases WHERE status=? ORDER BY updated_at DESC`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cases := []domain.Case{}
	for rows.Next() {
		var c domain.Case
		var created, updated string
		if err := rows.Scan(&c.ID, &c.TreeCode, &c.Species, &c.Location, &c.Owner, &c.Status, &c.Version, &created, &updated); err != nil {
			return nil, err
		}
		c.CreatedAt = parse(created)
		c.UpdatedAt = parse(updated)
		cases = append(cases, c)
	}
	return cases, rows.Err()
}
func (s *SQLite) FindCredential(ctx context.Context, hash string) (domain.Credential, error) {
	var c domain.Credential
	var issued, revoked sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id,case_id,kind,summary_hash,issued_at,issued_by,revoked_at FROM credentials WHERE summary_hash=?`, hash).Scan(&c.ID, &c.CaseID, &c.Kind, &c.SummaryHash, &issued, &c.IssuedBy, &revoked)
	if err == sql.ErrNoRows {
		return c, domain.ErrNotFound
	}
	if issued.Valid {
		t := parse(issued.String)
		c.IssuedAt = &t
	}
	if revoked.Valid {
		t := parse(revoked.String)
		c.RevokedAt = &t
	}
	return c, err
}
func (s *SQLite) FindEvents(ctx context.Context, action string) ([]domain.AuditEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,case_id,action,detail,actor,created_at FROM events WHERE action=? ORDER BY created_at DESC`, action)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.AuditEvent{}
	for rows.Next() {
		var e domain.AuditEvent
		var at string
		if err = rows.Scan(&e.ID, &e.CaseID, &e.Action, &e.Detail, &e.Actor, &at); err != nil {
			return nil, err
		}
		e.CreatedAt = parse(at)
		out = append(out, e)
	}
	return out, rows.Err()
}
