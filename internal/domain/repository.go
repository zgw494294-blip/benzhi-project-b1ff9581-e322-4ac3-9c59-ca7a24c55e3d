package domain

import "context"

type Repository interface {
	CreateCase(context.Context, Case, string) error
	GetCase(context.Context, string) (Case, error)
	UpdateCase(context.Context, Case, int) error
	AddSample(context.Context, SampleChain, int) error
	ListSamples(context.Context, string) ([]SampleChain, error)
	AddTest(context.Context, TestResult) error
	ListTests(context.Context, string) ([]TestResult, error)
	SaveRisk(context.Context, RiskAssessment) error
	GetRisk(context.Context, string) (RiskAssessment, error)
	SaveCredential(context.Context, Credential) error
	GetCredential(context.Context, string) (Credential, error)
	GetCredentialByCase(context.Context, string) (Credential, error)
	Events(context.Context, string) ([]AuditEvent, error)
}
