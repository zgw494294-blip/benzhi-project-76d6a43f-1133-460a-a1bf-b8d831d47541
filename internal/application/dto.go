package application

import "github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/domain/commissioning"
import "time"

type CaseInput struct {
	ZoneCode           string `json:"zoneCode"`
	CollectionCategory string `json:"collectionCategory"`
	OwnerName          string `json:"ownerName"`
}

type BaselineRevocationInput struct {
	Reason   string `json:"reason"`
	Operator string `json:"operator"`
}

type PlanRevisionInput struct {
	commissioning.ControlPlan
	Reason string `json:"reason"`
}

type RemediationInput struct {
	DeviationIDs       []string                         `json:"deviationIds"`
	TargetDeviationIDs []string                         `json:"targetDeviationIds,omitempty"`
	ResolutionNote     string                           `json:"resolutionNote"`
	RetestObservations []commissioning.TrialObservation `json:"retestObservations"`
	Observations       []commissioning.TrialObservation `json:"observations,omitempty"`
}

func (i RemediationInput) Targets() []string {
	if len(i.DeviationIDs) > 0 {
		return i.DeviationIDs
	}
	return i.TargetDeviationIDs
}

func (i RemediationInput) Retests() []commissioning.TrialObservation {
	if len(i.RetestObservations) > 0 {
		return i.RetestObservations
	}
	return i.Observations
}

type CaseFilter struct {
	State       commissioning.State
	ZoneCode    string
	OwnerName   string
	UpdatedFrom *time.Time
	UpdatedTo   *time.Time
	Page        int
	PageSize    int
}
type BaselineInput struct {
	TemperatureMin          float64 `json:"temperatureMin"`
	TemperatureMax          float64 `json:"temperatureMax"`
	HumidityMin             float64 `json:"humidityMin"`
	HumidityMax             float64 `json:"humidityMax"`
	SamplingIntervalMinutes int     `json:"samplingIntervalMinutes"`
	MinimumObservationCount int     `json:"minimumObservationCount"`
}

func (i BaselineInput) Domain(id string) commissioning.BaselineProfile {
	return commissioning.BaselineProfile{BaselineID: "baseline-" + id, CaseID: id, TemperatureMin: i.TemperatureMin, TemperatureMax: i.TemperatureMax, HumidityMin: i.HumidityMin, HumidityMax: i.HumidityMax, SamplingIntervalMinutes: i.SamplingIntervalMinutes, MinimumObservationCount: i.MinimumObservationCount}
}
