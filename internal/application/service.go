package application

import (
	"context"
	"fmt"
	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
	"strings"
	"sync"
	"time"
)

type Service struct{ repo domain.Repository }

func New(repo domain.Repository) *Service { return &Service{repo: repo} }

type CaseView struct {
	Case              domain.Case
	Samples           []domain.SampleChain
	HandoffExceptions []domain.HandoffException
	Tests             []domain.TestResult
	Treatments        []domain.TreatmentItem
	Risk              *domain.RiskAssessment
	Credential        *domain.Credential
	Events            []domain.AuditEvent
}

type SearchFilter struct {
	Status, Risk, TreeCode, Pathogen, Method, Sort string
	UpdatedFrom, UpdatedTo                         time.Time
	WaitFrom, WaitTo, DueWithin                    time.Duration
	Cursor                                         string
	Limit                                          int
}
type SearchResult struct {
	Cases            []CaseView     `json:"cases"`
	Counts           map[string]int `json:"counts"`
	RiskDistribution map[string]int `json:"riskDistribution"`
	NextCursor       string         `json:"nextCursor,omitempty"`
	WaitStats        map[string]int `json:"waitStats,omitempty"`
	OldestWait       time.Duration  `json:"oldestWait,omitempty"`
	DueCount         int            `json:"dueCount,omitempty"`
	DataQualityCount int            `json:"dataQualityCount,omitempty"`
}

func (s *Service) Search(ctx context.Context, f SearchFilter) (SearchResult, error) {
	if f.Limit <= 0 {
		f.Limit = 20
	}
	if f.Limit > 100 {
		return SearchResult{}, fmt.Errorf("%w：分页大小无效", domain.ErrValidation)
	}
	if f.Status != "" {
		valid := false
		for _, x := range domain.StatusSequence() {
			if string(x) == f.Status {
				valid = true
			}
		}
		if f.Status == string(domain.StatusRejected) {
			valid = true
		}
		if f.Status == string(domain.StatusReleased) {
			valid = true
		}
		if !valid {
			return SearchResult{}, fmt.Errorf("%w：非法状态", domain.ErrValidation)
		}
	}
	if f.Risk != "" && domain.RiskOrder(f.Risk) == 0 {
		return SearchResult{}, fmt.Errorf("%w：非法风险等级", domain.ErrValidation)
	}
	if !f.UpdatedFrom.IsZero() && !f.UpdatedTo.IsZero() && f.UpdatedTo.Before(f.UpdatedFrom) {
		return SearchResult{}, fmt.Errorf("%w：日期范围无效", domain.ErrValidation)
	}
	if f.Cursor != "" {
		cursorTime := f.Cursor
		if i := strings.Index(cursorTime, "|"); i >= 0 {
			cursorTime = cursorTime[:i]
		}
		if _, err := time.Parse(time.RFC3339Nano, cursorTime); err != nil {
			return SearchResult{}, fmt.Errorf("%w：分页游标无效", domain.ErrValidation)
		}
	}
	if q, ok := s.repo.(interface {
		SearchQueueCases(context.Context, string, string, string, string, string, string, int) ([]domain.Case, string, error)
	}); ok && (f.Pathogen != "" || f.Method != "" || f.WaitFrom != 0 || f.WaitTo != 0 || f.DueWithin != 0 || f.Sort != "") {
		cases, next, err := q.SearchQueueCases(ctx, f.Status, f.Risk, f.TreeCode, f.Pathogen, f.Method, f.Cursor, f.Limit)
		if err != nil {
			return SearchResult{}, err
		}
		out := SearchResult{Cases: []CaseView{}, Counts: map[string]int{}, RiskDistribution: map[string]int{}, WaitStats: map[string]int{}, NextCursor: next}
		now := time.Now().UTC()
		for _, c := range cases {
			v, e := s.View(ctx, c.ID)
			if e != nil {
				return out, e
			}
			if v.Risk == nil || len(v.Tests) == 0 {
				out.DataQualityCount++
				continue
			}
			wait := now.Sub(c.UpdatedAt)
			if !c.PendingReviewSince.IsZero() {
				wait = now.Sub(c.PendingReviewSince)
			}
			if f.WaitFrom > 0 && wait < f.WaitFrom {
				continue
			}
			if f.WaitTo > 0 && wait > f.WaitTo {
				continue
			}
			if f.DueWithin > 0 && wait < 24*time.Hour-f.DueWithin {
				continue
			}
			if f.Pathogen != "" || f.Method != "" {
				matched := false
				for _, t := range v.Tests {
					if t.InvalidatedAt == nil && (f.Pathogen == "" || t.Pathogen == f.Pathogen) && (f.Method == "" || t.Method == f.Method) {
						matched = true
					}
				}
				if !matched {
					continue
				}
			}
			out.Cases = append(out.Cases, v)
			out.RiskDistribution[v.Risk.Level]++
			if wait > out.OldestWait {
				out.OldestWait = wait
			}
			switch {
			case wait < 24*time.Hour:
				out.WaitStats["24小时内"]++
			case wait < 72*time.Hour:
				out.WaitStats["24-72小时"]++
			default:
				out.WaitStats["超过72小时"]++
			}
			if f.DueWithin > 0 && wait >= 24*time.Hour-f.DueWithin {
				out.DueCount++
			}
		}
		return out, nil
	}
	q, ok := s.repo.(interface {
		SearchCases(context.Context, string, string, string, time.Time, time.Time, string, int) ([]domain.Case, string, error)
	})
	if !ok {
		return SearchResult{}, fmt.Errorf("%w：暂不支持检索", domain.ErrValidation)
	}
	cases, next, err := q.SearchCases(ctx, f.Status, f.Risk, f.TreeCode, f.UpdatedFrom, f.UpdatedTo, f.Cursor, f.Limit)
	if err != nil {
		return SearchResult{}, err
	}
	out := SearchResult{Counts: map[string]int{}, RiskDistribution: map[string]int{}, NextCursor: next}
	viewErrors := make(chan error, len(cases))
	riskLevels := make(chan string, len(cases))
	var views sync.WaitGroup
	for _, c := range cases {
		views.Add(1)
		go func(caseID string) {
			defer views.Done()
			v, e := s.View(ctx, caseID)
			if e != nil {
				viewErrors <- e
				return
			}
			out.Cases = append(out.Cases, v)
			if v.Risk != nil {
				riskLevels <- v.Risk.Level
			}
		}(c.ID)
	}
	views.Wait()
	close(viewErrors)
	close(riskLevels)
	for e := range viewErrors {
		return out, e
	}
	for level := range riskLevels {
		out.RiskDistribution[level]++
	}
	if cq, ok := s.repo.(interface {
		SummaryCounts(context.Context) (map[string]int, error)
	}); ok {
		if counts, e := cq.SummaryCounts(ctx); e == nil {
			out.Counts = counts
		}
	}
	return out, nil
}

func (s *Service) CreateCase(ctx context.Context, tree, species, location, owner, actor string) (domain.Case, error) {
	if !domain.ValidateActor(actor) {
		return domain.Case{}, fmt.Errorf("%w：登记人无效", domain.ErrValidation)
	}
	c, e := domain.NewCase(tree, species, location, owner)
	if e != nil {
		return c, e
	}
	return c, s.repo.CreateCase(ctx, c, actor)
}
func (s *Service) AddSample(ctx context.Context, caseID, code, collector, seal, receiver, condition string, collected, handoff time.Time, expected int) (domain.SampleChain, error) {
	samples, e := s.repo.ListSamples(ctx, caseID)
	if e != nil {
		return domain.SampleChain{}, e
	}
	x := domain.SampleChain{ID: domain.NewID("sample"), CaseID: caseID, SampleCode: strings.TrimSpace(code), Collector: strings.TrimSpace(collector), SealCode: strings.TrimSpace(seal), Receiver: strings.TrimSpace(receiver), Condition: strings.TrimSpace(condition), CollectedAt: collected, HandoffAt: handoff, Sequence: len(samples) + 1}
	err := s.repo.AddSample(ctx, x, expected)
	if err != nil {
		if q, ok := s.repo.(interface {
			RecordHandoffException(context.Context, domain.HandoffException) error
		}); ok {
			_ = q.RecordHandoffException(ctx, domain.HandoffException{ID: domain.NewID("handoff-exception"), CaseID: caseID, SampleCode: x.SampleCode, Collector: x.Collector, SealCode: x.SealCode, Receiver: x.Receiver, Condition: x.Condition, CollectedAt: x.CollectedAt, HandoffAt: x.HandoffAt, Sequence: x.Sequence, Reason: err.Error(), OccurredAt: time.Now().UTC()})
		}
	}
	return x, err
}
func (s *Service) AddTest(ctx context.Context, caseID, testType, operator, pathogen, load, method, result, notes string, performed time.Time) (domain.TestResult, error) {
	if !domain.ValidateActor(operator) {
		return domain.TestResult{}, fmt.Errorf("%w：检测人无效", domain.ErrValidation)
	}
	x := domain.TestResult{ID: domain.NewID("test"), CaseID: caseID, TestType: strings.TrimSpace(testType), Operator: strings.TrimSpace(operator), Pathogen: strings.TrimSpace(pathogen), Load: domain.NormalizeLoad(load), Method: strings.TrimSpace(method), Result: strings.TrimSpace(result), Notes: notes, PerformedAt: performed}
	if e := domain.ValidateTest(x); e != nil {
		return x, e
	}
	if e := s.repo.AddTest(ctx, x); e != nil {
		return x, e
	}
	return x, nil
}
func (s *Service) Review(ctx context.Context, caseID, decision, mitigation, reviewer string, expected int) (domain.RiskAssessment, error) {
	if txr, ok := s.repo.(interface {
		ReviewCase(context.Context, string, string, string, string, int) (domain.RiskAssessment, error)
	}); ok {
		if !domain.ReviewDecisionValid(decision) || !domain.ValidateActor(reviewer) {
			return domain.RiskAssessment{}, fmt.Errorf("%w：复核结论或复核人无效", domain.ErrValidation)
		}
		r, err := s.repo.GetRisk(ctx, caseID)
		if err != nil {
			return r, err
		}
		if !domain.RiskRequiresMitigation(r.Level, mitigation) {
			return r, fmt.Errorf("%w：中高风险通过必须填写处置方案", domain.ErrValidation)
		}
		return txr.ReviewCase(ctx, caseID, decision, mitigation, reviewer, expected)
	}
	c, e := s.repo.GetCase(ctx, caseID)
	if e != nil {
		return domain.RiskAssessment{}, e
	}
	if c.Version != expected {
		return domain.RiskAssessment{}, domain.ErrConflict
	}
	r, e := s.repo.GetRisk(ctx, caseID)
	if e != nil {
		return r, e
	}
	if !domain.ReviewDecisionValid(decision) || !domain.ValidateActor(reviewer) {
		return r, fmt.Errorf("%w：复核结论与复核人不能为空", domain.ErrValidation)
	}
	if !domain.RiskRequiresMitigation(r.Level, mitigation) {
		return r, fmt.Errorf("%w：中高风险通过必须填写处置方案", domain.ErrValidation)
	}
	r.Decision = decision
	r.Mitigation = mitigation
	r.Reviewer = reviewer
	r.ReviewedAt = time.Now().UTC()
	if decision == "通过" {
		if e = c.Advance(domain.StatusPendingRetest); e != nil {
			return r, e
		}
	} else {
		if e = c.Advance(domain.StatusRejected); e != nil {
			return r, e
		}
	}
	if e = s.repo.UpdateCase(ctx, c, expected); e != nil {
		return r, e
	}
	return r, s.repo.SaveRisk(ctx, r)
}
func (s *Service) Retest(ctx context.Context, caseID, result, operator string, expected int) (domain.Case, error) {
	if _, ok := s.repo.(interface {
		RetestCase(context.Context, string, string, string, string, string, int) (domain.Case, error)
	}); ok {
		return s.RetestWithKey(ctx, caseID, result, operator, "", "", expected)
	}
	c, e := s.repo.GetCase(ctx, caseID)
	if e != nil {
		return c, e
	}
	if c.Version != expected {
		return c, domain.ErrConflict
	}
	r, e := s.repo.GetRisk(ctx, caseID)
	if e != nil {
		return c, e
	}
	if !domain.ValidateActor(operator) {
		return c, fmt.Errorf("%w：复检人无效", domain.ErrValidation)
	}
	if e = domain.CanRetest(c, r, result); e != nil {
		return c, e
	}
	to := domain.StatusReleased
	if domain.IsFailingResult(result) {
		to = domain.StatusRejected
	}
	if e = c.Advance(to); e != nil {
		return c, e
	}
	if e = s.repo.UpdateCase(ctx, c, expected); e != nil {
		return c, e
	}
	return c, nil
}

func (s *Service) RetestWithKey(ctx context.Context, caseID, result, operator, notes, key string, expected int) (domain.Case, error) {
	c, err := s.repo.GetCase(ctx, caseID)
	if err != nil {
		return c, err
	}
	r, err := s.repo.GetRisk(ctx, caseID)
	if err != nil {
		return c, err
	}
	if !domain.ValidateActor(operator) || !domain.IsPassingResult(result) && !domain.IsFailingResult(result) {
		return c, fmt.Errorf("%w：复检人或结果无效", domain.ErrValidation)
	}
	if err := domain.CanRetest(c, r, result); err != nil {
		return c, err
	}
	if txr, ok := s.repo.(interface {
		RetestCase(context.Context, string, string, string, string, string, int) (domain.Case, error)
	}); ok {
		return txr.RetestCase(ctx, caseID, result, operator, notes, key, expected)
	}
	return s.Retest(ctx, caseID, result, operator, expected)
}
func (s *Service) RevokeCredential(ctx context.Context, id, actor, reason string) (domain.Credential, error) {
	if txr, ok := s.repo.(interface {
		RevokeCredential(context.Context, string, string, string) (domain.Credential, error)
	}); ok {
		return txr.RevokeCredential(ctx, id, actor, reason)
	}
	return domain.Credential{}, fmt.Errorf("%w：暂不支持撤销", domain.ErrValidation)
}
func (s *Service) IssueCredential(ctx context.Context, caseID, issuer string) (domain.Credential, error) {
	if !domain.ValidateActor(issuer) {
		return domain.Credential{}, fmt.Errorf("%w：签发人无效", domain.ErrValidation)
	}
	c, e := s.repo.GetCase(ctx, caseID)
	if e != nil {
		return domain.Credential{}, e
	}
	if c.Status != domain.StatusReleased {
		return domain.Credential{}, fmt.Errorf("%w：案卷尚未通过复检", domain.ErrValidation)
	}
	if old, e := s.repo.GetCredentialByCase(ctx, caseID); e == nil {
		return old, nil
	}
	r, e := s.repo.GetRisk(ctx, caseID)
	if e != nil {
		return domain.Credential{}, e
	}
	if tests, e := s.repo.ListTests(ctx, caseID); e == nil && len(tests) > 0 {
		passedRetest := false
		for _, test := range tests {
			if domain.IsPassingResult(test.Result) && test.InvalidatedAt == nil {
				passedRetest = true
			}
		}
		if !passedRetest {
			return domain.Credential{}, fmt.Errorf("%w：复检结果未记录为通过", domain.ErrValidation)
		}
	}
	now := time.Now().UTC()
	x := domain.Credential{ID: domain.NewID("cred"), CaseID: caseID, Kind: "古树保护放行", SummaryHash: c.CredentialHash(r), IssuedAt: &now, IssuedBy: issuer}
	return x, s.repo.SaveCredential(ctx, x)
}

func (s *Service) View(ctx context.Context, id string) (CaseView, error) {
	c, e := s.repo.GetCase(ctx, id)
	if e != nil {
		return CaseView{}, e
	}
	v := CaseView{Case: c}
	v.Samples, _ = s.repo.ListSamples(ctx, id)
	if q, ok := s.repo.(interface {
		ListHandoffExceptions(context.Context, string) ([]domain.HandoffException, error)
	}); ok {
		v.HandoffExceptions, _ = q.ListHandoffExceptions(ctx, id)
	}
	v.Tests, _ = s.repo.ListTests(ctx, id)
	if q, ok := s.repo.(interface {
		ListTreatmentItems(context.Context, string) ([]domain.TreatmentItem, error)
	}); ok {
		v.Treatments, _ = q.ListTreatmentItems(ctx, id)
	}
	if r, e := s.repo.GetRisk(ctx, id); e == nil {
		v.Risk = &r
	}
	if x, e := s.repo.GetCredentialByCase(ctx, id); e == nil {
		v.Credential = &x
	}
	v.Events, _ = s.repo.Events(ctx, id)
	return v, nil
}

func (s *Service) RecordHandoffException(ctx context.Context, x domain.HandoffException) error {
	q, ok := s.repo.(interface {
		RecordHandoffException(context.Context, domain.HandoffException) error
	})
	if !ok {
		return fmt.Errorf("%w：暂不支持异常留痕", domain.ErrValidation)
	}
	return q.RecordHandoffException(ctx, x)
}
func (s *Service) Rehandoff(ctx context.Context, exceptionID, newSeal, correction, actor string, expected int) (domain.SampleChain, error) {
	q, ok := s.repo.(interface {
		Rehandoff(context.Context, string, string, string, string, int) (domain.SampleChain, error)
	})
	if !ok {
		return domain.SampleChain{}, fmt.Errorf("%w：暂不支持重新封签", domain.ErrValidation)
	}
	return q.Rehandoff(ctx, exceptionID, newSeal, correction, actor, expected)
}
func (s *Service) CorrectTest(ctx context.Context, caseID, originalID string, replacement domain.TestResult, corrector, reason, requestID string, expected int) (domain.TestResult, error) {
	q, ok := s.repo.(interface {
		InvalidateAndReplaceTest(context.Context, string, string, domain.TestResult, string, string, string, int) (domain.TestResult, error)
	})
	if !ok {
		return domain.TestResult{}, fmt.Errorf("%w：暂不支持实验更正", domain.ErrValidation)
	}
	return q.InvalidateAndReplaceTest(ctx, caseID, originalID, replacement, corrector, reason, requestID, expected)
}
func (s *Service) AddTreatment(ctx context.Context, item domain.TreatmentItem, expected int) error {
	q, ok := s.repo.(interface {
		AddTreatmentItem(context.Context, domain.TreatmentItem, int) error
	})
	if !ok {
		return fmt.Errorf("%w：暂不支持处置项", domain.ErrValidation)
	}
	return q.AddTreatmentItem(ctx, item, expected)
}
func (s *Service) CompleteTreatment(ctx context.Context, caseID, itemID, assignee, evidence string, completed time.Time, expected int) error {
	q, ok := s.repo.(interface {
		CompleteTreatmentItem(context.Context, string, string, string, string, time.Time, int) error
	})
	if !ok {
		return fmt.Errorf("%w：暂不支持处置项", domain.ErrValidation)
	}
	return q.CompleteTreatmentItem(ctx, caseID, itemID, assignee, evidence, completed, expected)
}
func (s *Service) VerifyCredentialBatch(ctx context.Context, ids []string) (domain.CredentialVerificationReceipt, error) {
	q, ok := s.repo.(interface {
		VerifyCredentialBatch(context.Context, []string) (domain.CredentialVerificationReceipt, error)
	})
	if !ok {
		return domain.CredentialVerificationReceipt{}, fmt.Errorf("%w：暂不支持批量核验", domain.ErrValidation)
	}
	return q.VerifyCredentialBatch(ctx, ids)
}
func (s *Service) VerificationReceipt(ctx context.Context, id string) (domain.CredentialVerificationReceipt, error) {
	q, ok := s.repo.(interface {
		GetVerificationReceipt(context.Context, string) (domain.CredentialVerificationReceipt, error)
	})
	if !ok {
		return domain.CredentialVerificationReceipt{}, domain.ErrNotFound
	}
	return q.GetVerificationReceipt(ctx, id)
}

func (s *Service) VerifyCredential(ctx context.Context, id string) (domain.Credential, domain.Case, error) {
	credential, err := s.repo.GetCredential(ctx, id)
	if err != nil {
		return domain.Credential{}, domain.Case{}, err
	}
	caseRecord, err := s.repo.GetCase(ctx, credential.CaseID)
	if err != nil {
		return domain.Credential{}, domain.Case{}, err
	}
	risk, err := s.repo.GetRisk(ctx, credential.CaseID)
	if err != nil {
		return domain.Credential{}, domain.Case{}, err
	}
	if !domain.CredentialKindValid(credential.Kind) || credential.IssuedAt == nil || !domain.HashLooksValid(credential.SummaryHash) {
		return domain.Credential{}, domain.Case{}, fmt.Errorf("%w：凭据字段无效", domain.ErrValidation)
	}
	if credential.SummaryHash != caseRecord.CredentialHash(risk) {
		return domain.Credential{}, domain.Case{}, fmt.Errorf("%w：凭据摘要校验失败", domain.ErrValidation)
	}
	return credential, caseRecord, nil
}
