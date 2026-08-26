package application

import (
	"context"
	"fmt"
	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
	"time"
)

type Workflow struct{ service *Service }

func NewWorkflow(service *Service) *Workflow { return &Workflow{service: service} }
func (w *Workflow) Snapshot(ctx context.Context, id string) (CaseView, error) {
	return w.service.View(ctx, id)
}
func (w *Workflow) StartCase(ctx context.Context, command CaseCommand) (domain.Case, error) {
	return w.service.HandleCase(ctx, command)
}
func (w *Workflow) Capture(ctx context.Context, command SampleCommand) (domain.SampleChain, error) {
	if command.CollectedAt.IsZero() {
		command.CollectedAt = time.Now().UTC()
	}
	if command.HandoffAt.IsZero() {
		command.HandoffAt = command.CollectedAt
	}
	return w.service.HandleSample(ctx, command)
}
func (w *Workflow) Detect(ctx context.Context, command TestCommand) (domain.TestResult, error) {
	if command.PerformedAt.IsZero() {
		command.PerformedAt = time.Now().UTC()
	}
	return w.service.HandleTest(ctx, command)
}
func (w *Workflow) Approve(ctx context.Context, command ReviewCommand) (domain.RiskAssessment, error) {
	return w.service.Review(ctx, command.CaseID, command.Decision, command.Mitigation, command.Reviewer, command.ExpectedVersion)
}
func (w *Workflow) Verify(ctx context.Context, command RetestCommand) (domain.Case, error) {
	return w.service.RetestWithKey(ctx, command.CaseID, command.Result, command.Operator, command.Notes, command.IdempotencyKey, command.ExpectedVersion)
}
func (w *Workflow) Release(ctx context.Context, caseID, issuer string) (domain.Credential, error) {
	if issuer == "" {
		return domain.Credential{}, fmt.Errorf("%w：签发人不能为空", domain.ErrValidation)
	}
	return w.service.IssueCredential(ctx, caseID, issuer)
}
func (w *Workflow) ValidateChain(ctx context.Context, id string) error {
	view, err := w.service.View(ctx, id)
	if err != nil {
		return err
	}
	for i, sample := range view.Samples {
		if sample.Sequence != i+1 {
			return fmt.Errorf("%w：样本链在第 %d 条断裂", domain.ErrValidation, i+1)
		}
	}
	if len(view.Samples) == 0 && view.Case.Status != domain.StatusPendingSample {
		return fmt.Errorf("%w：状态与样本数量不一致", domain.ErrValidation)
	}
	return nil
}
func (w *Workflow) ValidateRelease(ctx context.Context, id string) (bool, []string) {
	view, err := w.service.View(ctx, id)
	if err != nil {
		return false, []string{err.Error()}
	}
	issues := []string{}
	if view.Case.Status != domain.StatusReleased {
		issues = append(issues, "案卷未通过复检")
	}
	if len(view.Samples) == 0 {
		issues = append(issues, "缺少样本链")
	}
	if view.Risk == nil {
		issues = append(issues, "缺少风险评定")
	}
	return len(issues) == 0, issues
}
