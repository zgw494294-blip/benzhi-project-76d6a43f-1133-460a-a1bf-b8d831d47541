package filecase

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/application"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/domain/commissioning"
	"os"
	"path/filepath"
	"time"
)

func (s *Store) ExportCase(id, target string) error {
	_, err := s.ExportSnapshot(id, target)
	return err
}

func (s *Store) ExportSnapshot(id, target string) (application.ExportResult, error) {
	c, err := s.Get(id)
	if err != nil {
		return application.ExportResult{}, err
	}
	if c.CaseID != id || c.ExpectedVersion <= 0 {
		return application.ExportResult{}, commissioning.ErrStorageCorrupt
	}
	// Marshal on the declared case struct gives stable field order and includes all history.
	b, err := json.Marshal(c)
	if err != nil {
		return application.ExportResult{}, fmt.Errorf("%w: %v", commissioning.ErrExportInvalid, err)
	}
	base, err := filepath.Abs(s.dir)
	if err != nil {
		return application.ExportResult{}, fmt.Errorf("%w: %v", commissioning.ErrExportInvalid, err)
	}
	if target == "" {
		target = filepath.Join(s.dir, id+"-export-"+time.Now().UTC().Format("20060102T150405.000000000Z")+".json")
	} else if !filepath.IsAbs(target) {
		target = filepath.Join(s.dir, target)
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return application.ExportResult{}, fmt.Errorf("%w: %v", commissioning.ErrExportInvalid, err)
	}
	rel, err := filepath.Rel(base, absTarget)
	if err != nil || rel == ".." || len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator) || filepath.IsAbs(rel) {
		return application.ExportResult{}, commissioning.ErrExportInvalid
	}
	casePath, _ := filepath.Abs(s.path(id))
	idempotencyPath, _ := filepath.Abs(filepath.Join(s.dir, "idempotency.json"))
	auditPath, _ := filepath.Abs(filepath.Join(s.dir, id+".audit.jsonl"))
	if absTarget == casePath || absTarget == idempotencyPath || absTarget == auditPath {
		return application.ExportResult{}, commissioning.ErrExportInvalid
	}
	if err := os.MkdirAll(filepath.Dir(absTarget), 0755); err != nil {
		return application.ExportResult{}, fmt.Errorf("%w: %v", commissioning.ErrExportInvalid, err)
	}
	tmp := absTarget + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return application.ExportResult{}, fmt.Errorf("%w: %v", commissioning.ErrExportInvalid, err)
	}
	if err := os.Rename(tmp, absTarget); err != nil {
		_ = os.Remove(tmp)
		return application.ExportResult{}, fmt.Errorf("%w: %v", commissioning.ErrExportInvalid, err)
	}
	sum := sha256.Sum256(b)
	return application.ExportResult{ByteCount: len(b), RelativeFileName: filepath.ToSlash(rel), SHA256: fmt.Sprintf("%x", sum[:])}, nil
}

func (s *Store) Cases() ([]*commissioning.CommissioningCase, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	result := make([]*commissioning.CommissioningCase, 0)
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" || entry.Name() == "idempotency.json" {
			continue
		}
		id := entry.Name()[:len(entry.Name())-5]
		b, err := os.ReadFile(s.path(id))
		if err != nil {
			return nil, err
		}
		var w struct {
			SchemaVersion int                              `json:"schemaVersion"`
			Case          *commissioning.CommissioningCase `json:"case"`
		}
		if err := json.Unmarshal(b, &w); err != nil {
			return nil, fmt.Errorf("%w: %s", commissioning.ErrStorageCorrupt, entry.Name())
		}
		if w.SchemaVersion != 1 || w.Case == nil {
			return nil, fmt.Errorf("%w: %s", commissioning.ErrStorageCorrupt, entry.Name())
		}
		if w.Case.CaseID != id {
			return nil, fmt.Errorf("%w: %s", commissioning.ErrStorageCorrupt, entry.Name())
		}
		result = append(result, w.Case)
	}
	return result, nil
}
