package commissioning

import (
	"fmt"
	"strings"
	"time"
)

const observationFutureTolerance = 5 * time.Minute

func (c *CommissioningCase) AddObservation(o TrialObservation, now time.Time) error {
	return c.AddObservations([]TrialObservation{o}, now)
}

// AddObservations validates and evaluates a complete observation batch on a
// detached case copy. The receiver is replaced only after every observation
// succeeds, so callers can persist the batch as one versioned mutation.
func (c *CommissioningCase) AddObservations(observations []TrialObservation, now time.Time) error {
	if c.State != TrialRunning && c.State != NeedsRemediation {
		return ErrInvalidTransition
	}
	if c.Baseline == nil || c.Plan == nil {
		return ErrInvalidTransition
	}
	if len(observations) == 0 {
		return ErrInvalidInput
	}
	working := c.clone()
	for _, observation := range observations {
		if observation.SupersedesID != "" {
			return ErrRemediationTarget
		}
		if err := working.appendObservation(observation, now, false); err != nil {
			return err
		}
	}
	working.evaluate()
	working.touch(now)
	*c = *working
	return nil
}

func (c *CommissioningCase) RemediateDeviations(targetIDs []string, note string, retests []TrialObservation, now time.Time) error {
	if c.State != NeedsRemediation || c.Baseline == nil || c.Plan == nil {
		return ErrInvalidTransition
	}
	note = strings.TrimSpace(note)
	if note == "" || len(targetIDs) == 0 || len(retests) == 0 {
		return ErrInvalidInput
	}
	if len(targetIDs) != len(retests) {
		return ErrRemediationTarget
	}

	targets := make(map[string]Deviation, len(targetIDs))
	for _, id := range targetIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			return ErrRemediationTarget
		}
		if _, duplicate := targets[id]; duplicate {
			return ErrRemediationTarget
		}
		found := false
		for _, deviation := range c.Deviations {
			if deviation.DeviationID == id && deviation.Status == DeviationOpen && deviation.ObservationID != "" && !c.isObservationSuperseded(deviation.ObservationID) {
				targets[id] = deviation
				found = true
				break
			}
		}
		if !found {
			return ErrRemediationTarget
		}
	}

	working := c.clone()
	added := make([]TrialObservation, 0, len(retests))
	seenSupersedes := make(map[string]bool, len(retests))
	for _, retest := range retests {
		if strings.TrimSpace(retest.SupersedesID) == "" {
			return ErrRemediationTarget
		}
		matchesTarget := false
		for _, target := range targets {
			if target.ObservationID == retest.SupersedesID {
				matchesTarget = true
				break
			}
		}
		if !matchesTarget {
			return ErrRemediationTarget
		}
		if seenSupersedes[retest.SupersedesID] {
			return ErrRemediationTarget
		}
		seenSupersedes[retest.SupersedesID] = true
		if err := working.appendObservation(retest, now, true); err != nil {
			return err
		}
		added = append(added, working.Observations[len(working.Observations)-1])
	}

	for id, target := range targets {
		resolved := false
		for _, retest := range added {
			if retest.SupersedesID == target.ObservationID && working.observationPassesRule(retest, target.RuleCode) {
				resolved = true
				break
			}
		}
		if !resolved {
			continue
		}
		for i := range working.Deviations {
			if working.Deviations[i].DeviationID == id {
				working.Deviations[i].Status = DeviationResolved
				working.Deviations[i].ResolutionNote = note
				working.Deviations[i].ResolvedAt = &now
				break
			}
		}
	}
	working.evaluate()
	working.touch(now)
	*c = *working
	return nil
}

func (c *CommissioningCase) isObservationSuperseded(observationID string) bool {
	for _, observation := range c.Observations {
		if observation.SupersedesID == observationID {
			return true
		}
	}
	return false
}

func (c *CommissioningCase) appendObservation(o TrialObservation, now time.Time, remediation bool) error {
	if err := c.prepareObservation(&o, now, remediation); err != nil {
		return err
	}
	previous := c.lastEffectiveObservation()
	c.Observations = append(c.Observations, o)
	if previous != nil && o.ObservedAt.Sub(previous.ObservedAt) > time.Duration(c.Baseline.SamplingIntervalMinutes)*time.Minute {
		c.ensureDeviation("SAMPLE_GAP", "medium", "相邻有效观测间隔超过锁定采样频率", o.ObservationID)
	}
	for _, result := range EvaluateObservation(o, *c.Baseline) {
		c.ensureDeviation(result.Code, result.Severity, result.Description, o.ObservationID)
	}
	return nil
}

func (c *CommissioningCase) prepareObservation(o *TrialObservation, now time.Time, remediation bool) error {
	expectedSequence := len(c.Observations) + 1
	if o.Sequence != expectedSequence {
		return ErrObservationOrder
	}
	if o.ObservationID == "" {
		o.ObservationID = fmt.Sprintf("obs-%s-%d", c.CaseID, expectedSequence)
	}
	o.ObservationID = strings.TrimSpace(o.ObservationID)
	o.RecordedBy = strings.TrimSpace(o.RecordedBy)
	o.SupersedesID = strings.TrimSpace(o.SupersedesID)
	if o.ObservationID == "" || o.RecordedBy == "" || !finite(o.Temperature) || !finite(o.Humidity) {
		return ErrInvalidObservation
	}
	if o.DeviceStatus != DeviceNormal && o.DeviceStatus != DeviceAbnormal {
		return ErrInvalidObservation
	}
	if o.ObservedAt.IsZero() {
		o.ObservedAt = now
	}
	if o.ObservedAt.After(now.Add(observationFutureTolerance)) {
		return ErrInvalidObservation
	}
	if len(c.Observations) > 0 && !o.ObservedAt.After(c.Observations[len(c.Observations)-1].ObservedAt) {
		return ErrObservationOrder
	}
	for _, existing := range c.Observations {
		if existing.ObservationID == o.ObservationID {
			return ErrInvalidObservation
		}
	}
	if remediation {
		original := c.observationByID(o.SupersedesID)
		if original == nil || original.ObservationID == o.ObservationID {
			return ErrRemediationTarget
		}
		for _, existing := range c.Observations {
			if existing.SupersedesID == o.SupersedesID {
				return ErrRemediationTarget
			}
		}
		for current := original; current != nil && current.SupersedesID != ""; current = c.observationByID(current.SupersedesID) {
			if current.SupersedesID == o.ObservationID {
				return ErrRemediationTarget
			}
		}
	} else if o.SupersedesID != "" {
		return ErrRemediationTarget
	}
	o.CaseID = c.CaseID
	return nil
}

func (c *CommissioningCase) observationPassesRule(o TrialObservation, ruleCode string) bool {
	switch ruleCode {
	case "TEMP_RANGE":
		return o.Temperature >= c.Baseline.TemperatureMin && o.Temperature <= c.Baseline.TemperatureMax
	case "HUMIDITY_RANGE":
		return o.Humidity >= c.Baseline.HumidityMin && o.Humidity <= c.Baseline.HumidityMax
	case "DEVICE_STATUS":
		return o.DeviceStatus == DeviceNormal
	case "SAMPLE_GAP":
		for _, deviation := range c.Deviations {
			if deviation.RuleCode == "SAMPLE_GAP" && deviation.ObservationID == o.ObservationID && deviation.Status == DeviationOpen {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func (c *CommissioningCase) evaluate() {
	if c.Plan == nil || c.Baseline == nil {
		return
	}
	if c.OpenDeviations() > 0 {
		c.State = NeedsRemediation
		return
	}
	effective := c.EffectiveObservations()
	if TrialWindowSatisfied(effective, *c.Plan, *c.Baseline) {
		c.State = AwaitingReview
		return
	}
	c.State = TrialRunning
}

func (c *CommissioningCase) EffectiveObservations() []TrialObservation {
	superseded := make(map[string]bool)
	for _, observation := range c.Observations {
		if observation.SupersedesID != "" {
			superseded[observation.SupersedesID] = true
		}
	}
	result := make([]TrialObservation, 0, len(c.Observations))
	for _, observation := range c.Observations {
		if !superseded[observation.ObservationID] {
			result = append(result, observation)
		}
	}
	return result
}

func (c *CommissioningCase) lastEffectiveObservation() *TrialObservation {
	effective := c.EffectiveObservations()
	if len(effective) == 0 {
		return nil
	}
	last := effective[len(effective)-1]
	return &last
}

func (c *CommissioningCase) observationByID(id string) *TrialObservation {
	for i := range c.Observations {
		if c.Observations[i].ObservationID == id {
			return &c.Observations[i]
		}
	}
	return nil
}

func (c *CommissioningCase) clone() *CommissioningCase {
	copyCase := *c
	copyCase.BaselineHistory = append([]BaselineProfile(nil), c.BaselineHistory...)
	copyCase.BaselineRevocations = append([]BaselineRevocation(nil), c.BaselineRevocations...)
	copyCase.PlanHistory = append([]PlanRevision(nil), c.PlanHistory...)
	copyCase.Observations = append([]TrialObservation(nil), c.Observations...)
	copyCase.Deviations = append([]Deviation(nil), c.Deviations...)
	copyCase.Reviews = append([]ReviewDecision(nil), c.Reviews...)
	if c.Baseline != nil {
		baseline := *c.Baseline
		copyCase.Baseline = &baseline
	}
	if c.Plan != nil {
		plan := *c.Plan
		copyCase.Plan = &plan
	}
	if c.Permit != nil {
		permit := *c.Permit
		copyCase.Permit = &permit
	}
	return &copyCase
}
