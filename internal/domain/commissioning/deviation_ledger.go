package commissioning

import (
	"sort"
	"strings"
	"time"
)

const MaxDeviationPageSize = 100

type DeviationLedgerQuery struct {
	Status         DeviationStatus
	RuleCode       string
	Severity       string
	ObservedAtFrom *time.Time
	ObservedAtTo   *time.Time
	Page           int
	PageSize       int
}

type DeviationLedgerItem struct {
	DeviationID               string          `json:"deviationId"`
	RuleCode                  string          `json:"ruleCode"`
	Severity                  string          `json:"severity"`
	Description               string          `json:"description"`
	Status                    DeviationStatus `json:"status"`
	ObservationID             string          `json:"observationId"`
	ObservedAt                time.Time       `json:"observedAt"`
	SupersedesID              string          `json:"supersedesId,omitempty"`
	SupersededByObservationID string          `json:"supersededByObservationId,omitempty"`
	ResolutionNote            string          `json:"resolutionNote,omitempty"`
	ResolvedAt                *time.Time      `json:"resolvedAt,omitempty"`
	CanRemediate              bool            `json:"canRemediate"`
}

type DeviationLedger struct {
	CaseID       string                  `json:"caseId"`
	Items        []DeviationLedgerItem   `json:"items"`
	Total        int                     `json:"total"`
	Page         int                     `json:"page"`
	PageSize     int                     `json:"pageSize"`
	TotalPages   int                     `json:"totalPages"`
	StatusCounts map[DeviationStatus]int `json:"statusCounts"`
}

func (c *CommissioningCase) DeviationLedger(query DeviationLedgerQuery) (DeviationLedger, error) {
	if query.Page == 0 {
		query.Page = 1
	}
	if query.PageSize == 0 {
		query.PageSize = 20
	}
	query.RuleCode = strings.TrimSpace(query.RuleCode)
	query.Severity = strings.TrimSpace(query.Severity)
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > MaxDeviationPageSize ||
		(query.Status != "" && query.Status != DeviationOpen && query.Status != DeviationResolved) ||
		(query.ObservedAtFrom != nil && query.ObservedAtTo != nil && query.ObservedAtFrom.After(*query.ObservedAtTo)) {
		return DeviationLedger{}, ErrInvalidInput
	}

	observations, supersededBy, err := c.validateDeviationReferences()
	if err != nil {
		return DeviationLedger{}, err
	}
	counts := map[DeviationStatus]int{DeviationOpen: 0, DeviationResolved: 0}
	items := make([]DeviationLedgerItem, 0, len(c.Deviations))
	seenDeviationIDs := make(map[string]bool, len(c.Deviations))
	for _, deviation := range c.Deviations {
		observation, exists := observations[deviation.ObservationID]
		if deviation.DeviationID == "" || seenDeviationIDs[deviation.DeviationID] || deviation.CaseID != c.CaseID || !exists ||
			deviation.RuleCode == "" || deviation.Severity == "" || deviation.Description == "" ||
			(deviation.Status != DeviationOpen && deviation.Status != DeviationResolved) || !validResolution(deviation, observation.ObservedAt) {
			return DeviationLedger{}, ErrStorageCorrupt
		}
		seenDeviationIDs[deviation.DeviationID] = true
		if query.Status != "" && deviation.Status != query.Status ||
			query.RuleCode != "" && deviation.RuleCode != query.RuleCode ||
			query.Severity != "" && deviation.Severity != query.Severity ||
			query.ObservedAtFrom != nil && observation.ObservedAt.Before(*query.ObservedAtFrom) ||
			query.ObservedAtTo != nil && observation.ObservedAt.After(*query.ObservedAtTo) {
			continue
		}
		counts[deviation.Status]++
		item := DeviationLedgerItem{
			DeviationID: deviation.DeviationID, RuleCode: deviation.RuleCode, Severity: deviation.Severity,
			Description: deviation.Description, Status: deviation.Status, ObservationID: deviation.ObservationID,
			ObservedAt: observation.ObservedAt, SupersedesID: observation.SupersedesID,
			SupersededByObservationID: supersededBy[observation.ObservationID], ResolutionNote: deviation.ResolutionNote,
			ResolvedAt:   deviation.ResolvedAt,
			CanRemediate: deviation.Status == DeviationOpen && supersededBy[observation.ObservationID] == "",
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ObservedAt.Equal(items[j].ObservedAt) {
			return items[i].DeviationID < items[j].DeviationID
		}
		return items[i].ObservedAt.Before(items[j].ObservedAt)
	})
	total := len(items)
	start := (query.Page - 1) * query.PageSize
	if start > total {
		start = total
	}
	end := start + query.PageSize
	if end > total {
		end = total
	}
	totalPages := 0
	if total > 0 {
		totalPages = (total + query.PageSize - 1) / query.PageSize
	}
	return DeviationLedger{CaseID: c.CaseID, Items: items[start:end], Total: total, Page: query.Page, PageSize: query.PageSize, TotalPages: totalPages, StatusCounts: counts}, nil
}

func (c *CommissioningCase) validateDeviationReferences() (map[string]TrialObservation, map[string]string, error) {
	observations := make(map[string]TrialObservation, len(c.Observations))
	for i, observation := range c.Observations {
		if observation.ObservationID == "" || observation.CaseID != c.CaseID || observation.Sequence != i+1 || observation.ObservedAt.IsZero() {
			return nil, nil, ErrStorageCorrupt
		}
		if _, duplicate := observations[observation.ObservationID]; duplicate {
			return nil, nil, ErrStorageCorrupt
		}
		if i > 0 && !observation.ObservedAt.After(c.Observations[i-1].ObservedAt) {
			return nil, nil, ErrStorageCorrupt
		}
		observations[observation.ObservationID] = observation
	}
	supersededBy := make(map[string]string)
	for _, observation := range c.Observations {
		if observation.SupersedesID == "" {
			continue
		}
		original, exists := observations[observation.SupersedesID]
		if !exists || original.ObservationID == observation.ObservationID || !observation.ObservedAt.After(original.ObservedAt) || supersededBy[original.ObservationID] != "" {
			return nil, nil, ErrStorageCorrupt
		}
		supersededBy[original.ObservationID] = observation.ObservationID
	}
	for id := range observations {
		seen := make(map[string]bool)
		for current := id; current != ""; current = supersededBy[current] {
			if seen[current] {
				return nil, nil, ErrStorageCorrupt
			}
			seen[current] = true
		}
	}
	return observations, supersededBy, nil
}

func validResolution(deviation Deviation, observedAt time.Time) bool {
	note := strings.TrimSpace(deviation.ResolutionNote)
	if deviation.Status == DeviationOpen {
		return note == "" && deviation.ResolvedAt == nil
	}
	return note != "" && deviation.ResolvedAt != nil && !deviation.ResolvedAt.IsZero() && !deviation.ResolvedAt.Before(observedAt)
}
