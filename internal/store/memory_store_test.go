package store

import (
	"context"
	"errors"
	"temperature-hub/internal/domain"
	"testing"
	"time"
)

var (
	ErrCompWrongLen = errors.New("wrong len")
	ErrWrongElem    = errors.New("got wrong elem")
)

func compare(l []domain.Reading, r []domain.Reading) (bool, error) {
	if len(l) != len(r) {
		return false, ErrCompWrongLen
	}
	for i := 0; i < len(l); i++ {
		if l[i] != r[i] {
			return false, ErrWrongElem
		}
	}
	return true, nil
}

func TestAppend(t *testing.T) {
	data := time.Date(2000, time.April, 12, 12, 0, 0, 0, time.Local)
	reading := domain.Reading{SensorID: "1", Timestamp: data, TempC: 30}
	referenceStore := []domain.Reading{reading}
	reference := MemoryStore{store: referenceStore}

	s := NewMemoryStore()
	err := s.Append(context.Background(), reading)
	if err != nil {
		t.Fatalf("got err in adding data: %v", err)
	}
	if comp, err := compare(reference.store, s.store); !comp {
		t.Fatalf("got wrong reading, error: %v", err)
	}
}

func TestEmptyId(t *testing.T) {
	s := NewMemoryStore()
	reading := domain.Reading{SensorID: "", Timestamp: time.Now(), TempC: 30}
	if err := s.Append(context.Background(), reading); !errors.Is(err, ErrEmptySensorID) {
		t.Fatalf("expected %v error, got %v", ErrEmptySensorID, err)
	}
}

func TestIncTemp(t *testing.T) {
	s := NewMemoryStore()
	reading := domain.Reading{SensorID: "1", Timestamp: time.Now(), TempC: 300}
	if err := s.Append(context.Background(), reading); !errors.Is(err, ErrIncorrectTemp) {
		t.Fatalf("expected %v error, got %v", ErrIncorrectTemp, err)
	}
}

func TestGetList(t *testing.T) {
	s := NewMemoryStore()
	reading := domain.Reading{SensorID: "1", Timestamp: time.Now(), TempC: 30}
	if err := s.Append(context.Background(), reading); err != nil {
		t.Fatalf("got err %v", err)
	}

	l, err := s.List(context.Background(), domain.ReadingFilter{})
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if comp, err := compare([]domain.Reading{reading}, l); !comp {
		t.Fatalf("got wrong List, error: %v", err)
	}
}

func TestNegLimit(t *testing.T) {
	f := domain.ReadingFilter{SensorID: "1", Limit: -1}
	s := NewMemoryStore()
	reading := domain.Reading{SensorID: "1", Timestamp: time.Now(), TempC: 30}
	if err := s.Append(context.Background(), reading); err != nil {
		t.Fatalf("got err %v", err)
	}

	if _, err := s.List(context.Background(), f); !errors.Is(err, ErrNegativeLimit) {
		t.Fatalf("got wrong error: %v, expected %v", err, ErrNegativeLimit)
	}
}

func TestLimitList(t *testing.T) {
	f := domain.ReadingFilter{SensorID: "1", Limit: 3}

	s := NewMemoryStore()
	data := time.Date(2000, time.April, 12, 12, 0, 0, 0, time.Local)
	reading := domain.Reading{SensorID: "1", Timestamp: data, TempC: 30}

	for i := 0; i < 5; i++ {
		err := s.Append(context.Background(), reading)
		if err != nil {
			t.Fatalf("got error: %v", err)
		}
		reading.TempC++
	}

	l, err := s.List(context.Background(), f)
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	ref := []domain.Reading{
		{SensorID: "1", Timestamp: data, TempC: 32},
		{SensorID: "1", Timestamp: data, TempC: 33},
		{SensorID: "1", Timestamp: data, TempC: 34},
	}
	if comp, err := compare(ref, l); !comp {
		t.Fatalf("got wrong list, error: %v", err)
	}
}

func TestLimitZero(t *testing.T) {
	f := domain.ReadingFilter{SensorID: "1", Limit: 0}

	s := NewMemoryStore()
	data := time.Date(2000, time.April, 12, 12, 0, 0, 0, time.Local)
	reading := domain.Reading{SensorID: "1", Timestamp: data, TempC: 30}

	for i := 0; i < 5; i++ {
		err := s.Append(context.Background(), reading)
		if err != nil {
			t.Fatalf("got error: %v", err)
		}
		reading.TempC++
	}

	for i := 0; i < 5; i++ {
		reading.SensorID = "2"
		err := s.Append(context.Background(), reading)
		if err != nil {
			t.Fatalf("got error: %v", err)
		}
	}

	l, err := s.List(context.Background(), f)
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	ref := []domain.Reading{
		{SensorID: "1", Timestamp: data, TempC: 30},
		{SensorID: "1", Timestamp: data, TempC: 31},
		{SensorID: "1", Timestamp: data, TempC: 32},
		{SensorID: "1", Timestamp: data, TempC: 33},
		{SensorID: "1", Timestamp: data, TempC: 34},
	}
	if comp, err := compare(ref, l); !comp {
		t.Fatalf("got wrong list, error: %v", err)
	}
}
