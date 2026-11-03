package server

import (
	"context"
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/swayrider/routerservice/internal/logic/logictest"
	vhtypes "github.com/swayrider/routerservice/restclients/valhalla/types"
	log "github.com/swayrider/swlib/logger"
)

type fakeTracer struct {
	mu       sync.Mutex
	requests []*vhtypes.TraceAttributesRequest
	regions  []string
	// answer is called for every lookup; the shape match of the request tells
	// the lookup over the route (edge_walk) from the lap (map_snap).
	answer func(req *vhtypes.TraceAttributesRequest) (*vhtypes.TraceAttributesResponse, error)
}

func (f *fakeTracer) TraceAttributes(
	_ context.Context,
	region string,
	req *vhtypes.TraceAttributesRequest,
) (*vhtypes.TraceAttributesResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, req)
	f.regions = append(f.regions, region)
	return f.answer(req)
}

func (f *fakeTracer) matches() []string {
	var out []string
	for _, r := range f.requests {
		out = append(out, r.ShapeMatch)
	}
	return out
}

func ringEdge(begin, end int, arms ...vhtypes.TraceIntersectingEdge) vhtypes.TraceEdge {
	e := vhtypes.TraceEdge{Roundabout: true, BeginHeading: begin, EndHeading: end}
	if len(arms) > 0 {
		e.EndNode = &vhtypes.TraceNode{IntersectingEdges: arms}
	}
	return e
}

func roadEdge(begin, end int) vhtypes.TraceEdge {
	return vhtypes.TraceEdge{BeginHeading: begin, EndHeading: end}
}

// firstExitTaken is the route over a roundabout that leaves by the east exit,
// the first one.
func firstExitTaken() *vhtypes.TraceAttributesResponse {
	return &vhtypes.TraceAttributesResponse{Edges: []vhtypes.TraceEdge{
		roadEdge(0, 0), ringEdge(80, 50), roadEdge(90, 90),
	}}
}

// lapOf is a lap round the same roundabout, map-matched: east, north and west
// exits, the first leaving [east] degrees to the outside of the ring (90 is
// straight out), and the shape the edges point into.
func lapOf(t *testing.T, east float64) *vhtypes.TraceAttributesResponse {
	t.Helper()
	edges, shape := logictest.LapAround(30, false, []logictest.Node{
		{Travelled: 90, Arms: []logictest.Arm{logictest.ExitArm(east)}},
		{Travelled: 180, Arms: []logictest.Arm{logictest.ExitArm(90)}},
		{Travelled: 270, Arms: []logictest.Arm{logictest.ExitArm(90)}},
		{Travelled: 330},
	})
	var flat []float64
	for _, p := range shape {
		flat = append(flat, p.Lat, p.Lon)
	}
	encoded, err := polylineCodec.EncodeFlatCoords(nil, flat)
	if err != nil {
		t.Fatal(err)
	}
	return &vhtypes.TraceAttributesResponse{Shape: string(encoded), Edges: edges}
}

func answers(route, lap func() *vhtypes.TraceAttributesResponse) func(*vhtypes.TraceAttributesRequest) (*vhtypes.TraceAttributesResponse, error) {
	return func(req *vhtypes.TraceAttributesRequest) (*vhtypes.TraceAttributesResponse, error) {
		if req.ShapeMatch == "map_snap" {
			return lap(), nil
		}
		return route(), nil
	}
}

// circleRoute is a route with a straight approach, a roundabout of the given
// radius (13 points over a third of the circle) and a short way out. The
// roundabout is entered at maneuver 1 and left at maneuver 2.
func circleRoute(t *testing.T, radius float64, exitCount int) *vhtypes.RouteResponse {
	t.Helper()
	const lat0, lon0 = 51.0, 4.0
	cosLat := math.Cos(lat0 * math.Pi / 180)
	var flat []float64
	for i := 0; i < 5; i++ { // approach, from the south
		flat = append(flat, lat0-float64(5-i)*0.0001, lon0)
	}
	for i := 0; i < 13; i++ { // roundabout
		a := float64(i) * 10 * math.Pi / 180
		flat = append(flat, lat0+radius*math.Sin(a)/111320, lon0+radius*math.Cos(a)/(cosLat*111320))
	}
	for i := 0; i < 3; i++ { // way out
		flat = append(flat, lat0+radius*math.Sin(2.1)/111320+float64(i+1)*0.0001, lon0+radius*math.Cos(2.1)/(cosLat*111320))
	}
	shape, err := polylineCodec.EncodeFlatCoords(nil, flat)
	if err != nil {
		t.Fatal(err)
	}
	return &vhtypes.RouteResponse{Trip: vhtypes.Trip{Legs: []vhtypes.Leg{{
		Shape: string(shape),
		Maneuvers: []vhtypes.Maneuver{
			{Type: 2, BeginShapeIndex: 0, EndShapeIndex: 5},
			{Type: maneuverRoundaboutEnter, BeginShapeIndex: 5, EndShapeIndex: 17, RoundaboutExitCount: intPtr(exitCount)},
			{Type: maneuverRoundaboutExit, BeginShapeIndex: 17, EndShapeIndex: 20},
			{Type: 4, BeginShapeIndex: 20, EndShapeIndex: 20},
		},
	}}}}
}

func exitsOf(route *vhtypes.RouteResponse) []int {
	return route.Trip.Legs[0].Maneuvers[1].RoundaboutExitAngles
}

func run(t *testing.T, tracer *fakeTracer, route *vhtypes.RouteResponse) {
	t.Helper()
	addRoundaboutArms(context.Background(), tracer, "be", time.Second, route, log.New())
}

func TestAddRoundaboutArms_AllExitsOfTheRoundabout(t *testing.T) {
	tracer := &fakeTracer{answer: answers(firstExitTaken, func() *vhtypes.TraceAttributesResponse { return lapOf(t, 90) })}
	route := circleRoute(t, 30, 1)

	run(t, tracer, route)

	if want := []int{90, 0, 270}; !reflect.DeepEqual(exitsOf(route), want) {
		t.Errorf("exits = %v, want %v (the exits after the one taken as well)", exitsOf(route), want)
	}
	if want := []string{"edge_walk", "map_snap"}; !reflect.DeepEqual(tracer.matches(), want) {
		t.Errorf("lookups = %v, want %v", tracer.matches(), want)
	}
	for _, r := range tracer.regions {
		if r != "be" {
			t.Errorf("region = %q, want be", r)
		}
	}
	// The lap goes on round the circle after the route's own points: 3 before
	// the roundabout, its 13 points, and the 23 that carry on to the entry.
	if got := len(tracer.requests[1].Shape); got != 3+13+23 {
		t.Errorf("lap shape points = %d, want %d", got, 3+13+23)
	}
	if got := len(tracer.requests[0].Shape); got != 3+13+3 {
		t.Errorf("route shape points = %d, want %d", got, 3+13+3)
	}
}

func TestAddRoundaboutArms_LapFailureLeavesNoExits(t *testing.T) {
	tracer := &fakeTracer{answer: func(req *vhtypes.TraceAttributesRequest) (*vhtypes.TraceAttributesResponse, error) {
		if req.ShapeMatch == "map_snap" {
			return nil, errors.New("valhalla slow")
		}
		return firstExitTaken(), nil
	}}
	route := circleRoute(t, 30, 1)

	run(t, tracer, route)

	// Never the partial list: the app would draw a roundabout with one road.
	if exitsOf(route) != nil {
		t.Errorf("exits = %v, want none", exitsOf(route))
	}
	if len(route.Trip.Legs[0].Maneuvers) != 4 {
		t.Error("the route must be left as it was")
	}
}

func TestAddRoundaboutArms_RouteLookupFailureLeavesNoExits(t *testing.T) {
	tracer := &fakeTracer{answer: func(*vhtypes.TraceAttributesRequest) (*vhtypes.TraceAttributesResponse, error) {
		return nil, errors.New("valhalla down")
	}}
	route := circleRoute(t, 30, 1)

	run(t, tracer, route)

	if exitsOf(route) != nil {
		t.Errorf("exits = %v, want none", exitsOf(route))
	}
	if got := tracer.matches(); !reflect.DeepEqual(got, []string{"edge_walk"}) {
		t.Errorf("lookups = %v: without the route lookup there is no lap to ask for", got)
	}
}

func TestAddRoundaboutArms_DisagreementWithExitCountIsIgnored(t *testing.T) {
	tracer := &fakeTracer{answer: answers(firstExitTaken, func() *vhtypes.TraceAttributesResponse { return lapOf(t, 90) })}
	route := circleRoute(t, 30, 3) // the lookup finds the first exit

	run(t, tracer, route)

	if exitsOf(route) != nil {
		t.Errorf("exits = %v, want none", exitsOf(route))
	}
}

func TestAddRoundaboutArms_LapThatIsAnotherRoundaboutIsIgnored(t *testing.T) {
	// The lap finds the east exit leaving at 50 degrees, not the 90 of the route.
	tracer := &fakeTracer{answer: answers(firstExitTaken, func() *vhtypes.TraceAttributesResponse { return lapOf(t, 50) })}
	route := circleRoute(t, 30, 1)

	run(t, tracer, route)

	if exitsOf(route) != nil {
		t.Errorf("exits = %v, want none", exitsOf(route))
	}
}

func TestAddRoundaboutArms_RoundaboutThatIsNotRoundIsNotWalked(t *testing.T) {
	tracer := &fakeTracer{answer: answers(firstExitTaken, func() *vhtypes.TraceAttributesResponse { return lapOf(t, 90) })}
	route := circleRoute(t, 400, 1) // not a roundabout's size

	run(t, tracer, route)

	if exitsOf(route) != nil {
		t.Errorf("exits = %v, want none", exitsOf(route))
	}
	if got := tracer.matches(); !reflect.DeepEqual(got, []string{"edge_walk"}) {
		t.Errorf("lookups = %v, want only the route lookup", got)
	}
}

func TestAddRoundaboutArms_NoRoundaboutNoLookup(t *testing.T) {
	tracer := &fakeTracer{answer: answers(firstExitTaken, func() *vhtypes.TraceAttributesResponse { return lapOf(t, 90) })}
	route := circleRoute(t, 30, 1)
	route.Trip.Legs[0].Maneuvers = route.Trip.Legs[0].Maneuvers[:1]

	run(t, tracer, route)

	if len(tracer.requests) != 0 {
		t.Errorf("lookups = %d, want 0", len(tracer.requests))
	}
}

func TestCreateManeuver_RoundaboutExits(t *testing.T) {
	got, err := createManeuver(&vhtypes.Maneuver{RoundaboutExitAngles: []int{90, 0, 270}}, log.New())
	if err != nil {
		t.Fatal(err)
	}
	if want := []int32{90, 0, 270}; !reflect.DeepEqual(got.RoundaboutExitAngles, want) {
		t.Errorf("RoundaboutExitAngles = %v, want %v", got.RoundaboutExitAngles, want)
	}
	if got.RoundaboutEntryAngles != nil {
		t.Errorf("RoundaboutEntryAngles = %v: the router no longer sends them", got.RoundaboutEntryAngles)
	}

	none, err := createManeuver(&vhtypes.Maneuver{}, log.New())
	if err != nil {
		t.Fatal(err)
	}
	if none.RoundaboutExitAngles != nil {
		t.Error("no exits known: the list must be empty")
	}
}
