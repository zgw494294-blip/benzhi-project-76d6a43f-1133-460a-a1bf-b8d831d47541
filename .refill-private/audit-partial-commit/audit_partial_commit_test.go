package audit_partial_commit_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/domain/commissioning"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/storage/filecase"
)

func TestFailedAuditAppendDoesNotCommitSnapshot(t *testing.T) {
	dir := t.TempDir()
	repo, err := filecase.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	c, err := commissioning.NewCase("case-audit-failure", "A-1", "书画", "负责人", time.Unix(1, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, c.CaseID+".audit.jsonl"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(c, ""); err == nil {
		t.Fatal("expected audit append failure")
	}
	_, err = repo.Get(c.CaseID)
	if !errors.Is(err, commissioning.ErrNotFound) {
		t.Fatalf("snapshot was committed despite failed Save: %v", err)
	}
}
