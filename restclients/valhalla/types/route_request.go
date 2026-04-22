package types

import "math"

type RouteRequest struct { 
	// Name your route request.
	// If id is specified, the naming will be sent thru to the response.
	Id					*string					`json:"id,omitempty"`

	// You specify locations as an ordered list of two or more locations within
	// a JSON array.
	// Locations are visited in the order specified.
	Locations 			[]Location				`json:"locations"`

	// The local date and time at the location.
	DateTime			*DateTime				`json:"datetime,omitempty"`

	// Prioritize bidirectional a* when date_time.type = depart_at/current.
	// By default time_dependent_forward a* is used in these cases,
	// but bidirectional a* is much faster.
	// Currently it does not update the time (and speeds) when searching for the
	// route path, but the ETA on that route is recalculated based on the
	// time-dependent speeds.
	PrioritizeBidirectional	*bool				`json:"prioritize_bidirectional,omitempty"`

	// A set of locations to exclude or avoid within a route can be specified
	// using a JSON array of avoid_locations. The avoid_locations have the same
	// format as the locations list.
	// At a minimum each avoid location must include latitude and longitude.
	// The avoid_locations are mapped to the closest road or roads and these
	// roads are excluded from the route path computation.
	ExcludeLocations	[]Location				`json:"avoid_locations,omitempty"`

	// One or multiple exterior rings of polygons in the form of nested JSON
	// arrays, e.g. [[[lon1, lat1], [lon2,lat2]],[[lon1,lat1],[lon2,lat2]]].
	// Roads intersecting these rings will be avoided during path finding.
	// If you only need to avoid a few specific roads, it's much more efficient
	// to use exclude_locations.
	// Valhalla will close open rings (i.e. copy the first coordinate to the last
	// position).
	ExcludePolygons		[][][]float64			`json:"avoid_polygons,omitempty"`

	// Elevation interval (meters) for requesting elevation along the route.
	// Valhalla data must have been generated with elevation data.
	// If no elevation_interval is specified, no elevation will be returned for
	// the route.
	// An elevation interval of 30 meters is recommended when elevation along the
	// route is desired, matching the default data source's resolution.
	ElevationInterval	*int					`json:"elevation_interval,omitempty"`

	// Valhalla's routing service uses dynamic, run-time costing to generate the
	// route path. The route request must include the name of the costing model
	// and can include optional parameters available for the chosen costing model.
	Costing    			CostingModel			`json:"costing"`
	
	// Costing methods can have several options that can be adjusted to develop
	// the route path, as well as for estimating time along the path.
	//
	// - Cost options are fixed costs in seconds that are added to both the path
	//   cost and the estimated time. Examples of costs are gate_costs and
	//   toll_booth_costs, where a fixed amount of time is added. Costs are not
	//   generally used to influence the route path; instead, use penalties to
	//   do this.
	//   Costs must be in the range of 0.0 seconds to 43200.0 seconds (12 hours),
	//   otherwise a default value will be assigned.
	//
	// - Penalty options are fixed costs in seconds that are only added to the
	//   path cost. Penalties can influence the route path determination but do
	//   not add to the estimated time along the path. For example, add a
	//   toll_booth_penalty to create route paths that tend to avoid toll booths.
	//   Penalties must be in the range of 0.0 seconds to 43200.0 secondsx
	//   (12 hours), otherwise a default value will be assigned.
	//
	// - Factor options are used to multiply the cost along an edge or road
	//   section in a way that influences the path to favor or avoid a particular
	//	 attribute.
	//   Factor options do not impact estimated time along the path, though.
	//   Factors must be in the range 0.1 to 100000.0, where factors of 1.0 have
	//   no influence on cost. Anything outside of this range will be assigned a
	//   default value.
	//   Use a factor less than 1.0 to attempt to favor paths containing
	//   preferred attributes, and a value greater than 1.0 to avoid paths with
	//   undesirable attributes.
	//   Avoidance factors are more effective than favor factors at influencing
	//   a path. A factor's impact also depends on the length of road containing
	//   the specified attribute, as longer roads have more impact on the costing
	//   than very short roads. For this reason, penalty options tend to be
	//   better at influencing paths.
	//
	// A special costing option is shortest, which, when true, will solely use
	// distance as cost and disregard all other costs, penalties and factors.
	// It's available for all costing models except multimodal & bikeshare.
	//
	// Another special case is disable_hierarchy_pruning costing option.
	// As the name indicates, disable_hierarchy_pruning = true will disable
	// hierarchies in routing algorithms, which allows us to find the actual
	// optimal route even in edge cases.
	// For example, together with shortest = true they can find the actual
	// shortest route.
	// When disable_hierarchy_pruning is true and arc distances between source
	// and target are not above the max limit, the actual optimal route will be
	// calculated at the expense of performance.
	// Note that if arc distances between locations exceed the max limit,
	// disable_hierarchy_pruning is true will not be applied. This costing
	// option is available for all motorized costing models, i.e auto, motorcycle,
	// motor_scooter, bus, truck & taxi. For bicycle and pedestrian hierarchies
	// are always disabled by default.
	CostingOptions 		CostingOptions			`json:"costing_options"`

	// Direction Options

	// Distance units for output. Allowable unit types are miles (or mi) and
	// kilometers (or km).
	// If no unit type is specified, the units default to kilometers.
	Units				*Unit					`json:"units,omitempty"`

	// The language of the narration instructions based on the IETF BCP 47
	// language tag string.
	//If no language is specified or the specified language is unsupported,
	// United States-based English (en-US) is used.
	Language			*Language				`json:"language,omitempty"`

	// An enum with 3 values.
	// - none indicating no maneuvers or instructions should be returned.
    // - maneuvers indicating that only maneuvers be returned.
    // - instructions indicating that maneuvers with instructions should be
	//   returned (this is the default if not specified).
	DirectionsType		*Directions				`json:"directions_type,omitempty"`

	// Four options are available:
    // - json is default valhalla routing directions JSON format
    // - gpx returns the route as a GPX (GPS exchange format) XML track
    // - osrm creates a OSRM compatible route directions JSON
    // - pbf formats the result using protocol buffers
	Format				*Format					`json:"format,omitempty"`

	// If "format" : "osrm" is set: Specifies the optional format for the path
	// shape of each connection. One of polyline6 (default), polyline5, geojson
	// or no_shape.
	ShapeFormat			*ShapeFormat			`json:"shape_format,omitempty"`

	// If the format is osrm, this boolean indicates if each step should have the
	// additional bannerInstructions attribute, which can be displayed in some
	// navigation system SDKs.
	BannerInstructions	*bool					`json:"banner_instructions,omitempty"`

	// If the format is osrm, this boolean indicates if each step should have the
	// additional voiceInstructions attribute, which can be heard in some
	// navigation system SDKs.
	VoiceInstructions	*bool					`json:"voice_instructions,omitempty"`

	// A number denoting how many alternate routes should be provided.
	// There may be no alternates or less alternates than the user specifies.
	// Alternates are not yet supported on multipoint routes (that is, routes
	// with more than 2 locations).
	// They are also not supported on time dependent routes.
	Alternates			*int					`json:"alternates,omitempty"`

	// When present and true, the successful route response will include a key
	// linear_references.
	// Its value is an array of base64-encoded OpenLR location references, one
	// for each graph edge of the road network matched by the input trace.
	// https://www.openlr-association.com/fileadmin/user_upload/openlr-whitepaper_v1.5.pdf
	LinearReferences	*bool					`json:"linear_references,omitempty"`

	// A boolean indicating whether exit instructions at roundabouts should be
	// added to the output or not.
	// Default is true.
	RoundaboutExits		*bool					`json:"roundabout_exits,omitempty"`

	// When present and true, the successful route summary will include the two
	// keys admins and admin_crossings.
	// admins is an array of administrative regions the route lies within.
	// admin_crossings is an array of objects that contain from_admin_index and
	// to_admin_index, which are indices into the admins array.
	// They also contain from_shape_index and to_shape_index, which are start and
	// end indices of the edge along which an administrative boundary is crossed.
	AdminCrossings		*bool					`json:"admin_crossings,omitempty"`

	// When present and true, each maneuver in the route response can include a
	// lanes array describing lane-level guidance.
	// The lanes array details possible directions, as well as which lanes are
	// valid or active for following the maneuver.
	TurnLanes			*bool					`json:"turn_lanes,omitempty"`
}

type RouteDetailsMode = uint8

const (
	RDNoInfo			uint8 = 0x00
	RDDisplayInfo		uint8 = 0x01
	RDDetailsInfo		uint8 = 0x02
	RDNavigationIndo	uint8 = 0x04

	RDDisplay			uint8 = RDDisplayInfo
	RDDisplayWithDetail	uint8 = (RDDisplayInfo | RDDetailsInfo)
	RDFull				uint8 = (RDDisplayInfo | RDDetailsInfo | RDNavigationIndo)
)

func NewRouteRequest(
	costing CostingModel,
) *RouteRequest {
	return &RouteRequest{
		Costing:        costing,
		CostingOptions: CostingOptions{},
	}
}

func (r *RouteRequest) AddLocation(
	location Location,
) {
	r.Locations = append(r.Locations, location)
}

func (r *RouteRequest) SetRouteDetailsMode(
	_ string,
	mode RouteDetailsMode,
) {
	r.disableDetails()
	if mode&RDDisplayInfo == RDDisplayInfo {
		r.enableDisplayDetails()
	}
	if mode&RDDetailsInfo == RDDetailsInfo {
		r.enableDetailedDisplayDetails()
	}
	if mode&RDNavigationIndo == RDNavigationIndo {
		r.enableNavigationDetails()
	}
}

func (r *RouteRequest) disableDetails() {
	if r.ElevationInterval != nil {
		r.ElevationInterval = nil
	}
	r.setDirectionsType(NoDirections)
	r.disableRoundaboutExits()
	r.disableTurnLanes()
}

func (r *RouteRequest) enableDisplayDetails() {

}

func (r *RouteRequest) enableDetailedDisplayDetails() {
	if r.DirectionsType == nil {
		r.DirectionsType = new(Directions)
	}
	r.setDirectionsType(Maneuvers)
	r.setElevationInterval(30)
	r.enableRoundaboutExits()
}

func (r *RouteRequest) enableNavigationDetails() {
	r.setDirectionsType(Instructions)
	r.setElevationInterval(30)
	r.enableRoundaboutExits()
	r.enableTurnLanes()
}

func (r *RouteRequest) setElevationInterval(i int) {
	r.ElevationInterval = &i
}

func (r *RouteRequest) setDirectionsType(d Directions) {
	r.DirectionsType = &d
}

func (r *RouteRequest) disableRoundaboutExits() {
	if r.RoundaboutExits == nil {
		r.RoundaboutExits = new(bool)
	}
	*r.RoundaboutExits = false
}

func (r *RouteRequest) enableRoundaboutExits() {
	if r.RoundaboutExits == nil {
		r.RoundaboutExits = new(bool)
	}
	*r.RoundaboutExits = true
}

func (r *RouteRequest) disableTurnLanes() {
	if r.TurnLanes == nil {
		r.TurnLanes = new(bool)
	}
	*r.TurnLanes = false
}

func (r *RouteRequest) enableTurnLanes() {
	if r.TurnLanes == nil {
		r.TurnLanes = new(bool)
	}
	*r.TurnLanes = true
}

func (r *RouteRequest) SetTollPreference(model string, preference float64) {
	v := math.Max(0, math.Min(1, preference))
	obj := r.CostingOptions[model]
	obj.UseTolls = &v
	r.CostingOptions[model] = obj
}

func (r *RouteRequest) SetFerryPreference(model string, preference float64) {
	v := math.Max(0, math.Min(1, preference))
	obj := r.CostingOptions[model]
	obj.UseFerry = &v
	r.CostingOptions[model] = obj
}

func (r *RouteRequest) SetHighwayPreference(model string, preference float64) {
	v := math.Max(0, math.Min(1, preference))
	obj := r.CostingOptions[model]
	obj.UseHighways = &v
	r.CostingOptions[model] = obj
}

func (r *RouteRequest) SetLivingStreetsPreference(model string, preference float64) {
	v := math.Max(0, math.Min(1, preference))
	obj := r.CostingOptions[model]
	obj.UseLivingStreets = &v
	r.CostingOptions[model] = obj
}

func (r *RouteRequest) SetTracksPreference(model string, preference float64) {
	v := math.Max(0, math.Min(1, preference))
	obj := r.CostingOptions[model]
	obj.UseTracks = &v
	r.CostingOptions[model] = obj
}

func (r *RouteRequest) SetShortestPath(model string, shortestPath bool) {
	obj := r.CostingOptions[model]
	obj.Shortest = &shortestPath
	r.CostingOptions[model] = obj
}

func (r *RouteRequest) SetShortestDistancePreference(model string, preference float64) {
	v := math.Max(0, math.Min(1, preference))
	obj := r.CostingOptions[model]
	obj.UseDistance = &v
	r.CostingOptions[model] = obj
}

func (r *RouteRequest) SetTrailsPreference(model string, preference float64) {
	v := math.Max(0, math.Min(1, preference))
	obj := r.CostingOptions[model]
	obj.UseTrails = &v
	r.CostingOptions[model] = obj
}

func (r *RouteRequest) SetExcludeUnpaved(model string, excludeUnpaved bool) {
	obj := r.CostingOptions[model]
	obj.ExcludeUnpaved = &excludeUnpaved
	r.CostingOptions[model] = obj
}

func (r *RouteRequest) SetTopSpeed(model string, topSpeed float64) {
	obj := r.CostingOptions[model]
	obj.TopSpeed = &topSpeed
	r.CostingOptions[model] = obj
}

func (r *RouteRequest) SetCurvyness(model string, curvyness float64) {
	penalty := 10.0 * curvyness
	obj := r.CostingOptions[model]
	obj.ManeuverPenalty = &penalty
	r.CostingOptions[model] = obj
}

func (r *RouteRequest) SetPrimaryPreference(model string, preference float64) {
	v := math.Max(0, math.Min(1, preference))
	obj := r.CostingOptions[model]
	obj.UsePrimary = &v
}

