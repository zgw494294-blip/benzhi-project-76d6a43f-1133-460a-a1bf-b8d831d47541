package review_package_cache_stale_test

import (
	"testing"
	"time"

	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/application"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/domain/commissioning"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/storage/filecase"
)

func TestReviewPackageCacheTracksCaseVersion(t *testing.T) {
	repo, err := filecase.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := application.New(repo)
	c, err := service.Create("A-01", "书画", "负责人", "")
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.Baseline(c.CaseID, "", c.ExpectedVersion, commissioning.BaselineProfile{
		TemperatureMin: 18, TemperatureMax: 24,
		HumidityMin: 45, HumidityMax: 55,
		SamplingIntervalMinutes: 180, MinimumObservationCount: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.Plan(c.CaseID, "", c.ExpectedVersion, commissioning.ControlPlan{
		DeviceLabel: "HVAC-1", ControlMode: "auto",
		SetpointTemperature: 21, SetpointHumidity: 50,
		TrialDurationHours: 1, SubmittedBy: "负责人",
	})
	if err != nil {
		t.Fatal(err)
	}
	c, err = service.Start(c.CaseID, "", c.ExpectedVersion)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	c, err = service.Observe(c.CaseID, "", c.ExpectedVersion, commissioning.TrialObservation{
		Sequence: 1, ObservedAt: now.Add(-2 * time.Hour), Temperature: 21, Humidity: 50,
		DeviceStatus: commissioning.DeviceNormal, RecordedBy: "现场人员",
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.ReviewPackage(c.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	if first.ExpectedVersion != c.ExpectedVersion {
		t.Fatalf("首次复核资料包版本=%d，档案版本=%d", first.ExpectedVersion, c.ExpectedVersion)
	}

	c, err = service.Observe(c.CaseID, "", c.ExpectedVersion, commissioning.TrialObservation{
		Sequence: 2, ObservedAt: now, Temperature: 21, Humidity: 50,
		DeviceStatus: commissioning.DeviceNormal, RecordedBy: "现场人员",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.State != commissioning.AwaitingReview {
		t.Fatalf("第二次观测后的状态=%s", c.State)
	}
	second, err := service.ReviewPackage(c.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	if second.ExpectedVersion != c.ExpectedVersion {
		t.Fatalf("复核资料包缓存仍为版本=%d，最新档案版本=%d", second.ExpectedVersion, c.ExpectedVersion)
	}
	if second.PackageFingerprint == first.PackageFingerprint {
		t.Fatal("观测变化后仍返回旧的复核资料包指纹")
	}
}
