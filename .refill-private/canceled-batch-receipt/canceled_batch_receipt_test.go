package canceled_batch_receipt_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/benzhi/ancient-tree-pathogen/internal/application"
	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
)

type batchRepository struct {
	started  chan struct{}
	release  chan struct{}
	finished chan struct{}

	mu        sync.Mutex
	persisted bool
}

func newBatchRepository() *batchRepository {
	return &batchRepository{
		started:  make(chan struct{}),
		release:  make(chan struct{}),
		finished: make(chan struct{}),
	}
}

func (r *batchRepository) VerifyCredentialBatch(ctx context.Context, _ []string) (domain.CredentialVerificationReceipt, error) {
	close(r.started)
	defer close(r.finished)
	select {
	case <-ctx.Done():
		return domain.CredentialVerificationReceipt{}, ctx.Err()
	case <-r.release:
		r.mu.Lock()
		r.persisted = true
		r.mu.Unlock()
		return domain.CredentialVerificationReceipt{ID: "receipt-after-cancel"}, nil
	}
}

func (r *batchRepository) didPersist() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.persisted
}

func waitFor(t *testing.T, ch <-chan struct{}, stage string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("等待%s超时", stage)
	}
}

func TestCanceledBatchDoesNotPersistReceipt(t *testing.T) {
	repo := newBatchRepository()
	service := application.New(repo)
	ctx, cancel := context.WithCancel(context.Background())
	returned := make(chan error, 1)

	go func() {
		_, err := service.VerifyCredentialBatch(ctx, []string{"cred-00000001"})
		returned <- err
	}()

	waitFor(t, repo.started, "批量核验进入仓储")
	cancel()
	select {
	case err := <-returned:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("取消请求应返回 context.Canceled，实际为 %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("取消后的服务调用未及时返回")
	}

	close(repo.release)
	waitFor(t, repo.finished, "后台仓储调用结束")
	if repo.didPersist() {
		t.Fatal("TestCanceledBatchDoesNotPersistReceipt：取消后的批量核验仍持久化回执")
	}
}

func (r *batchRepository) CreateCase(context.Context, domain.Case, string) error { return nil }
func (r *batchRepository) GetCase(context.Context, string) (domain.Case, error) {
	return domain.Case{}, domain.ErrNotFound
}
func (r *batchRepository) UpdateCase(context.Context, domain.Case, int) error { return nil }
func (r *batchRepository) AddSample(context.Context, domain.SampleChain, int) error {
	return nil
}
func (r *batchRepository) ListSamples(context.Context, string) ([]domain.SampleChain, error) {
	return nil, nil
}
func (r *batchRepository) AddTest(context.Context, domain.TestResult) error { return nil }
func (r *batchRepository) ListTests(context.Context, string) ([]domain.TestResult, error) {
	return nil, nil
}
func (r *batchRepository) SaveRisk(context.Context, domain.RiskAssessment) error { return nil }
func (r *batchRepository) GetRisk(context.Context, string) (domain.RiskAssessment, error) {
	return domain.RiskAssessment{}, domain.ErrNotFound
}
func (r *batchRepository) SaveCredential(context.Context, domain.Credential) error { return nil }
func (r *batchRepository) GetCredential(context.Context, string) (domain.Credential, error) {
	return domain.Credential{}, domain.ErrNotFound
}
func (r *batchRepository) GetCredentialByCase(context.Context, string) (domain.Credential, error) {
	return domain.Credential{}, domain.ErrNotFound
}
func (r *batchRepository) Events(context.Context, string) ([]domain.AuditEvent, error) {
	return nil, nil
}
