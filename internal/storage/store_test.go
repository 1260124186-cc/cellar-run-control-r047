package storage

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/1260124186-cc/solo-0016-cellar-run-control/internal/domain"
)

func TestFailedUpdateDoesNotConsumeOrPublishRevision(t *testing.T) {
	dataDir := t.TempDir()
	store, err := Open(dataDir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	failed := errors.New("validation failed")
	for range 3 {
		err := store.Execute(func(*domain.Snapshot) error {
			return failed
		})
		if !errors.Is(err, failed) {
			t.Fatalf("Update error = %v, want %v", err, failed)
		}
	}

	if revision := store.Revision(); revision != 0 {
		t.Fatalf("revision after failed updates = %d, want 0", revision)
	}
	if err := store.View(func(snapshot domain.Snapshot) error {
		if snapshot.Revision != 0 || snapshot.RevisionFence.CommittedRevision != 0 {
			t.Fatalf("snapshot revision = %d/%d, want 0/0",
				snapshot.Revision, snapshot.RevisionFence.CommittedRevision)
		}
		return nil
	}); err != nil {
		t.Fatalf("View() error = %v", err)
	}

	if err := store.Execute(func(*domain.Snapshot) error {
		return nil
	}); err != nil {
		t.Fatalf("successful Update() error = %v", err)
	}
	if revision := store.Revision(); revision != 1 {
		t.Fatalf("revision after first success = %d, want 1", revision)
	}

	if err := store.Execute(func(*domain.Snapshot) error {
		return failed
	}); !errors.Is(err, failed) {
		t.Fatalf("second failed Update error = %v, want %v", err, failed)
	}
	if revision := store.Revision(); revision != 1 {
		t.Fatalf("revision after later failure = %d, want 1", revision)
	}

	reopened, err := Open(dataDir)
	if err != nil {
		t.Fatalf("Open(after failed updates) error = %v", err)
	}
	if revision := reopened.Revision(); revision != 1 {
		t.Fatalf("revision after failed updates and restart = %d, want 1", revision)
	}
}

func TestSuccessfulUpdatesIncrementContinuouslyAndSurviveRestart(t *testing.T) {
	dataDir := t.TempDir()
	store, err := Open(dataDir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	for i := int64(1); i <= 3; i++ {
		if err := store.Execute(func(*domain.Snapshot) error {
			return nil
		}); err != nil {
			t.Fatalf("Update %d error = %v", i, err)
		}
		if revision := store.Revision(); revision != i {
			t.Fatalf("revision after update %d = %d, want %d", i, revision, i)
		}
	}

	reopened, err := Open(dataDir)
	if err != nil {
		t.Fatalf("Open(existing directory) error = %v", err)
	}
	if revision := reopened.Revision(); revision != 3 {
		t.Fatalf("revision after restart = %d, want 3", revision)
	}
	if err := reopened.Execute(func(*domain.Snapshot) error {
		return nil
	}); err != nil {
		t.Fatalf("Update after restart error = %v", err)
	}
	if revision := reopened.Revision(); revision != 4 {
		t.Fatalf("revision after restart and update = %d, want 4", revision)
	}
}

func TestLoadPreservesPersistedRevisionWhenLegacyFenceIsOlder(t *testing.T) {
	dataDir := t.TempDir()
	raw := []byte(`{
  "revision": 5,
  "revision_fence": {
    "committed_revision": 0,
    "attempt_count": 0,
    "recovery_count": 0
  },
  "formulas": {},
  "vessels": {},
  "runs": {},
  "formula_name_index": {},
  "vessel_code_index": {},
  "run_code_index": {}
}`)
	if err := os.WriteFile(filepath.Join(dataDir, stateFileName), raw, 0o600); err != nil {
		t.Fatalf("write legacy snapshot: %v", err)
	}

	store, err := Open(dataDir)
	if err != nil {
		t.Fatalf("Open(legacy snapshot) error = %v", err)
	}
	if revision := store.Revision(); revision != 5 {
		t.Fatalf("revision after load = %d, want 5", revision)
	}
	if err := store.Execute(func(*domain.Snapshot) error {
		return nil
	}); err != nil {
		t.Fatalf("Update after legacy load error = %v", err)
	}
	if revision := store.Revision(); revision != 6 {
		t.Fatalf("revision after legacy load and update = %d, want 6", revision)
	}
}

func TestConcurrentSuccessfulUpdatesRemainSequential(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	const workers = 20
	var wg sync.WaitGroup
	wg.Add(workers)
	start := make(chan struct{})
	errs := make(chan error, workers)
	for range workers {
		go func() {
			defer wg.Done()
			<-start
			errs <- store.Execute(func(*domain.Snapshot) error {
				return nil
			})
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Update error = %v", err)
		}
	}

	if revision := store.Revision(); revision != workers {
		t.Fatalf("revision after %d successful updates = %d, want %d",
			workers, revision, workers)
	}
	reopened, err := Open(filepath.Dir(store.path))
	if err != nil {
		t.Fatalf("Open() after concurrent updates error = %v", err)
	}
	if revision := reopened.Revision(); revision != workers {
		t.Fatalf("revision after restart = %d, want %d", revision, workers)
	}
}
