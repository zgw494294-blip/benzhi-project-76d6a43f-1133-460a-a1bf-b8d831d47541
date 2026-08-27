package commissioning

import (
	"fmt"
	"time"
)

func (c *CommissioningCase) Activate(now time.Time) (*ActivationPermit, error) {
	if c.State != Approved {
		return nil, ErrInvalidTransition
	}
	if len(c.Reviews) == 0 {
		return nil, ErrInvalidTransition
	}
	p := &ActivationPermit{PermitID: fmt.Sprintf("permit-%s", c.CaseID), CaseID: c.CaseID, PermitCode: fmt.Sprintf("CH-%s-%d", c.CaseID, now.Unix()), ActivatedVersion: c.ExpectedVersion + 1, IssuedAt: now, EffectiveUntil: now.AddDate(1, 0, 0), Status: "active"}
	c.Permit = p
	c.State = Activated
	c.touch(now)
	return p, nil
}
func (c *CommissioningCase) OpenDeviations() int {
	n := 0
	for _, d := range c.Deviations {
		if d.Status == DeviationOpen {
			n++
		}
	}
	return n
}
