package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

type CaseStatus string

const (
	StatusPendingSample CaseStatus = "待取样"
	StatusPendingTest   CaseStatus = "待检测"
	StatusPendingReview CaseStatus = "待复核"
	StatusPendingRetest CaseStatus = "待复检"
	StatusReleased      CaseStatus = "已放行"
	StatusRejected      CaseStatus = "已驳回"
)

type Case struct {
	ID, TreeCode, Species, Location, Owner string
	Status                                 CaseStatus
	Version                                int
	CreatedAt, UpdatedAt                   time.Time
	PendingReviewSince                     time.Time
}
type SampleChain struct {
	ID, CaseID, SampleCode, Collector, SealCode, Receiver, Condition string
	CollectedAt, HandoffAt                                           time.Time
	Sequence                                                         int
}
type TestResult struct {
	ID, CaseID, TestType, Operator, Pathogen, Load, Method, Result, Notes  string
	PerformedAt                                                            time.Time
	InvalidatedAt                                                          *time.Time
	InvalidatedBy, InvalidationReason, ReplacesTestID, CorrectionRequestID string
}
type RiskAssessment struct {
	ID, CaseID, Level, Factors, Decision, Mitigation, Reviewer string
	ReviewedAt                                                 time.Time
}
type Credential struct {
	ID, CaseID, Kind, SummaryHash, IssuedBy string
	IssuedAt, RevokedAt                     *time.Time
}
type AuditEvent struct {
	ID, CaseID, Action, Detail, Actor string
	CreatedAt                         time.Time
}

var ErrInvalidTransition = errors.New("非法状态迁移")
var ErrConflict = errors.New("版本冲突")
var ErrValidation = errors.New("业务校验失败")

func NewCase(treeCode, species, location, owner string) (Case, error) {
	treeCode = strings.ToUpper(strings.TrimSpace(treeCode))
	if !ValidateTreeCode(treeCode) || strings.TrimSpace(species) == "" {
		return Case{}, fmt.Errorf("%w：树木编号格式或树种不能为空", ErrValidation)
	}
	now := time.Now().UTC()
	return Case{ID: NewID("case"), TreeCode: treeCode, Species: species, Location: location, Owner: owner, Status: StatusPendingSample, Version: 1, CreatedAt: now, UpdatedAt: now}, nil
}
func NewID(prefix string) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())))
	return prefix + "_" + hex.EncodeToString(h[:])[:16]
}
func (c *Case) Advance(to CaseStatus) error {
	allowed := map[CaseStatus][]CaseStatus{StatusPendingSample: {StatusPendingTest}, StatusPendingTest: {StatusPendingReview}, StatusPendingReview: {StatusPendingRetest, StatusRejected}, StatusPendingRetest: {StatusReleased, StatusRejected}}
	for _, s := range allowed[c.Status] {
		if s == to {
			c.Status = to
			c.Version++
			c.UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return fmt.Errorf("%w：%s -> %s", ErrInvalidTransition, c.Status, to)
}
func ValidateSample(s SampleChain, previous int) error {
	if s.CaseID == "" || s.SampleCode == "" || s.Collector == "" || s.SealCode == "" {
		return fmt.Errorf("%w：样本字段不完整", ErrValidation)
	}
	if s.Sequence != previous+1 {
		return fmt.Errorf("%w：样本链序号必须为 %d", ErrValidation, previous+1)
	}
	if s.CollectedAt.IsZero() || s.HandoffAt.IsZero() || s.HandoffAt.Before(s.CollectedAt) {
		return fmt.Errorf("%w：采集与交接时间无效", ErrValidation)
	}
	if !ValidateActor(s.Collector) || !ValidateActor(s.Receiver) || !ConditionAcceptable(s.Condition) {
		return fmt.Errorf("%w：交接信息不完整", ErrValidation)
	}
	return nil
}
func ValidateTest(t TestResult) error {
	if t.CaseID == "" || t.TestType == "" || t.Operator == "" || t.Pathogen == "" || t.Load == "" || t.Method == "" || t.Result == "" {
		return fmt.Errorf("%w：实验结果字段不完整", ErrValidation)
	}
	if !TestTypeKnown(t.TestType) || !MethodKnown(t.Method) {
		return fmt.Errorf("%w：检测类型或方法不在目录中", ErrValidation)
	}
	if _, ok := FindPathogen(t.Pathogen); !ok {
		return fmt.Errorf("%w：病原不在目录中", ErrValidation)
	}
	t.Load = NormalizeLoad(t.Load)
	if t.Load != "高" && t.Load != "中" && t.Load != "低" {
		return fmt.Errorf("%w：载量必须为高、中或低", ErrValidation)
	}
	return nil
}
func AssessRisk(t TestResult, sampleOK bool) (level, factors, mitigation string) {
	load := strings.ToLower(t.Load)
	level = "低风险"
	factors = "载量低；样本链完整"
	mitigation = "常规养护并季度复查"
	if !sampleOK {
		level = "高风险"
		factors = "样本链不完整"
		mitigation = "立即隔离、补采样并制定专项保护方案"
	} else if strings.Contains(load, "高") || strings.Contains(load, "重") || strings.Contains(load, ">10") {
		level = "高风险"
		factors = "病原载量高"
		mitigation = "隔离病灶、消毒处理并进行复检"
	} else if strings.Contains(load, "中") || strings.Contains(load, "moderate") {
		level = "中风险"
		factors = "病原载量中等"
		mitigation = "制定修复方案并在30日内复检"
	}
	if profile, ok := FindPathogen(t.Pathogen); ok && RiskOrder(profile.Risk) > RiskOrder(level) {
		level = profile.Risk
		mitigation = profile.Action
		factors = "病原目录风险：" + profile.Name
	}
	return
}
func CanRetest(c Case, risk RiskAssessment, result string) error {
	if c.Status != StatusPendingRetest {
		return fmt.Errorf("%w：案卷尚未进入复检", ErrValidation)
	}
	if strings.TrimSpace(risk.Mitigation) == "" || strings.TrimSpace(result) == "" {
		return fmt.Errorf("%w：处置方案与复检结果不能为空", ErrValidation)
	}
	if !IsPassingResult(result) && !IsFailingResult(result) {
		return fmt.Errorf("%w：复检结果必须为通过或失败", ErrValidation)
	}
	return nil
}
func (c Case) CredentialHash(r RiskAssessment) string {
	h := sha256.Sum256([]byte(c.ID + "|" + c.TreeCode + "|" + string(c.Status) + "|" + r.Level + "|" + r.Decision))
	return hex.EncodeToString(h[:])
}
