package types

type LocateRequest struct {
	// Wether to produce verbose output
	Verbose *bool `json:"verbose,omitempty"`

	// Locations for which to retrieve information
	Locations []Location `json:"locations"`

	// Optional costing model
	Costing *CostingModel `json:"costing,omitempty"`

	// Optional costng options
	CostingOptions *CostingOptions `json:"costing_options,omitempty"`
}

func NewLocateRequest(
	lat, lon float64,
) *LocateRequest {
	req := &LocateRequest{}
	req.Verbose = new(bool)
	*req.Verbose = true
	req.Locations = []Location{{Lat: lat, Lon: lon}}
	return req
}
