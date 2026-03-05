package domain

import "time"

type Reading struct {
	SensorID  string
	Timestamp time.Time
	TempC     float64
}
