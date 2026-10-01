package service

import (
	"strings"

	"github.com/1260124186-cc/solo-0016-cellar-run-control/internal/clock"
	"github.com/1260124186-cc/solo-0016-cellar-run-control/internal/domain"
	"github.com/1260124186-cc/solo-0016-cellar-run-control/internal/storage"
)

type Service struct {
	store *storage.Store
	clock clock.Clock
}

func New(store *storage.Store, timeSource clock.Clock) *Service {
	if timeSource == nil {
		timeSource = clock.System{}
	}
	return &Service{store: store, clock: timeSource}
}

func (s *Service) Revision() int64 {
	return s.RevisionStatus().Revision
}

func (s *Service) RevisionStatus() domain.RevisionStatus {
	status := s.store.RevisionStatus()
	if status.Revision < 0 {
		status.Revision = 0
	}
	if status.DurableRevision < 0 {
		status.DurableRevision = 0
	}
	if status.PublishedRevision < 0 {
		status.PublishedRevision = 0
	}
	if status.PendingRevision < 0 {
		status.PendingRevision = 0
	}
	return status
}

func (s *Service) checkExpected(current int64, expected *int64) error {
	if expected != nil && current != *expected {
		return domain.NewError(domain.CodeRevisionConflict,
			"the aggregate version does not match X-Expected-Version")
	}
	return nil
}

func generateEntityID(prefix string) (string, error) {
	return domain.NewID(prefix)
}

func generatedRunCode() (string, error) {
	value, err := domain.NewID("run")
	if err != nil {
		return "", err
	}
	suffix := strings.ToUpper(strings.TrimPrefix(value, "run_"))
	return "RUN-" + suffix, nil
}
