package searchsnapshotstale_test

import (
	"context"
	"testing"
	"time"

	"github.com/benzhi/ancient-tree-pathogen/internal/application"
	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
)

func TestReviewInvalidatesCachedSearchSnapshot(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	repo := &statefulRepository{
		caseRecord: domain.Case{
			ID: "case_search_cache", TreeCode: "SC-901", Species: "银杏",
			Status: domain.StatusPendingReview, Version: 7, CreatedAt: now, UpdatedAt: now,
		},
		risk: domain.RiskAssessment{
			ID: "risk_search_cache", CaseID: "case_search_cache", Level: "中风险",
			Decision: "待人工复核", Mitigation: "隔离病灶并复检",
		},
	}
	service := application.New(repo)
	filter := application.SearchFilter{Status: string(domain.StatusPendingReview), Limit: 20}

	before, err := service.Search(ctx, filter)
	if err != nil {
		t.Fatalf("首次搜索失败: %v", err)
	}
	if len(before.Cases) != 1 || before.Cases[0].Case.Status != domain.StatusPendingReview {
		t.Fatalf("首次搜索未返回待复核案卷: %+v", before.Cases)
	}

	updated, err := service.Review(ctx, repo.caseRecord.ID, "通过", "隔离病灶并复检", "复核负责人", 7)
	if err != nil {
		t.Fatalf("复核状态迁移失败: %v", err)
	}
	_ = updated
	if repo.caseRecord.Status != domain.StatusPendingRetest {
		t.Fatalf("仓储状态未推进到待复检: %s", repo.caseRecord.Status)
	}

	after, err := service.Search(ctx, filter)
	if err != nil {
		t.Fatalf("复核后的搜索失败: %v", err)
	}
	if len(after.Cases) != 0 {
		t.Fatalf("复核后仍返回缓存中的待复核案卷: status=%s version=%d", after.Cases[0].Case.Status, after.Cases[0].Case.Version)
	}
}

type statefulRepository struct {
	caseRecord domain.Case
	risk       domain.RiskAssessment
}

func (r *statefulRepository) SearchCases(_ context.Context, status, _, _ string, _, _ time.Time, _ string, _ int) ([]domain.Case, string, error) {
	if status != "" && string(r.caseRecord.Status) != status {
		return nil, "", nil
	}
	return []domain.Case{r.caseRecord}, "", nil
}

func (r *statefulRepository) CreateCase(context.Context, domain.Case, string) error { return nil }

func (r *statefulRepository) GetCase(_ context.Context, id string) (domain.Case, error) {
	if id != r.caseRecord.ID {
		return domain.Case{}, domain.ErrNotFound
	}
	return r.caseRecord, nil
}

func (r *statefulRepository) UpdateCase(_ context.Context, next domain.Case, expected int) error {
	if r.caseRecord.Version != expected {
		return domain.ErrConflict
	}
	r.caseRecord = next
	return nil
}

func (r *statefulRepository) AddSample(context.Context, domain.SampleChain, int) error { return nil }

func (r *statefulRepository) ListSamples(context.Context, string) ([]domain.SampleChain, error) {
	return nil, nil
}

func (r *statefulRepository) AddTest(context.Context, domain.TestResult) error { return nil }

func (r *statefulRepository) ListTests(context.Context, string) ([]domain.TestResult, error) {
	return nil, nil
}

func (r *statefulRepository) SaveRisk(_ context.Context, risk domain.RiskAssessment) error {
	r.risk = risk
	return nil
}

func (r *statefulRepository) GetRisk(_ context.Context, caseID string) (domain.RiskAssessment, error) {
	if caseID != r.risk.CaseID {
		return domain.RiskAssessment{}, domain.ErrNotFound
	}
	return r.risk, nil
}

func (r *statefulRepository) SaveCredential(context.Context, domain.Credential) error { return nil }

func (r *statefulRepository) GetCredential(context.Context, string) (domain.Credential, error) {
	return domain.Credential{}, domain.ErrNotFound
}

func (r *statefulRepository) GetCredentialByCase(context.Context, string) (domain.Credential, error) {
	return domain.Credential{}, domain.ErrNotFound
}

func (r *statefulRepository) Events(context.Context, string) ([]domain.AuditEvent, error) {
	return nil, nil
}
