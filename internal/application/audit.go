package application

import (
	"encoding/json"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/domain/commissioning"
	"time"
)

type AuditEvent struct {
	CaseID  string              `json:"caseId"`
	Version int64               `json:"version"`
	State   commissioning.State `json:"state"`
	At      time.Time           `json:"at"`
}

func NewAuditEvent(c *commissioning.CommissioningCase) AuditEvent {
	return AuditEvent{CaseID: c.CaseID, Version: c.ExpectedVersion, State: c.State, At: c.UpdatedAt}
}
func EncodeAuditEvent(e AuditEvent) []byte { b, _ := json.Marshal(e); return b }
