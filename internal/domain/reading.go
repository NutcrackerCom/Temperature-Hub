package domain

import "time"

type Reading struct {
	SensorID  string
	Timestamp time.Time
	TempC     float64
}

type ReadingFilter struct {
	SensorID string
	Limit    int
}
