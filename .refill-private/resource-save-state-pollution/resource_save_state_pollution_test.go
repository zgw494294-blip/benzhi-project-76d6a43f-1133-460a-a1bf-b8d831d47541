package resource_save_state_pollution_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/application"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/domain/commissioning"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/storage/filecase"
)

func TestResourceSaveFailureDoesNotCacheUncommittedState(t *testing.T) {
	dir := t.TempDir()
	repo, err := filecase.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	service := application.New(repo)
	created, err := service.Create("A-01", "书画", "负责人", "")
	if err != nil {
		t.Fatal(err)
	}

	temporarySnapshot := filepath.Join(dir, created.CaseID+".json.tmp")
	if err := os.Mkdir(temporarySnapshot, 0o755); err != nil {
		t.Fatal(err)
	}
	baseline := commissioning.BaselineProfile{
		TemperatureMin:          18,
		TemperatureMax:          24,
		HumidityMin:             45,
		HumidityMax:             55,
		SamplingIntervalMinutes: 180,
		MinimumObservationCount: 2,
	}
	const idempotencyKey = "lock-baseline-once"
	if _, err := service.Baseline(created.CaseID, idempotencyKey, created.ExpectedVersion, baseline); err == nil {
		t.Fatal("临时快照是目录时，保存应失败")
	}
	if err := os.Remove(temporarySnapshot); err != nil {
		t.Fatal(err)
	}

	retried, err := service.Baseline(created.CaseID, idempotencyKey, created.ExpectedVersion, baseline)
	if err != nil {
		t.Fatalf("清除资源故障后重试失败: %v", err)
	}
	persisted, err := repo.Get(created.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	if retried.State != commissioning.BaselineLocked || persisted.State != commissioning.BaselineLocked {
		t.Fatalf("失败保存污染了幂等状态: retry=%s persisted=%s", retried.State, persisted.State)
	}
	if retried.ExpectedVersion != persisted.ExpectedVersion {
		t.Fatalf("重试结果与持久化版本不一致: retry=%d persisted=%d", retried.ExpectedVersion, persisted.ExpectedVersion)
	}
}
