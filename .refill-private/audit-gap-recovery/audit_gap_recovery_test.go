package audit_gap_recovery

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/domain/commissioning"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/storage/filecase"
)

func TestRecoveryRejectsMissingAuditVersion(t *testing.T) {
	dir := t.TempDir()
	store, err := filecase.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)
	caseData, err := commissioning.NewCase("case-audit-gap", "A-1", "书画", "负责人", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(caseData, ""); err != nil {
		t.Fatal(err)
	}
	if err := caseData.SetBaseline(commissioning.BaselineProfile{
		CaseID:                  caseData.CaseID,
		TemperatureMin:          18,
		TemperatureMax:          24,
		HumidityMin:             40,
		HumidityMax:             60,
		SamplingIntervalMinutes: 60,
		MinimumObservationCount: 1,
	}, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(caseData, ""); err != nil {
		t.Fatal(err)
	}
	if err := caseData.SubmitPlan(commissioning.ControlPlan{
		CaseID:              caseData.CaseID,
		DeviceLabel:         "HVAC-1",
		ControlMode:         "auto",
		SetpointTemperature: 21,
		SetpointHumidity:    50,
		TrialDurationHours:  1,
		SubmittedBy:         "提交人",
	}, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(caseData, ""); err != nil {
		t.Fatal(err)
	}

	auditPath := filepath.Join(dir, caseData.CaseID+".audit.jsonl")
	contents, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(contents)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected three audit events, got %d", len(lines))
	}
	if err := os.WriteFile(auditPath, []byte(lines[0]+"\n"+lines[2]+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err = filecase.New(dir)
	if !errors.Is(err, filecase.ErrCorruptAudit) {
		t.Fatalf("expected missing audit version to be rejected, got %v", err)
	}
}
