package mutation_idempotency_payload_test

import (
	"errors"
	"testing"
	"time"

	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/application"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/domain/commissioning"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/storage/filecase"
)

func TestMutationIdempotencyRejectsChangedPayload(t *testing.T) {
	repo, err := filecase.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, err := commissioning.NewCase("case-idem-payload", "A-1", "书画", "负责人", time.Unix(1, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(c, ""); err != nil {
		t.Fatal(err)
	}
	app := application.New(repo)
	first := application.CaseInput{ZoneCode: "B-1", CollectionCategory: "陶瓷", OwnerName: "甲"}
	if _, err := app.ReviseIdentity(c.CaseID, "same-key", 1, first); err != nil {
		t.Fatalf("first mutation failed: %v", err)
	}
	changed := application.CaseInput{ZoneCode: "C-1", CollectionCategory: "金属", OwnerName: "乙"}
	_, err = app.ReviseIdentity(c.CaseID, "same-key", 1, changed)
	if !errors.Is(err, commissioning.ErrIdempotencyConflict) {
		t.Fatalf("changed payload was replayed as success, got %v", err)
	}
}
