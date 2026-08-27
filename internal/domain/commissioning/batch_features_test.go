package commissioning

import (
	"errors"
	"testing"
	"time"
)

func TestObservationBatchIsAtomicAndLedgerTracksRemediationEligibility(t *testing.T) {
	now := time.Now().UTC()
	c, err := NewCase("batch-case", "Z1", "书画", "负责人", now)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.SetBaseline(BaselineProfile{TemperatureMin: 18, TemperatureMax: 24, HumidityMin: 40, HumidityMax: 60, SamplingIntervalMinutes: 180, MinimumObservationCount: 2}, now); err != nil {
		t.Fatal(err)
	}
	if err = c.SubmitPlan(ControlPlan{DeviceLabel: "HVAC", ControlMode: "auto", SetpointTemperature: 21, SetpointHumidity: 50, TrialDurationHours: 1, SubmittedBy: "负责人"}, now); err != nil {
		t.Fatal(err)
	}
	if err = c.StartTrial(now); err != nil {
		t.Fatal(err)
	}
	version := c.ExpectedVersion
	invalid := []TrialObservation{
		{ObservationID: "obs-1", Sequence: 1, ObservedAt: now.Add(-2 * time.Hour), Temperature: 30, Humidity: 50, DeviceStatus: DeviceNormal, RecordedBy: "现场"},
		{ObservationID: "obs-2", Sequence: 3, ObservedAt: now.Add(-time.Hour), Temperature: 21, Humidity: 50, DeviceStatus: DeviceNormal, RecordedBy: "现场"},
	}
	if err = c.AddObservations(invalid, now); !errors.Is(err, ErrObservationOrder) {
		t.Fatalf("err=%v", err)
	}
	if c.ExpectedVersion != version || len(c.Observations) != 0 || len(c.Deviations) != 0 || c.State != TrialRunning {
		t.Fatalf("失败批次改变了档案: %#v", c)
	}

	valid := append([]TrialObservation(nil), invalid...)
	valid[1].Sequence = 2
	if err = c.AddObservations(valid, now); err != nil {
		t.Fatal(err)
	}
	if c.ExpectedVersion != version+1 || len(c.Observations) != 2 || len(c.Deviations) != 1 || c.State != NeedsRemediation {
		t.Fatalf("成功批次结果异常: %#v", c)
	}
	ledger, err := c.DeviationLedger(DeviationLedgerQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if ledger.Total != 1 || !ledger.Items[0].CanRemediate || ledger.Items[0].ObservedAt != valid[0].ObservedAt {
		t.Fatalf("ledger=%#v", ledger)
	}

	target := c.Deviations[0].DeviationID
	retest := TrialObservation{ObservationID: "obs-3", Sequence: 3, ObservedAt: now, Temperature: 21, Humidity: 50, DeviceStatus: DeviceNormal, RecordedBy: "现场", SupersedesID: "obs-1"}
	if err = c.RemediateDeviations([]string{target}, "复测正常", []TrialObservation{retest}, now); err != nil {
		t.Fatal(err)
	}
	ledger, err = c.DeviationLedger(DeviationLedgerQuery{Status: DeviationResolved})
	if err != nil {
		t.Fatal(err)
	}
	if ledger.Total != 1 || ledger.Items[0].CanRemediate || ledger.Items[0].SupersededByObservationID != "obs-3" {
		t.Fatalf("整改后 ledger=%#v", ledger)
	}
}

func TestDeviationLedgerRejectsBrokenObservationReference(t *testing.T) {
	now := time.Now().UTC()
	c := &CommissioningCase{
		CaseID: "broken", Observations: []TrialObservation{{ObservationID: "obs-1", CaseID: "broken", Sequence: 1, ObservedAt: now}},
		Deviations: []Deviation{{DeviationID: "dev-1", CaseID: "broken", RuleCode: "TEMP_RANGE", Severity: "high", Description: "温度异常", Status: DeviationOpen, ObservationID: "missing"}},
	}
	if _, err := c.DeviationLedger(DeviationLedgerQuery{}); !errors.Is(err, ErrStorageCorrupt) {
		t.Fatalf("err=%v", err)
	}
}
