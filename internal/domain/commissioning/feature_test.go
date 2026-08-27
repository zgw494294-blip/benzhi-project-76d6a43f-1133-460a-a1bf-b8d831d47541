package commissioning

import (
	"math"
	"testing"
	"time"
)

func TestDraftIdentityRevisionAndBaselineHistory(t *testing.T) {
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	c, err := NewCase("identity", " a-1 ", " 书画 ", " 甲 ", now)
	if err != nil {
		t.Fatal(err)
	}
	if c.ZoneCode != "A-1" || c.CollectionCategory != "书画" || c.OwnerName != "甲" {
		t.Fatalf("建档信息未规范化: %#v", c)
	}
	if err := c.ReviseIdentity(" b-2 ", " 陶瓷 ", " 乙 ", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	baseline := BaselineProfile{TemperatureMin: 18, TemperatureMax: 24, HumidityMin: 40, HumidityMax: 60, SamplingIntervalMinutes: 10, MinimumObservationCount: 2}
	if err := c.SetBaseline(baseline, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := c.ReviseIdentity("C", "金属", "丙", now.Add(3*time.Minute)); err != ErrInvalidTransition {
		t.Fatalf("锁定后修订返回 %v", err)
	}
	if err := c.RevokeBaseline(" 参数来源更新 ", " 管理员 ", now.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(c.BaselineHistory) != 1 || c.Baseline != nil || c.BaselineRevocations[0].OriginalBaselineRevision != 1 {
		t.Fatalf("撤销结果错误: %#v", c)
	}
	if err := c.SetBaseline(baseline, now.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if c.Baseline.Revision != 2 || c.Baseline.BaselineID == c.BaselineHistory[0].BaselineID {
		t.Fatalf("重新锁定未递增修订: %#v", c.Baseline)
	}
}

func TestPlanRevisionPreservesHistoryAndLatestSubmitter(t *testing.T) {
	now := time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)
	c := readyCase(t, "plans", now, 180, 2)
	firstID := c.Plan.PlanID
	revised := *c.Plan
	revised.SubmittedBy = "新提交人"
	revised.SetpointTemperature = 22
	if err := c.RevisePlan(revised, "优化设定值", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(c.PlanHistory) != 2 || c.PlanHistory[0].Plan.PlanID != firstID || c.PlanHistory[1].Revision != 2 || c.Plan.PlanID == firstID {
		t.Fatalf("方案历史错误: %#v", c.PlanHistory)
	}
	if err := c.StartTrial(now.Add(2 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := c.RevisePlan(revised, "再次修订", now.Add(3*time.Minute)); err != ErrInvalidTransition {
		t.Fatalf("试运行后修订返回 %v", err)
	}
	if err := c.AddObservation(TrialObservation{Sequence: 1, ObservedAt: now.Add(-2 * time.Hour), Temperature: 21, Humidity: 50, DeviceStatus: DeviceNormal, RecordedBy: "现场"}, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := c.AddObservation(TrialObservation{Sequence: 2, ObservedAt: now, Temperature: 21, Humidity: 50, DeviceStatus: DeviceNormal, RecordedBy: "现场"}, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	pkg, err := c.BuildReviewPackage()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Review(ReviewDecision{ReviewerName: "新提交人", Decision: DecisionApprove, ReviewedVersion: c.ExpectedVersion, PackageFingerprint: pkg.PackageFingerprint}, now.Add(4*time.Minute)); err != ErrIndependentReviewer {
		t.Fatalf("利益冲突返回 %v", err)
	}
}

func TestObservationValidationIsAtomicAndCreatesGapDeviation(t *testing.T) {
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	c := runningCase(t, "observations", now, 10, 2)
	version := c.ExpectedVersion
	err := c.AddObservation(TrialObservation{Sequence: 1, ObservedAt: now.Add(6 * time.Minute), Temperature: math.NaN(), Humidity: 50, DeviceStatus: "unknown", RecordedBy: ""}, now)
	if err != ErrInvalidObservation || len(c.Observations) != 0 || c.ExpectedVersion != version {
		t.Fatalf("非法观测发生部分写入: err=%v case=%#v", err, c)
	}
	if err := c.AddObservation(TrialObservation{ObservationID: "first", Sequence: 1, ObservedAt: now.Add(-20 * time.Minute), Temperature: 21, Humidity: 50, DeviceStatus: DeviceNormal, RecordedBy: "现场"}, now); err != nil {
		t.Fatal(err)
	}
	if err := c.AddObservation(TrialObservation{ObservationID: "second", Sequence: 2, ObservedAt: now.Add(-5 * time.Minute), Temperature: 21, Humidity: 50, DeviceStatus: DeviceNormal, RecordedBy: "现场"}, now); err != nil {
		t.Fatal(err)
	}
	if c.OpenDeviations() != 1 || c.Deviations[0].RuleCode != "SAMPLE_GAP" || c.Deviations[0].ObservationID != "second" {
		t.Fatalf("漏采偏差错误: %#v", c.Deviations)
	}
}

func TestTargetedRemediationAppendsRetestAndClosesOnlyPassingTargets(t *testing.T) {
	now := time.Date(2026, 8, 27, 11, 0, 0, 0, time.UTC)
	c := runningCase(t, "remediation", now, 10, 1)
	if err := c.AddObservation(TrialObservation{ObservationID: "bad", Sequence: 1, ObservedAt: now, Temperature: 30, Humidity: 70, DeviceStatus: DeviceNormal, RecordedBy: "现场"}, now); err != nil {
		t.Fatal(err)
	}
	if len(c.Deviations) != 2 {
		t.Fatalf("偏差数=%d", len(c.Deviations))
	}
	target := c.Deviations[0].DeviationID
	if err := c.RemediateDeviations([]string{target, target}, "重复", nil, now.Add(time.Minute)); err != ErrInvalidInput && err != ErrRemediationTarget {
		t.Fatalf("重复目标返回 %v", err)
	}
	retest := TrialObservation{ObservationID: "retest", Sequence: 2, ObservedAt: now.Add(time.Minute), Temperature: 21, Humidity: 70, DeviceStatus: DeviceNormal, RecordedBy: "现场", SupersedesID: "bad"}
	if err := c.RemediateDeviations([]string{target}, "温度复测通过", []TrialObservation{retest}, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(c.Observations) != 2 || c.Observations[0].Temperature != 30 || c.Deviations[0].Status != DeviationResolved || c.Deviations[1].Status != DeviationOpen || c.State != NeedsRemediation {
		t.Fatalf("目标整改结果错误: %#v", c)
	}
}

func TestReviewRequiresCurrentPackageFingerprint(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	c := runningCase(t, "review", now, 180, 2)
	if err := c.AddObservation(TrialObservation{Sequence: 1, ObservedAt: now.Add(-2 * time.Hour), Temperature: 21, Humidity: 50, DeviceStatus: DeviceNormal, RecordedBy: "现场"}, now); err != nil {
		t.Fatal(err)
	}
	if err := c.AddObservation(TrialObservation{Sequence: 2, ObservedAt: now, Temperature: 21, Humidity: 50, DeviceStatus: DeviceNormal, RecordedBy: "现场"}, now); err != nil {
		t.Fatal(err)
	}
	pkg, err := c.BuildReviewPackage()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Review(ReviewDecision{ReviewerName: "复核员", Decision: DecisionApprove, ReviewedVersion: c.ExpectedVersion, PackageFingerprint: "stale"}, now); err != ErrPackageStale {
		t.Fatalf("旧指纹返回 %v", err)
	}
	if err := c.Review(ReviewDecision{ReviewerName: "复核员", Decision: DecisionApprove, ReviewedVersion: c.ExpectedVersion, PackageFingerprint: pkg.PackageFingerprint}, now); err != nil {
		t.Fatal(err)
	}
	if c.State != Approved {
		t.Fatalf("state=%s", c.State)
	}
}

func readyCase(t *testing.T, id string, now time.Time, interval, minimum int) *CommissioningCase {
	t.Helper()
	c, err := NewCase(id, "Z1", "书画", "负责人", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.SetBaseline(BaselineProfile{TemperatureMin: 18, TemperatureMax: 24, HumidityMin: 40, HumidityMax: 60, SamplingIntervalMinutes: interval, MinimumObservationCount: minimum}, now); err != nil {
		t.Fatal(err)
	}
	if err := c.SubmitPlan(ControlPlan{DeviceLabel: "HVAC", ControlMode: "auto", SetpointTemperature: 21, SetpointHumidity: 50, TrialDurationHours: 1, SubmittedBy: "提交人"}, now); err != nil {
		t.Fatal(err)
	}
	return c
}

func runningCase(t *testing.T, id string, now time.Time, interval, minimum int) *CommissioningCase {
	t.Helper()
	c := readyCase(t, id, now, interval, minimum)
	if err := c.StartTrial(now); err != nil {
		t.Fatal(err)
	}
	return c
}
