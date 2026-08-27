package application

import (
	"strings"
	"time"

	"github.com/benzhi-project-76d6a43f-1133-460a-a1bf-b8d831d47541/internal/domain/commissioning"
)

func (s *Service) ObservationSummary(id string, from, to *time.Time) (commissioning.ObservationSummary, error) {
	c, err := s.repo.Get(id)
	if err != nil {
		return commissioning.ObservationSummary{}, err
	}
	return c.SummarizeObservations(from, to)
}

func (s *Service) ReviewHistory(id string, query commissioning.ReviewHistoryQuery) (commissioning.ReviewHistory, error) {
	c, err := s.repo.Get(id)
	if err != nil {
		return commissioning.ReviewHistory{}, err
	}
	return c.ReviewHistory(query)
}

func (s *Service) DeviationLedger(id string, query commissioning.DeviationLedgerQuery) (commissioning.DeviationLedger, error) {
	c, err := s.repo.Get(id)
	if err != nil {
		return commissioning.DeviationLedger{}, err
	}
	return c.DeviationLedger(query)
}

func (s *Service) Export(id, target string) (ExportResult, error) {
	exporter, ok := s.repo.(SnapshotExporter)
	if !ok {
		return ExportResult{}, commissioning.ErrExportInvalid
	}
	return exporter.ExportSnapshot(id, strings.TrimSpace(target))
}
