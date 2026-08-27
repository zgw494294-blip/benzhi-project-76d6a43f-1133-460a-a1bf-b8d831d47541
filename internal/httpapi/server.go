package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/application"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/domain/commissioning"
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
	s.mux.HandleFunc("/api/v1/commissioning-cases", s.create)
	s.mux.HandleFunc("/api/v1/commissioning-cases/", s.caseRoute)
	s.mux.HandleFunc("/api/v1/permits", s.permitBatch)
	s.mux.HandleFunc("/api/v1/permits/", s.permit)
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func readJSON(r *http.Request, v any) error {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	return d.Decode(v)
}
func (s *Server) fail(w http.ResponseWriter, e error) {
	status := 400
	if errors.Is(e, commissioning.ErrNotFound) || errors.Is(e, commissioning.ErrPermitNotFound) {
		status = 404
	}
	if errors.Is(e, commissioning.ErrVersionConflict) || errors.Is(e, commissioning.ErrPackageStale) {
		status = 409
	}
	if errors.Is(e, commissioning.ErrIdempotencyConflict) {
		status = 409
	}
	if errors.Is(e, commissioning.ErrInvalidTransition) || errors.Is(e, commissioning.ErrOpenDeviation) || errors.Is(e, commissioning.ErrRemediationTarget) {
		status = 422
	}
	if errors.Is(e, commissioning.ErrStorageCorrupt) || errors.Is(e, commissioning.ErrPermitStorage) {
		status = 500
	}
	if errors.Is(e, commissioning.ErrReviewHistory) || errors.Is(e, commissioning.ErrExportInvalid) {
		status = 500
	}
	writeJSON(w, status, map[string]any{"error": e.Error()})
}
func expected(r *http.Request) int64 {
	v, _ := strconv.ParseInt(r.Header.Get("X-Expected-Version"), 10, 64)
	if v == 0 {
		v, _ = strconv.ParseInt(r.URL.Query().Get("expectedVersion"), 10, 64)
	}
	return v
}
func idem(r *http.Request) string { return strings.TrimSpace(r.Header.Get("Idempotency-Key")) }
func caseID(path string) string {
	p := strings.TrimPrefix(path, "/api/v1/commissioning-cases/")
	return strings.Split(p, "/")[0]
}
func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		filter, err := parseFilter(r)
		if err != nil {
			s.fail(w, err)
			return
		}
		result, err := s.app.List(filter)
		if err != nil {
			s.fail(w, err)
			return
		}
		writeJSON(w, 200, result)
		return
	}
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	var in application.CaseInput
	if e := readJSON(r, &in); e != nil {
		s.fail(w, commissioning.ErrInvalidInput)
		return
	}
	c, e := s.app.Create(in.ZoneCode, in.CollectionCategory, in.OwnerName, idem(r))
	if e != nil {
		s.fail(w, e)
		return
	}
	writeJSON(w, 201, c)
}
func (s *Server) permit(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && (r.URL.Path == "/api/v1/permits/validation" || r.URL.Path == "/api/v1/permits/validations" || r.URL.Path == "/api/v1/permits/batch-validation") {
		s.permitBatch(w, r)
		return
	}
	if r.Method != "GET" {
		w.WriteHeader(405)
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/permits/"), "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		s.fail(w, commissioning.ErrPermitNotFound)
		return
	}
	code := parts[0]
	if len(parts) == 2 && parts[1] == "validation" {
		result, e := s.app.ValidatePermit(code, time.Now().UTC())
		if e != nil {
			s.fail(w, e)
			return
		}
		writeJSON(w, 200, result)
		return
	}
	if len(parts) != 1 {
		s.fail(w, commissioning.ErrPermitNotFound)
		return
	}
	p, e := s.app.Permit(code)
	if e != nil {
		s.fail(w, e)
		return
	}
	writeJSON(w, 200, p)
}

func (s *Server) permitBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	permitCodes, err := readPermitCodes(r)
	if err != nil {
		s.fail(w, commissioning.ErrInvalidInput)
		return
	}
	result, err := s.app.ValidatePermits(permitCodes, time.Now().UTC())
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func readPermitCodes(r *http.Request) ([]string, error) {
	var raw json.RawMessage
	if err := readJSON(r, &raw); err != nil {
		return nil, commissioning.ErrInvalidInput
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, commissioning.ErrInvalidInput
	}
	if raw[0] == '[' {
		var codes []string
		if json.Unmarshal(raw, &codes) != nil {
			return nil, commissioning.ErrInvalidInput
		}
		return codes, nil
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) != 1 {
		return nil, commissioning.ErrInvalidInput
	}
	encoded, ok := fields["permitCodes"]
	if !ok {
		return nil, commissioning.ErrInvalidInput
	}
	var codes []string
	if json.Unmarshal(encoded, &codes) != nil {
		return nil, commissioning.ErrInvalidInput
	}
	return codes, nil
}
func parseCaseAction(path string) (string, string) {
	p := strings.TrimPrefix(path, "/api/v1/commissioning-cases/")
	parts := strings.Split(strings.Trim(p, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], "/")
}
func (s *Server) caseRoute(w http.ResponseWriter, r *http.Request) {
	id, action := parseCaseAction(r.URL.Path)
	if id == "" {
		s.fail(w, commissioning.ErrNotFound)
		return
	}
	if action == "" && r.Method == "GET" {
		c, e := s.app.Get(id)
		if e != nil {
			s.fail(w, e)
			return
		}
		writeJSON(w, 200, c)
		return
	}
	if action == "" && r.Method == "PATCH" {
		var in application.CaseInput
		if err := readJSON(r, &in); err != nil {
			s.fail(w, commissioning.ErrInvalidInput)
			return
		}
		out, err := s.app.ReviseIdentity(id, idem(r), expected(r), in)
		if err != nil {
			s.fail(w, err)
			return
		}
		writeJSON(w, 200, out)
		return
	}
	var out *commissioning.CommissioningCase
	var e error
	switch action {
	case "identity":
		if r.Method != "PATCH" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var in application.CaseInput
		e = readJSON(r, &in)
		if e == nil {
			out, e = s.app.ReviseIdentity(id, idem(r), expected(r), in)
		}
	case "baseline":
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var in application.BaselineInput
		e = readJSON(r, &in)
		if e == nil {
			out, e = s.app.Baseline(id, idem(r), expected(r), in.Domain(id))
		}
	case "baseline/revoke", "baseline-revocation":
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var in application.BaselineRevocationInput
		e = readJSON(r, &in)
		if e == nil {
			out, e = s.app.RevokeBaseline(id, idem(r), expected(r), in)
		}
	case "plan":
		if r.Method == "POST" {
			var in commissioning.ControlPlan
			e = readJSON(r, &in)
			if e == nil {
				out, e = s.app.Plan(id, idem(r), expected(r), in)
			}
		} else if r.Method == "PUT" || r.Method == "PATCH" {
			var in application.PlanRevisionInput
			e = readJSON(r, &in)
			if e == nil {
				out, e = s.app.RevisePlan(id, idem(r), expected(r), in)
			}
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	case "plan/revisions", "plan-revision":
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var in application.PlanRevisionInput
		e = readJSON(r, &in)
		if e == nil {
			out, e = s.app.RevisePlan(id, idem(r), expected(r), in)
		}
	case "start":
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		out, e = s.app.Start(id, idem(r), expected(r))
	case "observations", "observations/batch":
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var observations []commissioning.TrialObservation
		observations, e = readObservations(r)
		if e == nil {
			out, e = s.app.ObserveBatch(id, idem(r), expected(r), observations)
		}
	case "deviations":
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		query, err := parseDeviationLedgerQuery(r)
		if err != nil {
			s.fail(w, err)
			return
		}
		result, err := s.app.DeviationLedger(id, query)
		if err != nil {
			s.fail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	case "observations/summary", "summary", "trial-summary":
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		from, to, parseErr := parseObservationWindow(r)
		if parseErr != nil {
			s.fail(w, parseErr)
			return
		}
		result, err := s.app.ObservationSummary(id, from, to)
		if err != nil {
			s.fail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	case "remediation":
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var in application.RemediationInput
		e = readJSON(r, &in)
		if e == nil {
			out, e = s.app.Remediate(id, idem(r), expected(r), in)
		}
	case "review-package":
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		pkg, err := s.app.ReviewPackage(id)
		if err != nil {
			s.fail(w, err)
			return
		}
		writeJSON(w, 200, pkg)
		return
	case "review":
		if r.Method == http.MethodGet {
			query, err := parseReviewHistoryQuery(r)
			if err != nil {
				s.fail(w, err)
				return
			}
			result, err := s.app.ReviewHistory(id, query)
			if err != nil {
				s.fail(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
			return
		}
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var in commissioning.ReviewDecision
		e = readJSON(r, &in)
		if e == nil {
			expectedVersion := expected(r)
			if expectedVersion == 0 {
				expectedVersion = in.ReviewedVersion
			}
			out, e = s.app.Review(id, idem(r), expectedVersion, in)
		}
	case "reviews", "review-history":
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		query, err := parseReviewHistoryQuery(r)
		if err != nil {
			s.fail(w, err)
			return
		}
		result, err := s.app.ReviewHistory(id, query)
		if err != nil {
			s.fail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	case "export", "snapshot", "snapshot/export":
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		result, err := s.app.Export(id, strings.TrimSpace(r.URL.Query().Get("target")))
		if err != nil {
			s.fail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	case "activate":
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		out, e = s.app.Activate(id, idem(r), expected(r))
	default:
		s.fail(w, fmt.Errorf("未知操作"))
		return
	}
	if e != nil {
		s.fail(w, e)
		return
	}
	writeJSON(w, 200, out)
}

func readObservations(r *http.Request) ([]commissioning.TrialObservation, error) {
	var raw json.RawMessage
	if err := readJSON(r, &raw); err != nil {
		return nil, commissioning.ErrInvalidInput
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, commissioning.ErrInvalidInput
	}
	if raw[0] == '[' {
		var observations []commissioning.TrialObservation
		if json.Unmarshal(raw, &observations) != nil {
			return nil, commissioning.ErrInvalidInput
		}
		return observations, nil
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return nil, commissioning.ErrInvalidInput
	}
	if batch, ok := fields["observations"]; ok {
		if len(fields) != 1 {
			return nil, commissioning.ErrInvalidInput
		}
		var observations []commissioning.TrialObservation
		if json.Unmarshal(batch, &observations) != nil {
			return nil, commissioning.ErrInvalidInput
		}
		return observations, nil
	}
	var observation commissioning.TrialObservation
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&observation) != nil {
		return nil, commissioning.ErrInvalidInput
	}
	return []commissioning.TrialObservation{observation}, nil
}

func parseFilter(r *http.Request) (application.CaseFilter, error) {
	q := r.URL.Query()
	page, err := parseOptionalInt(q.Get("page"))
	if err != nil {
		return application.CaseFilter{}, commissioning.ErrInvalidInput
	}
	pageSize, err := parseOptionalInt(q.Get("pageSize"))
	if err != nil {
		return application.CaseFilter{}, commissioning.ErrInvalidInput
	}
	filter := application.CaseFilter{State: commissioning.State(first(q.Get("state"), q.Get("State"))), ZoneCode: first(q.Get("zoneCode"), q.Get("ZoneCode")), OwnerName: first(q.Get("ownerName"), q.Get("OwnerName")), Page: page, PageSize: pageSize}
	if filter.UpdatedFrom, err = parseOptionalTime(first(q.Get("updatedAtFrom"), q.Get("UpdatedAtFrom"))); err != nil {
		return application.CaseFilter{}, commissioning.ErrInvalidInput
	}
	if filter.UpdatedTo, err = parseOptionalTime(first(q.Get("updatedAtTo"), q.Get("UpdatedAtTo"))); err != nil {
		return application.CaseFilter{}, commissioning.ErrInvalidInput
	}
	return filter, nil
}

func parseOptionalInt(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	return strconv.Atoi(value)
}

func parseOptionalTime(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseObservationWindow(r *http.Request) (*time.Time, *time.Time, error) {
	q := r.URL.Query()
	from, err := parseOptionalTime(first(q.Get("from"), q.Get("start"), q.Get("observedAtFrom")))
	if err != nil {
		return nil, nil, commissioning.ErrInvalidInput
	}
	to, err := parseOptionalTime(first(q.Get("to"), q.Get("end"), q.Get("observedAtTo")))
	if err != nil || (from != nil && to != nil && from.After(*to)) {
		return nil, nil, commissioning.ErrInvalidInput
	}
	return from, to, nil
}

func parseReviewHistoryQuery(r *http.Request) (commissioning.ReviewHistoryQuery, error) {
	q := r.URL.Query()
	from, err := parseOptionalTime(first(q.Get("reviewedAtFrom"), q.Get("from"), q.Get("start")))
	if err != nil {
		return commissioning.ReviewHistoryQuery{}, commissioning.ErrInvalidInput
	}
	to, err := parseOptionalTime(first(q.Get("reviewedAtTo"), q.Get("to"), q.Get("end")))
	if err != nil || (from != nil && to != nil && from.After(*to)) {
		return commissioning.ReviewHistoryQuery{}, commissioning.ErrInvalidInput
	}
	decision := commissioning.Decision(first(q.Get("decision"), q.Get("Decision")))
	return commissioning.ReviewHistoryQuery{Decision: decision, ReviewerName: first(q.Get("reviewerName"), q.Get("ReviewerName")), From: from, To: to}, nil
}

func parseDeviationLedgerQuery(r *http.Request) (commissioning.DeviationLedgerQuery, error) {
	q := r.URL.Query()
	page, err := parseOptionalInt(q.Get("page"))
	if err != nil {
		return commissioning.DeviationLedgerQuery{}, commissioning.ErrInvalidInput
	}
	pageSize, err := parseOptionalInt(q.Get("pageSize"))
	if err != nil {
		return commissioning.DeviationLedgerQuery{}, commissioning.ErrInvalidInput
	}
	from, err := parseOptionalTime(first(q.Get("observedAtFrom"), q.Get("from"), q.Get("start")))
	if err != nil {
		return commissioning.DeviationLedgerQuery{}, commissioning.ErrInvalidInput
	}
	to, err := parseOptionalTime(first(q.Get("observedAtTo"), q.Get("to"), q.Get("end")))
	if err != nil || from != nil && to != nil && from.After(*to) {
		return commissioning.DeviationLedgerQuery{}, commissioning.ErrInvalidInput
	}
	return commissioning.DeviationLedgerQuery{
		Status: commissioning.DeviationStatus(q.Get("status")), RuleCode: q.Get("ruleCode"), Severity: q.Get("severity"),
		ObservedAtFrom: from, ObservedAtTo: to, Page: page, PageSize: pageSize,
	}, nil
}

func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
