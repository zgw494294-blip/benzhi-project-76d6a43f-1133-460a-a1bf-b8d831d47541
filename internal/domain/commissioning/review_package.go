package commissioning

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

type ObservationReplacement struct {
	ObservationID string `json:"observationId"`
	SupersedesID  string `json:"supersedesId"`
}

type ReviewPackage struct {
	CaseID                string                   `json:"caseId"`
	ExpectedVersion       int64                    `json:"expectedVersion"`
	Baseline              *BaselineProfile         `json:"baseline,omitempty"`
	LatestPlanRevision    *PlanRevision            `json:"latestPlanRevision,omitempty"`
	EffectiveObservations []TrialObservation       `json:"effectiveObservations"`
	ReplacementRelations  []ObservationReplacement `json:"replacementRelations"`
	Deviations            []Deviation              `json:"deviations"`
	PackageFingerprint    string                   `json:"packageFingerprint"`
}

type reviewPackageContent struct {
	CaseID                string                   `json:"caseId"`
	ExpectedVersion       int64                    `json:"expectedVersion"`
	Baseline              *BaselineProfile         `json:"baseline,omitempty"`
	LatestPlanRevision    *PlanRevision            `json:"latestPlanRevision,omitempty"`
	EffectiveObservations []TrialObservation       `json:"effectiveObservations"`
	ReplacementRelations  []ObservationReplacement `json:"replacementRelations"`
	Deviations            []Deviation              `json:"deviations"`
}

func (c *CommissioningCase) BuildReviewPackage() (ReviewPackage, error) {
	content := reviewPackageContent{
		CaseID:                c.CaseID,
		ExpectedVersion:       c.ExpectedVersion,
		EffectiveObservations: append([]TrialObservation{}, c.EffectiveObservations()...),
		ReplacementRelations:  make([]ObservationReplacement, 0),
		Deviations:            append([]Deviation{}, c.Deviations...),
	}
	if c.Baseline != nil {
		baseline := *c.Baseline
		content.Baseline = &baseline
	}
	if len(c.PlanHistory) > 0 {
		latest := c.PlanHistory[0]
		for _, revision := range c.PlanHistory[1:] {
			if revision.Revision > latest.Revision {
				latest = revision
			}
		}
		content.LatestPlanRevision = &latest
	}
	for _, observation := range c.Observations {
		if observation.SupersedesID != "" {
			content.ReplacementRelations = append(content.ReplacementRelations, ObservationReplacement{ObservationID: observation.ObservationID, SupersedesID: observation.SupersedesID})
		}
	}
	payload, err := json.Marshal(content)
	if err != nil {
		return ReviewPackage{}, err
	}
	sum := sha256.Sum256(payload)
	return ReviewPackage{
		CaseID:                content.CaseID,
		ExpectedVersion:       content.ExpectedVersion,
		Baseline:              content.Baseline,
		LatestPlanRevision:    content.LatestPlanRevision,
		EffectiveObservations: content.EffectiveObservations,
		ReplacementRelations:  content.ReplacementRelations,
		Deviations:            content.Deviations,
		PackageFingerprint:    hex.EncodeToString(sum[:]),
	}, nil
}
