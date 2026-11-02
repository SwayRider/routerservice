package server

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	vhtypes "github.com/swayrider/routerservice/restclients/valhalla/types"
	log "github.com/swayrider/swlib/logger"
)

type fakeTracer struct {
	mu       sync.Mutex
	requests []*vhtypes.TraceAttributesRequest
	regions  []string
	resp     *vhtypes.TraceAttributesResponse
	err      error
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
	return f.resp, f.err
}

// roundaboutRoute is a route with a ten point shape and a roundabout from
// point 4 to point 7, entered at maneuver 1 and left at maneuver 2.
func roundaboutRoute(t *testing.T, exitCount int) *vhtypes.RouteResponse {
	t.Helper()
	flat := make([]float64, 0, 20)
	for i := 0; i < 10; i++ {
		flat = append(flat, 51+float64(i)*0.001, 4)
	}
	shape, err := polylineCodec.EncodeFlatCoords(nil, flat)
	if err != nil {
		t.Fatal(err)
	}
	return &vhtypes.RouteResponse{Trip: vhtypes.Trip{Legs: []vhtypes.Leg{{
		Shape: string(shape),
		Maneuvers: []vhtypes.Maneuver{
			{Type: 2, BeginShapeIndex: 0, EndShapeIndex: 4},
			{Type: maneuverRoundaboutEnter, BeginShapeIndex: 4, EndShapeIndex: 6, RoundaboutExitCount: intPtr(exitCount)},
			{Type: maneuverRoundaboutExit, BeginShapeIndex: 6, EndShapeIndex: 7},
			{Type: 4, BeginShapeIndex: 7, EndShapeIndex: 9},
		},
	}}}}
}

// fourArmsResponse is what Valhalla returns for a roundabout whose third exit is taken.
func fourArmsResponse() *vhtypes.TraceAttributesResponse {
	ring := func(begin, end int, arms ...vhtypes.TraceIntersectingEdge) vhtypes.TraceEdge {
		e := vhtypes.TraceEdge{Roundabout: true, BeginHeading: begin, EndHeading: end}
		if len(arms) > 0 {
			e.EndNode = &vhtypes.TraceNode{IntersectingEdges: arms}
		}
		return e
	}
	return &vhtypes.TraceAttributesResponse{Edges: []vhtypes.TraceEdge{
		{BeginHeading: 0, EndHeading: 0},
		ring(60, 30, vhtypes.TraceIntersectingEdge{BeginHeading: 90, Driveability: "forward", RoadClass: "residential"}),
		ring(10, 340, vhtypes.TraceIntersectingEdge{BeginHeading: 0, Driveability: "forward", RoadClass: "residential"}),
		ring(340, 300),
		{BeginHeading: 270, EndHeading: 270},
	}}
}

func TestAddRoundaboutArms_FillsTheManeuver(t *testing.T) {
	tracer := &fakeTracer{resp: fourArmsResponse()}
	route := roundaboutRoute(t, 3)

	addRoundaboutArms(context.Background(), tracer, "be", time.Second, route, log.New())

	m := route.Trip.Legs[0].Maneuvers[1]
	if want := []int{90, 0, 270}; !reflect.DeepEqual(m.RoundaboutExitAngles, want) {
		t.Errorf("RoundaboutExitAngles = %v, want %v", m.RoundaboutExitAngles, want)
	}
	if len(tracer.requests) != 1 {
		t.Fatalf("lookups = %d, want 1 (only the roundabout)", len(tracer.requests))
	}
	if tracer.regions[0] != "be" {
		t.Errorf("region = %q, want be", tracer.regions[0])
	}
	// Points 1..8: three before the roundabout (4), two after its end (7).
	if got := len(tracer.requests[0].Shape); got != 8 {
		t.Errorf("shape points = %d, want 8", got)
	}
	if tracer.requests[0].ShapeMatch != "edge_walk" {
		t.Errorf("ShapeMatch = %q, want edge_walk", tracer.requests[0].ShapeMatch)
	}
}

func TestAddRoundaboutArms_LookupFailureLeavesTheRoute(t *testing.T) {
	route := roundaboutRoute(t, 3)

	addRoundaboutArms(context.Background(), &fakeTracer{err: errors.New("valhalla down")}, "be", time.Second, route, log.New())

	m := route.Trip.Legs[0].Maneuvers[1]
	if m.RoundaboutExitAngles != nil || m.RoundaboutEntryAngles != nil {
		t.Errorf("got arms %v / %v, want none", m.RoundaboutExitAngles, m.RoundaboutEntryAngles)
	}
	if len(route.Trip.Legs[0].Maneuvers) != 4 {
		t.Error("the maneuvers must be left as they were")
	}
}

func TestAddRoundaboutArms_DisagreementWithExitCountIsIgnored(t *testing.T) {
	route := roundaboutRoute(t, 5) // the lookup finds 3 exits
	addRoundaboutArms(context.Background(), &fakeTracer{resp: fourArmsResponse()}, "be", time.Second, route, log.New())

	if got := route.Trip.Legs[0].Maneuvers[1].RoundaboutExitAngles; got != nil {
		t.Errorf("got %v, want none", got)
	}
}

func TestAddRoundaboutArms_NoRoundaboutNoLookup(t *testing.T) {
	tracer := &fakeTracer{resp: fourArmsResponse()}
	route := roundaboutRoute(t, 3)
	route.Trip.Legs[0].Maneuvers = route.Trip.Legs[0].Maneuvers[:1]

	addRoundaboutArms(context.Background(), tracer, "be", time.Second, route, log.New())

	if len(tracer.requests) != 0 {
		t.Errorf("lookups = %d, want 0", len(tracer.requests))
	}
}

func TestCreateManeuver_RoundaboutArms(t *testing.T) {
	got, err := createManeuver(&vhtypes.Maneuver{
		RoundaboutExitAngles:  []int{90, 0, 270},
		RoundaboutEntryAngles: []int{30},
	}, log.New())
	if err != nil {
		t.Fatal(err)
	}
	if want := []int32{90, 0, 270}; !reflect.DeepEqual(got.RoundaboutExitAngles, want) {
		t.Errorf("RoundaboutExitAngles = %v, want %v", got.RoundaboutExitAngles, want)
	}
	if want := []int32{30}; !reflect.DeepEqual(got.RoundaboutEntryAngles, want) {
		t.Errorf("RoundaboutEntryAngles = %v, want %v", got.RoundaboutEntryAngles, want)
	}

	none, err := createManeuver(&vhtypes.Maneuver{}, log.New())
	if err != nil {
		t.Fatal(err)
	}
	if none.RoundaboutExitAngles != nil || none.RoundaboutEntryAngles != nil {
		t.Error("no arms known: both lists must be empty")
	}
}
