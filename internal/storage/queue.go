package storage

import (
	"context"
	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
	"time"
)

func (s *SQLite) SearchQueueCases(ctx context.Context, status, risk, tree, pathogen, method, cursor string, limit int) ([]domain.Case, string, error) {
	if status == "" {
		status = string(domain.StatusPendingReview)
	}
	q := `SELECT c.id,c.tree_code,c.species,c.location,c.owner,c.status,c.version,c.created_at,c.updated_at,c.pending_review_since FROM cases c LEFT JOIN risks r ON r.case_id=c.id WHERE c.status=? AND r.level IS NOT NULL AND EXISTS (SELECT 1 FROM tests t WHERE t.case_id=c.id AND t.invalidated_at IS NULL)`
	args := []any{status}
	if risk != "" {
		q += " AND r.level=?"
		args = append(args, risk)
	}
	if tree != "" {
		q += " AND c.tree_code LIKE ?"
		args = append(args, tree+"%")
	}
	if pathogen != "" {
		q += " AND EXISTS (SELECT 1 FROM tests tp WHERE tp.case_id=c.id AND tp.pathogen=? AND tp.invalidated_at IS NULL)"
		args = append(args, pathogen)
	}
	if method != "" {
		q += " AND EXISTS (SELECT 1 FROM tests tm WHERE tm.case_id=c.id AND tm.method=? AND tm.invalidated_at IS NULL)"
		args = append(args, method)
	}
	if cursor != "" {
		cursorTime, cursorID := cursor, ""
		for i, r := range cursor {
			if r == '|' {
				cursorTime, cursorID = cursor[:i], cursor[i+1:]
				break
			}
		}
		if cursorID == "" {
			q += " AND c.updated_at<?"
			args = append(args, cursorTime)
		} else {
			q += " AND (c.updated_at<? OR (c.updated_at=? AND c.id<?))"
			args = append(args, cursorTime, cursorTime, cursorID)
		}
	}
	q += " ORDER BY c.updated_at DESC,c.id DESC LIMIT ?"
	args = append(args, limit+1)
	rows, e := s.db.QueryContext(ctx, q, args...)
	if e != nil {
		return nil, "", e
	}
	defer rows.Close()
	var out []domain.Case
	for rows.Next() {
		var c domain.Case
		var a, b, p *string
		if e = rows.Scan(&c.ID, &c.TreeCode, &c.Species, &c.Location, &c.Owner, &c.Status, &c.Version, &a, &b, &p); e != nil {
			return nil, "", e
		}
		c.CreatedAt = parse(*a)
		c.UpdatedAt = parse(*b)
		if p != nil {
			c.PendingReviewSince = parse(*p)
		}
		out = append(out, c)
	}
	if e = rows.Err(); e != nil {
		return nil, "", e
	}
	next := ""
	if len(out) > limit {
		next = out[limit-1].UpdatedAt.Format(time.RFC3339Nano) + "|" + out[limit-1].ID
		out = out[:limit]
	}
	return out, next, nil
}
