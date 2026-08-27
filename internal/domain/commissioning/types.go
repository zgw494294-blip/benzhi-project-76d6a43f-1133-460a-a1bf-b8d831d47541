package commissioning

import "time"

type State string

const (
	Draft            State = "Draft"
	BaselineLocked   State = "BaselineLocked"
	TrialReady       State = "TrialReady"
	TrialRunning     State = "TrialRunning"
	NeedsRemediation State = "NeedsRemediation"
	AwaitingReview   State = "AwaitingReview"
	Approved         State = "Approved"
	Activated        State = "Activated"
)

type DeviceStatus string

const (
	DeviceNormal   DeviceStatus = "normal"
	DeviceAbnormal DeviceStatus = "abnormal"
)

type Decision string

const (
	DecisionApprove Decision = "approve"
	DecisionReturn  Decision = "return"
)

type DeviationStatus string

const (
	DeviationOpen     DeviationStatus = "open"
	DeviationResolved DeviationStatus = "resolved"
)

type BaselineProfile struct {
	BaselineID              string    `json:"baselineId"`
	CaseID                  string    `json:"caseId"`
	TemperatureMin          float64   `json:"temperatureMin"`
	TemperatureMax          float64   `json:"temperatureMax"`
	HumidityMin             float64   `json:"humidityMin"`
	HumidityMax             float64   `json:"humidityMax"`
	SamplingIntervalMinutes int       `json:"samplingIntervalMinutes"`
	MinimumObservationCount int       `json:"minimumObservationCount"`
	LockedAt                time.Time `json:"lockedAt"`
	Revision                int       `json:"revision"`
}
type BaselineRevocation struct {
	Reason                   string    `json:"reason"`
	Operator                 string    `json:"operator"`
	RevokedAt                time.Time `json:"revokedAt"`
	OriginalBaselineRevision int       `json:"originalBaselineRevision"`
}
type PlanRevision struct {
	Revision    int         `json:"revision"`
	Plan        ControlPlan `json:"plan"`
	Reason      string      `json:"reason"`
	SubmittedAt time.Time   `json:"submittedAt"`
	SubmittedBy string      `json:"submittedBy"`
}
type ControlPlan struct {
	PlanID              string    `json:"planId"`
	CaseID              string    `json:"caseId"`
	DeviceLabel         string    `json:"deviceLabel"`
	ControlMode         string    `json:"controlMode"`
	SetpointTemperature float64   `json:"setpointTemperature"`
	SetpointHumidity    float64   `json:"setpointHumidity"`
	TrialDurationHours  int       `json:"trialDurationHours"`
	SubmittedBy         string    `json:"submittedBy"`
	SubmittedAt         time.Time `json:"submittedAt"`
}
type TrialObservation struct {
	ObservationID string       `json:"observationId"`
	CaseID        string       `json:"caseId"`
	Sequence      int          `json:"sequence"`
	ObservedAt    time.Time    `json:"observedAt"`
	Temperature   float64      `json:"temperature"`
	Humidity      float64      `json:"humidity"`
	DeviceStatus  DeviceStatus `json:"deviceStatus"`
	Note          string       `json:"note"`
	SupersedesID  string       `json:"supersedesId,omitempty"`
	RecordedBy    string       `json:"recordedBy"`
}
type Deviation struct {
	DeviationID    string          `json:"deviationId"`
	CaseID         string          `json:"caseId"`
	RuleCode       string          `json:"ruleCode"`
	Severity       string          `json:"severity"`
	Description    string          `json:"description"`
	Status         DeviationStatus `json:"status"`
	ResolutionNote string          `json:"resolutionNote,omitempty"`
	ResolvedAt     *time.Time      `json:"resolvedAt,omitempty"`
	ObservationID  string          `json:"observationId,omitempty"`
}
type ReviewDecision struct {
	ReviewID           string    `json:"reviewId"`
	CaseID             string    `json:"caseId"`
	ReviewerName       string    `json:"reviewerName"`
	Decision           Decision  `json:"decision"`
	Comment            string    `json:"comment"`
	ReviewedVersion    int64     `json:"reviewedVersion"`
	ReviewedAt         time.Time `json:"reviewedAt"`
	PackageFingerprint string    `json:"packageFingerprint,omitempty"`
}
type ActivationPermit struct {
	PermitID         string    `json:"permitId"`
	CaseID           string    `json:"caseId"`
	PermitCode       string    `json:"permitCode"`
	ActivatedVersion int64     `json:"activatedVersion"`
	IssuedAt         time.Time `json:"issuedAt"`
	EffectiveUntil   time.Time `json:"effectiveUntil"`
	Status           string    `json:"status"`
}
type CommissioningCase struct {
	CaseID              string               `json:"caseId"`
	ZoneCode            string               `json:"zoneCode"`
	CollectionCategory  string               `json:"collectionCategory"`
	OwnerName           string               `json:"ownerName"`
	State               State                `json:"state"`
	ExpectedVersion     int64                `json:"expectedVersion"`
	CreatedAt           time.Time            `json:"createdAt"`
	UpdatedAt           time.Time            `json:"updatedAt"`
	Baseline            *BaselineProfile     `json:"baseline,omitempty"`
	BaselineHistory     []BaselineProfile    `json:"baselineHistory,omitempty"`
	BaselineRevocations []BaselineRevocation `json:"baselineRevocations,omitempty"`
	Plan                *ControlPlan         `json:"plan,omitempty"`
	PlanHistory         []PlanRevision       `json:"planHistory,omitempty"`
	Observations        []TrialObservation   `json:"observations"`
	Deviations          []Deviation          `json:"deviations"`
	Reviews             []ReviewDecision     `json:"reviews"`
	Permit              *ActivationPermit    `json:"permit,omitempty"`
}

type ObservationSummary struct {
	From                      *time.Time     `json:"from,omitempty"`
	To                        *time.Time     `json:"to,omitempty"`
	EffectiveObservationCount int            `json:"effectiveObservationCount"`
	TemperatureMin            *float64       `json:"temperatureMin,omitempty"`
	TemperatureMax            *float64       `json:"temperatureMax,omitempty"`
	TemperatureAverage        float64        `json:"temperatureAverage"`
	HumidityMin               *float64       `json:"humidityMin,omitempty"`
	HumidityMax               *float64       `json:"humidityMax,omitempty"`
	HumidityAverage           float64        `json:"humidityAverage"`
	OpenDeviationCounts       map[string]int `json:"openDeviationCounts"`
	ResolvedDeviationCounts   map[string]int `json:"resolvedDeviationCounts"`
	ObservationProgress       int            `json:"observationProgress"`
	ObservationRequired       int            `json:"observationRequired"`
	ObservationCompletion     float64        `json:"observationCompletion"`
	DurationMinutes           int64          `json:"durationMinutes"`
	DurationRequiredMinutes   int            `json:"durationRequiredMinutes"`
	DurationCompletion        float64        `json:"durationCompletion"`
}
