package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
)

func (s *SQLite) VerifyCredentialBatch(ctx context.Context, ids []string) (domain.CredentialVerificationReceipt, error) {
	now := time.Now().UTC()
	out := domain.CredentialVerificationReceipt{ID: domain.NewID("receipt"), RequestedAt: now}
	seen := map[string]bool{}
	var clean []string
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if !seen[id] {
			seen[id] = true
			clean = append(clean, id)
		}
	}
	if len(clean) == 0 {
		return out, fmt.Errorf("%w：核验批次不能为空", domain.ErrValidation)
	}
	if len(clean) > domain.MaxCredentialBatch {
		return out, fmt.Errorf("%w：核验批次超过%d条", domain.ErrValidation, domain.MaxCredentialBatch)
	}
	err := tx(ctx, s.db, func(t *sql.Tx) error {
		for _, id := range clean {
			item := domain.CredentialVerificationItem{CredentialID: id, Conclusion: "未找到"}
			if !domain.ValidateCredentialID(id) {
				item.Conclusion = "编号格式非法"
			} else {
				var c domain.Credential
				var issued, rev sql.NullString
				if e := t.QueryRowContext(ctx, `SELECT id,case_id,kind,summary_hash,issued_at,issued_by,revoked_at FROM credentials WHERE id=?`, id).Scan(&c.ID, &c.CaseID, &c.Kind, &c.SummaryHash, &issued, &c.IssuedBy, &rev); e == nil {
					if issued.Valid {
						x := parse(issued.String)
						c.IssuedAt = &x
					}
					if rev.Valid {
						x := parse(rev.String)
						c.RevokedAt = &x
					}
					caseRec := domain.Case{}
					var status string
					var risk domain.RiskAssessment
					var rt sql.NullString
					if e = t.QueryRowContext(ctx, `SELECT id,tree_code,status FROM cases WHERE id=?`, c.CaseID).Scan(&caseRec.ID, &caseRec.TreeCode, &status); e == nil {
						caseRec.Status = domain.CaseStatus(status)
						item.TreeCode = caseRec.TreeCode
						item.CaseStatus = status
					}
					if c.RevokedAt != nil {
						item.Conclusion = "已撤销"
					} else if !domain.CredentialKindValid(c.Kind) || !domain.CredentialActive(c, now) {
						item.Conclusion = "签发时间无效"
					} else if e = t.QueryRowContext(ctx, `SELECT level,decision FROM risks WHERE case_id=?`, c.CaseID).Scan(&risk.Level, &risk.Decision); e != nil {
						item.Conclusion = "摘要不匹配"
					} else {
						if e = t.QueryRowContext(ctx, `SELECT summary_hash FROM credentials WHERE id=?`, id).Scan(&rt); e == nil {
							item.SummaryHash = rt.String
						}
						if c.SummaryHash == caseRec.CredentialHash(risk) {
							item.Conclusion = "有效"
						} else {
							item.Conclusion = "摘要不匹配"
						}
					}
				}
			}
			if item.Conclusion == "有效" {
				out.ValidCount++
			} else {
				out.InvalidCount++
			}
			out.Items = append(out.Items, item)
		}
		if _, e := t.ExecContext(ctx, `INSERT INTO verification_receipts(id,requested_at,valid_count,invalid_count) VALUES(?,?,?,?)`, out.ID, ts(now), out.ValidCount, out.InvalidCount); e != nil {
			return e
		}
		for _, item := range out.Items {
			if _, e := t.ExecContext(ctx, `INSERT INTO verification_items(receipt_id,credential_id,conclusion,tree_code,case_status,summary_hash) VALUES(?,?,?,?,?,?)`, out.ID, item.CredentialID, item.Conclusion, item.TreeCode, item.CaseStatus, item.SummaryHash); e != nil {
				return e
			}
		}
		return nil
	})
	return out, err
}

func (s *SQLite) GetVerificationReceipt(ctx context.Context, id string) (domain.CredentialVerificationReceipt, error) {
	var out domain.CredentialVerificationReceipt
	var at string
	if e := s.db.QueryRowContext(ctx, `SELECT id,requested_at,valid_count,invalid_count FROM verification_receipts WHERE id=?`, id).Scan(&out.ID, &at, &out.ValidCount, &out.InvalidCount); e == sql.ErrNoRows {
		return out, domain.ErrNotFound
	} else if e != nil {
		return out, e
	}
	out.RequestedAt = parse(at)
	rows, e := s.db.QueryContext(ctx, `SELECT credential_id,conclusion,tree_code,case_status,summary_hash FROM verification_items WHERE receipt_id=?`, id)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var x domain.CredentialVerificationItem
		if e = rows.Scan(&x.CredentialID, &x.Conclusion, &x.TreeCode, &x.CaseStatus, &x.SummaryHash); e != nil {
			return out, e
		}
		out.Items = append(out.Items, x)
	}
	return out, rows.Err()
}
