package types

type AdminCrossing struct {
	FromAdminIndex	int				`json:"from_admin_index"`
	ToAdminIndex	int				`json:"to_admin_index"`
}

type Summary struct {
	// Estimated elapsed time to complete the trip.
	Time 			float64 			`json:"time"`

	// Distance traveled for the entire trip. Units are either miles or
	// kilometers based on the input units specified.
	Length			float64				`json:"length"`

	// Flag indicating if the path uses one or more toll segments.
	HasToll			bool				`json:"has_toll"`

	// Flag indicating if the path uses one or more highway segments.
	HasHighway		bool				`json:"has_highway"`

	// Flag indicating if the path uses one or more ferry segments.
	HasFerry		bool				`json:"has_ferry"`

	// Minimum latitude of a bounding box containing the route.
	MinLat			float64				`json:"min_lat"`

	// Minimum longitude of a bounding box containing the route.
	MinLon			float64				`json:"min_lon"`

	// Maximum latitude of a bounding box containing the route.
	MaxLat			float64				`json:"max_lat"`

	// Maximum longitude of a bounding box containing the route.
	MaxLon			float64				`json:"max_lon"`

	Admins			[]string			`json:"admins,omitempty"`

	AdminCrossings  []AdminCrossing		`json:"admin_crossings,omitempty"`
}
