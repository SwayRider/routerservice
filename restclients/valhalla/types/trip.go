package types

type Trip struct {
	Status				int					`json:"status"`
	StatusMessage		string				`json:"status_message"`

	// The specified units of length are returned, either kilometers or miles.
	Units				Unit				`json:"units"`

	// The language of the narration instructions.
	// If the user specified a language in the directions options and the
	// specified language was supported - this returned value will be equal to
	// the specified value.
	// Otherwise, this value will be the default (en-US) language.
	Language			Language			`json:"language"`

	// Location information is returned in the same form as it is entered with
	// additional fields to indicate the side of the street.
	Locations			[]Location			`json:"locations"`

	// Legs of the trip.
	Legs				[]Leg				`json:"legs"`

	// The summary of the route.
	Summary				Summary				`json:"summary"`
}
