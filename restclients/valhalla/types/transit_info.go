package types

type TransitInfo struct {
	// Global transit route identifier.
	OnestopId			string					`json:"onestop_id"`

	// Short name describing the transit route. For example "N"
	ShortName			string					`json:"short_name"`

	// Long name describing the transit route. For example "Broadway Express".
	LongName			string					`json:"long_name"`

	// The sign on a public transport vehicle that identifies the route
	// destination to passengers.
	// For example "ASTORIA - DITMARS BLVD".
	Headsign			string					`json:"headsign"`

	// The numeric color value associated with a transit route.
	// The value for yellow would be "16567306".
	Color				int						`json:"color"`

	// The numeric text color value associated with a transit route.
	// The value for black would be "0".
	TextColor			int						`json:"text_color"`

	// The description of the transit route.
	// For example "Trains operate from Ditmars Boulevard, Queens, to Stillwell
	// Avenue, Brooklyn, at all times. N trains in Manhattan operate along
	// Broadway and across the Manhattan Bridge to and from Brooklyn. Trains in
	// Brooklyn operate along 4th Avenue, then through Borough Park to Gravesend.
	// Trains typically operate local in Queens, and either express or local in
	// Manhattan and Brooklyn, depending on the time. Late night trains operate
	// via Whitehall Street, Manhattan. Late night service is local".
	Description			string					`json:"description"`

	// Global operator/agency identifier.
	OperatorOnestopId	string					`json:"operator_onestop_id"`

	// Operator/agency name. For example, "BART", "King County Marine Division",
	// and so on. Short name is used over long name.
	OperatorName		string					`json:"operator_name"`

	// Operator/agency URL. For example, "https://web.mta.info/".
	OperatorUrl			string					`json:"operator_url"`

	// A list of the stops/stations associated with a specific transit route
	TransitStops		[]TransitStop			`json:"transit_stops"`
}
