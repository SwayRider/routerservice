package types

type CostingModel string

const (
	Auto			CostingModel = "auto"
	Bicycle			CostingModel = "bicycle"
	Bus				CostingModel = "bus"
	BikeShare		CostingModel = "bikeshare"
	Truck			CostingModel = "truck"
	Taxi			CostingModel = "taxi"
	MotorScooter	CostingModel = "motor_scooter"
	Motorcycle		CostingModel = "motorcycle"
	Multimodal		CostingModel = "multimodal"
	Pedestrian		CostingModel = "pedestrian"
)

type CostingOptions map[string]CostingOptionValues

type CostingOptionValues struct{
	// [auto, bicycle, bus, motorcycle, truck]
	// A penalty applied when transitioning between roads that do not have
	// consistent naming–in other words, no road names in common.
	// This penalty can be used to create simpler routes that tend to have fewer
	// maneuvers or narrative guidance instructions.
	// The default maneuver penalty is five seconds.
	ManeuverPenalty		*float64				`json:"maneuver_penalty,omitempty"`

	// [auto, bicycle, bus, motorcycle, truck]
	// A cost applied when a gate with undefined or private access is encountered.
	// This cost is added to the estimated time / elapsed time.
	// The default gate cost is 30 seconds.
	GateCost			*float64				`json:"gate_cost,omitempty"`

	// [auto, bicycle, bus, truc, motorcycle]
	// A penalty applied when a gate with no access information is on the road.
	// The default gate penalty is 300 seconds.
	GatePenalty			*float64				`json:"gate_penalty,omitempty"`

	// [auto, bus, motorcycle, truck]
	// A penalty applied when a gate or bollard with access=private is
	// encountered.
	// The default private access penalty is 450 seconds.
	PrivateAccessPenalty	*float64				`json:"private_access_penalty,omitempty"`

	// [auto, bicycle, bus, motorcycle, truck]
	// A penalty applied when entering an road which is only allowed to enter if
	// necessary to reach the destination.
	DestinationOnlyPenalty	*float64				`json:"destination_only_penalty,omitempty"`

	// [auto, bus, motorcycle, motorscooter, truck]
	// A cost applied when a toll booth is encountered.
	// This cost is added to the estimated and elapsed times.
	// The default cost is 15 seconds.
	TollBoothCost		*float64				`json:"toll_booth_cost,omitempty"`

	// [auto, bus, motorcycle, motorscooter, truck]
	// A penalty applied to the cost when a toll booth is encountered.
	// This penalty can be used to create paths that avoid toll roads.
	// The default toll booth penalty is 0.
	TollBoothPenalty	*float64				`json:"toll_booth_penalty,omitempty"`

	// [auto, bus, motorcycle, motorscooter, truck]
	// A cost applied when entering a ferry.
	// This cost is added to the estimated and elapsed times.
	// The default cost is 300 seconds (5 minutes).
	FerryCost			*float64				`json:"ferry_cost,omitempty"`

	// [auto, bicycle, bus, motorcycle, motorscooter, pedestrian, truck]
	// This value indicates the willingness to take ferries.
	// This is a range of values between 0 and 1. Values near 0 attempt to avoid
	// ferries and values near 1 will favor ferries.
	// The default value is 0.5. Note that sometimes ferries are required to
	// complete a route so values of 0 are not guaranteed to avoid ferries
	// entirely.
	UseFerry			*float64				`json:"use_ferry,omitempty"`

	// [auto, bus, motorcycle, motorscooter, truck]
	// This value indicates the willingness to take highways.
	// This is a range of values between 0 and 1. Values near 0 attempt to avoid
	// highways and values near 1 will favor highways.
	// The default value is 0.5. Note that sometimes highways are required to
	// complete a route so values of 0 are not guaranteed to avoid highways
	// entirely.
	UseHighways			*float64				`json:"use_highways,omitempty"`

	// [auto, bus, motorcycle, motorscooter, truck]
	// This value indicates the willingness to take roads with tolls.
	// This is a range of values between 0 and 1. Values smaller than 0.5 attempt
	// to avoid tolls and values greater than 0.5 will slightly favor them.
	// The default value is 0.5, indicating no preference. Note that sometimes
	// roads with tolls are required to complete a route so values of 0 are not
	// guaranteed to avoid them entirely.
	UseTolls			*float64				`json:"use_tolls,omitempty"`

	// [auto, bicycle, bus, motorcycle, motorscooter, pedestrian, truck]
	// This value indicates the willingness to take living streets.
	// This is a range of values between 0 and 1. Values near 0 attempt to avoid
	// living streets and values near 1 will favor living streets.
	// The default value is 0 for trucks, 0.1 for cars, buses, motor scooters and
	// motorcycles, and 0.5 for bicycles.
	// Note that sometimes living streets are required to complete a
	// route so values of 0 are not guaranteed to avoid living streets entirely.
	UseLivingStreets	*float64					`json:"use_living_streets,omitempty"`

	// [auto, bus, motorcycle, motorscooter, pedestrian, truck]
	// This value indicates the willingness to take track roads.
	// This is a range of values between 0 and 1. Values near 0 attempt to avoid
	// tracks and values near 1 will favor tracks a little bit.
	// The default value is 0 for autos, 0.5 for motor scooters and motorcycles.
	// Note that sometimes tracks are required to complete a route so values of 0
	// are not guaranteed to avoid tracks entirely.
	UseTracks			*float64				`json:"use_tracks,omitempty"`

	// [auto, bicycle, bus, motorcycle, motorscooter, pedestrian, truck]
	// A penalty applied for transition to generic service road.
	// The default penalty is 0 trucks and 15 for cars, buses, motor scooters
	// and motorcycles.
	ServicePenalty		*float64				`json:"service_penalty,omitempty"`

	// [auto, bus, motorcycle, motorscooter, pedestrian, truck]
	// A factor that modifies (multiplies) the cost when generic service roads
	// are encountered.
	// The default service_factor is 1.
	ServiceFactor		*float64				`json:"service_factor,omitempty"`

	// [auto, bicycle, bus, motorcycle, motorscooter, truck]
	// A cost applied when encountering an international border. This cost is
	// added to the estimated and elapsed times.
	// The default cost is 600 seconds.
	CountryCrossingCost	*float64				`json:"country_crossing_cost,omitempty"`

	// [auto, bicycle, bus, motorcycle, motorscooter, truck]
	// A penalty applied for a country crossing. This penalty can be used to
	// create paths that avoid spanning country boundaries.
	// The default penalty is 0.
	CountryCrossingPenalty	*float64			`json:"country_crossing_penalty,omitempty"`

	// [auto, bicycle, bus, motorcycle, motorscooter, taxi, pedestrian, truck]
	// Changes the metric to quasi-shortest, i.e. purely distance-based costing.
	// Note, this will disable all other costings & penalties.
	// Also note, shortest will not disable hierarchy pruning, leading to
	// potentially sub-optimal routes for some costing models.
	// The default is false.
	Shortest			*bool					`json:"shortest,omitempty"`

	// [auto, bus, motorcycle, motorscooter, truck]
	// A factor that allows controlling the contribution of distance and time to
	// the route costs. The value is in range between 0 and 1, where 0 only takes
	// time into account (default) and 1 only distance.
	// A factor of 0.5 will weight them roughly equally.
	// Note: this costing is currently only available for auto costing.
	UseDistance			*float64				`json:"use_distance,omitempty"`

	// [auto, bus, motorcycle, motorscooter, truck]
	// Disable hierarchies to calculate the actual optimal route.
	// The default is false.
	// Note: This could be quite a performance drainer so there is a upper limit
	// of distance. If the upper limit is exceeded, this option will always be
	// false.
	DisableHierarchyPruning	*bool				`json:"disable_hierarchy_pruning,omitempty"`

	// [auto, bus, motorcycle, motorscooter, truck]
	// Top speed the vehicle can go.
	// Also used to avoid roads with higher speeds than this value.
	// top_speed must be between 10 and 252 KPH.
	// The default value is 120 KPH for truck and 140 KPH for auto and bus.
	TopSpeed			*float64				`json:"top_speed,omitempty"`

	// [auto, bus, motorcycle, motorscooter, truck]
	// Fixed speed the vehicle can go. Used to override the calculated speed.
	// Can be useful if speed of vehicle is known.
	// fixed_speed must be between 1 and 252 KPH.
	// The default value is 0 KPH which disables fixed speed and falls back to
	// the standard calculated speed based on the road attribution.
	FixedSpeed			*float64				`json:"fixed_speed,omitempty"`

	// [auto, bus, motorcycle, motorscooter, truck]
	// If set to true, ignores all closures, marked due to live traffic closures,
	// during routing.
	// Note: This option cannot be set if location.search_filter.exclude_closures
	// is also specified in the request and will return an error if it is.
	// Default is false
	IgnoreClosures		*bool					`json:"ignore_closures,omitempty"`

	// [auto, bus, motorcycle, motorscooter, truck]
	// A factor that penalizes the cost when traversing a closed edge (eg: if
	// search_filter.exclude_closures is false for origin and/or destination
	// location and the route starts/ends on closed edges).
	// Its value can range from 1.0 - don't penalize closed edges,
	// to 10.0 - apply high cost penalty to closed edges.
	// Default value is 9.0.
	// Note: This factor is applicable only for motorized modes of transport,
	// i.e auto, motorcycle, motor_scooter, bus, truck & taxi.
	ClosureFactor		*float64				`json:"closure_factor,omitempty"`

	// [auto, bus, motorcycle, motorscooter, truck]
	// If set to true, ignores any restrictions (e.g. turn/dimensional/conditional
	// restrictions).
	// Especially useful for matching GPS traces to the road network regardless
	// of restrictions.
	// Default is false.
	IgnoreRestrictions	*bool					`json:"ignore_restrictions,omitempty"`

	// [auto, bus, motorcycle, motorscooter, truck]
	// If set to true, ignores one-way restrictions.
	// Especially useful for matching GPS traces to the road network ignoring
	// uni-directional traffic rules.
	// Not included in ignore_restrictions option.
	// Default is false
	IgnoreOneways		*bool					`json:"ignore_oneways,omitempty"`

	// [auto, bus, motorcycle, motorscooter, truck]
	// Similar to ignore_restrictions, but will respect restrictions that impact
	// vehicle safety, such as weight and size restrictions.
	IgnoreNonVehicularRestritions	*bool		`json:"ignore_non_vehicular_restrictions,omitempty"`

	// [auto, bus, motorcycle, motorscooter, truck]
	// Will ignore mode-specific access tags.
	// Especially useful for matching GPS traces to the road network regardless
	// of restrictions.
	// Default is false.
	IgnoreAccess		*bool					`json:"ignore_access,omitempty"`

	// [auto, bus, motorcycle, motorscooter, truck]
	// Will ignore construction tags. Only works when the include_construction
	// option is set before building the graph.
	// Useful for planning future routes.
	// Default is false.
	IgnoreConstruction	*bool					`json:"ignore_construction,omitempty"`

	// [auto, bus, motorcycle, motorscooter, truck]
	// Will determine which speed sources are used, if available.
	// A list of strings with the following possible values:
    // - freeflow
    // - constrained
    // - predicted
    // - current
	// Default is all sources (again, only if available).
	SpeedTypes 			[]Speed					`json:"speed_types,omitempty"`

	// [auto, bus, motorcycle, motorscooter, taxi, truck]
	// The height of the vehicle (in meters).
	// Default 1.9 for car, bus, taxi and 4.11 for truck.
	// Default for motorcycle = 2.0	(non-valhalla-default)
	Height				*float64				`json:"height,omitempty"`

	// [auto, bus, motorcycle, motorscooter, taxi, truck]
	// The width of the vehicle (in meters).
	// Default 1.6 for car, bus, taxi and 2.6 for truck.
	// Default for motorcycle = 1.0	(non-valhalla-default)
	Width				*float64				`json:"width,omitempty"`

	// [auto, bus, motorcycle, motorscooter, taxi, truck]
	// The length of the vehicle (in meters).
	// Default 2.7 for car, bus, taxi and 21.64 for truck.
	// Default for motorcycle = 2.0	(non-valhalla-default)
	Length				*float64				`json:"length,omitempty"`

	// [auto, bus, motorcycle, motorscooter, taxi, truck]
	// The weight of the vehicle (in tons).
	// Default 0.8 for car, bus, taxi and 21.77 for truck.
	// Default for motorcycle = 0.25 (non-valhalla-default)
	Weight				*float64				`json:"weight,omitempty"`

	// [auto, bus, motorcycle, motorscooter, taxi, truck]
	// This value indicates whether or not the path may include unpaved roads.
	// If exclude_unpaved is set to 1 it is allowed to start and end with unpaved
	// roads, but is not allowed to have them in the middle of the route path,
	// otherwise they are allowed.
	// Default false.
	ExcludeUnpaved		*bool					`json:"exclude_unpaved,omitempty"`

	// [auto, bus, motorcycle, motorscooter, taxi, truck]
	// A boolean value which indicates the desire to avoid routes with cash-only
	// tolls.
	// Default false.
	ExcludeCashOnlyTolls	*bool				`json:"exclude_cash_only_tolls,omitempty"`

	// [auto, bus, motorcycle, motorscooter, taxi, truck]
	// A boolean value which indicates the desire to include HOV roads with a
	// 2-occupant requirement in the route when advantageous.
	// Default false.
	IncludeHov2			*bool					`json:"include_hov2,omitempty"`

	// [auto, bus, motorcycle, motorscooter, taxi, truck]
	// A boolean value which indicates the desire to include HOV roads with a
	// 3-occupant requirement in the route when advantageous.
	// Default false.
	IncludeHov3			*bool					`json:"include_hov3,omitempty"`

	// [auto, bus, motorcycle, motorscooter, taxi, truck]
	// A boolean value which indicates the desire to include tolled HOV roads
	// which require the driver to pay a toll if the occupant requirement isn't
	// met.
	// Default false.
	IncludeHot			*bool					`json:"include_hot,omitempty"`

	// [auto, bus, motorcycle, motorscooter, taxi, truck]
	// Penalty factor applied for edges when edge speed is faster than top speed.
	// The default value is 0.05.
	SpeedPenaltyFactor	*float64				`json:"speed_penalty_factor,omitempty"`

	// [truck]
	// The axle load of the truck (in metric tons).
	// Default 9.07.
	AxleLoad			*float64				`json:"axle_load,omitempty"`

	// [truck]
	// The axle count of the truck.
	// Default 5.
	AxleCount			*int					`json:"axle_count,omitempty"`

	// [truck]
	// A value indicating if the truck is carrying hazardous materials.
	// Default false
	Hazmat				*bool					`json:"hazmat,omitempty"`
	
	// [truck]
	// A penalty applied to roads with no HGV/truck access.
	// If set to a value less than 43200 seconds, HGV will be allowed on these
	// roads and the penalty applies.
	// Default 43200, i.e. trucks are not allowed.
	HgvNoAccessPenalty	*float64					`json:"hgv_no_access_penalty,omitempty"`

	// [truck]
	// A penalty (in seconds) which is applied when going to residential or
	// service roads.
	// Default is 30 seconds.
	LowClassPenalty		*float64					`json:"low_class_penalty,omitempty"`

	// [truck]
	// This value is a range of values from 0 to 1, where 0 indicates a light
	// preference for streets marked as truck routes, and 1 indicates that
	// streets not marked as truck routes should be avoided.
	// This information is derived from the hgv=designated tag.
	// Note that even with values near 1, there is no guarantee the returned
	// route will include streets marked as truck routes.
	// The default value is 0.
	UseTruckRoute		*float64					`json:"use_truck_route,omitempty"`

	// [bicyle]
	// The type of bicycle.
	// The default type is hybrid.
    // - road: a road-style bicycle with narrow tires that is generally
	//   lightweight and designed for speed on paved surfaces.
    // - hybrid or city: a bicycle made mostly for city riding or casual riding
	//   on roads and paths with good surfaces.
    // - cross: a cyclo-cross bicycle, which is similar to a road bicycle but
	//   with wider tires suitable to rougher surfaces.
    // - mountain: a mountain bicycle suitable for most surfaces but generally
	//   heavier and slower on paved surfaces.
	BicycleType 		*BicycleKind				`json:"bicycle_type,omitempty"`

	// [bicycle]
	// Cycling speed is the average travel speed along smooth, flat roads.
	// This is meant to be the speed a rider can comfortably maintain over the
	// desired distance of the route. It can be modified (in the costing method)
	// by surface type in conjunction with bicycle type and (coming soon) by
	// hilliness of the road section.
	// When no speed is specifically provided, the default speed is determined by
	// the bicycle type and are as follows:
	// - road = 25 KPH (15.5 MPH),
    // - cross = 20 KPH (13 MPH),
    // - hybrid/city = 18 KPH (11.5 MPH),
    // - and mountain = 16 KPH (10 MPH).
	CyclingSpeed		*float64				`json:"cycling_speed,omitempty"`

	// [bicycle]
	// A cyclist's propensity to use roads alongside other vehicles.
	// This is a range of values from 0 to 1, where 0 attempts to avoid roads and
	// stay on cycleways and paths, and 1 indicates the rider is more comfortable
	// riding on roads.
	// Based on the use_roads factor, roads with certain classifications and
	// higher speeds are penalized in an attempt to avoid them when finding the
	// best path.
	// The default value is 0.5.
	UseRoads 			*float64 				`json:"use_roads,omitempty"`

	// [bicycle, pedestrian]
	// A cyclist's desire to tackle hills in their routes.
	// This is a range of values from 0 to 1, where 0 attempts to avoid hills and
	// steep grades even if it means a longer (time and distance) path, while 1
	// indicates the rider does not fear hills and steeper grades.
	// Based on the use_hills factor, penalties are applied to roads based on
	// elevation change and grade. These penalties help the path avoid hilly
	// roads in favor of flatter roads or less steep grades where available.
	// Note that it is not always possible to find alternate paths to avoid hills
	// (for example when route locations are in mountainous areas).
	// The default value is 0.5.
	UseHills			*float64				`json:"use_hills,omitempty"`

	// [bicycle]
	// This value is meant to represent how much a cyclist wants to avoid roads
	// with poor surfaces relative to the bicycle type being used.
	// This is a range of values between 0 and 1. When the value is 0, there is
	// no penalization of roads with different surface types; only bicycle speed
	// on each surface is taken into account. As the value approaches 1, roads
	// with poor surfaces for the bike are penalized heavier so that they are
	// only taken if they significantly improve travel time. When the value is
	// equal to 1, all bad surfaces are completely disallowed from routing,
	// including start and end points.
	// The default value is 0.25.
	AvoidBadSurfaces	*float64				`json:"avoid_bad_surfaces,omitempty"`

	// [bicycle]
	// This value is useful when bikeshare is chosen as travel mode.
	// It is meant to give the time will be used to return a rental bike.
	// This value will be displayed in the final directions and used to calculate
	// the whole duration.
	// The default value is 120 seconds.
	BssReturnCost		*float64				`json:"bss_return_cost,omitempty"`	

	// [bicycle]
	// This value is useful when bikeshare is chosen as travel mode.
	// It is meant to describe the potential effort to return a rental bike.
	// This value won't be displayed and used only inside of the algorithm.
	BssReturnPenalty	*float64				`json:"bss_return_penalty,omitempty"`

	// [motorscooter]
	// A rider's propensity to use primary roads.
	// This is a range of values from 0 to 1, where 0 attempts to avoid primary
	// roads, and 1 indicates the rider is more comfortable riding on primary
	// roads. Based on the use_primary factor, roads with certain classifications
	// and higher speeds are penalized in an attempt to avoid them when finding
	// the best path.
	// The default value is 0.5.
	UsePrimary			*float64				`json:"use_primary,omitempty"`

	// [motorcycle]
	// A riders's desire for adventure in their routes.
	// This is a range of values from 0 to 1, where 0 will avoid trails, tracks,
	// unclassified or bad surfaces and values towards 1 will tend to avoid major
	// roads and route on secondary roads.
	// The default value is 0.0.
	UseTrails			*float64				`json:"use_trails,omitempty"`

	// [Pedestrian]
	// Walking speed in kilometers per hour.
	// Must be between 0.5 and 25 km/hr.
	// Defaults to 5.1 km/hr (3.1 miles/hour).
	WalkingSpeed		*float64				`json:"walking_speed,omitempty"`

	// [Pedestrian]
	// A factor that modifies the cost when encountering roads classified as
	// footway (no motorized vehicles allowed), which may be designated footpaths
	// or designated sidewalks along residential roads.
	// Pedestrian routes generally attempt to favor using these walkways and
	// sidewalks.
	// The default walkway_factor is 1.0.
	WalkwayFactor		*float64				`json:"walkway_factor,omitempty"`

	// [Pedestrian]
	// A factor that modifies the cost when encountering roads with dedicated
	// sidewalks.
	// Pedestrian routes generally attempt to favor using sidewalks.
	// The default sidewalk_factor is 1.0.
	SidewalkFactor		*float64				`json:"sidewalk_factor,omitempty"`

	// [Pedestrian]
	// A factor that modifies (multiplies) the cost when alleys are encountered.
	// Pedestrian routes generally want to avoid alleys or narrow service roads
	// between buildings.
	// The default alley_factor is 2.0.
	AlleyFactor			*float64				`json:"alley_factor,omitempty"`

	// [Pedestrian]
	// A factor that modifies (multiplies) the cost when encountering a driveway,
	// which is often a private, service road.
	// Pedestrian routes generally want to avoid driveways (private).
	// The default driveway factor is 5.0.
	DrivewayFactor		*float64				`json:"driveway_factor,omitempty"`

	// [Pedestrian]
	// A penalty in seconds added to each transition onto a path with steps or
	// stairs. Higher values apply larger cost penalties to avoid paths that
	// contain flights of steps.
	StepPenalty			*float64				`json:"step_penalty,omitempty"`

	// [Pedestrian]
	// A penalty in seconds added to each transition via an elevator node or onto
	// an elevator edge. Higher values apply larger cost penalties to avoid
	// elevators.
	ElevatorPenalty		*float64				`json:"elevator_penalty,omitempty"`

	// [Pedestrian]
	// This value is a range of values from 0 to 1, where 0 indicates
	// indifference towards lit streets, and 1 indicates that unlit streets
	// should be avoided.
	// Note that even with values near 1, there is no guarantee the returned
	// route will include lit segments.
	// The default value is 0.
	UseLit				*float64				`json:"use_lit,omitempty"`

	// [Pedestrian]
	// This value indicates the maximum difficulty of hiking trails that is
	// allowed. Values between 0 and 6 are allowed. The values correspond to
	// sac_scale values within OpenStreetMap.
	// The default value is 1 which means that well cleared trails that are
	// mostly flat or slightly sloped are allowed.
	// Higher difficulty trails can be allowed by specifying a higher value for
	// max_hiking_difficully,
	MaxHikingDifficulty	*SacScale				`json:"max_hiking_difficulty,omitempty"`

	// [Pedestrian]
	// This value is useful when bikeshare is chosen as travel mode.
	// It is meant to give the time will be used to rent a bike from a bike share
	// station.
	// This value will be displayed in the final directions and used to calculate
	// the whole duration.
	// The default value is 120 seconds.
	BssRentCost			*float64				`json:"bss_rent_cost,omitempty"`

	// [Pedestrian]
	// This value is useful when bikeshare is chosen as travel mode. It is meant
	// to describe the potential effort to rent a bike from a bike share station.
	// This value won't be displayed and used only inside of the algorithm.
	BssRentPenalty		*float64				`json:"bss_rent_penalty,omitempty"`

	// [Pedestrian]
	// Sets the maximum total walking distance of a route.
	// Default is 100 km (~62 miles).
	MaxDistance			*float64				`json:"max_distance,omitempty"`

	// [Pedestrian]
	// A pedestrian option that can be added to the request to extend the
	// defaults (2145 meters or approximately 1.5 miles).
	// This is the maximum walking distance at the beginning or end of a route.
	TransitStartEndMaxDistance	*float64		`json:"transit_start_end_max_distance,omitempty"`

	// [Pedestrian]
	// A pedestrian option that can be added to the request to extend the
	// defaults (800 meters or 0.5 miles).
	// This is the maximum walking distance between transfers.
	TransitTransferMaxDistance	*float64		`json:"transit_transfer_max_distance,omitempty"`

	// [Pedestrian]
	// - If set to blind, enables additional route instructions, especially
	//   useful for blind users: Announcing crossed streets, the stairs,
	//   bridges, tunnels, gates and bollards, which need to be passed on route;
	//   information about traffic signals on crosswalks; route numbers not
	//   announced for named routes.
    // - If set to wheelchair, changes the defaults for max_distance,
	//   walking_speed, and step_penalty to be better aligned to the needs of
	//   wheelchair users.
	// These two options are mutually exclusive. In case you want to combine
	// them, please use blind and pass the options adjusted for wheelchair users
	// manually.
	// Default foot
	Type				*PedestrianKind			`json:"type,omitempty"`

	// [Pedestrian]
	// A factor which the cost of a pedestrian edge will be multiplied with on
	// multimodal request, e.g. bss or multimodal/transit.
	// Default is a factor of 1.5, i.e. avoiding walking.
	ModeFactor			*float64				`json:"mode_factor,omitempty"`

	// [Transit/Multimodal]
	// User's desire to use buses.
	// Range of values from 0 (try to avoid buses) to 1 (strong preference for
	// riding buses).
	UseBuses			*float64				`json:"use_buses,omitempty"`

	// [Transig/Multimodal]
	// User's desire to use rail/subway/metro.
	// Range of values from 0 (try to avoid rail) to 1 (strong preference for
	// riding rail).
	UseRails			*float64				`json:"use_rails,omitempty"`

	// [Transit/Multimodal]
	// User's desire to favor transfers
	// Range of values from 0 (try to avoid transfers) to 1 (totally comfortable
	// with transfers).
	UseTransfers		*float64				`json:"use_transfers,omitempty"`
}
