package viewsiblingcontextleak_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/benzhi/ancient-tree-pathogen/internal/application"
	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
)

var errRiskBackend = errors.New("风险仓储不可用")

type aggregateRepository struct {
	sampleStarted  chan struct{}
	sampleCanceled chan struct{}
	releaseSample  chan struct{}
}

func newAggregateRepository() *aggregateRepository {
	return &aggregateRepository{
		sampleStarted:  make(chan struct{}),
		sampleCanceled: make(chan struct{}),
		releaseSample:  make(chan struct{}),
	}
}

func (r *aggregateRepository) GetCase(context.Context, string) (domain.Case, error) {
	return domain.Case{ID: "case_context_boundary", Status: domain.StatusPendingReview}, nil
}

func (r *aggregateRepository) ListSamples(ctx context.Context, _ string) ([]domain.SampleChain, error) {
	close(r.sampleStarted)
	select {
	case <-ctx.Done():
		close(r.sampleCanceled)
		return nil, ctx.Err()
	case <-r.releaseSample:
		return nil, nil
	}
}

func (r *aggregateRepository) GetRisk(context.Context, string) (domain.RiskAssessment, error) {
	<-r.sampleStarted
	return domain.RiskAssessment{}, errRiskBackend
}

func TestViewCancelsSiblingReadsAfterAggregateFailure(t *testing.T) {
	repo := newAggregateRepository()
	service := application.New(repo)

	_, err := service.View(context.Background(), "case_context_boundary")
	if !errors.Is(err, errRiskBackend) {
		close(repo.releaseSample)
		t.Fatalf("聚合读取应返回风险仓储错误，实际为 %v", err)
	}

	select {
	case <-repo.sampleCanceled:
		close(repo.releaseSample)
	case <-time.After(time.Second):
		close(repo.releaseSample)
		t.Fatal("TestViewCancelsSiblingReadsAfterAggregateFailure：聚合失败后同级读取仍占用调用方资源")
	}
}

func (r *aggregateRepository) CreateCase(context.Context, domain.Case, string) error { return nil }
func (r *aggregateRepository) UpdateCase(context.Context, domain.Case, int) error    { return nil }
func (r *aggregateRepository) AddSample(context.Context, domain.SampleChain, int) error {
	return nil
}
func (r *aggregateRepository) AddTest(context.Context, domain.TestResult) error { return nil }
func (r *aggregateRepository) ListTests(context.Context, string) ([]domain.TestResult, error) {
	return nil, nil
}
func (r *aggregateRepository) SaveRisk(context.Context, domain.RiskAssessment) error { return nil }
func (r *aggregateRepository) SaveCredential(context.Context, domain.Credential) error {
	return nil
}
func (r *aggregateRepository) GetCredential(context.Context, string) (domain.Credential, error) {
	return domain.Credential{}, domain.ErrNotFound
}
func (r *aggregateRepository) GetCredentialByCase(context.Context, string) (domain.Credential, error) {
	return domain.Credential{}, domain.ErrNotFound
}
func (r *aggregateRepository) Events(context.Context, string) ([]domain.AuditEvent, error) {
	return nil, nil
}
