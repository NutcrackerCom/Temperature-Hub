package store

import (
	"context"
	"sync"
	"temperature-hub/internal/domain"
)

type MemoryStore struct {
	store []domain.Reading
	mu    sync.Mutex
}

func min(l, r int) int {
	if l < r {
		return l
	}
	return r
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

func (s *MemoryStore) Append(ctx context.Context, r domain.Reading) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if r.SensorID == "" {
		return ErrEmptySensorID
	}
	if r.TempC > 200 || r.TempC < -200 {
		return ErrIncorrectTemp
	}

	s.mu.Lock()
	s.store = append(s.store, r)
	s.mu.Unlock()

	return nil
}

func (s *MemoryStore) List(ctx context.Context, f domain.ReadingFilter) ([]domain.Reading, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if f.Limit < 0 {
		return nil, ErrNegativeLimit
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var limit int
	if f.Limit == 0 {
		limit = len(s.store)
	} else {
		limit = min(f.Limit, len(s.store))
	}

	if f.SensorID == "" {
		return append([]domain.Reading(nil), s.store[len(s.store)-limit:]...), nil
	}

	data := []domain.Reading{}

	for _, val := range s.store {
		if val.SensorID == f.SensorID {
			data = append(data, val)
		}
	}

	limitData := min(f.Limit, len(data))
	if limitData == 0 {
		limitData = len(data)
	}
	return append([]domain.Reading(nil), data[len(data)-limitData:]...), nil

}
