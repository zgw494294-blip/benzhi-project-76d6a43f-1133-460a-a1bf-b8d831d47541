package application

import (
	"sort"
	"strings"
	"time"

	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/domain/commissioning"
)

const maxPageSize = 100

type CaseListItem struct {
	CaseID             string              `json:"caseId"`
	ZoneCode           string              `json:"zoneCode"`
	CollectionCategory string              `json:"collectionCategory"`
	OwnerName          string              `json:"ownerName"`
	State              commissioning.State `json:"state"`
	ExpectedVersion    int64               `json:"expectedVersion"`
	UpdatedAt          time.Time           `json:"updatedAt"`
	OpenDeviationCount *int                `json:"openDeviationCount,omitempty"`
}

type CaseList struct {
	Items       []CaseListItem              `json:"items"`
	Total       int                         `json:"total"`
	Page        int                         `json:"page"`
	PageSize    int                         `json:"pageSize"`
	TotalPages  int                         `json:"totalPages"`
	StateCounts map[commissioning.State]int `json:"stateCounts"`
}

func (s *Service) List(filter CaseFilter) (CaseList, error) {
	if filter.Page == 0 {
		filter.Page = 1
	}
	if filter.PageSize == 0 {
		filter.PageSize = 20
	}
	if filter.Page < 1 || filter.PageSize < 1 || filter.PageSize > maxPageSize || (filter.State != "" && !validState(filter.State)) {
		return CaseList{}, commissioning.ErrInvalidInput
	}
	if filter.UpdatedFrom != nil && filter.UpdatedTo != nil && filter.UpdatedFrom.After(*filter.UpdatedTo) {
		return CaseList{}, commissioning.ErrInvalidInput
	}
	cases, err := s.repo.Cases()
	if err != nil {
		return CaseList{}, err
	}
	zone := commissioning.NormalizeZoneCode(filter.ZoneCode)
	owner := strings.TrimSpace(filter.OwnerName)
	matched := make([]*commissioning.CommissioningCase, 0, len(cases))
	counts := make(map[commissioning.State]int)
	for _, c := range cases {
		if filter.State != "" && c.State != filter.State {
			continue
		}
		if zone != "" && c.ZoneCode != zone {
			continue
		}
		if owner != "" && c.OwnerName != owner {
			continue
		}
		if filter.UpdatedFrom != nil && c.UpdatedAt.Before(*filter.UpdatedFrom) {
			continue
		}
		if filter.UpdatedTo != nil && c.UpdatedAt.After(*filter.UpdatedTo) {
			continue
		}
		matched = append(matched, c)
		counts[c.State]++
	}
	sort.Slice(matched, func(i, j int) bool {
		if matched[i].UpdatedAt.Equal(matched[j].UpdatedAt) {
			return matched[i].CaseID < matched[j].CaseID
		}
		return matched[i].UpdatedAt.After(matched[j].UpdatedAt)
	})
	total := len(matched)
	start := (filter.Page - 1) * filter.PageSize
	if start > total {
		start = total
	}
	end := start + filter.PageSize
	if end > total {
		end = total
	}
	items := make([]CaseListItem, 0, end-start)
	for _, c := range matched[start:end] {
		item := CaseListItem{CaseID: c.CaseID, ZoneCode: c.ZoneCode, CollectionCategory: c.CollectionCategory, OwnerName: c.OwnerName, State: c.State, ExpectedVersion: c.ExpectedVersion, UpdatedAt: c.UpdatedAt}
		if c.State == commissioning.NeedsRemediation {
			count := c.OpenDeviations()
			item.OpenDeviationCount = &count
		}
		items = append(items, item)
	}
	totalPages := 0
	if total > 0 {
		totalPages = (total + filter.PageSize - 1) / filter.PageSize
	}
	return CaseList{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize, TotalPages: totalPages, StateCounts: counts}, nil
}

func Snapshot(c *commissioning.CommissioningCase) commissioning.CommissioningCase {
	out := *c
	out.BaselineHistory = append([]commissioning.BaselineProfile(nil), c.BaselineHistory...)
	out.BaselineRevocations = append([]commissioning.BaselineRevocation(nil), c.BaselineRevocations...)
	out.PlanHistory = append([]commissioning.PlanRevision(nil), c.PlanHistory...)
	out.Observations = append([]commissioning.TrialObservation(nil), c.Observations...)
	out.Deviations = append([]commissioning.Deviation(nil), c.Deviations...)
	out.Reviews = append([]commissioning.ReviewDecision(nil), c.Reviews...)
	return out
}

func validState(state commissioning.State) bool {
	switch state {
	case commissioning.Draft, commissioning.BaselineLocked, commissioning.TrialReady, commissioning.TrialRunning, commissioning.NeedsRemediation, commissioning.AwaitingReview, commissioning.Approved, commissioning.Activated:
		return true
	default:
		return false
	}
}
