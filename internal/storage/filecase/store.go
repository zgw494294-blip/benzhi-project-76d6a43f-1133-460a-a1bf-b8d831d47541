package filecase

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/application"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/domain/commissioning"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	dir  string
	mu   sync.RWMutex
	idem map[string]application.IdempotentResult
}

func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, idem: map[string]application.IdempotentResult{}}
	if b, e := os.ReadFile(filepath.Join(dir, "idempotency.json")); e == nil {
		json.Unmarshal(b, &s.idem)
	}
	entries, e := os.ReadDir(dir)
	if e != nil {
		return nil, e
	}
	for _, entry := range entries {
		n := entry.Name()
		if len(n) > len(".audit.jsonl") && filepath.Ext(n) == ".jsonl" {
			if e := s.ValidateAudit(n[:len(n)-len(".audit.jsonl")]); e != nil {
				return nil, e
			}
		}
	}
	return s, nil
}
func (s *Store) path(id string) string { return filepath.Join(s.dir, id+".json") }
func (s *Store) Save(c *commissioning.CommissioningCase, key string) error {
	fingerprint := ""
	if key != "" {
		identity, _ := json.Marshal(struct{ Zone, Category, Owner string }{c.ZoneCode, c.CollectionCategory, c.OwnerName})
		sum := sha256.Sum256(identity)
		fingerprint = fmt.Sprintf("%x", sum[:])
	}
	return s.SaveWithFingerprint(c, key, fingerprint)
}

func (s *Store) SaveWithFingerprint(c *commissioning.CommissioningCase, key, fingerprint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, e := json.MarshalIndent(struct {
		SchemaVersion int                              `json:"schemaVersion"`
		Case          *commissioning.CommissioningCase `json:"case"`
	}{1, c}, "", "  ")
	if e != nil {
		return e
	}
	casePath := s.path(c.CaseID)
	// Capture the prior snapshot (if any) so it can be restored when a later
	// step fails; otherwise a failed Save would leave the new state visible
	// to Store.Get even though the caller received an error.
	prevSnapshot, prevSnapshotErr := os.ReadFile(casePath)
	prevExists := prevSnapshotErr == nil
	tmp := casePath + ".tmp"
	if e = os.WriteFile(tmp, b, 0644); e != nil {
		return e
	}
	if e = os.Rename(tmp, casePath); e != nil {
		return e
	}
	// Remember the prior idempotency entry so it can be restored on failure.
	var prevIdem application.IdempotentResult
	hadIdem := false
	if key != "" {
		if r, ok := s.idem[key]; ok {
			prevIdem = r
			hadIdem = true
		}
		s.idem[key] = application.IdempotentResult{Status: 200, Body: b, Fingerprint: fingerprint}
	}
	if e = s.appendAudit(c); e != nil {
		// Roll back to the pre-save state so a failed Save does not publish
		// or retain a snapshot/idempotency entry that Store.Get would surface.
		if prevExists {
			_ = os.WriteFile(casePath, prevSnapshot, 0644)
		} else {
			_ = os.Remove(casePath)
		}
		if key != "" {
			if hadIdem {
				s.idem[key] = prevIdem
			} else {
				delete(s.idem, key)
			}
		}
		return e
	}
	if key != "" {
		ib, _ := json.Marshal(s.idem)
		_ = os.WriteFile(filepath.Join(s.dir, "idempotency.json"), ib, 0644)
	}
	return nil
}
func (s *Store) appendAudit(c *commissioning.CommissioningCase) error {
	f, e := os.OpenFile(filepath.Join(s.dir, c.CaseID+".audit.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if e != nil {
		return e
	}
	defer f.Close()
	ev := map[string]any{"version": c.ExpectedVersion, "state": c.State, "at": c.UpdatedAt}
	b, _ := json.Marshal(ev)
	_, e = f.Write(append(b, '\n'))
	return e
}
func (s *Store) Get(id string) (*commissioning.CommissioningCase, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, e := os.ReadFile(s.path(id))
	if os.IsNotExist(e) {
		return nil, commissioning.ErrNotFound
	}
	if e != nil {
		return nil, e
	}
	var w struct {
		SchemaVersion int                              `json:"schemaVersion"`
		Case          *commissioning.CommissioningCase `json:"case"`
	}
	if e = json.Unmarshal(b, &w); e != nil {
		return nil, fmt.Errorf("%w: 档案快照损坏", commissioning.ErrStorageCorrupt)
	}
	if w.SchemaVersion != 1 || w.Case == nil {
		return nil, fmt.Errorf("%w: 档案快照版本不受支持", commissioning.ErrStorageCorrupt)
	}
	if w.Case.CaseID != id {
		return nil, fmt.Errorf("%w: 档案编号与文件不一致", commissioning.ErrStorageCorrupt)
	}
	return w.Case, nil
}
func (s *Store) GetIdempotency(key string) (*application.IdempotentResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.idem[key]
	if !ok {
		return nil, application.ErrMissingIdempotency
	}
	return &r, nil
}
func (s *Store) SaveIdempotency(key string, r application.IdempotentResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.idem[key] = r
	b, e := json.Marshal(s.idem)
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(s.dir, "idempotency.json"), b, 0644)
}
func (s *Store) FindPermit(code string) (*commissioning.ActivationPermit, error) {
	cases, e := s.Cases()
	if e != nil {
		return nil, e
	}
	var found *commissioning.ActivationPermit
	for _, c := range cases {
		if c.Permit != nil && c.Permit.PermitCode == code {
			if c.Permit.CaseID != c.CaseID || found != nil {
				return nil, commissioning.ErrStorageCorrupt
			}
			permit := *c.Permit
			found = &permit
		}
	}
	if found != nil {
		return found, nil
	}
	return nil, commissioning.ErrPermitNotFound
}
