package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/swayrider/grpcclients/regionclient"
	pbgeo "github.com/swayrider/protos/common_types/geo"
	routerv1 "github.com/swayrider/protos/router/v1"
	vhtypes "github.com/swayrider/routerservice/restclients/valhalla/types"
	log "github.com/swayrider/swlib/logger"
)

func tripWithTime(seconds float64) *routerv1.RouteResponse {
	return &routerv1.RouteResponse{Trip: &routerv1.Trip{
		Summary: &routerv1.Summary{Time: seconds},
	}}
}

func candidate(index int, seconds, remaining, penalty float64) rejoinCandidate {
	return rejoinCandidate{
		index: index, resp: tripWithTime(seconds),
		seconds: seconds, remaining: remaining, penalty: penalty,
	}
}

func TestChooseRejoin(t *testing.T) {
	cfg := DefaultRejoinConfig()
	fresh := func(seconds float64) *routerv1.RouteResponse { return tripWithTime(seconds) }

	tests := []struct {
		name       string
		fresh      *routerv1.RouteResponse
		candidates []rejoinCandidate
		wantKind   routerv1.RejoinKind
		wantIndex  int
		wantErr    bool
	}{
		{
			name:       "the cheapest rejoin point wins",
			fresh:      fresh(700),
			candidates: []rejoinCandidate{candidate(0, 60, 1000, 0), candidate(1, 100, 700, 0)},
			wantKind:   routerv1.RejoinKind_RJ_REJOIN, wantIndex: 1,
		},
		{
			name:       "an earlier point wins when the difference is within the tie margin",
			fresh:      fresh(700),
			candidates: []rejoinCandidate{candidate(0, 100, 700, 0), candidate(1, 90, 700, 0)},
			wantKind:   routerv1.RejoinKind_RJ_REJOIN, wantIndex: 0,
		},
		{
			name:       "a clearly better later point beats an earlier one",
			fresh:      fresh(700),
			candidates: []rejoinCandidate{candidate(0, 100, 700, 0), candidate(1, 40, 700, 0)},
			wantKind:   routerv1.RejoinKind_RJ_REJOIN, wantIndex: 1,
		},
		{
			name:       "a turn-back penalty moves the choice to a forward point",
			fresh:      fresh(900),
			candidates: []rejoinCandidate{candidate(0, 60, 800, 90), candidate(1, 120, 700, 0)},
			wantKind:   routerv1.RejoinKind_RJ_REJOIN, wantIndex: 1,
		},
		{
			name:       "a fresh route that is much faster wins",
			fresh:      fresh(300),
			candidates: []rejoinCandidate{candidate(0, 60, 1000, 0)},
			wantKind:   routerv1.RejoinKind_RJ_FRESH,
		},
		{
			name:       "a rejoin just inside the slack still wins",
			fresh:      fresh(600),
			candidates: []rejoinCandidate{candidate(0, 60, 650, 0)}, // 710 <= 600 + 120
			wantKind:   routerv1.RejoinKind_RJ_REJOIN, wantIndex: 0,
		},
		{
			name:       "no connector found: fresh",
			fresh:      fresh(600),
			candidates: nil,
			wantKind:   routerv1.RejoinKind_RJ_FRESH,
		},
		{
			name:       "no fresh route: the best rejoin",
			fresh:      nil,
			candidates: []rejoinCandidate{candidate(2, 60, 500, 0)},
			wantKind:   routerv1.RejoinKind_RJ_REJOIN, wantIndex: 2,
		},
		{
			name:    "nothing at all is an error",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, index, resp, err := chooseRejoin(cfg, tt.fresh, tt.candidates)
			if tt.wantErr {
				if err == nil {
					t.Fatal("want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if kind != tt.wantKind || index != tt.wantIndex {
				t.Errorf("got %v %d, want %v %d", kind, index, tt.wantKind, tt.wantIndex)
			}
			if resp == nil {
				t.Error("want a response")
			}
		})
	}
}

func TestStartsWithTurnBack(t *testing.T) {
	heading := int32(90) // east
	route := func(bearing int32, firstLengthKm float64, second routerv1.ManeuverType) *routerv1.RouteResponse {
		return &routerv1.RouteResponse{Trip: &routerv1.Trip{Legs: []*routerv1.Leg{{
			Maneuvers: []*routerv1.Maneuver{
				{Type: routerv1.ManeuverType_M_START, BearingAfter: bearing, Length: firstLengthKm},
				{Type: second},
			},
		}}}}
	}
	if startsWithTurnBack(route(100, 1, routerv1.ManeuverType_M_RIGHT), &heading) {
		t.Error("a start close to the heading is not a turn-back")
	}
	if !startsWithTurnBack(route(270, 1, routerv1.ManeuverType_M_RIGHT), &heading) {
		t.Error("leaving the opposite way is a turn-back")
	}
	if !startsWithTurnBack(route(90, 0.05, routerv1.ManeuverType_M_UTURN_LEFT), &heading) {
		t.Error("an immediate U-turn is a turn-back")
	}
	if startsWithTurnBack(route(90, 1, routerv1.ManeuverType_M_UTURN_LEFT), &heading) {
		t.Error("a U-turn far along the route is not a turn-back at the start")
	}
	if startsWithTurnBack(route(270, 1, routerv1.ManeuverType_M_RIGHT), nil) {
		t.Error("without a heading nothing can be a turn-back")
	}
	// 350 vs 10 wraps around north: 20 degrees apart.
	north := int32(10)
	if startsWithTurnBack(route(350, 1, routerv1.ManeuverType_M_RIGHT), &north) {
		t.Error("the angle must wrap around north")
	}
}

func TestConnectorRequest(t *testing.T) {
	heading, tolerance := int32(90), int32(30)
	fresh := &routerv1.RouteRequest{
		Mode: routerv1.RoutingMode_RM_MOTORSCOOTER,
		Locations: []*routerv1.RouteLocation{
			{Location: &pbgeo.Coordinate{Lat: 51, Lon: 4}, PreferredHeading: &heading, HeadingTolerance: &tolerance},
			{Location: &pbgeo.Coordinate{Lat: 51.5, Lon: 4.5}},
			{Location: &pbgeo.Coordinate{Lat: 52, Lon: 5}},
		},
		ExcludePolygons: []*pbgeo.Polygon{{Points: []*pbgeo.Coordinate{{Lat: 1, Lon: 1}}}},
	}
	point := &routerv1.RejoinPoint{Location: &pbgeo.Coordinate{Lat: 51.2, Lon: 4.2}, Heading: 180}

	with := connectorRequest(fresh, point, true)
	if len(with.Locations) != 2 {
		t.Fatalf("want start and rejoin point, got %d locations", len(with.Locations))
	}
	if h := with.Locations[0].PreferredHeading; h == nil || *h != 90 {
		t.Errorf("start heading: %v", h)
	}
	end := with.Locations[1]
	if end.PreferredHeading == nil || *end.PreferredHeading != 180 ||
		end.HeadingTolerance == nil || *end.HeadingTolerance != arrivalHeadingTolerance {
		t.Errorf("arrival heading: %v tolerance %v", end.PreferredHeading, end.HeadingTolerance)
	}
	if len(with.ExcludePolygons) != 1 || with.Mode != routerv1.RoutingMode_RM_MOTORSCOOTER {
		t.Error("options and exclusions are kept")
	}

	without := connectorRequest(fresh, point, false)
	if without.Locations[0].PreferredHeading != nil || without.Locations[0].HeadingTolerance != nil {
		t.Error("the start heading must be dropped")
	}
	if fresh.Locations[0].PreferredHeading == nil || len(fresh.Locations) != 3 {
		t.Error("the fresh request must not be modified")
	}
}

func TestValidateRejoin(t *testing.T) {
	good := func() *routerv1.RejoinRequest {
		return &routerv1.RejoinRequest{
			Fresh: &routerv1.RouteRequest{Locations: []*routerv1.RouteLocation{
				{Location: &pbgeo.Coordinate{Lat: 51, Lon: 4}},
				{Location: &pbgeo.Coordinate{Lat: 52, Lon: 5}},
			}},
			RejoinPoints: []*routerv1.RejoinPoint{
				{Location: &pbgeo.Coordinate{Lat: 51.1, Lon: 4.1}, Heading: 90, RemainingSeconds: 100},
			},
		}
	}
	if err := validateRejoin(good()); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	mutations := map[string]func(*routerv1.RejoinRequest){
		"no fresh":      func(r *routerv1.RejoinRequest) { r.Fresh = nil },
		"one location":  func(r *routerv1.RejoinRequest) { r.Fresh.Locations = r.Fresh.Locations[:1] },
		"bad heading":   func(r *routerv1.RejoinRequest) { r.RejoinPoints[0].Heading = 360 },
		"negative time": func(r *routerv1.RejoinRequest) { r.RejoinPoints[0].RemainingSeconds = -1 },
		"no coordinate": func(r *routerv1.RejoinRequest) { r.RejoinPoints[0].Location = nil },
		"bad latitude":  func(r *routerv1.RejoinRequest) { r.RejoinPoints[0].Location.Lat = 120 },
		"too many points": func(r *routerv1.RejoinRequest) {
			for len(r.RejoinPoints) <= maxRejoinPoints {
				r.RejoinPoints = append(r.RejoinPoints, r.RejoinPoints[0])
			}
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			r := good()
			mutate(r)
			if err := validateRejoin(r); err == nil {
				t.Error("want a validation error")
			}
		})
	}
}

// rejoinValhalla is a fake Valhalla region whose answers depend on the
// destination (by latitude) and on whether the start carries a heading.
type rejoinValhalla struct {
	// seconds by destination latitude (rounded to 3 decimals).
	seconds map[float64]float64
	// noRouteWithHeading makes requests whose start has a heading fail.
	noRouteWithHeading bool
	// freshLat is the latitude of the final destination (the fresh route).
	freshSeconds float64
}

func (v rejoinValhalla) serve(t *testing.T) *httptest.Server {
	t.Helper()
	return newE2ERegionServerFunc(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Locations []struct {
				Lat     float64 `json:"lat"`
				Lon     float64 `json:"lon"`
				Heading *int    `json:"heading"`
			} `json:"locations"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("bad request body: %v", err)
		}
		first, last := body.Locations[0], body.Locations[len(body.Locations)-1]
		w.Header().Set("Content-Type", "application/json")
		if v.noRouteWithHeading && first.Heading != nil {
			_ = json.NewEncoder(w).Encode(&vhtypes.RouteResponse{Trip: vhtypes.Trip{Status: 171, StatusMessage: "No suitable edges near location"}})
			return
		}
		seconds, ok := v.seconds[last.Lat]
		if !ok {
			seconds = v.freshSeconds
		}
		resp := minimalVhResponse(nil, first.Lat, first.Lon, last.Lat, last.Lon)
		resp.Trip.Summary.Time = seconds
		resp.Trip.Legs[0].Summary.Time = seconds
		_ = json.NewEncoder(w).Encode(resp)
	})
}

func newE2ERegionServerFunc(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/route" {
			h(w, r)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func rejoinServer(t *testing.T, v rejoinValhalla) *RouterServer {
	srv := v.serve(t)
	cfg := newE2EValhallaConfig(t, map[string]*httptest.Server{"be": srv})
	fake := &fakeRegionQuerier{
		searchPointFn: func(ctx context.Context, token string, location regionclient.Coordinate, includeExtended bool) (regionclient.RegionList, error) {
			return regionclient.RegionList{CoreRegions: []string{"be"}}, nil
		},
		findRouteRegionPathsFn: func(ctx context.Context, token string, waypoints []regionclient.Coordinate, widthKm float64) ([][]string, error) {
			return [][]string{{"be"}}, nil
		},
	}
	return NewRouterServer(cfg, fake, log.New())
}

func rejoinRequest() *routerv1.RejoinRequest {
	heading := int32(90)
	return &routerv1.RejoinRequest{
		Fresh: &routerv1.RouteRequest{
			Mode: routerv1.RoutingMode_RM_MOTORSCOOTER,
			Locations: []*routerv1.RouteLocation{
				{Location: &pbgeo.Coordinate{Lat: 51.000, Lon: 4.000}, PreferredHeading: &heading},
				{Location: &pbgeo.Coordinate{Lat: 51.900, Lon: 4.900}},
			},
		},
		RejoinPoints: []*routerv1.RejoinPoint{
			{Location: &pbgeo.Coordinate{Lat: 51.100, Lon: 4.100}, Heading: 90, RemainingSeconds: 1000},
			{Location: &pbgeo.Coordinate{Lat: 51.200, Lon: 4.200}, Heading: 90, RemainingSeconds: 700},
		},
	}
}

func TestRejoin_PicksTheBestRejoinPoint(t *testing.T) {
	s := rejoinServer(t, rejoinValhalla{
		seconds:      map[float64]float64{51.1: 60, 51.2: 100},
		freshSeconds: 700,
	})

	resp, err := s.Rejoin(context.Background(), rejoinRequest())
	if err != nil {
		t.Fatalf("Rejoin error: %v", err)
	}
	if resp.Kind != routerv1.RejoinKind_RJ_REJOIN || resp.RejoinIndex != 1 {
		t.Errorf("got %v index %d, want a rejoin at index 1", resp.Kind, resp.RejoinIndex)
	}
	if resp.Trip == nil || resp.Trip.Summary.Time != 100 {
		t.Errorf("want the connector to point 1 (100 s), got %+v", resp.Trip)
	}
}

func TestRejoin_FreshWhenRejoiningIsMuchWorse(t *testing.T) {
	s := rejoinServer(t, rejoinValhalla{
		seconds:      map[float64]float64{51.1: 60, 51.2: 100},
		freshSeconds: 300,
	})

	resp, err := s.Rejoin(context.Background(), rejoinRequest())
	if err != nil {
		t.Fatalf("Rejoin error: %v", err)
	}
	if resp.Kind != routerv1.RejoinKind_RJ_FRESH {
		t.Errorf("got %v, want a fresh route", resp.Kind)
	}
	if resp.Trip.Summary.Time != 300 {
		t.Errorf("want the fresh route (300 s), got %v", resp.Trip.Summary.Time)
	}
}

func TestRejoin_FallsBackWithoutHeadingWhenNoRoadMatches(t *testing.T) {
	s := rejoinServer(t, rejoinValhalla{
		seconds:            map[float64]float64{51.1: 60, 51.2: 100},
		noRouteWithHeading: true,
		freshSeconds:       700,
	})

	resp, err := s.Rejoin(context.Background(), rejoinRequest())
	if err != nil {
		t.Fatalf("Rejoin error: %v", err)
	}
	if resp.Kind != routerv1.RejoinKind_RJ_REJOIN || resp.RejoinIndex != 1 {
		t.Errorf("got %v index %d, want a rejoin at index 1", resp.Kind, resp.RejoinIndex)
	}
}

func TestRejoin_InvalidRequest(t *testing.T) {
	s := rejoinServer(t, rejoinValhalla{freshSeconds: 700})
	_, err := s.Rejoin(context.Background(), &routerv1.RejoinRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("want InvalidArgument, got %v", err)
	}
}
