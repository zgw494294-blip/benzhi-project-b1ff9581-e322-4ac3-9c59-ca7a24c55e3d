package storage

import (
	"context"
	"database/sql"
	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
	_ "github.com/mattn/go-sqlite3"
	"time"
)

type SQLite struct{ db *sql.DB }

func Open(path string) (*SQLite, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &SQLite{db: db}
	if err = s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}
func (s *SQLite) Close() error { return s.db.Close() }
func (s *SQLite) migrate() error {
	_, err := s.db.Exec(`PRAGMA foreign_keys=ON; CREATE TABLE IF NOT EXISTS cases(id TEXT PRIMARY KEY,tree_code TEXT,species TEXT,location TEXT,owner TEXT,status TEXT,version INTEGER,created_at TEXT,updated_at TEXT,pending_review_since TEXT); CREATE TABLE IF NOT EXISTS samples(id TEXT PRIMARY KEY,case_id TEXT,sample_code TEXT,collected_at TEXT,collector TEXT,seal_code TEXT,handoff_at TEXT,receiver TEXT,condition TEXT,sequence INTEGER); CREATE TABLE IF NOT EXISTS tests(id TEXT PRIMARY KEY,case_id TEXT,test_type TEXT,performed_at TEXT,operator TEXT,pathogen TEXT,load TEXT,method TEXT,result TEXT,notes TEXT,invalidated_at TEXT,invalidated_by TEXT,invalidation_reason TEXT,replaces_test_id TEXT,correction_request_id TEXT); CREATE TABLE IF NOT EXISTS risks(id TEXT PRIMARY KEY,case_id TEXT UNIQUE,level TEXT,factors TEXT,decision TEXT,mitigation TEXT,reviewer TEXT,reviewed_at TEXT); CREATE TABLE IF NOT EXISTS credentials(id TEXT PRIMARY KEY,case_id TEXT UNIQUE,kind TEXT,summary_hash TEXT,issued_at TEXT,issued_by TEXT,revoked_at TEXT); CREATE TABLE IF NOT EXISTS events(id TEXT PRIMARY KEY,case_id TEXT,action TEXT,detail TEXT,actor TEXT,created_at TEXT); CREATE TABLE IF NOT EXISTS retests(id TEXT PRIMARY KEY,case_id TEXT UNIQUE,idempotency_key TEXT UNIQUE,result TEXT,operator TEXT,notes TEXT,performed_at TEXT); CREATE TABLE IF NOT EXISTS handoff_exceptions(id TEXT PRIMARY KEY,case_id TEXT,sample_code TEXT,collected_at TEXT,collector TEXT,seal_code TEXT,handoff_at TEXT,receiver TEXT,condition TEXT,sequence INTEGER,reason TEXT,occurred_at TEXT,correction TEXT,new_seal_code TEXT,resolved_sample_id TEXT,closed_at TEXT); CREATE TABLE IF NOT EXISTS treatment_items(id TEXT PRIMARY KEY,case_id TEXT,content TEXT,assignee TEXT,planned_at TEXT,required INTEGER,completed_at TEXT,evidence TEXT,created_at TEXT); CREATE TABLE IF NOT EXISTS verification_receipts(id TEXT PRIMARY KEY,requested_at TEXT,valid_count INTEGER,invalid_count INTEGER); CREATE TABLE IF NOT EXISTS verification_items(receipt_id TEXT,credential_id TEXT,conclusion TEXT,tree_code TEXT,case_status TEXT,summary_hash TEXT); CREATE TABLE IF NOT EXISTS corrections(request_id TEXT PRIMARY KEY,replacement_test_id TEXT,case_id TEXT,created_at TEXT); CREATE UNIQUE INDEX IF NOT EXISTS samples_case_sample ON samples(case_id,sample_code); CREATE UNIQUE INDEX IF NOT EXISTS samples_case_seal ON samples(case_id,seal_code);`)
	if err != nil {
		return err
	}
	for _, stmt := range []string{
		`ALTER TABLE cases ADD COLUMN pending_review_since TEXT`,
		`ALTER TABLE tests ADD COLUMN invalidated_at TEXT`, `ALTER TABLE tests ADD COLUMN invalidated_by TEXT`, `ALTER TABLE tests ADD COLUMN invalidation_reason TEXT`, `ALTER TABLE tests ADD COLUMN replaces_test_id TEXT`, `ALTER TABLE tests ADD COLUMN correction_request_id TEXT`,
	} {
		_, _ = s.db.Exec(stmt)
	}
	_, _ = s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS tests_correction_request ON tests(correction_request_id)`)
	return nil
}
func tx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	t, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err = fn(t); err != nil {
		t.Rollback()
		return err
	}
	return t.Commit()
}
func ts(t time.Time) string    { return t.UTC().Format(time.RFC3339Nano) }
func parse(v string) time.Time { t, _ := time.Parse(time.RFC3339Nano, v); return t }
func (s *SQLite) eventTx(t *sql.Tx, e domain.AuditEvent) error {
	_, err := t.Exec(`INSERT INTO events(id,case_id,action,detail,actor,created_at) VALUES(?,?,?,?,?,?)`, e.ID, e.CaseID, e.Action, e.Detail, e.Actor, ts(e.CreatedAt))
	return err
}
