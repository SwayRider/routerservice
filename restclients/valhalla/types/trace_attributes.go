package types

// ShapePoint is one point of a shape sent to trace_attributes.
type ShapePoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// TraceFilters selects the attributes trace_attributes returns.
type TraceFilters struct {
	// "include" returns only the listed attributes.
	Action     string   `json:"action"`
	Attributes []string `json:"attributes"`
}

// TraceAttributesRequest asks Valhalla which edges and nodes a shape follows.
type TraceAttributesRequest struct {
	Shape      []ShapePoint `json:"shape"`
	Costing    string       `json:"costing"`
	ShapeMatch string       `json:"shape_match"`
	Filters    TraceFilters `json:"filters"`
}

// TraceAttributesResponse holds the edges of the matched shape. Only the
// fields the router uses are decoded.
type TraceAttributesResponse struct {
	Edges []TraceEdge `json:"edges"`
}

// TraceEdge is one edge of the matched shape.
type TraceEdge struct {
	Roundabout   bool       `json:"roundabout"`
	BeginHeading int        `json:"begin_heading"`
	EndHeading   int        `json:"end_heading"`
	RoadClass    string     `json:"road_class"`
	DriveOnRight *bool      `json:"drive_on_right,omitempty"`
	EndNode      *TraceNode `json:"end_node,omitempty"`
}

// TraceNode is the node at the end of a [TraceEdge].
type TraceNode struct {
	IntersectingEdges []TraceIntersectingEdge `json:"intersecting_edges,omitempty"`
}

// TraceIntersectingEdge is a road that meets the path at a node. A missing
// Driveability means the edge cannot be driven (a cycleway, for example).
type TraceIntersectingEdge struct {
	BeginHeading int    `json:"begin_heading"`
	Driveability string `json:"driveability,omitempty"`
	RoadClass    string `json:"road_class,omitempty"`
}

// Driveability values of an intersecting edge, seen from the node.
const (
	DriveForward  = "forward"
	DriveBackward = "backward"
	DriveBoth     = "both"
)
