package credential_lookup_error

import (
	"context"
	"errors"
	"testing"

	"github.com/benzhi/ancient-tree-pathogen/internal/application"
	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
)

var errCredentialLookup = errors.New("credential lookup unavailable")

type repository struct {
	saved int
}

func (r *repository) CreateCase(context.Context, domain.Case, string) error { return nil }
func (r *repository) GetCase(context.Context, string) (domain.Case, error) {
	return domain.Case{ID: "case-1", TreeCode: "GT-001", Status: domain.StatusReleased, Version: 4}, nil
}
func (r *repository) UpdateCase(context.Context, domain.Case, int) error       { return nil }
func (r *repository) AddSample(context.Context, domain.SampleChain, int) error { return nil }
func (r *repository) ListSamples(context.Context, string) ([]domain.SampleChain, error) {
	return nil, nil
}
func (r *repository) AddTest(context.Context, domain.TestResult) error { return nil }
func (r *repository) ListTests(context.Context, string) ([]domain.TestResult, error) {
	return []domain.TestResult{{ID: "retest-1", CaseID: "case-1", TestType: "复检", Result: "通过"}}, nil
}
func (r *repository) SaveRisk(context.Context, domain.RiskAssessment) error { return nil }
func (r *repository) GetRisk(context.Context, string) (domain.RiskAssessment, error) {
	return domain.RiskAssessment{ID: "risk-1", CaseID: "case-1", Level: "低风险", Decision: "通过", Mitigation: "常规养护"}, nil
}
func (r *repository) SaveCredential(context.Context, domain.Credential) error {
	r.saved++
	return nil
}
func (r *repository) GetCredential(context.Context, string) (domain.Credential, error) {
	return domain.Credential{}, domain.ErrNotFound
}
func (r *repository) GetCredentialByCase(context.Context, string) (domain.Credential, error) {
	return domain.Credential{}, errCredentialLookup
}
func (r *repository) Events(context.Context, string) ([]domain.AuditEvent, error) { return nil, nil }

func TestCredentialLookupFailureIsPropagated(t *testing.T) {
	repo := &repository{}
	service := application.New(repo)

	_, err := service.IssueCredential(context.Background(), "case-1", "负责人")
	if !errors.Is(err, errCredentialLookup) {
		t.Fatalf("expected credential lookup error, got %v", err)
	}
	if repo.saved != 0 {
		t.Fatalf("credential was persisted after lookup failure: %d writes", repo.saved)
	}
}
