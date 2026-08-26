package application

import (
	"time"

	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
)

func (s *Service) cachedSearch(filter SearchFilter) (SearchResult, bool) {
	s.searchMu.RLock()
	result, ok := s.searchCache[filter]
	s.searchMu.RUnlock()
	if !ok {
		return SearchResult{}, false
	}
	return cloneSearchResult(result), true
}

func (s *Service) rememberSearch(filter SearchFilter, result SearchResult) {
	s.searchMu.Lock()
	s.searchCache[filter] = cloneSearchResult(result)
	s.searchMu.Unlock()
}

func cloneSearchResult(in SearchResult) SearchResult {
	out := in
	out.Cases = make([]CaseView, len(in.Cases))
	for i := range in.Cases {
		out.Cases[i] = cloneCaseView(in.Cases[i])
	}
	out.Counts = cloneIntMap(in.Counts)
	out.RiskDistribution = cloneIntMap(in.RiskDistribution)
	out.WaitStats = cloneIntMap(in.WaitStats)
	return out
}

func cloneCaseView(in CaseView) CaseView {
	out := in
	out.Samples = append([]domain.SampleChain(nil), in.Samples...)
	out.HandoffExceptions = append([]domain.HandoffException(nil), in.HandoffExceptions...)
	for i := range out.HandoffExceptions {
		out.HandoffExceptions[i].ClosedAt = cloneTime(in.HandoffExceptions[i].ClosedAt)
	}
	out.Tests = append([]domain.TestResult(nil), in.Tests...)
	for i := range out.Tests {
		out.Tests[i].InvalidatedAt = cloneTime(in.Tests[i].InvalidatedAt)
	}
	out.Treatments = append([]domain.TreatmentItem(nil), in.Treatments...)
	for i := range out.Treatments {
		out.Treatments[i].CompletedAt = cloneTime(in.Treatments[i].CompletedAt)
	}
	if in.Risk != nil {
		risk := *in.Risk
		out.Risk = &risk
	}
	if in.Credential != nil {
		credential := *in.Credential
		credential.IssuedAt = cloneTime(in.Credential.IssuedAt)
		credential.RevokedAt = cloneTime(in.Credential.RevokedAt)
		out.Credential = &credential
	}
	out.Events = append([]domain.AuditEvent(nil), in.Events...)
	return out
}

func cloneIntMap(in map[string]int) map[string]int {
	if in == nil {
		return nil
	}
	out := make(map[string]int, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneTime(in *time.Time) *time.Time {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}
