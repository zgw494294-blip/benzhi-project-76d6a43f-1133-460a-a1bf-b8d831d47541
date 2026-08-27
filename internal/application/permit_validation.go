package application

import (
	"errors"
	"strings"
	"time"

	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/domain/commissioning"
)

type PermitValidation struct {
	PermitCode         string    `json:"permitCode"`
	Status             string    `json:"status"`
	CaseID             string    `json:"caseId,omitempty"`
	ZoneCode           string    `json:"zoneCode,omitempty"`
	CollectionCategory string    `json:"collectionCategory,omitempty"`
	BaselineRevision   int       `json:"baselineRevision,omitempty"`
	PlanRevision       int       `json:"planRevision,omitempty"`
	ApprovedBy         string    `json:"approvedBy,omitempty"`
	IssuedAt           time.Time `json:"issuedAt,omitempty"`
	EffectiveUntil     time.Time `json:"effectiveUntil,omitempty"`
	Error              string    `json:"error,omitempty"`
}

type PermitValidationSummary struct {
	Active   int `json:"active"`
	Expired  int `json:"expired"`
	NotFound int `json:"not_found"`
	Error    int `json:"error"`
}

type PermitBatchValidation struct {
	Items   []PermitValidation      `json:"items"`
	Summary PermitValidationSummary `json:"summary"`
}

func (s *Service) ValidatePermit(code string, now time.Time) (PermitValidation, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return PermitValidation{}, commissioning.ErrInvalidInput
	}
	permit, err := s.repo.FindPermit(code)
	if err != nil {
		return PermitValidation{}, err
	}
	c, err := s.repo.Get(permit.CaseID)
	if err != nil {
		return PermitValidation{}, commissioning.ErrPermitStorage
	}
	if c.CaseID != permit.CaseID || c.State != commissioning.Activated || c.Permit == nil ||
		c.Permit.PermitID != permit.PermitID || c.Permit.CaseID != c.CaseID || c.Permit.PermitCode != code || permit.PermitCode != code ||
		permit.Status != "active" || c.Permit.Status != "active" || permit.ActivatedVersion != c.ExpectedVersion || c.Permit.ActivatedVersion != c.ExpectedVersion ||
		permit.IssuedAt.IsZero() || permit.EffectiveUntil.IsZero() || !permit.EffectiveUntil.After(permit.IssuedAt) ||
		!c.Permit.IssuedAt.Equal(permit.IssuedAt) || !c.Permit.EffectiveUntil.Equal(permit.EffectiveUntil) {
		return PermitValidation{}, commissioning.ErrStorageCorrupt
	}
	result := PermitValidation{PermitCode: permit.PermitCode, Status: "active", CaseID: c.CaseID, ZoneCode: c.ZoneCode, CollectionCategory: c.CollectionCategory, IssuedAt: permit.IssuedAt, EffectiveUntil: permit.EffectiveUntil}
	if !now.Before(permit.EffectiveUntil) {
		result.Status = "expired"
	}
	if c.Baseline != nil && c.Baseline.CaseID == c.CaseID {
		result.BaselineRevision = c.Baseline.Revision
	}
	if c.Plan != nil && c.Plan.CaseID == c.CaseID && len(c.PlanHistory) > 0 {
		latest := c.PlanHistory[len(c.PlanHistory)-1]
		if latest.Plan.PlanID == c.Plan.PlanID && latest.Plan.CaseID == c.CaseID {
			result.PlanRevision = latest.Revision
		}
	}
	for i := len(c.Reviews) - 1; i >= 0; i-- {
		if c.Reviews[i].Decision == commissioning.DecisionApprove && c.Reviews[i].CaseID == c.CaseID {
			result.ApprovedBy = strings.TrimSpace(c.Reviews[i].ReviewerName)
			break
		}
	}
	if result.BaselineRevision == 0 || result.PlanRevision == 0 || result.ApprovedBy == "" {
		return PermitValidation{}, commissioning.ErrStorageCorrupt
	}
	return result, nil
}

func (s *Service) ValidatePermits(codes []string, now time.Time) (PermitBatchValidation, error) {
	if len(codes) == 0 || len(codes) > 100 {
		return PermitBatchValidation{}, commissioning.ErrInvalidInput
	}
	normalized := make([]string, 0, len(codes))
	seen := make(map[string]bool, len(codes))
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" || seen[code] {
			return PermitBatchValidation{}, commissioning.ErrInvalidInput
		}
		seen[code] = true
		normalized = append(normalized, code)
	}
	result := PermitBatchValidation{Items: make([]PermitValidation, 0, len(normalized))}
	for _, code := range normalized {
		item, err := s.ValidatePermit(code, now)
		switch {
		case err == nil:
			if item.Status == "active" {
				result.Summary.Active++
			} else {
				result.Summary.Expired++
			}
		case errors.Is(err, commissioning.ErrPermitNotFound):
			item = PermitValidation{PermitCode: code, Status: "not_found"}
			result.Summary.NotFound++
		case errors.Is(err, commissioning.ErrStorageCorrupt), errors.Is(err, commissioning.ErrPermitStorage):
			return PermitBatchValidation{}, err
		default:
			item = PermitValidation{PermitCode: code, Status: "error", Error: err.Error()}
			result.Summary.Error++
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}
