package domain

import (
	"fmt"
	"strings"
	"time"
)

type PolicyResult struct {
	Allowed bool
	Code    string
	Message string
	Next    CaseStatus
}

func Policy(status CaseStatus, action string) PolicyResult {
	policies := map[CaseStatus]map[string]PolicyResult{
		StatusPendingSample: {"samples": {true, "SAMPLE_OPEN", "允许录入第一条样本链记录", StatusPendingTest}},
		StatusPendingTest:   {"tests": {true, "TEST_OPEN", "允许录入病原实验结果", StatusPendingReview}},
		StatusPendingReview: {"review": {true, "REVIEW_OPEN", "允许负责人复核风险评定", StatusPendingRetest}},
		StatusPendingRetest: {"retest": {true, "RETEST_OPEN", "允许登记复检结果", StatusReleased}},
		StatusReleased:      {"credential": {true, "CREDENTIAL_OPEN", "允许签发保护放行凭据", StatusReleased}},
	}
	if result, ok := policies[status][action]; ok {
		return result
	}
	return PolicyResult{Code: "ACTION_BLOCKED", Message: fmt.Sprintf("状态 %s 不允许操作 %s", status, action)}
}

func RequireVersion(actual, expected int) error {
	if actual != expected {
		return ErrConflict
	}
	return nil
}
func RequireNonEmpty(values ...string) error {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return ErrValidation
		}
	}
	return nil
}
func RequireFutureSafe(t time.Time) error {
	if t.IsZero() || t.After(time.Now().UTC().Add(5*time.Minute)) {
		return ErrValidation
	}
	return nil
}
func RequireChronology(first, second time.Time) error {
	if first.IsZero() || second.IsZero() || second.Before(first) {
		return ErrValidation
	}
	return nil
}
func RequireSequence(sequence, previous int) error {
	if sequence != previous+1 {
		return ErrValidation
	}
	return nil
}
func RequireStatus(c Case, expected CaseStatus) error {
	if c.Status != expected {
		return fmt.Errorf("%w：当前状态为 %s", ErrValidation, c.Status)
	}
	return nil
}
func RiskRequiresRetest(level string) bool { return level == "高风险" || level == "中风险" }
func RiskRequiresMitigation(level, mitigation string) bool {
	return !RiskRequiresRetest(level) || strings.TrimSpace(mitigation) != ""
}
func DecisionAllowsRelease(decision string) bool { return strings.TrimSpace(decision) == "通过" }
func HashLooksValid(hash string) bool            { return len(strings.TrimSpace(hash)) == 64 }
func CredentialActive(c Credential, now time.Time) bool {
	if c.ID == "" || c.IssuedAt == nil || c.RevokedAt != nil {
		return false
	}
	return !c.IssuedAt.After(now)
}
func CredentialRevoked(c Credential) bool { return c.RevokedAt != nil }
func CaseMutable(c Case) bool             { return !IsTerminal(c.Status) }
func CaseNeedsSample(c Case) bool         { return c.Status == StatusPendingSample }
func CaseNeedsTest(c Case) bool           { return c.Status == StatusPendingTest }
func CaseNeedsReview(c Case) bool         { return c.Status == StatusPendingReview }
func CaseNeedsRetest(c Case) bool         { return c.Status == StatusPendingRetest }
func CaseReadyForCredential(c Case) bool  { return c.Status == StatusReleased }
