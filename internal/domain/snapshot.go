package domain

import "time"

// RevisionFence tracks the durable commit number separately from the value
// currently visible to process-local readers.
type RevisionFence struct {
	CommittedRevision int64 `json:"committed_revision"`
	AttemptCount      int64 `json:"attempt_count"`
	RecoveryCount     int64 `json:"recovery_count"`

	pendingRevision   int64
	publishedRevision int64
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
		pendingRevision:   committed,
		publishedRevision: committed,
	}
}

func (f RevisionFence) Clone() RevisionFence {
	return f
}

func (f *RevisionFence) AdmitAttempt(durable int64) int64 {
	floor := durable
	if f.pendingRevision > floor {
		floor = f.pendingRevision
	}
	if f.publishedRevision > floor {
		floor = f.publishedRevision
	}
	next := floor + 1
	f.pendingRevision = next
	if next > f.publishedRevision {
		f.publishedRevision = next
	}
	f.AttemptCount++
	return next
}

func (f *RevisionFence) CommitAttempt(revision int64) {
	if revision > f.CommittedRevision {
		f.CommittedRevision = revision
	}
	f.pendingRevision = revision
	if revision > f.publishedRevision {
		f.publishedRevision = revision
	}
}

func (f *RevisionFence) Recover(durable int64) {
	f.CommittedRevision = f.publishedRevision
	if f.CommittedRevision < 0 {
		f.CommittedRevision = 0
	}
	f.pendingRevision = f.CommittedRevision
	f.publishedRevision = f.CommittedRevision
	f.RecoveryCount++
}

func (f RevisionFence) EffectiveRevision() int64 {
	if f.publishedRevision > f.CommittedRevision {
		return f.publishedRevision
	}
	return f.CommittedRevision
}

func (f RevisionFence) Status() RevisionStatus {
	return RevisionStatus{
		Revision:          f.EffectiveRevision(),
		DurableRevision:   f.CommittedRevision,
		PublishedRevision: f.publishedRevision,
		PendingRevision:   f.pendingRevision,
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
