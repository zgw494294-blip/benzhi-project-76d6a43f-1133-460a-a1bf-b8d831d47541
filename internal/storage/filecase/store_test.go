package filecase

import (
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/domain/commissioning"
	"os"
	"testing"
	"time"
)

func TestAtomicSnapshotRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, e := New(dir)
	if e != nil {
		t.Fatal(e)
	}
	c, _ := commissioning.NewCase("x", "z", "cat", "o", time.Now())
	if e = s.Save(c, "k"); e != nil {
		t.Fatal(e)
	}
	got, e := s.Get("x")
	if e != nil || got.CaseID != "x" {
		t.Fatalf("get=%v %v", got, e)
	}
	if _, e = os.Stat(dir + "/x.audit.jsonl"); e != nil {
		t.Fatal(e)
	}
}
