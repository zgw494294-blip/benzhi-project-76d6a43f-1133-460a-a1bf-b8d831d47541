package concurrent_idempotency_replay_test

import (
	"sync"
	"testing"
	"time"

	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/application"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/domain/commissioning"
	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/storage/filecase"
)

type gatedRepository struct {
	*filecase.Store
	key     string
	mu      sync.Mutex
	waiting int
	ready   chan struct{}
	release chan struct{}
}

func (r *gatedRepository) GetIdempotency(key string) (*application.IdempotentResult, error) {
	if key != r.key {
		return r.Store.GetIdempotency(key)
	}
	r.mu.Lock()
	r.waiting++
	waiting := r.waiting
	r.mu.Unlock()
	if waiting <= 2 {
		if waiting == 2 {
			close(r.ready)
		}
		<-r.release
	}
	return r.Store.GetIdempotency(key)
}

func TestConcurrentMutationWithSameIdempotencyKeyReplays(t *testing.T) {
	store, err := filecase.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	caseValue, err := commissioning.NewCase("case-concurrent", "A-01", "书画", "负责人", now())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(caseValue, ""); err != nil {
		t.Fatal(err)
	}
	repo := &gatedRepository{
		Store:   store,
		key:     "case-concurrent:idem-concurrent",
		ready:   make(chan struct{}),
		release: make(chan struct{}),
	}
	service := application.New(repo)

	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := service.Baseline("case-concurrent", "idem-concurrent", 1, commissioning.BaselineProfile{
				TemperatureMin: 18, TemperatureMax: 24,
				HumidityMin: 45, HumidityMax: 55,
				SamplingIntervalMinutes: 60, MinimumObservationCount: 2,
			})
			results <- err
		}()
	}
	<-repo.ready
	close(repo.release)
	wg.Wait()
	close(results)

	var errs []error
	for err := range results {
		errs = append(errs, err)
	}
	if len(errs) != 2 || errs[0] != nil || errs[1] != nil {
		t.Fatalf("并发相同幂等键应重放首次成功结果，得到错误: %v", errs)
	}
}

func now() time.Time { return time.Unix(1700000000, 0).UTC() }
