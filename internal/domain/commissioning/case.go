package commissioning

import (
	"fmt"
	"strings"
	"time"
)

func NewCase(id, zone, category, owner string, now time.Time) (*CommissioningCase, error) {
	zone, category, owner = NormalizeIdentity(zone, category, owner)
	if err := ValidateIdentity(zone, category, owner); err != nil {
		return nil, err
	}
	return &CommissioningCase{CaseID: id, ZoneCode: zone, CollectionCategory: category, OwnerName: owner, State: Draft, ExpectedVersion: 1, CreatedAt: now, UpdatedAt: now, Observations: []TrialObservation{}, Deviations: []Deviation{}, Reviews: []ReviewDecision{}}, nil
}
func (c *CommissioningCase) touch(now time.Time) { c.ExpectedVersion++; c.UpdatedAt = now }

func (c *CommissioningCase) ReviseIdentity(zone, category, owner string, now time.Time) error {
	if c.State != Draft {
		return ErrInvalidTransition
	}
	zone, category, owner = NormalizeIdentity(zone, category, owner)
	if err := ValidateIdentity(zone, category, owner); err != nil {
		return err
	}
	c.ZoneCode = zone
	c.CollectionCategory = category
	c.OwnerName = owner
	c.touch(now)
	return nil
}

func (c *CommissioningCase) SetBaseline(b BaselineProfile, now time.Time) error {
	if c.State != Draft {
		return ErrInvalidTransition
	}
	if err := ValidateBaseline(b); err != nil {
		return err
	}
	b = QuantizeBaseline(b)
	b.CaseID = c.CaseID
	b.LockedAt = now
	b.Revision = nextBaselineRevision(c.BaselineHistory)
	b.BaselineID = fmt.Sprintf("baseline-%s-%d", c.CaseID, b.Revision)
	c.Baseline = &b
	c.BaselineHistory = append(c.BaselineHistory, b)
	c.State = BaselineLocked
	c.touch(now)
	return nil
}
func (c *CommissioningCase) SubmitPlan(p ControlPlan, now time.Time) error {
	if c.State != BaselineLocked || c.Baseline == nil {
		return ErrInvalidTransition
	}
	if err := ValidatePlan(p, *c.Baseline); err != nil {
		return err
	}
	p.DeviceLabel = strings.TrimSpace(p.DeviceLabel)
	p.ControlMode = strings.ToLower(strings.TrimSpace(p.ControlMode))
	p.SubmittedBy = strings.TrimSpace(p.SubmittedBy)
	p.CaseID = c.CaseID
	p.SubmittedAt = now
	p.PlanID = fmt.Sprintf("plan-%s-%d", c.CaseID, nextPlanRevision(c.PlanHistory))
	c.Plan = &p
	c.PlanHistory = append(c.PlanHistory, PlanRevision{Revision: nextPlanRevision(c.PlanHistory), Plan: p, SubmittedAt: now, SubmittedBy: p.SubmittedBy})
	c.State = TrialReady
	c.touch(now)
	return nil
}

func (c *CommissioningCase) RevokeBaseline(reason, operator string, now time.Time) error {
	if c.State != BaselineLocked || c.Plan != nil || c.Baseline == nil {
		return ErrInvalidTransition
	}
	reason, operator = strings.TrimSpace(reason), strings.TrimSpace(operator)
	if reason == "" || operator == "" {
		return ErrInvalidInput
	}
	c.BaselineRevocations = append(c.BaselineRevocations, BaselineRevocation{Reason: reason, Operator: operator, RevokedAt: now, OriginalBaselineRevision: c.Baseline.Revision})
	c.Baseline = nil
	c.State = Draft
	c.touch(now)
	return nil
}

func (c *CommissioningCase) RevisePlan(p ControlPlan, reason string, now time.Time) error {
	if c.State != TrialReady || c.Baseline == nil || c.Plan == nil {
		return ErrInvalidTransition
	}
	reason = strings.TrimSpace(reason)
	p.DeviceLabel = strings.TrimSpace(p.DeviceLabel)
	p.ControlMode = strings.ToLower(strings.TrimSpace(p.ControlMode))
	p.SubmittedBy = strings.TrimSpace(p.SubmittedBy)
	if reason == "" {
		return ErrInvalidInput
	}
	if err := ValidatePlan(p, *c.Baseline); err != nil {
		return err
	}
	revision := nextPlanRevision(c.PlanHistory)
	p.CaseID = c.CaseID
	p.SubmittedAt = now
	p.PlanID = fmt.Sprintf("plan-%s-%d", c.CaseID, revision)
	c.Plan = &p
	c.PlanHistory = append(c.PlanHistory, PlanRevision{Revision: revision, Plan: p, Reason: reason, SubmittedAt: now, SubmittedBy: p.SubmittedBy})
	c.touch(now)
	return nil
}
func (c *CommissioningCase) StartTrial(now time.Time) error {
	if c.State != TrialReady {
		return ErrInvalidTransition
	}
	c.State = TrialRunning
	c.touch(now)
	return nil
}
func (c *CommissioningCase) ensureDeviation(code, severity, desc, observationID string) {
	for i := range c.Deviations {
		if c.Deviations[i].RuleCode == code && c.Deviations[i].ObservationID == observationID {
			return
		}
	}
	c.Deviations = append(c.Deviations, Deviation{DeviationID: fmt.Sprintf("dev-%s-%d", c.CaseID, len(c.Deviations)+1), CaseID: c.CaseID, RuleCode: code, Severity: severity, Description: desc, Status: DeviationOpen, ObservationID: observationID})
}

func nextBaselineRevision(history []BaselineProfile) int {
	next := 1
	for _, item := range history {
		if item.Revision >= next {
			next = item.Revision + 1
		}
	}
	return next
}

func nextPlanRevision(history []PlanRevision) int {
	next := 1
	for _, item := range history {
		if item.Revision >= next {
			next = item.Revision + 1
		}
	}
	return next
}
