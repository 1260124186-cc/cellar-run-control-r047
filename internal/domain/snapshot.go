package domain

import "time"

// RevisionFence keeps the durable commit number authoritative while
// holding a process-local high-water mark. A revision is reserved only
// for a write that has fully completed, and once a value has been
// published it can never move backwards within the process.
type RevisionFence struct {
	CommittedRevision int64 `json:"committed_revision"`
	AttemptCount      int64 `json:"attempt_count"`
	RecoveryCount     int64 `json:"recovery_count"`

	// highWater is process-local state (never serialized). It records the
	// largest revision this process has held, so an in-memory state can
	// never fall behind a revision that was already published.
	highWater int64
}

type RevisionStatus struct {
	Revision          int64
	DurableRevision   int64
	PublishedRevision int64
	PendingRevision   int64
	AttemptCount      int64
	RecoveryCount     int64
}

func NewRevisionFence(committed int64) RevisionFence {
	if committed < 0 {
		committed = 0
	}
	return RevisionFence{
		CommittedRevision: committed,
		highWater:         committed,
	}
}

func (f RevisionFence) Clone() RevisionFence {
	return f
}

// Reserve returns the revision a new successful write would use without
// publishing it. A rejected or unfinished write must not keep this value,
// so the fence itself is left untouched; reservation only advances the
// durable, monotonically increasing commit number.
func (f *RevisionFence) Reserve(durable int64) int64 {
	if durable < f.CommittedRevision {
		durable = f.CommittedRevision
	}
	if durable < 0 {
		durable = 0
	}
	return durable + 1
}

// Commit publishes a revision only after its snapshot has been persisted
// durably. The high-water mark advances together with the durable value,
// making the new fact immediately visible to concurrent readers.
func (f *RevisionFence) Commit(revision int64) {
	if revision <= f.CommittedRevision {
		return
	}
	f.CommittedRevision = revision
	f.AttemptCount++
	if revision > f.highWater {
		f.highWater = revision
	}
}

// Recover reconciles process-local state with the durable value loaded
// from disk. The durable high-water mark is the largest revision that
// may already have been observed by clients, so it is preserved and
// never reset to an earlier value.
func (f *RevisionFence) Recover(durable int64) {
	if durable < f.CommittedRevision {
		durable = f.CommittedRevision
	}
	if durable < f.highWater {
		durable = f.highWater
	}
	if durable < 0 {
		durable = 0
	}
	f.CommittedRevision = durable
	f.highWater = durable
	f.RecoveryCount++
}

// EffectiveRevision is the single continuous fact visible to readers: the
// revision of the last completely persisted, successful commit.
func (f RevisionFence) EffectiveRevision() int64 {
	if f.highWater > f.CommittedRevision {
		return f.highWater
	}
	return f.CommittedRevision
}

func (f RevisionFence) Status() RevisionStatus {
	revision := f.EffectiveRevision()
	return RevisionStatus{
		Revision:          revision,
		DurableRevision:   f.CommittedRevision,
		PublishedRevision: revision,
		PendingRevision:   0,
		AttemptCount:      f.AttemptCount,
		RecoveryCount:     f.RecoveryCount,
	}
}

type Snapshot struct {
	Revision      int64                      `json:"revision"`
	UpdatedAt     time.Time                  `json:"updated_at"`
	RevisionFence RevisionFence              `json:"revision_fence"`
	Formulas      map[string]Formula         `json:"formulas"`
	Vessels       map[string]Vessel          `json:"vessels"`
	Runs          map[string]FermentationRun `json:"runs"`
	NameIndex     map[string]string          `json:"formula_name_index"`
	VesselIndex   map[string]string          `json:"vessel_code_index"`
	RunIndex      map[string]string          `json:"run_code_index"`
}

func EmptySnapshot() Snapshot {
	return Snapshot{
		Revision:      0,
		RevisionFence: NewRevisionFence(0),
		Formulas:      map[string]Formula{},
		Vessels:       map[string]Vessel{},
		Runs:          map[string]FermentationRun{},
		NameIndex:     map[string]string{},
		VesselIndex:   map[string]string{},
		RunIndex:      map[string]string{},
	}
}

func (s Snapshot) Clone() Snapshot {
	cloned := EmptySnapshot()
	cloned.Revision = s.Revision
	cloned.UpdatedAt = s.UpdatedAt
	cloned.RevisionFence = s.RevisionFence.Clone()
	for key, value := range s.Formulas {
		cloned.Formulas[key] = value.Clone()
	}
	for key, value := range s.Vessels {
		cloned.Vessels[key] = value
		if value.ActiveRunID != nil {
			runID := *value.ActiveRunID
			cloned.Vessels[key] = value
			cloned.Vessels[key] = withActiveRun(value, &runID)
		}
	}
	for key, value := range s.Runs {
		cloned.Runs[key] = value.Clone()
	}
	for key, value := range s.NameIndex {
		cloned.NameIndex[key] = value
	}
	for key, value := range s.VesselIndex {
		cloned.VesselIndex[key] = value
	}
	for key, value := range s.RunIndex {
		cloned.RunIndex[key] = value
	}
	return cloned
}

func withActiveRun(v Vessel, runID *string) Vessel {
	value := v
	if runID == nil {
		value.ActiveRunID = nil
		return value
	}
	copied := *runID
	value.ActiveRunID = &copied
	return value
}
