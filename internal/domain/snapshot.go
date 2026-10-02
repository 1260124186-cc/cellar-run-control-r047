package domain

import "time"

// RevisionStatus reports the single revision represented by a snapshot. The
// additional fields are retained so callers that inspect the revision fence
// continue to see the committed value.
type RevisionStatus struct {
	Revision          int64
	DurableRevision   int64
	PublishedRevision int64
	PendingRevision   int64
	AttemptCount      int64
	RecoveryCount     int64
}

// RevisionFence keeps the durable snapshot revision. Storage assigns the next
// number only after mutating the complete candidate successfully, and that
// number is not visible until the resulting snapshot has been stored.
type RevisionFence struct {
	CommittedRevision int64 `json:"committed_revision"`
	AttemptCount      int64 `json:"attempt_count"`
	RecoveryCount     int64 `json:"recovery_count"`
}

func NewRevisionFence(committed int64) RevisionFence {
	if committed < 0 {
		committed = 0
	}
	return RevisionFence{CommittedRevision: committed}
}

func (f RevisionFence) Clone() RevisionFence {
	return f
}

// Commit records a write that completed successfully. It is called before the
// result is stored, and the resulting snapshot is published only after the
// durable replacement has succeeded.
func (f *RevisionFence) Commit(revision int64) {
	if revision > f.CommittedRevision {
		f.CommittedRevision = revision
	}
	f.AttemptCount++
}

// Recover establishes the loaded durable snapshot as the revision floor. It can
// never move to an older process-local number.
func (f *RevisionFence) Recover(durable int64) {
	if durable < 0 {
		durable = 0
	}
	f.CommittedRevision = durable
	f.RecoveryCount++
}

func (f RevisionFence) Status() RevisionStatus {
	return RevisionStatus{
		Revision:          f.CommittedRevision,
		DurableRevision:   f.CommittedRevision,
		PublishedRevision: f.CommittedRevision,
		PendingRevision:   f.CommittedRevision,
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
		if value.ActiveRunID != nil {
			runID := *value.ActiveRunID
			cloned.Vessels[key] = withActiveRun(value, &runID)
			continue
		}
		cloned.Vessels[key] = value
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
