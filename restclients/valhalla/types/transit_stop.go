package types

import "time"

type TransitStopKind int

const (
	SimpleStop = iota
	Station
)

type TransitStop struct {
	Type        		TransitStopKind 		`json:"type"`

	// Name of the stop or station. For example "14 St - Union Sq".
	Name        		string					`json:"name"`

	// Arrival date and time using the ISO 8601 format (YYYY-MM-DDThh:mm).
	// For example, "2015-12-29T08:06".
	ArrivalDateTime		time.Time				`json:"arrival_date_time"`

	// Departure date and time using the ISO 8601 format (YYYY-MM-DDThh:mm).
	// For example, "2015-12-29T08:06".
	DepartureDateTime	time.Time				`json:"departure_date_time"`

	// True if this stop is a marked as a parent stop.
	IsParentStop		bool					`json:"is_parent_stop"`

	// True if the times are based on an assumed schedule because the actual
	// schedule is not known.
	AssumedSchedule		bool					`json:"assumed_schedule"`

	// Latitude of the transit stop in degrees
	Lat					float64					`json:"lat"`
	
	// Longitude of the transit stop in degrees
	Lon					float64					`json:"lon"`
}
