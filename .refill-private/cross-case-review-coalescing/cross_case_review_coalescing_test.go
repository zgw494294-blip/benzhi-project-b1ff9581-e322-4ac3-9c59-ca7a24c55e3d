package crosscasereviewcoalescing_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/benzhi/ancient-tree-pathogen/internal/application"
	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
)

type observedContext struct {
	context.Context
	joined chan struct{}
	never  chan struct{}
	once   sync.Once
}

func (c *observedContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.joined) })
	return c.never
}

type reviewRepository struct {
	firstEntered  chan struct{}
	secondEntered chan struct{}
	releaseFirst  chan struct{}
	firstOnce     sync.Once
	secondOnce    sync.Once
	reviewCalls   atomic.Int32
}

func (r *reviewRepository) GetRisk(_ context.Context, caseID string) (domain.RiskAssessment, error) {
	switch caseID {
	case "case-a":
		r.firstOnce.Do(func() { close(r.firstEntered) })
		<-r.releaseFirst
	case "case-b":
		r.secondOnce.Do(func() { close(r.secondEntered) })
	}
	return domain.RiskAssessment{ID: "risk-" + caseID, CaseID: caseID, Level: "高风险", Mitigation: "隔离病灶并复检"}, nil
}

func (r *reviewRepository) ReviewCase(_ context.Context, caseID, decision, mitigation, reviewer string, _ int) (domain.RiskAssessment, error) {
	r.reviewCalls.Add(1)
	return domain.RiskAssessment{ID: "risk-" + caseID, CaseID: caseID, Level: "高风险", Decision: decision, Mitigation: mitigation, Reviewer: reviewer}, nil
}

func (r *reviewRepository) CreateCase(context.Context, domain.Case, string) error { return nil }
func (r *reviewRepository) GetCase(context.Context, string) (domain.Case, error) {
	return domain.Case{}, nil
}
func (r *reviewRepository) UpdateCase(context.Context, domain.Case, int) error { return nil }
func (r *reviewRepository) AddSample(context.Context, domain.SampleChain, int) error {
	return nil
}
func (r *reviewRepository) ListSamples(context.Context, string) ([]domain.SampleChain, error) {
	return nil, nil
}
func (r *reviewRepository) AddTest(context.Context, domain.TestResult) error { return nil }
func (r *reviewRepository) ListTests(context.Context, string) ([]domain.TestResult, error) {
	return nil, nil
}
func (r *reviewRepository) SaveRisk(context.Context, domain.RiskAssessment) error { return nil }
func (r *reviewRepository) SaveCredential(context.Context, domain.Credential) error {
	return nil
}
func (r *reviewRepository) GetCredential(context.Context, string) (domain.Credential, error) {
	return domain.Credential{}, domain.ErrNotFound
}
func (r *reviewRepository) GetCredentialByCase(context.Context, string) (domain.Credential, error) {
	return domain.Credential{}, domain.ErrNotFound
}
func (r *reviewRepository) Events(context.Context, string) ([]domain.AuditEvent, error) {
	return nil, nil
}

type reviewResult struct {
	risk domain.RiskAssessment
	err  error
}

func TestConcurrentReviewsKeepCaseResultsIsolated(t *testing.T) {
	repo := &reviewRepository{
		firstEntered:  make(chan struct{}),
		secondEntered: make(chan struct{}),
		releaseFirst:  make(chan struct{}),
	}
	service := application.New(repo)
	firstResult := make(chan reviewResult, 1)
	secondResult := make(chan reviewResult, 1)

	go func() {
		risk, err := service.Review(context.Background(), "case-a", "通过", "隔离病灶并复检", "复核员甲", 3)
		firstResult <- reviewResult{risk: risk, err: err}
	}()
	<-repo.firstEntered

	joined := make(chan struct{})
	secondContext := &observedContext{Context: context.Background(), joined: joined, never: make(chan struct{})}
	go func() {
		risk, err := service.Review(secondContext, "case-b", "通过", "清除积水并专项消毒", "复核员乙", 7)
		secondResult <- reviewResult{risk: risk, err: err}
	}()

	select {
	case <-joined:
	case <-repo.secondEntered:
	}
	close(repo.releaseFirst)

	first := <-firstResult
	second := <-secondResult
	if first.err != nil || second.err != nil {
		t.Fatalf("并发复核不应失败：first=%v second=%v", first.err, second.err)
	}
	if first.risk.CaseID != "case-a" {
		t.Fatalf("首个案卷返回错误风险评定：%q", first.risk.CaseID)
	}
	if second.risk.CaseID != "case-b" {
		t.Fatalf("第二个案卷复用了其他案卷的风险评定：got=%q want=%q", second.risk.CaseID, "case-b")
	}
	if got := repo.reviewCalls.Load(); got != 2 {
		t.Fatalf("两个独立案卷必须分别提交复核事务：got=%d want=2", got)
	}
}
