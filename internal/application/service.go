package application

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/domain/commissioning"
	"sync"
	"time"
)

type Service struct {
	repo           Repository
	locks          sync.Map
	reviewPackages sync.Map
}

func New(repo Repository) *Service { return &Service{repo: repo} }
func (s *Service) lock(id string) *sync.Mutex {
	v, _ := s.locks.LoadOrStore(id, &sync.Mutex{})
	return v.(*sync.Mutex)
}
func (s *Service) mutate(id, key string, expected int64, fn func(*commissioning.CommissioningCase) error) (*commissioning.CommissioningCase, error) {
	storeKey := key
	if key != "" {
		storeKey = id + ":" + key
	}
	if key != "" {
		if r, e := s.repo.GetIdempotency(storeKey); e == nil && r != nil {
			var w struct {
				Case *commissioning.CommissioningCase `json:"case"`
			}
			if json.Unmarshal(r.Body, &w) == nil && w.Case != nil {
				return w.Case, nil
			}
		}
	}
	m := s.lock(id)
	m.Lock()
	defer m.Unlock()
	c, e := s.repo.Get(id)
	if e != nil {
		return nil, e
	}
	if expected <= 0 || c.ExpectedVersion != expected {
		return nil, commissioning.ErrVersionConflict
	}
	if e = fn(c); e != nil {
		return nil, e
	}
	if e = s.repo.Save(c, storeKey); e != nil {
		return nil, e
	}
	// The case version advanced, so any cached review package is now stale.
	s.reviewPackages.Delete(id)
	return c, nil
}

func (s *Service) Create(zone, cat, owner, key string) (*commissioning.CommissioningCase, error) {
	zone, cat, owner = commissioning.NormalizeIdentity(zone, cat, owner)
	fingerprint := identityFingerprint(zone, cat, owner)
	if key != "" {
		m := s.lock("create:" + key)
		m.Lock()
		defer m.Unlock()
	}
	if key != "" {
		if r, e := s.repo.GetIdempotency(key); e == nil && r != nil {
			var w struct {
				SchemaVersion int                              `json:"schemaVersion"`
				Case          *commissioning.CommissioningCase `json:"case"`
			}
			if json.Unmarshal(r.Body, &w) == nil && w.Case != nil {
				storedFingerprint := r.Fingerprint
				if storedFingerprint == "" {
					storedFingerprint = identityFingerprint(w.Case.ZoneCode, w.Case.CollectionCategory, w.Case.OwnerName)
				}
				if storedFingerprint != fingerprint {
					return nil, commissioning.ErrIdempotencyConflict
				}
				return w.Case, nil
			}
		}
	}
	id := fmt.Sprintf("case-%d", time.Now().UnixNano())
	now := time.Now().UTC()
	c, e := commissioning.NewCase(id, zone, cat, owner, now)
	if e != nil {
		return nil, e
	}
	e = s.repo.Save(c, key)
	return c, e
}

func identityFingerprint(zone, category, owner string) string {
	b, _ := json.Marshal(struct{ Zone, Category, Owner string }{zone, category, owner})
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum[:])
}
func (s *Service) Get(id string) (*commissioning.CommissioningCase, error) { return s.repo.Get(id) }
func (s *Service) ReviseIdentity(id, key string, expected int64, in CaseInput) (*commissioning.CommissioningCase, error) {
	return s.mutate(id, key, expected, func(c *commissioning.CommissioningCase) error {
		return c.ReviseIdentity(in.ZoneCode, in.CollectionCategory, in.OwnerName, time.Now().UTC())
	})
}
func (s *Service) Baseline(id, key string, expected int64, b commissioning.BaselineProfile) (*commissioning.CommissioningCase, error) {
	return s.mutate(id, key, expected, func(c *commissioning.CommissioningCase) error { return c.SetBaseline(b, time.Now().UTC()) })
}
func (s *Service) RevokeBaseline(id, key string, expected int64, in BaselineRevocationInput) (*commissioning.CommissioningCase, error) {
	return s.mutate(id, key, expected, func(c *commissioning.CommissioningCase) error {
		return c.RevokeBaseline(in.Reason, in.Operator, time.Now().UTC())
	})
}
func (s *Service) Plan(id, key string, expected int64, p commissioning.ControlPlan) (*commissioning.CommissioningCase, error) {
	return s.mutate(id, key, expected, func(c *commissioning.CommissioningCase) error { return c.SubmitPlan(p, time.Now().UTC()) })
}
func (s *Service) RevisePlan(id, key string, expected int64, in PlanRevisionInput) (*commissioning.CommissioningCase, error) {
	return s.mutate(id, key, expected, func(c *commissioning.CommissioningCase) error {
		return c.RevisePlan(in.ControlPlan, in.Reason, time.Now().UTC())
	})
}
func (s *Service) Start(id, key string, expected int64) (*commissioning.CommissioningCase, error) {
	return s.mutate(id, key, expected, func(c *commissioning.CommissioningCase) error { return c.StartTrial(time.Now().UTC()) })
}
func (s *Service) Observe(id, key string, expected int64, o commissioning.TrialObservation) (*commissioning.CommissioningCase, error) {
	return s.ObserveBatch(id, key, expected, []commissioning.TrialObservation{o})
}

func (s *Service) ObserveBatch(id, key string, expected int64, observations []commissioning.TrialObservation) (*commissioning.CommissioningCase, error) {
	fingerprint := requestFingerprint(observations)
	storeKey := ""
	if key != "" {
		storeKey = id + ":" + key
	}
	m := s.lock(id)
	m.Lock()
	defer m.Unlock()
	if storeKey != "" {
		if result, err := s.repo.GetIdempotency(storeKey); err == nil && result != nil {
			if result.Fingerprint != "" && result.Fingerprint != fingerprint {
				return nil, commissioning.ErrIdempotencyConflict
			}
			var stored struct {
				Case *commissioning.CommissioningCase `json:"case"`
			}
			if json.Unmarshal(result.Body, &stored) != nil || stored.Case == nil {
				return nil, commissioning.ErrStorageCorrupt
			}
			return stored.Case, nil
		}
	}
	c, err := s.repo.Get(id)
	if err != nil {
		return nil, err
	}
	if expected <= 0 || c.ExpectedVersion != expected {
		return nil, commissioning.ErrVersionConflict
	}
	if err := c.AddObservations(observations, time.Now().UTC()); err != nil {
		return nil, err
	}
	if saver, ok := s.repo.(RequestFingerprintSaver); ok {
		if err := saver.SaveWithFingerprint(c, storeKey, fingerprint); err != nil {
			return nil, err
		}
		s.reviewPackages.Delete(id)
		return c, nil
	}
	if err := s.repo.Save(c, storeKey); err != nil {
		return nil, err
	}
	if storeKey != "" {
		result, err := s.repo.GetIdempotency(storeKey)
		if err != nil || result == nil {
			return nil, commissioning.ErrStorageCorrupt
		}
		result.Fingerprint = fingerprint
		if err := s.repo.SaveIdempotency(storeKey, *result); err != nil {
			return nil, err
		}
	}
	s.reviewPackages.Delete(id)
	return c, nil
}
func (s *Service) Remediate(id, key string, expected int64, in RemediationInput) (*commissioning.CommissioningCase, error) {
	return s.mutate(id, key, expected, func(c *commissioning.CommissioningCase) error {
		return c.RemediateDeviations(in.Targets(), in.ResolutionNote, in.Retests(), time.Now().UTC())
	})
}
func (s *Service) Review(id, key string, expected int64, r commissioning.ReviewDecision) (*commissioning.CommissioningCase, error) {
	return s.mutate(id, key, expected, func(c *commissioning.CommissioningCase) error { return c.Review(r, time.Now().UTC()) })
}
func (s *Service) Activate(id, key string, expected int64) (*commissioning.CommissioningCase, error) {
	return s.mutate(id, key, expected, func(c *commissioning.CommissioningCase) error { _, e := c.Activate(time.Now().UTC()); return e })
}
func (s *Service) Permit(code string) (*commissioning.ActivationPermit, error) {
	return s.repo.FindPermit(code)
}

func (s *Service) ReviewPackage(id string) (commissioning.ReviewPackage, error) {
	if cached, ok := s.reviewPackages.Load(id); ok {
		return cached.(commissioning.ReviewPackage), nil
	}
	// Build the package under the per-case mutation lock so that a concurrent
	// mutation cannot persist a new version (and invalidate the cache) between
	// the cache miss and the Store below, which would otherwise re-introduce a
	// stale package.
	m := s.lock(id)
	m.Lock()
	defer m.Unlock()
	c, err := s.repo.Get(id)
	if err != nil {
		return commissioning.ReviewPackage{}, err
	}
	pkg, err := c.BuildReviewPackage()
	if err != nil {
		return commissioning.ReviewPackage{}, err
	}
	// Re-check the cache: a mutation that completed while we were waiting on
	// the lock already invalidated and may have rebuilt it with fresh data.
	if fresh, ok := s.reviewPackages.Load(id); ok {
		return fresh.(commissioning.ReviewPackage), nil
	}
	s.reviewPackages.Store(id, pkg)
	return pkg, nil
}

func requestFingerprint(value any) string {
	b, _ := json.Marshal(value)
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum[:])
}
