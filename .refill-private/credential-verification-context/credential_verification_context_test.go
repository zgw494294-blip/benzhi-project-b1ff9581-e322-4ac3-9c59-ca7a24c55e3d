package credential_verification_context_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/benzhi/ancient-tree-pathogen/internal/application"
	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
)

type observedContext struct {
	context.Context
	once     sync.Once
	observed chan struct{}
}

func (c *observedContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.observed) })
	return c.Context.Done()
}

type gatedRepository struct {
	credential   domain.Credential
	caseRecord   domain.Case
	risk         domain.RiskAssessment
	firstStarted chan struct{}
	releaseFirst chan struct{}
	reads        atomic.Int32
}

func (r *gatedRepository) GetCredential(ctx context.Context, _ string) (domain.Credential, error) {
	if r.reads.Add(1) == 1 {
		close(r.firstStarted)
		<-r.releaseFirst
		if err := ctx.Err(); err != nil {
			return domain.Credential{}, err
		}
	} else {
		_ = ctx.Done()
	}
	return r.credential, nil
}

func (r *gatedRepository) GetCase(ctx context.Context, _ string) (domain.Case, error) {
	if err := ctx.Err(); err != nil {
		return domain.Case{}, err
	}
	return r.caseRecord, nil
}

func (r *gatedRepository) GetRisk(ctx context.Context, _ string) (domain.RiskAssessment, error) {
	if err := ctx.Err(); err != nil {
		return domain.RiskAssessment{}, err
	}
	return r.risk, nil
}

func (r *gatedRepository) CreateCase(context.Context, domain.Case, string) error { return nil }
func (r *gatedRepository) UpdateCase(context.Context, domain.Case, int) error    { return nil }
func (r *gatedRepository) AddSample(context.Context, domain.SampleChain, int) error {
	return nil
}
func (r *gatedRepository) ListSamples(context.Context, string) ([]domain.SampleChain, error) {
	return nil, nil
}
func (r *gatedRepository) AddTest(context.Context, domain.TestResult) error { return nil }
func (r *gatedRepository) ListTests(context.Context, string) ([]domain.TestResult, error) {
	return nil, nil
}
func (r *gatedRepository) SaveRisk(context.Context, domain.RiskAssessment) error { return nil }
func (r *gatedRepository) SaveCredential(context.Context, domain.Credential) error {
	return nil
}
func (r *gatedRepository) GetCredentialByCase(context.Context, string) (domain.Credential, error) {
	return domain.Credential{}, domain.ErrNotFound
}
func (r *gatedRepository) Events(context.Context, string) ([]domain.AuditEvent, error) {
	return nil, nil
}

type verificationResult struct {
	credential domain.Credential
	caseRecord domain.Case
	err        error
}

func TestConcurrentCredentialVerificationKeepsCallerContextsIsolated(t *testing.T) {
	now := time.Date(2026, time.August, 26, 9, 0, 0, 0, time.UTC)
	caseRecord := domain.Case{
		ID:       "case-context-isolation",
		TreeCode: "TREE-CONTEXT-01",
		Status:   domain.StatusReleased,
		Version:  8,
	}
	risk := domain.RiskAssessment{
		ID:       "risk-context-isolation",
		CaseID:   caseRecord.ID,
		Level:    "低风险",
		Decision: "通过",
	}
	repo := &gatedRepository{
		caseRecord: caseRecord,
		risk:       risk,
		credential: domain.Credential{
			ID:          "cred_context_isolation",
			CaseID:      caseRecord.ID,
			Kind:        "古树保护放行",
			SummaryHash: caseRecord.CredentialHash(risk),
			IssuedAt:    &now,
			IssuedBy:    "保护负责人",
		},
		firstStarted: make(chan struct{}),
		releaseFirst: make(chan struct{}),
	}
	service := application.New(repo)

	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	leaderResult := make(chan verificationResult, 1)
	go func() {
		credential, record, err := service.VerifyCredential(leaderCtx, repo.credential.ID)
		leaderResult <- verificationResult{credential: credential, caseRecord: record, err: err}
	}()
	<-repo.firstStarted

	followerCtx := &observedContext{Context: context.Background(), observed: make(chan struct{})}
	followerResult := make(chan verificationResult, 1)
	go func() {
		credential, record, err := service.VerifyCredential(followerCtx, repo.credential.ID)
		followerResult <- verificationResult{credential: credential, caseRecord: record, err: err}
	}()
	<-followerCtx.observed

	cancelLeader()
	close(repo.releaseFirst)
	follower := <-followerResult
	<-leaderResult

	if follower.err != nil {
		t.Fatalf("健康调用方继承了先发请求的取消结果: %v", follower.err)
	}
	if follower.credential.ID != repo.credential.ID || follower.caseRecord.ID != caseRecord.ID {
		t.Fatalf("健康调用方未取得完整凭据核验结果: credential=%q case=%q", follower.credential.ID, follower.caseRecord.ID)
	}
}
