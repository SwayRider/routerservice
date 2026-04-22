package types

import "time"

type LocationKind string

const (
	Break			LocationKind = "break"
	Through			LocationKind = "through"
	Via				LocationKind = "via"
	BreakThrough	LocationKind = "break_through"
)

// Location
// A location must include a latitude and longitude in decimal degrees.
// The coordinates can come from many input sources, such as a GPS location,
// a point or a click on a map, a geocoding service, and so on.
// Note that the Valhalla cannot search for names or addresses or perform
// geocoding or reverse geocoding. External search services, such as Mapbox
// Geocoding, can be used to find places and geocode addresses, which must be
// converted to coordinates for input.
// 
// To build a route, you need to specify two break locations.
// In addition, you can include through, via or break_through locations to
// influence the route path.
type Location struct {
	// Latitude of the location in degrees.
	// This is assumed to be both the routing location and the display location
	// if no display_lat and display_lon are provided.
	Lat 				float64 				`json:"lat"`

	// Longitude of the location in degrees.
	// This is assumed to be both the routing location and the display location
	// if no display_lat and display_lon are provided.
	Lon					float64					`json:"lon"`

	// Type of location, either break, through, via or break_through.
	// Each type controls two characteristics: whether or not to allow a u-turn
	// at the location and whether or not to generate guidance/legs at
	// the location.
	// - A break is a location at which we allows u-turns and generate legs and
	//   arrival/departure maneuvers.
	// - A through location is a location at which we neither allow u-turns nor
	//   generate legs or arrival/departure maneuvers.
	// - A via location is a location at which we allow u-turns but do not
	//   generate legs or arrival/departure maneuvers.
	// - A break_through location is a location at which we do not allow u-turns
	//   but do generate legs and arrival/departure maneuvers.
	// If no type is provided, the type is assumed to be a break.
	// The types of the first and last locations are ignored and are treated as
	// breaks.
	LocationKind		*LocationKind			`json:"location_type,omitempty"`

	// (optional) Preferred direction of travel for the start from the location.
	// This can be useful for mobile routing where a vehicle is traveling in a
	// specific direction along a road, and the route should start in that
	// direction.
	// The heading is indicated in degrees from north in a clockwise direction,
	// where north is 0°, east is 90°, south is 180°, and west is 270°.
	Heading				*int					`json:"heading,omitempty"`

	// (optional) How close in degrees a given street's angle must be in orderu
	// for it to be considered as in the same direction of the heading parameter.
	// The default value is 60 degrees.
	HeadingTolerance	*int					`json:"heading_tolerance,omitempty"`

	// (optional) Street name. The street name may be used to assist finding the
	// correct routing location at the specified latitude, longitude.
	// This is not currently implemented.
	Street				*string					`json:"street,omitempty"`

	// (optional) OpenStreetMap identification number for a polyline way.
	// The way ID may be used to assist finding the correct routing location at
	// the specified latitude, longitude. This is not currently implemented.
	WayId				*int					`json:"way_id,omitempty"`

	// Minimum number of nodes (intersections) reachable for a given edge
	// (road between intersections) to consider that edge as belonging to a
	// connected region.
	// When correlating this location to the route network, try to find
	// candidates who are reachable from this many or more nodes (intersections).
	// If a given candidate edge reaches less than this number of nodes its
	// considered to be a disconnected island and we'll search for more candidates
	// until we find at least one that isn't considered a disconnected island.
	// If this value is larger than the configured service limit it will be
	// clamped to that limit.
	// The default is a minimum of 50 reachable nodes.
	MinimumReacability	*int					`json:"minimum_reachability,omitempty"`

	// The number of meters about this input location within which edges
	// (roads between intersections) will be considered as candidates for said
	// location.
	// When correlating this location to the route network, try to only return
	// results within this distance (meters) from this location. If there are no
	// candidates within this distance it will return the closest candidate
	// within reason.
	// If this value is larger than the configured service limit it will be
	// clamped to that limit.
	// The default is 0 meters.
	Radius				*int					`json:"radius,omitempty"`

	// Whether or not to rank the edge candidates for this location.
	// The ranking is used as a penalty within the routing algorithm so that some
	// edges will be penalized more heavily than others.
	// If true candidates will be ranked according to their distance from the
	// input and various other attributes.
	// If false the candidates will all be treated as equal which should lead to
	// routes that are just the most optimal path with emphasis about which edges
	// were selected.
	RankCandidates		*bool 					`json:"rank_candidates,omitempty"`

	// If the location is not offset from the road centerline or is closest to an
	// intersection this option has no effect. Otherwise the determined side of
	// street is used to determine whether or not the location should be visited
	// from the same, opposite or either side of the road with respect to the
	// side of the road the given locale drives on.
	// In Germany (driving on the right side of the road), passing a value of
	// same will only allow you to leave from or arrive at a location such that
	// the location will be on your right.
	// In Australia (driving on the left side of the road), passing a value of
	// same will force the location to be on your left.
	// A value of opposite will enforce arriving/departing from a location on the
	// opposite side of the road from that which you would be driving on while a
	// value of either will make no attempt limit the side of street that is
	// available for the route.
	PreferredSide		*Side					`json:"preferred_side,omitempty"`

	// Latitude of the map location in degrees.
	// If provided the lat and lon parameters will be treated as the routing
	// location and the display_lat and display_lon will be used to determine
	// the side of street.
	// Both display_lat and display_lon must be provided and valid to achieve the
	// desired effect.
	DisplayLat			*float64				`json:"display_lat,omitempty"`

	// Longitude of the map location in degrees.
	// If provided the lat and lon parameters will be treated as the routing
	// location and the display_lat and display_lon will be used to determine
	// the side of street.
	// Both display_lat and display_lon must be provided and valid to achieve the
	// desired effect.
	DisplayLon			*float64				`json:"display_lon,omitempty"`

	// The cutoff at which we will assume the input is too far away from
	// civilisation to be worth correlating to the nearest graph elements.
	// The default is 35 km.
	SearchCutoff		*int					`json:"search_cutoff,omitempty"`

	// During edge correlation this is the tolerance used to determine whether or
	// not to snap to the intersection rather than along the street, if the snap
	// location is within this distance from the intersection the intersection is
	// used instead.
	// The default is 5 meters.
	NodeSnapTolerance	*int					`json:"node_snap_tolerance,omitempty"`

	// If your input coordinate is less than this tolerance away from the edge
	// centerline then we set your side of street to none otherwise your side of
	// street will be left or right depending on direction of travel.
	// The default is 5 meters.
	StreetSideTolerance	*int					`json:"street_side_tolerance,omitempty"`

	// The max distance in meters that the input coordinates or display can be
	// from the edge centerline for them to be used for determining the side of
	// street.
	// Beyond this distance the side of street is set to none.
	// The default is 1000 meters.
	StreetSideMaxDistance	*int				`json:"street_side_max_distance,omitempty"`

	// Disables the preferred_side when set to same or opposite if the edge has
	// a road class less than that provided by street_side_cutoff.
	// The road class must be one of the following strings:
	// - motorway,
	// - trunk,
	// - primary,
	// - secondary,
	// - tertiary,
	// - unclassified,
	// - residential,
	// - service_other.
	// The default value is service_other so that preferred_side will not be
	// disabled for any edges. 
	StreetSideCutoff	*RoadClass				`json:"street_side_cutoff,omitempty"`

	// A set of optional filters to exclude candidate edges based on their
	// attribution.
	SearchFilter 		*SearchFilters			`json:"search_filter,omitempty"`

	// The layer on which edges should be considered.
	// If provided, edges whose layer does not match the provided value will be
	// discarded from the candidate search.
	PreferredLayer		*string					`json:"preferred_layer,omitempty"`

	// Optionally, you can include the following location information without
	// impacting the routing.
	// This information is carried through the request and returned as a convenience.

	// Location of business name
	Name				*string 				`json:"name,omitempty"`

	// City name
	City				*string					`json:"city,omitempty"`

	// State name
	State				*string					`json:"state,omitempty"`

	// Postal code
	PostalCode			*string					`json:"postal_code,omitempty"`

	// Country name
	Country				*string					`json:"country,omitempty"`

	// Telephone number
	Phone				*string					`json:"phone,omitempty"`

	// URL for the place or location
	Url					*string					`json:"url,omitempty"`

	// The waiting time in seconds at this location.
	// E.g. when the route describes a pizza delivery tour, each location has a
	// service time, which can be respected by setting waiting on the location,
	// then the departure will be delayed by this amount in seconds.
	// Only works for break or break_through types.
	Waiting				*int					`json:"waiting,omitempty"`

	// [Resonse Only]
	// The side of street of a break location that is determined based on the
	// actual route when the location is offset from the street.
	// The possible values are left and right.
	SideOfStreet		*SideOfStreet			`json:"side_of_street,omitempty"`

	// [Response Only]
	// Expected date/time for the user to be at the location using the ISO
	// 8601 format (YYYY-MM-DDThh:mm) in the local time zone of departure or
	// arrival.
	// For example "2015-12-29T08:00".
	// If waiting was set on this location in the request, and it's an
	// intermediate location, the date_time will describe the departure time at
	// this location.
	DateTime			*time.Time				`json:"datetime,omitempty"`

	// [Response Only]
	OriginalIndex		*int					`json:"original_index,omitempty"`

	// [Response Only]
	TimeZoneOffset		*string					`json:"time_zone_offset,omitempty"`

	// [Response Only]
	TimeZoneName		*string					`json:"time_zone_name,omitempty"`
}

func NewLocation(
	lat float64,
	lon float64,
) *Location {
	return &Location{
		Lat: lat,
		Lon: lon,
	}
}

func (l *Location) SetKind(
	kind LocationKind,
) {
	l.LocationKind = &kind
}
