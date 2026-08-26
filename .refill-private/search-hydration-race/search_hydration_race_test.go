package searchhydrationrace_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/benzhi/ancient-tree-pathogen/internal/application"
	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
)

const caseCount = 24

type searchRepository struct {
	mu      sync.Mutex
	arrived int
	release chan struct{}
}

func (r *searchRepository) SearchCases(context.Context, string, string, string, time.Time, time.Time, string, int) ([]domain.Case, string, error) {
	cases := make([]domain.Case, caseCount)
	for i := range cases {
		cases[i] = domain.Case{ID: fmt.Sprintf("case-%02d", i), TreeCode: fmt.Sprintf("TREE-%02d", i), Status: domain.StatusPendingReview}
	}
	return cases, "", nil
}

func (r *searchRepository) Events(context.Context, string) ([]domain.AuditEvent, error) {
	r.mu.Lock()
	r.arrived++
	if r.arrived == caseCount {
		close(r.release)
	}
	r.mu.Unlock()
	<-r.release
	return nil, nil
}

func (r *searchRepository) CreateCase(context.Context, domain.Case, string) error { return nil }
func (r *searchRepository) GetCase(_ context.Context, id string) (domain.Case, error) {
	return domain.Case{ID: id, TreeCode: "TREE-" + id, Status: domain.StatusPendingReview}, nil
}
func (r *searchRepository) UpdateCase(context.Context, domain.Case, int) error { return nil }
func (r *searchRepository) AddSample(context.Context, domain.SampleChain, int) error {
	return nil
}
func (r *searchRepository) ListSamples(context.Context, string) ([]domain.SampleChain, error) {
	return nil, nil
}
func (r *searchRepository) AddTest(context.Context, domain.TestResult) error { return nil }
func (r *searchRepository) ListTests(context.Context, string) ([]domain.TestResult, error) {
	return nil, nil
}
func (r *searchRepository) SaveRisk(context.Context, domain.RiskAssessment) error { return nil }
func (r *searchRepository) GetRisk(_ context.Context, caseID string) (domain.RiskAssessment, error) {
	return domain.RiskAssessment{ID: "risk-" + caseID, CaseID: caseID, Level: "中风险"}, nil
}
func (r *searchRepository) SaveCredential(context.Context, domain.Credential) error { return nil }
func (r *searchRepository) GetCredential(context.Context, string) (domain.Credential, error) {
	return domain.Credential{}, domain.ErrNotFound
}
func (r *searchRepository) GetCredentialByCase(context.Context, string) (domain.Credential, error) {
	return domain.Credential{}, domain.ErrNotFound
}

func TestConcurrentSearchHydrationDoesNotShareResultState(t *testing.T) {
	repo := &searchRepository{release: make(chan struct{})}
	service := application.New(repo)

	result, err := service.Search(context.Background(), application.SearchFilter{Limit: caseCount})
	if err != nil {
		t.Fatalf("并发装配搜索结果不应失败：%v", err)
	}
	if len(result.Cases) != caseCount {
		t.Fatalf("并发装配丢失案卷：got=%d want=%d", len(result.Cases), caseCount)
	}
	for i, view := range result.Cases {
		want := fmt.Sprintf("case-%02d", i)
		if view.Case.ID != want {
			t.Fatalf("并发装配改变仓储分页顺序：index=%d got=%q want=%q", i, view.Case.ID, want)
		}
	}
}
