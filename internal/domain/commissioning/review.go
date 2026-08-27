package commissioning

import (
	"fmt"
	"strings"
	"time"
)

type ReviewHistoryQuery struct {
	Decision     Decision
	ReviewerName string
	From         *time.Time
	To           *time.Time
}

type ReviewHistory struct {
	CaseID         string              `json:"caseId"`
	CurrentState   State               `json:"currentState"`
	CurrentVersion int64               `json:"currentExpectedVersion"`
	Items          []ReviewHistoryItem `json:"items"`
}

type ReviewHistoryItem struct {
	ReviewDecision
	IsCurrentVersion bool `json:"isCurrentVersion"`
}

func (c *CommissioningCase) ReviewHistory(query ReviewHistoryQuery) (ReviewHistory, error) {
	if query.From != nil && query.To != nil && query.From.After(*query.To) {
		return ReviewHistory{}, ErrInvalidInput
	}
	if query.Decision != "" && query.Decision != DecisionApprove && query.Decision != DecisionReturn {
		return ReviewHistory{}, ErrInvalidInput
	}
	query.ReviewerName = strings.TrimSpace(query.ReviewerName)
	items := make([]ReviewHistoryItem, 0, len(c.Reviews))
	var previous int64
	for _, review := range c.Reviews {
		if review.CaseID != c.CaseID {
			return ReviewHistory{}, ErrReviewHistory
		}
		if review.Decision != DecisionApprove && review.Decision != DecisionReturn {
			return ReviewHistory{}, ErrReviewHistory
		}
		if review.ReviewedVersion <= 0 || review.ReviewedVersion < previous || review.ReviewedVersion > c.ExpectedVersion {
			return ReviewHistory{}, ErrReviewHistory
		}
		if review.ReviewID == "" || review.ReviewedAt.IsZero() || review.ReviewerName == "" || review.PackageFingerprint == "" {
			return ReviewHistory{}, ErrReviewHistory
		}
		previous = review.ReviewedVersion
		if query.Decision != "" && review.Decision != query.Decision {
			continue
		}
		if query.ReviewerName != "" && review.ReviewerName != query.ReviewerName {
			continue
		}
		if query.From != nil && review.ReviewedAt.Before(*query.From) {
			continue
		}
		if query.To != nil && review.ReviewedAt.After(*query.To) {
			continue
		}
		items = append(items, ReviewHistoryItem{ReviewDecision: review, IsCurrentVersion: review.ReviewedVersion == c.ExpectedVersion})
	}
	if (c.State == Approved || c.State == Activated) && len(c.Reviews) == 0 {
		return ReviewHistory{}, ErrReviewHistory
	}
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && (items[j].ReviewedVersion < items[j-1].ReviewedVersion || (items[j].ReviewedVersion == items[j-1].ReviewedVersion && items[j].ReviewedAt.Before(items[j-1].ReviewedAt))); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
	return ReviewHistory{CaseID: c.CaseID, CurrentState: c.State, CurrentVersion: c.ExpectedVersion, Items: items}, nil
}

func (c *CommissioningCase) Review(r ReviewDecision, now time.Time) error {
	if c.State != AwaitingReview {
		return ErrInvalidTransition
	}
	r.ReviewerName = strings.TrimSpace(r.ReviewerName)
	r.Comment = strings.TrimSpace(r.Comment)
	if r.ReviewerName == "" || (r.Decision != DecisionApprove && r.Decision != DecisionReturn) {
		return ErrInvalidInput
	}
	if r.Decision == DecisionReturn && r.Comment == "" {
		return ErrInvalidInput
	}
	latestSubmitter := ""
	if len(c.PlanHistory) > 0 {
		latestSubmitter = c.PlanHistory[len(c.PlanHistory)-1].SubmittedBy
	} else if c.Plan != nil {
		latestSubmitter = c.Plan.SubmittedBy
	}
	if r.ReviewerName == latestSubmitter {
		return ErrIndependentReviewer
	}
	if r.ReviewedVersion != c.ExpectedVersion {
		return ErrPackageStale
	}
	pkg, err := c.BuildReviewPackage()
	if err != nil {
		return err
	}
	if r.PackageFingerprint == "" || r.PackageFingerprint != pkg.PackageFingerprint {
		return ErrPackageStale
	}
	if r.Decision == DecisionApprove && c.OpenDeviations() > 0 {
		return ErrOpenDeviation
	}
	r.CaseID = c.CaseID
	r.ReviewedAt = now
	r.ReviewID = fmt.Sprintf("review-%s-%d", c.CaseID, len(c.Reviews)+1)
	c.Reviews = append(c.Reviews, r)
	if r.Decision == DecisionApprove {
		c.State = Approved
	} else {
		c.State = NeedsRemediation
	}
	c.touch(now)
	return nil
}
