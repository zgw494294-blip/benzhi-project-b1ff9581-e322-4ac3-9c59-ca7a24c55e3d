package application

import (
	"context"
	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
	"time"
)

type CaseCommand struct{ TreeCode, Species, Location, Owner, Actor string }
type SampleCommand struct {
	CaseID, SampleCode, Collector, SealCode, Receiver, Condition string
	CollectedAt, HandoffAt                                       time.Time
	ExpectedVersion                                              int
}
type TestCommand struct {
	CaseID, TestType, Operator, Pathogen, Load, Method, Result, Notes string
	PerformedAt                                                       time.Time
}
type ReviewCommand struct {
	CaseID, Decision, Mitigation, Reviewer string
	ExpectedVersion                        int
}
type RetestCommand struct {
	CaseID, Result, Operator, Notes, IdempotencyKey string
	ExpectedVersion                                 int
}

func (s *Service) HandleCase(ctx context.Context, c CaseCommand) (domain.Case, error) {
	return s.CreateCase(ctx, c.TreeCode, c.Species, c.Location, c.Owner, c.Actor)
}
func (s *Service) HandleSample(ctx context.Context, c SampleCommand) (domain.SampleChain, error) {
	return s.AddSample(ctx, c.CaseID, c.SampleCode, c.Collector, c.SealCode, c.Receiver, c.Condition, c.CollectedAt, c.HandoffAt, c.ExpectedVersion)
}
func (s *Service) HandleTest(ctx context.Context, c TestCommand) (domain.TestResult, error) {
	return s.AddTest(ctx, c.CaseID, c.TestType, c.Operator, c.Pathogen, c.Load, c.Method, c.Result, c.Notes, c.PerformedAt)
}
