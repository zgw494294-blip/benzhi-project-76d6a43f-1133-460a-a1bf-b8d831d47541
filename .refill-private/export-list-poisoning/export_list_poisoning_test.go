package export_list_poisoning_test

import (
	"testing"
	"time"

	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/application"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/domain/commissioning"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/storage/filecase"
)

func TestExportDoesNotCorruptCaseListing(t *testing.T) {
	dir := t.TempDir()
	repo, err := filecase.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	c, err := commissioning.NewCase("case-export-list", "A-1", "书画", "负责人", time.Unix(1, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(c, ""); err != nil {
		t.Fatal(err)
	}
	app := application.New(repo)
	if _, err := app.Export(c.CaseID, ""); err != nil {
		t.Fatalf("export failed: %v", err)
	}
	listed, err := app.List(application.CaseFilter{})
	if err != nil {
		t.Fatalf("listing failed after successful export: %v", err)
	}
	if listed.Total != 1 || len(listed.Items) != 1 || listed.Items[0].CaseID != c.CaseID {
		t.Fatalf("unexpected listing after export: %+v", listed)
	}
}
