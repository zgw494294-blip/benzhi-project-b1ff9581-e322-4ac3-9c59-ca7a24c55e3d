package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type HandoffException struct {
	ID, CaseID, SampleCode, Collector, SealCode, Receiver, Condition string
	CollectedAt, HandoffAt, OccurredAt                               time.Time
	Sequence                                                         int
	Reason, Correction, NewSealCode, ResolvedSampleID                string
	ClosedAt                                                         *time.Time
}

type TreatmentItem struct {
	ID, CaseID, Content, Assignee, Evidence string
	PlannedAt, CreatedAt                    time.Time
	Required                                bool
	CompletedAt                             *time.Time
}

type CredentialVerificationItem struct {
	CredentialID, Conclusion, TreeCode, CaseStatus, SummaryHash string
}

type CredentialVerificationReceipt struct {
	ID                       string
	RequestedAt              time.Time
	ValidCount, InvalidCount int
	Items                    []CredentialVerificationItem
}

const MaxCredentialBatch = 50

var credentialIDPattern = regexp.MustCompile(`^cred_[a-f0-9]{16}$`)

func ValidateCredentialID(id string) bool {
	return credentialIDPattern.MatchString(strings.TrimSpace(id))
}

func ValidateCorrection(original TestResult, replacement TestResult, sampleHandoff, now time.Time, corrector, reason, requestID string) error {
	if original.ID == "" || original.InvalidatedAt != nil {
		return fmt.Errorf("%w：原实验不存在或已失效", ErrValidation)
	}
	if !ValidateActor(corrector) || strings.TrimSpace(reason) == "" || strings.TrimSpace(requestID) == "" {
		return fmt.Errorf("%w：更正人、原因和请求标识不能为空", ErrValidation)
	}
	if err := ValidateTest(replacement); err != nil {
		return err
	}
	if replacement.PerformedAt.Before(sampleHandoff) || replacement.PerformedAt.After(now.Add(5*time.Minute)) {
		return fmt.Errorf("%w：检测时间必须在样本交接后且不得晚于当前允许时间", ErrValidation)
	}
	return nil
}

func ValidateTreatmentItem(item TreatmentItem, now time.Time) error {
	if strings.TrimSpace(item.Content) == "" || !ValidateActor(item.Assignee) {
		return fmt.Errorf("%w：处置内容和负责人无效", ErrValidation)
	}
	if item.PlannedAt.IsZero() || item.PlannedAt.Before(now) {
		return fmt.Errorf("%w：计划完成时间必须晚于当前时间", ErrValidation)
	}
	return nil
}

func ValidateTreatmentCompletion(item TreatmentItem, assignee, evidence string, completedAt, now time.Time) error {
	if item.CompletedAt != nil {
		return fmt.Errorf("%w：处置项已完成且不可修改", ErrValidation)
	}
	if strings.TrimSpace(assignee) != strings.TrimSpace(item.Assignee) || !ValidateActor(assignee) {
		return fmt.Errorf("%w：完成人必须与处置负责人一致", ErrValidation)
	}
	if strings.TrimSpace(evidence) == "" {
		return fmt.Errorf("%w：证据说明不能为空", ErrValidation)
	}
	if completedAt.IsZero() || completedAt.Before(item.CreatedAt) || completedAt.After(now.Add(5*time.Minute)) {
		return fmt.Errorf("%w：完成时间无效", ErrValidation)
	}
	return nil
}

func HighestRisk(tests []TestResult, sampleOK bool) (level, factors, mitigation string) {
	level, mitigation = "低风险", "常规养护并季度复查"
	max := 0
	for _, test := range tests {
		if test.InvalidatedAt != nil {
			continue
		}
		candidate, _, action := AssessRisk(test, sampleOK)
		if n := RiskOrder(candidate); n > max {
			max, level, mitigation = n, candidate, action
		}
		factors += test.Pathogen + "/" + test.Method + "/" + NormalizeLoad(test.Load) + "（" + test.ID + "）;"
	}
	if factors == "" {
		factors = "暂无有效实验结果;"
	}
	if sampleOK {
		factors += "样本链完整"
	} else {
		factors += "样本链不完整"
		level = "高风险"
		mitigation = "立即隔离、补采样并制定专项保护方案"
	}
	return
}
