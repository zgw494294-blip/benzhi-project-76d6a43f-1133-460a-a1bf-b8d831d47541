package idempotencydurabilityloss

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/application"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/domain/commissioning"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/storage/filecase"
)

func TestIdempotencyPersistenceFailureIsReported(t *testing.T) {
	dir := t.TempDir()
	store, err := filecase.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	app := application.New(store)
	caseValue, err := app.Create("A-01", "书画", "负责人", "")
	if err != nil {
		t.Fatal(err)
	}

	// 档案创建后令幂等结果持久化目标不可写。
	if err := os.Mkdir(filepath.Join(dir, "idempotency.json"), 0755); err != nil {
		t.Fatal(err)
	}
	_, err = app.Baseline(caseValue.CaseID, "baseline-once", caseValue.ExpectedVersion, commissioning.BaselineProfile{
		TemperatureMin:          18,
		TemperatureMax:          24,
		HumidityMin:             45,
		HumidityMax:             55,
		SamplingIntervalMinutes: 60,
		MinimumObservationCount: 1,
	})
	if err == nil {
		t.Fatal("expected idempotency persistence failure to be returned")
	}
	if errors.Is(err, commissioning.ErrInvalidTransition) {
		t.Fatalf("unexpected domain error: %v", err)
	}
}
