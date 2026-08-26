package domain

import "strings"

type PathogenProfile struct {
	Code      string
	Name      string
	Risk      string
	Threshold string
	Action    string
}

var Pathogens = []PathogenProfile{
	{Code: "PHY-001", Name: "腐霉", Risk: "高风险", Threshold: "高", Action: "隔离病灶并复检"},
	{Code: "PHY-002", Name: "疫霉", Risk: "高风险", Threshold: ">10", Action: "清除积水并专项消毒"},
	{Code: "FUN-001", Name: "炭疽病菌", Risk: "中风险", Threshold: "中", Action: "修剪病枝并复查"},
	{Code: "FUN-002", Name: "木腐菌", Risk: "高风险", Threshold: "高", Action: "结构评估与支撑保护"},
	{Code: "BAC-001", Name: "细菌性溃疡病菌", Risk: "中风险", Threshold: "中", Action: "伤口封护并复检"},
}

func FindPathogen(name string) (PathogenProfile, bool) {
	for _, profile := range Pathogens {
		if strings.EqualFold(profile.Name, strings.TrimSpace(name)) {
			return profile, true
		}
	}
	return PathogenProfile{}, false
}

func RiskOrder(level string) int {
	switch level {
	case "低风险":
		return 1
	case "中风险":
		return 2
	case "高风险":
		return 3
	default:
		return 0
	}
}

func IsTerminal(status CaseStatus) bool {
	return status == StatusReleased || status == StatusRejected
}

func StatusLabel(status CaseStatus) string {
	labels := map[CaseStatus]string{
		StatusPendingSample: "待取样",
		StatusPendingTest:   "待检测",
		StatusPendingReview: "待复核",
		StatusPendingRetest: "待复检",
		StatusReleased:      "已放行",
		StatusRejected:      "已驳回",
	}
	return labels[status]
}

func StatusSequence() []CaseStatus {
	return []CaseStatus{
		StatusPendingSample,
		StatusPendingTest,
		StatusPendingReview,
		StatusPendingRetest,
		StatusReleased,
	}
}

func AllowedActions(status CaseStatus) []string {
	actions := map[CaseStatus][]string{
		StatusPendingSample: {"samples"},
		StatusPendingTest:   {"tests"},
		StatusPendingReview: {"review"},
		StatusPendingRetest: {"retest"},
		StatusReleased:      {"credential"},
		StatusRejected:      {},
	}
	return append([]string(nil), actions[status]...)
}

func ValidateTreeCode(code string) bool {
	code = strings.TrimSpace(code)
	if len(code) < 4 || len(code) > 32 {
		return false
	}
	for _, r := range code {
		if !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '-' {
			return false
		}
	}
	return true
}

func ValidateActor(actor string) bool {
	return len([]rune(strings.TrimSpace(actor))) >= 2
}

func NormalizeLoad(load string) string {
	switch strings.ToLower(strings.TrimSpace(load)) {
	case "high", "严重", "高":
		return "高"
	case "medium", "moderate", "中":
		return "中"
	case "low", "轻", "低":
		return "低"
	default:
		return strings.TrimSpace(load)
	}
}

func IsPassingResult(result string) bool {
	v := strings.ToLower(strings.TrimSpace(result))
	return v == "通过" || v == "阴性" || v == "pass" || v == "negative"
}

func IsFailingResult(result string) bool {
	v := strings.ToLower(strings.TrimSpace(result))
	return v == "未通过" || v == "阳性" || v == "fail" || v == "positive"
}

func ReviewDecisionValid(decision string) bool {
	return strings.TrimSpace(decision) == "通过" || strings.TrimSpace(decision) == "驳回"
}

func ConditionAcceptable(condition string) bool {
	v := strings.TrimSpace(condition)
	return v == "完整" || v == "封签完整" || v == "合格"
}

func MethodKnown(method string) bool {
	known := map[string]bool{"qPCR": true, "PCR": true, "显微镜": true, "培养": true, "LAMP": true}
	return known[strings.TrimSpace(method)]
}

func TestTypeKnown(testType string) bool {
	known := map[string]bool{"病原筛查": true, "PCR": true, "复检": true, "培养鉴定": true}
	return known[strings.TrimSpace(testType)]
}

func CredentialKindValid(kind string) bool {
	return strings.TrimSpace(kind) == "古树保护放行"
}
