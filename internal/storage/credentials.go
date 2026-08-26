package storage

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
	"strings"
	"time"
)

func (s *SQLite) RevokeCredential(ctx context.Context, id, actor, reason string) (domain.Credential, error) {
	if !domain.ValidateActor(actor) || strings.TrimSpace(reason) == "" {
		return domain.Credential{}, fmt.Errorf("%w：撤销人和原因不能为空", domain.ErrValidation)
	}
	var c domain.Credential
	err := tx(ctx, s.db, func(t *sql.Tx) error {
		var issued, revoked sql.NullString
		if err := t.QueryRowContext(ctx, `SELECT id,case_id,kind,summary_hash,issued_at,issued_by,revoked_at FROM credentials WHERE id=?`, id).Scan(&c.ID, &c.CaseID, &c.Kind, &c.SummaryHash, &issued, &c.IssuedBy, &revoked); err != nil {
			return domain.ErrNotFound
		}
		if issued.Valid {
			x := parse(issued.String)
			c.IssuedAt = &x
		}
		if revoked.Valid {
			x := parse(revoked.String)
			c.RevokedAt = &x
		}
		if c.RevokedAt != nil {
			return nil
		}
		now := time.Now().UTC()
		c.RevokedAt = &now
		if _, err := t.ExecContext(ctx, `UPDATE credentials SET revoked_at=? WHERE id=? AND revoked_at IS NULL`, ts(now), id); err != nil {
			return err
		}
		return s.eventTx(t, domain.AuditEvent{ID: domain.NewID("event"), CaseID: c.CaseID, Action: "凭据撤销", Detail: reason, Actor: actor, CreatedAt: now})
	})
	if err != nil {
		return c, err
	}
	return c, nil
}

func (s *SQLite) SaveCredential(ctx context.Context, c domain.Credential) error {
	if c.IssuedAt == nil {
		return fmt.Errorf("%w：签发时间不能为空", domain.ErrValidation)
	}
	return tx(ctx, s.db, func(t *sql.Tx) error {
		if _, e := t.ExecContext(ctx, `INSERT INTO credentials(id,case_id,kind,summary_hash,issued_at,issued_by,revoked_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(case_id) DO NOTHING`, c.ID, c.CaseID, c.Kind, c.SummaryHash, ts(*c.IssuedAt), c.IssuedBy, nil); e != nil {
			return e
		}
		return s.eventTx(t, domain.AuditEvent{ID: domain.NewID("event"), CaseID: c.CaseID, Action: "凭据签发", Detail: c.ID, Actor: c.IssuedBy, CreatedAt: *c.IssuedAt})
	})
}
func (s *SQLite) GetCredential(ctx context.Context, id string) (domain.Credential, error) {
	var c domain.Credential
	var issued, rev sql.NullString
	e := s.db.QueryRowContext(ctx, `SELECT id,case_id,kind,summary_hash,issued_at,issued_by,revoked_at FROM credentials WHERE id=?`, id).Scan(&c.ID, &c.CaseID, &c.Kind, &c.SummaryHash, &issued, &c.IssuedBy, &rev)
	if e == sql.ErrNoRows {
		return c, domain.ErrNotFound
	}
	if issued.Valid {
		t := parse(issued.String)
		c.IssuedAt = &t
	}
	if rev.Valid {
		t := parse(rev.String)
		c.RevokedAt = &t
	}
	return c, e
}
func (s *SQLite) GetCredentialByCase(ctx context.Context, id string) (domain.Credential, error) {
	var c domain.Credential
	var issued, rev sql.NullString
	e := s.db.QueryRowContext(ctx, `SELECT id,case_id,kind,summary_hash,issued_at,issued_by,revoked_at FROM credentials WHERE case_id=?`, id).Scan(&c.ID, &c.CaseID, &c.Kind, &c.SummaryHash, &issued, &c.IssuedBy, &rev)
	if e == sql.ErrNoRows {
		return c, domain.ErrNotFound
	}
	if issued.Valid {
		t := parse(issued.String)
		c.IssuedAt = &t
	}
	if rev.Valid {
		t := parse(rev.String)
		c.RevokedAt = &t
	}
	return c, e
}
