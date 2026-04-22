package types

type SearchFilters struct {
	// Wether to exclude roads marked as tunnels.
	// Default is false
	ExcludeTunnel		*bool					`json:"exclude_tunnel,omitempty"`

	// Wether to exclude roads marked as bridges.
	// Default is false
	ExcludeBridge		*bool					`json:"exclude_bridge,omitempty"`

	// Wether to exclude toll.
	// Default is false
	ExcludeToll			*bool					`json:"exclude_toll,omitempty"`

	// Wether to exclude ferries.
	// Default is false
	ExcludeFerry		*bool					`json:"exclude_ferry,omitempty"`

	// Whether to exclude link roads marked as ramps, note that some turn
	// channels are also marked as ramps
	// Default is false
	ExcludeRamp  		*bool					`json:"exclude_ramp,omitempty"`

	// Whether to exclude roads considered closed due to live traffic closure.
	// Note: This option cannot be set if costing_options.<costing>.ignore_closures
	// is also specified. An error is returned if both options are specified. 
	// Note2: Ignoring closures at destination and source locations does NOT work
	// for date_time type 0/1 & 2 respectively
	ExcludeClosures		*bool					`json:"exclude_closures,omitempty"`

	// Lowest road class allowd.
	// Default is service_other
	MinRoadClass		*RoadClass				`json:"min_road_class,omitempty"`

	// Highest road class allowed.
	// Default is motorway
	MaxRoadClass		*RoadClass				`json:"max_road_class,omitempty"`

	// BETA
	// If specified, will only consider edges that are on or traverse the passed
	// floor level.
	// It will set search_cutoff to a default value of 300 meters if no cutoff
	// value is passed.
	// Additionally, if a search_cutoff is passed, it will be clamped to
	// 1000 meters.
	Level				*float64				`json:"level,omitempty"`
}
