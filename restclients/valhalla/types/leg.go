package types

// A trip contains one or more legs. For n number of break locations, there are
// n-1 legs. Through locations do not create separate legs.
// 
// Each leg of the trip includes a summary, which is comprised of the same
// information as a trip summary but applied to the single leg of the trip.
// It also includes a shape, which is an encoded polyline of the route path
// (with 6 digits decimal precision), and a list of maneuvers as a JSON array.
// 
// If elevation_interval is specified, each leg of the trip will return elevation
// along the route as a JSON array.
// The elevation_interval is also returned.
// Units for both elevation and elevation_interval are either meters or feet
// based on the input units specified.

type Leg struct {
	// The list of maneuvers
	Maneuvers					[]Maneuver		`json:"maneuvers"`

	// Json array of elevations
	Elevation					[]float64		`json:"elevation,omitempty"`

	// The elevation interval
	ElevationInterval			*float64			`json:"elevation_interval,omitempty"`

	// Encode polyline of the route path (6 digit precision)
	Shape						string			`json:"shape"`

	// The summary of the leg
	Summary						Summary			`json:"summary"`
}
