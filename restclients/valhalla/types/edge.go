package types

type EdgeId struct {
	Id		int `json:"id"`
	Value 	int `json:"value"`
	TileId	int `json:"tile_id"`
	Level 	int `json:"level"`
}

type EdgeInfo struct {
	Shape	string 		`json:"shape"`
	WayId	int 		`json:"way_id"`
	Names	[]string	`json:"names"`
}

type EdgeClassification struct {
	Link bool `json:"link"`
	Internal bool `json:"internal"`
	Surface string `json:"surface"`
	Classification RoadClass `json:"classification"`
}

type Restriction struct {
	Moped bool `json:"moped"`
	Wheelchair bool `json:"wheelchair"`
	Taxi bool `json:"taxi"`
	HOV bool `json:"HOV"`
	Truck bool `json:"truck"`
	Emergency bool `json:"emergency"`
	Pedestrian bool `json:"pedestrian"`
	Car bool `json:"car"`
	Bus bool `json:"bus"`
	Bicycle bool `json:"bicycle"`
}

type GeoAttributes struct {
	WeightGrade float64 `json:"weight_grade"`
	Length int `json:"length"`
}

type BikeNetwork struct {
	Mountain bool `json:"mountain"`
	Local bool `json:"local"`
	Regional bool `json:"regional"`
	National bool `json:"national"`
}

type EdgeDetails struct {
	Classification EdgeClassification `json:"classification"`
	EndNode NodeId `json:"end_node"`
	Speed int `json:"speed"`
	TrafficSignal bool `json:"traffic_signal"`
	StartRestriction Restriction `json:"start_restriction"`
	SpeedLimit int `json:"speed_limit"`
	GeoAttributes GeoAttributes `json:"geo_attributes"`
	CycleLane string `json:"cycle_lane"`
	AccessRestriction bool `json:"access_restriction"`
	PartOfComplexRestriction bool `json:"part_of_complex_restriction"`
	CountryCrossing bool `json:"country_crossing"`
	HasExitSign bool `json:"has_exit_sign"`
	LaneCount int `json:"lane_count"`
	SpeedType string `json:"speed_type"`
	DriveOnRight bool `json:"drive_on_right"`
	DestinationOnly bool `json:"destination_only"`
	Seasonal bool `json:"seasonal"`
	Tunnel bool `json:"tunnel"`
	Bridge bool `json:"bridge"`
	Access Restriction `json:"access"`
	Toll bool `json:"toll"`
	RoundAbout bool `json:"round_about"`
	BikeNetwork BikeNetwork `json:"bike_network"`
	EndRestriction Restriction `json:"end_restriction"`
	Unreachable bool `json:"unreachable"`
	Forward bool `json:"forward"`
	NotThru bool `json:"not_thru"`
	TruckRoute bool `json:"truck_route"`
	Use string `json:"use"`
}

type Edge struct {
	WayId 			*int 			`json:"way_id,omitempty"`
	CorrelatedLat 	*float64 		`json:"correlated_lat,omitempty"`
	CorrelatedLon	*float64 		`json:"correlated_lon,omitempty"`
	SideOfStreet 	*SideOfStreet	`json:"side_of_street,omitempty"`
	PercentAlong 	*float64 		`json:"percent_along,omitempty"`

	EdgeId *EdgeId `json:"edge_id,omitempty"`
	EdgeInfo *EdgeInfo `json:"edge_info,omitempty"`
	Edge *EdgeDetails `json:"edge,omitempty"`
	MinimumReachability int `json:"minimum_reachability"`
	Score float64 `json:"score"`
}
