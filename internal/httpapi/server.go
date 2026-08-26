package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/benzhi/ancient-tree-pathogen/internal/application"
	"github.com/benzhi/ancient-tree-pathogen/internal/domain"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	app *application.Service
	mux *http.ServeMux
}

func New(app *application.Service) *Server {
	s := &Server{app: app, mux: http.NewServeMux()}
	s.routes()
	return s
}
func (s *Server) Handler() http.Handler { return s.mux }
func (s *Server) routes() {
	s.mux.HandleFunc("/api/cases", s.cases)
	s.mux.HandleFunc("/api/search", s.search)
	s.mux.HandleFunc("/api/cases/", s.caseActions)
	s.mux.HandleFunc("/api/credentials/", s.credential)
	s.mux.HandleFunc("/api/verification/", s.verificationReceipt)
	s.mux.HandleFunc("/api/verification", s.verifyBatch)
	s.mux.Handle("/", http.FileServer(http.Dir("web")))
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, e error) {
	status := http.StatusBadRequest
	if errors.Is(e, domain.ErrNotFound) {
		status = http.StatusNotFound
	}
	if errors.Is(e, domain.ErrConflict) {
		status = http.StatusConflict
	}
	write(w, status, map[string]string{"error": e.Error()})
}
func decode(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}
func (s *Server) cases(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		s.search(w, r)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var in struct{ TreeCode, Species, Location, Owner, Actor string }
	if e := decode(r, &in); e != nil {
		fail(w, e)
		return
	}
	c, e := s.app.CreateCase(r.Context(), in.TreeCode, in.Species, in.Location, in.Owner, in.Actor)
	if e != nil {
		fail(w, e)
		return
	}
	write(w, 201, c)
}
func (s *Server) caseActions(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		http.NotFound(w, r)
		return
	}
	id, action := parts[2], ""
	if len(parts) > 3 {
		action = parts[3]
	}
	switch action {
	case "samples":
		s.samples(w, r, id)
	case "rehandoff":
		s.rehandoff(w, r, id)
	case "tests":
		s.tests(w, r, id)
	case "correct-test":
		s.correctTest(w, r, id)
	case "treatments":
		s.treatments(w, r, id)
	case "treatment-complete":
		s.completeTreatment(w, r, id)
	case "review":
		s.review(w, r, id)
	case "retest":
		s.retest(w, r, id)
	case "credential":
		s.issue(w, r, id)
	default:
		s.view(w, r, id)
	}
}

func (s *Server) rehandoff(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var in struct {
		ExceptionID, NewSeal, Correction, Actor string
		ExpectedVersion                         int
	}
	if e := decode(r, &in); e != nil {
		fail(w, e)
		return
	}
	x, e := s.app.Rehandoff(r.Context(), in.ExceptionID, in.NewSeal, in.Correction, in.Actor, in.ExpectedVersion)
	if e != nil {
		fail(w, e)
		return
	}
	write(w, 200, x)
}
func (s *Server) view(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	v, e := s.app.View(r.Context(), id)
	if e != nil {
		fail(w, e)
		return
	}
	write(w, 200, v)
}
func (s *Server) samples(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var in struct {
		SampleCode, Collector, SealCode, Receiver, Condition string
		CollectedAt, HandoffAt                               string
		ExpectedVersion                                      int
	}
	if e := decode(r, &in); e != nil {
		fail(w, e)
		return
	}
	a, _ := time.Parse(time.RFC3339, in.CollectedAt)
	b, _ := time.Parse(time.RFC3339, in.HandoffAt)
	x, e := s.app.AddSample(r.Context(), id, in.SampleCode, in.Collector, in.SealCode, in.Receiver, in.Condition, a, b, in.ExpectedVersion)
	if e != nil {
		fail(w, e)
		return
	}
	write(w, 201, x)
}
func (s *Server) tests(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var in struct{ TestType, Operator, Pathogen, Load, Method, Result, Notes, PerformedAt string }
	if e := decode(r, &in); e != nil {
		fail(w, e)
		return
	}
	p, _ := time.Parse(time.RFC3339, in.PerformedAt)
	x, e := s.app.AddTest(r.Context(), id, in.TestType, in.Operator, in.Pathogen, in.Load, in.Method, in.Result, in.Notes, p)
	if e != nil {
		fail(w, e)
		return
	}
	write(w, 201, x)
}
func (s *Server) correctTest(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var in struct {
		OriginalTestID, TestType, Operator, Pathogen, Load, Method, Result, Notes, PerformedAt, Corrector, Reason, RequestID string
		ExpectedVersion                                                                                                      int
	}
	if e := decode(r, &in); e != nil {
		fail(w, e)
		return
	}
	p, _ := time.Parse(time.RFC3339, in.PerformedAt)
	x, e := s.app.CorrectTest(r.Context(), id, in.OriginalTestID, domain.TestResult{ID: domain.NewID("test"), CaseID: id, TestType: in.TestType, Operator: in.Operator, Pathogen: in.Pathogen, Load: in.Load, Method: in.Method, Result: in.Result, Notes: in.Notes, PerformedAt: p}, in.Corrector, in.Reason, in.RequestID, in.ExpectedVersion)
	if e != nil {
		fail(w, e)
		return
	}
	write(w, 200, x)
}
func (s *Server) treatments(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method == "GET" {
		v, e := s.app.View(r.Context(), id)
		if e != nil {
			fail(w, e)
			return
		}
		write(w, 200, v.Treatments)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var in struct {
		Content, Assignee, PlannedAt string
		Required                     bool
		ExpectedVersion              int
	}
	if e := decode(r, &in); e != nil {
		fail(w, e)
		return
	}
	p, _ := time.Parse(time.RFC3339, in.PlannedAt)
	e := s.app.AddTreatment(r.Context(), domain.TreatmentItem{ID: domain.NewID("treatment"), CaseID: id, Content: in.Content, Assignee: in.Assignee, PlannedAt: p, Required: in.Required, CreatedAt: time.Now().UTC()}, in.ExpectedVersion)
	if e != nil {
		fail(w, e)
		return
	}
	write(w, 201, map[string]string{"status": "已新增"})
}
func (s *Server) completeTreatment(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var in struct {
		ItemID, Assignee, Evidence, CompletedAt string
		ExpectedVersion                         int
	}
	if e := decode(r, &in); e != nil {
		fail(w, e)
		return
	}
	p, _ := time.Parse(time.RFC3339, in.CompletedAt)
	if e := s.app.CompleteTreatment(r.Context(), id, in.ItemID, in.Assignee, in.Evidence, p, in.ExpectedVersion); e != nil {
		fail(w, e)
		return
	}
	write(w, 200, map[string]string{"status": "已完成"})
}
func (s *Server) review(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var in struct {
		Decision, Mitigation, Reviewer string
		ExpectedVersion                int
	}
	if e := decode(r, &in); e != nil {
		fail(w, e)
		return
	}
	x, e := s.app.Review(r.Context(), id, in.Decision, in.Mitigation, in.Reviewer, in.ExpectedVersion)
	if e != nil {
		fail(w, e)
		return
	}
	write(w, 200, x)
}
func (s *Server) retest(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var in struct {
		Result, Operator, Notes, IdempotencyKey string
		ExpectedVersion                         int
	}
	if e := decode(r, &in); e != nil {
		fail(w, e)
		return
	}
	x, e := s.app.RetestWithKey(r.Context(), id, in.Result, in.Operator, in.Notes, in.IdempotencyKey, in.ExpectedVersion)
	if e != nil {
		fail(w, e)
		return
	}
	write(w, 200, x)
}
func (s *Server) issue(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var in struct{ IssuedBy string }
	if e := decode(r, &in); e != nil {
		fail(w, e)
		return
	}
	x, e := s.app.IssueCredential(r.Context(), id, in.IssuedBy)
	if e != nil {
		fail(w, e)
		return
	}
	write(w, 201, x)
}
func (s *Server) credential(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/revoke") {
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/credentials/"), "/revoke")
		var in struct{ Actor, Reason string }
		if e := decode(r, &in); e != nil {
			fail(w, e)
			return
		}
		c, e := s.app.RevokeCredential(r.Context(), id, in.Actor, in.Reason)
		if e != nil {
			fail(w, e)
			return
		}
		write(w, 200, c)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/credentials/")
	credential, caseRecord, e := s.app.VerifyCredential(r.Context(), id)
	if e != nil {
		fail(w, e)
		return
	}
	active := domain.CredentialActive(credential, time.Now().UTC())
	reason := ""
	if credential.RevokedAt != nil {
		reason = "凭据已撤销"
	}
	write(w, 200, map[string]any{"credential": credential, "case": caseRecord, "verified": true, "active": active, "invalidReason": reason})
}

func (s *Server) verifyBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var in struct {
		IDs []string `json:"ids"`
	}
	if e := decode(r, &in); e != nil {
		fail(w, e)
		return
	}
	out, e := s.app.VerifyCredentialBatch(r.Context(), in.IDs)
	if e != nil {
		fail(w, e)
		return
	}
	write(w, 200, out)
}
func (s *Server) verificationReceipt(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/verification/")
	out, e := s.app.VerificationReceipt(r.Context(), id)
	if e != nil {
		fail(w, e)
		return
	}
	write(w, 200, out)
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	parseTime := func(v string) (time.Time, error) {
		if v == "" {
			return time.Time{}, nil
		}
		return time.Parse(time.RFC3339, v)
	}
	from, e := parseTime(requestValue(r, "updatedFrom"))
	if e != nil {
		fail(w, fmt.Errorf("%w：日期参数无效", domain.ErrValidation))
		return
	}
	to, e := parseTime(requestValue(r, "updatedTo"))
	if e != nil {
		fail(w, fmt.Errorf("%w：日期参数无效", domain.ErrValidation))
		return
	}
	limit := 20
	if v := requestValue(r, "limit"); v != "" {
		if n, er := strconv.Atoi(v); er == nil {
			limit = n
		} else {
			fail(w, fmt.Errorf("%w：分页参数无效", domain.ErrValidation))
			return
		}
	}
	var waitFrom, waitTo, due time.Duration
	if v := requestValue(r, "waitFromHours"); v != "" {
		if n, er := strconv.Atoi(v); er == nil {
			waitFrom = time.Duration(n) * time.Hour
		}
	}
	if v := requestValue(r, "waitToHours"); v != "" {
		if n, er := strconv.Atoi(v); er == nil {
			waitTo = time.Duration(n) * time.Hour
		}
	}
	if v := requestValue(r, "dueWithinHours"); v != "" {
		if n, er := strconv.Atoi(v); er == nil {
			due = time.Duration(n) * time.Hour
		}
	}
	out, e := s.app.Search(r.Context(), application.SearchFilter{Status: requestValue(r, "status"), Risk: requestValue(r, "risk"), TreeCode: requestValue(r, "treeCode"), Pathogen: requestValue(r, "pathogen"), Method: requestValue(r, "method"), Sort: requestValue(r, "sort"), UpdatedFrom: from, UpdatedTo: to, Cursor: requestValue(r, "cursor"), Limit: limit, WaitFrom: waitFrom, WaitTo: waitTo, DueWithin: due})
	if e != nil {
		fail(w, e)
		return
	}
	write(w, 200, out)
}
