package snapshot_nested_reference_drift

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/application"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/domain/commissioning"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/storage/filecase"
)

func TestRestartRejectsNestedCaseReferenceDrift(t *testing.T) {
	dir := t.TempDir()
	store, err := filecase.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 27, 15, 0, 0, 0, time.UTC)
	c, err := commissioning.NewCase("case-drift", "Z-1", "书画", "负责人", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.SetBaseline(commissioning.BaselineProfile{
		TemperatureMin: 18, TemperatureMax: 24, HumidityMin: 40, HumidityMax: 60,
		SamplingIntervalMinutes: 60, MinimumObservationCount: 1,
	}, now); err != nil {
		t.Fatal(err)
	}
	if err := c.SubmitPlan(commissioning.ControlPlan{
		DeviceLabel: "HVAC", ControlMode: "auto", SetpointTemperature: 21,
		SetpointHumidity: 50, TrialDurationHours: 1, SubmittedBy: "负责人",
	}, now); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(c, ""); err != nil {
		t.Fatal(err)
	}

	snapshotPath := filepath.Join(dir, "case-drift.json")
	b, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	var wrapper struct {
		SchemaVersion int                              `json:"schemaVersion"`
		Case          *commissioning.CommissioningCase `json:"case"`
	}
	if err := json.Unmarshal(b, &wrapper); err != nil {
		t.Fatal(err)
	}
	if wrapper.Case == nil || wrapper.Case.Plan == nil {
		t.Fatal("快照未包含方案")
	}
	// 外层包装和档案编号仍然有效，但嵌套方案已归属于另一档案，恢复或读取时应拒绝。
	wrapper.Case.Plan.CaseID = "case-other"
	b, err = json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(snapshotPath, b, 0644); err != nil {
		t.Fatal(err)
	}

	restarted, err := filecase.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = application.New(restarted).ReviewPackage("case-drift")
	if !errors.Is(err, commissioning.ErrStorageCorrupt) {
		t.Fatalf("嵌套方案归属漂移应报告 ErrStorageCorrupt，得到 %v", err)
	}
}
