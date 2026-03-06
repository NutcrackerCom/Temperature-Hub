package store

import (
	"context"
	"errors"
	"temperature-hub/internal/domain"
)

var (
	ErrEmptySensorID = errors.New("empty id")
	ErrIncorrectTemp = errors.New("incorrect temp")
	ErrNegativeLimit = errors.New("negative limit")
)

type ReadingStore interface {
	Append(ctx context.Context, r domain.Reading) error
	List(ctx context.Context, f domain.ReadingFilter) ([]domain.Reading, error)
}
