package commissioning

import (
	"testing"
	"time"
)

func TestLifecycleReachesAwaitingReview(t *testing.T) {
	now := time.Now()
	c, e := NewCase("c1", "Z1", "书画", "owner", now)
	if e != nil {
		t.Fatal(e)
	}
	if e = c.SetBaseline(BaselineProfile{TemperatureMin: 18, TemperatureMax: 24, HumidityMin: 40, HumidityMax: 60, SamplingIntervalMinutes: 180, MinimumObservationCount: 2}, now); e != nil {
		t.Fatal(e)
	}
	if e = c.SubmitPlan(ControlPlan{DeviceLabel: "hvac", ControlMode: "auto", SetpointTemperature: 21, SetpointHumidity: 50, TrialDurationHours: 1, SubmittedBy: "owner"}, now); e != nil {
		t.Fatal(e)
	}
	if e = c.StartTrial(now); e != nil {
		t.Fatal(e)
	}
	if e = c.AddObservation(TrialObservation{Sequence: 1, ObservedAt: now.Add(-2 * time.Hour), Temperature: 21, Humidity: 50, DeviceStatus: DeviceNormal, RecordedBy: "现场"}, now); e != nil {
		t.Fatal(e)
	}
	if e = c.AddObservation(TrialObservation{Sequence: 2, ObservedAt: now, Temperature: 21, Humidity: 50, DeviceStatus: DeviceNormal, RecordedBy: "现场"}, now); e != nil {
		t.Fatal(e)
	}
	if c.State != AwaitingReview {
		t.Fatalf("state=%s", c.State)
	}
}
func TestOutOfRangeCreatesDeviation(t *testing.T) {
	now := time.Now()
	c, _ := NewCase("c2", "Z1", "陶瓷", "owner", now)
	_ = c.SetBaseline(BaselineProfile{TemperatureMin: 18, TemperatureMax: 24, HumidityMin: 40, HumidityMax: 60, SamplingIntervalMinutes: 10, MinimumObservationCount: 1}, now)
	_ = c.SubmitPlan(ControlPlan{DeviceLabel: "hvac", ControlMode: "auto", SetpointTemperature: 21, SetpointHumidity: 50, TrialDurationHours: 1, SubmittedBy: "owner"}, now)
	_ = c.StartTrial(now)
	_ = c.AddObservation(TrialObservation{Sequence: 1, ObservedAt: now, Temperature: 30, Humidity: 50, DeviceStatus: DeviceNormal, RecordedBy: "现场"}, now)
	if c.OpenDeviations() != 1 || c.State != NeedsRemediation {
		t.Fatalf("unexpected state=%s deviations=%d", c.State, c.OpenDeviations())
	}
}
