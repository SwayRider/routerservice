package types

import "time"

type DateTimeKind int

const (
	CurrentDepartureTime DateTimeKind = iota
	SpecifiedDepartureType
	SpecifiedArrivalTime
	InvariantSpecifiedTime
)

type DateTime struct {
	Type		DateTimeKind			`json:"type"`
	Value		*time.Time				`json:"value"`
}
