package infra

import (
	"context"
	"sort"
	"sync"

	"github.com/openware-io/open-green-pass/internal/trusted/domain"
)

// FakeCostTrendStore is a deterministic in-memory projection for unit tests.
type FakeCostTrendStore struct {
	mu    sync.RWMutex
	facts map[int64]domain.CostFact
}

func NewFakeCostTrendStore() *FakeCostTrendStore {
	return &FakeCostTrendStore{facts: make(map[int64]domain.CostFact)}
}

func (s *FakeCostTrendStore) UpsertFact(_ context.Context, fact domain.CostFact) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.facts[fact.SourceCostID]; exists {
		return false, nil
	}
	s.facts[fact.SourceCostID] = fact
	return true, nil
}

func (s *FakeCostTrendStore) DeleteTeamFacts(_ context.Context, teamID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, fact := range s.facts {
		if fact.TeamID == teamID {
			delete(s.facts, id)
		}
	}
	return nil
}

func (s *FakeCostTrendStore) Trend(_ context.Context, filter domain.CostTrendFilter) ([]domain.CostTrendPoint, error) {
	duration, err := filter.Bucket.Duration()
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	type key struct {
		startNanos int64
		category   domain.CostCategory
	}
	points := make(map[key]domain.CostTrendPoint)
	for _, fact := range s.facts {
		if fact.TeamID != filter.TeamID || fact.OccurredAt.Before(filter.From) || !fact.OccurredAt.Before(filter.To) ||
			(filter.TargetID != 0 && fact.TargetID != filter.TargetID) || (filter.CaseID != 0 && fact.CaseID != filter.CaseID) ||
			(filter.Model != "" && fact.Model != filter.Model) || (filter.Category != "" && fact.Category != filter.Category) {
			continue
		}
		start := fact.OccurredAt.UTC().Truncate(duration)
		projectionKey := key{startNanos: start.UnixNano(), category: fact.Category}
		point := points[projectionKey]
		point.BucketStart = start
		point.Category = fact.Category
		point.Amount += fact.Amount
		point.TokensIn += fact.TokensIn
		point.TokensOut += fact.TokensOut
		point.CallCount++
		points[projectionKey] = point
	}
	out := make([]domain.CostTrendPoint, 0, len(points))
	for _, point := range points {
		out = append(out, point)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].BucketStart.Equal(out[j].BucketStart) {
			return out[i].Category < out[j].Category
		}
		return out[i].BucketStart.Before(out[j].BucketStart)
	})
	return out, nil
}

var _ domain.CostTrendRepository = (*FakeCostTrendStore)(nil)
