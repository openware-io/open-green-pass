package infra

import (
	"context"
	"sort"
	"sync"
	"time"

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
	points := s.aggregateTrend(filter, duration)
	out := make([]domain.CostTrendPoint, 0, len(points))
	for _, point := range points {
		out = append(out, point)
	}
	sortTrendPoints(out)
	return out, nil
}

type trendKey struct {
	startNanos int64
	category   domain.CostCategory
}

func (s *FakeCostTrendStore) aggregateTrend(filter domain.CostTrendFilter, duration time.Duration) map[trendKey]domain.CostTrendPoint {
	points := make(map[trendKey]domain.CostTrendPoint)
	for _, fact := range s.facts {
		if !matchesTrendFilter(fact, filter) {
			continue
		}
		start := fact.OccurredAt.UTC().Truncate(duration)
		key := trendKey{startNanos: start.UnixNano(), category: fact.Category}
		point := points[key]
		point.BucketStart = start
		point.Category = fact.Category
		point.Amount += fact.Amount
		point.TokensIn += fact.TokensIn
		point.TokensOut += fact.TokensOut
		point.CallCount++
		points[key] = point
	}
	return points
}

func matchesTrendFilter(fact domain.CostFact, filter domain.CostTrendFilter) bool {
	if fact.TeamID != filter.TeamID || fact.OccurredAt.Before(filter.From) || !fact.OccurredAt.Before(filter.To) {
		return false
	}
	if filter.TargetID != 0 && fact.TargetID != filter.TargetID {
		return false
	}
	if filter.CaseID != 0 && fact.CaseID != filter.CaseID {
		return false
	}
	if filter.Model != "" && fact.Model != filter.Model {
		return false
	}
	return filter.Category == "" || fact.Category == filter.Category
}

func sortTrendPoints(points []domain.CostTrendPoint) {
	sort.Slice(points, func(i, j int) bool {
		if points[i].BucketStart.Equal(points[j].BucketStart) {
			return points[i].Category < points[j].Category
		}
		return points[i].BucketStart.Before(points[j].BucketStart)
	})
}

var _ domain.CostTrendRepository = (*FakeCostTrendStore)(nil)
