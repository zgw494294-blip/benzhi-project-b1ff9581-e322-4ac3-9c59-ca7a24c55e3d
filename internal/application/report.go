package application

import (
	"context"
	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
	"sort"
	"time"
)

type TimelineItem struct {
	Action string    `json:"action"`
	Detail string    `json:"detail"`
	Actor  string    `json:"actor"`
	At     time.Time `json:"at"`
}

type CaseReport struct {
	ID          string            `json:"id"`
	TreeCode    string            `json:"treeCode"`
	Status      domain.CaseStatus `json:"status"`
	Risk        string            `json:"risk"`
	SampleCount int               `json:"sampleCount"`
	TestCount   int               `json:"testCount"`
	Timeline    []TimelineItem    `json:"timeline"`
}

func (s *Service) Report(ctx context.Context, id string) (CaseReport, error) {
	v, err := s.View(ctx, id)
	if err != nil {
		return CaseReport{}, err
	}
	report := CaseReport{ID: v.Case.ID, TreeCode: v.Case.TreeCode, Status: v.Case.Status, SampleCount: len(v.Samples), TestCount: len(v.Tests)}
	if v.Risk != nil {
		report.Risk = v.Risk.Level
	}
	for _, event := range v.Events {
		report.Timeline = append(report.Timeline, TimelineItem{Action: event.Action, Detail: event.Detail, Actor: event.Actor, At: event.CreatedAt})
	}
	sort.SliceStable(report.Timeline, func(i, j int) bool { return report.Timeline[i].At.Before(report.Timeline[j].At) })
	return report, nil
}

func (s *Service) CanIssue(ctx context.Context, id string) (bool, string) {
	v, err := s.View(ctx, id)
	if err != nil {
		return false, err.Error()
	}
	if v.Case.Status != domain.StatusReleased {
		return false, "案卷尚未进入已放行状态"
	}
	if v.Risk == nil {
		return false, "缺少风险评定"
	}
	if !domain.IsPassingResult(v.RaseResult()) {
		return false, "复检结果未记录为通过"
	}
	return true, "允许签发"
}

func (v CaseView) RaseResult() string {
	for i := len(v.Tests) - 1; i >= 0; i-- {
		if v.Tests[i].TestType == "复检" && v.Tests[i].InvalidatedAt == nil {
			return v.Tests[i].Result
		}
	}
	return ""
}
