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

func routeWithTime(seconds float64) *routerv1.RouteResponse {
	return &routerv1.RouteResponse{Trip: &routerv1.Trip{
		Summary: &routerv1.Summary{Time: seconds},
	}}
}

func TestChooseReroute(t *testing.T) {
	cfg := DefaultRerouteConfig() // 300 s extra
	r := routeWithTime

	tests := []struct {
		name     string
		c        rerouteCandidates
		wantName string
		wantTime float64
		wantErr  bool
	}{
		{
			name:     "avoiding the closure and going forward wins when affordable",
			c:        rerouteCandidates{HE: r(200), E: r(150), H: r(180), C: r(140)},
			wantName: "HE", wantTime: 200,
		},
		{
			name:     "exactly at the limit is still acceptable",
			c:        rerouteCandidates{HE: r(440), C: r(140)},
			wantName: "HE", wantTime: 440,
		},
		{
			name:     "just over the limit is not",
			c:        rerouteCandidates{HE: r(441), E: r(150), C: r(140)},
			wantName: "E", wantTime: 150,
		},
		{
			name:     "a closure is avoided before a turn-back is",
			c:        rerouteCandidates{HE: r(900), E: r(300), H: r(200), C: r(140)},
			wantName: "E", wantTime: 300,
		},
		{
			name:     "forward only, when the exclusion route is too slow",
			c:        rerouteCandidates{HE: r(900), E: r(900), H: r(200), C: r(140)},
			wantName: "H", wantTime: 200,
		},
		{
			name:     "reversing is allowed when going forward costs more than five minutes",
			c:        rerouteCandidates{H: r(500), C: r(140)},
			wantName: "C", wantTime: 140,
		},
		{
			name:     "no constrained route: the plain one",
			c:        rerouteCandidates{C: r(140)},
			wantName: "C", wantTime: 140,
		},
		{
			name:     "the free route failed: the best constrained one",
			c:        rerouteCandidates{E: r(300), H: r(900)},
			wantName: "E", wantTime: 300,
		},
		{
			name:    "nothing at all is an error",
			c:       rerouteCandidates{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, resp, err := chooseReroute(cfg, tt.c)
			if tt.wantErr {
				if err == nil {
					t.Fatal("want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if name != tt.wantName || routeSeconds(resp) != tt.wantTime {
				t.Errorf("got %s (%.0f s), want %s (%.0f s)", name, routeSeconds(resp), tt.wantName, tt.wantTime)
			}
		})
	}
}

func TestChooseReroute_LimitIsConfigurable(t *testing.T) {
	cfg := RerouteConfig{MaxExtraSeconds: 60, MaxParallel: 1}
	name, _, err := chooseReroute(cfg, rerouteCandidates{H: routeWithTime(250), C: routeWithTime(140)})
	if err != nil || name != "C" {
		t.Errorf("a 110 s detour is too much with a 60 s limit: got %q, %v", name, err)
	}
}

// rerouteValhalla is a fake Valhalla region whose answer depends on whether
// the start carries a heading and whether the request has exclusions.
type rerouteValhalla struct {
	free            float64 // seconds without constraints
	extraForHeading float64
	extraForClosure float64
	noRouteWithBoth bool
	noRouteWithAny  bool // fails whenever there is a heading or exclusions
}

func (v rerouteValhalla) serve(t *testing.T) *httptest.Server {
	t.Helper()
	return newE2ERegionServerFunc(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Locations []struct {
				Lat     float64 `json:"lat"`
				Lon     float64 `json:"lon"`
				Heading *int    `json:"heading"`
			} `json:"locations"`
			ExcludePolygons [][][]float64 `json:"exclude_polygons"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("bad request body: %v", err)
		}
		first, last := body.Locations[0], body.Locations[len(body.Locations)-1]
		heading := first.Heading != nil
		closure := len(body.ExcludePolygons) > 0
		w.Header().Set("Content-Type", "application/json")
		if (v.noRouteWithAny && (heading || closure)) || (v.noRouteWithBoth && heading && closure) {
			_ = json.NewEncoder(w).Encode(&vhtypes.RouteResponse{Trip: vhtypes.Trip{Status: 171, StatusMessage: "No suitable edges near location"}})
			return
		}
		seconds := v.free
		if heading {
			seconds += v.extraForHeading
		}
		if closure {
			seconds += v.extraForClosure
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

func rerouteServer(t *testing.T, v rerouteValhalla) *RouterServer {
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

func rerouteRequest(heading, closure bool) *routerv1.RouteRequest {
	req := &routerv1.RouteRequest{
		Mode: routerv1.RoutingMode_RM_MOTORSCOOTER,
		Locations: []*routerv1.RouteLocation{
			{Location: &pbgeo.Coordinate{Lat: 51.000, Lon: 4.000}},
			{Location: &pbgeo.Coordinate{Lat: 51.900, Lon: 4.900}},
		},
	}
	if heading {
		h, tol := int32(90), int32(45)
		req.Locations[0].PreferredHeading = &h
		req.Locations[0].HeadingTolerance = &tol
	}
	if closure {
		req.ExcludePolygons = []*pbgeo.Polygon{{Points: []*pbgeo.Coordinate{
			{Lat: 51.0, Lon: 4.0}, {Lat: 51.0, Lon: 4.001}, {Lat: 51.001, Lon: 4.001},
		}}}
	}
	return req
}

func TestReroute_ForwardWhenAffordable(t *testing.T) {
	s := rerouteServer(t, rerouteValhalla{free: 140, extraForHeading: 74})

	resp, err := s.Reroute(context.Background(), rerouteRequest(true, false))
	if err != nil {
		t.Fatalf("Reroute error: %v", err)
	}
	if got := routeSeconds(resp); got != 214 {
		t.Errorf("want the route that keeps the heading (214 s), got %v s", got)
	}
}

func TestReroute_ReversesWhenForwardIsMuchSlower(t *testing.T) {
	s := rerouteServer(t, rerouteValhalla{free: 140, extraForHeading: 400})

	resp, err := s.Reroute(context.Background(), rerouteRequest(true, false))
	if err != nil {
		t.Fatalf("Reroute error: %v", err)
	}
	if got := routeSeconds(resp); got != 140 {
		t.Errorf("want the free route (140 s), got %v s", got)
	}
}

func TestReroute_AvoidsTheClosureAndKeepsTheHeading(t *testing.T) {
	s := rerouteServer(t, rerouteValhalla{free: 140, extraForHeading: 50, extraForClosure: 100})

	resp, err := s.Reroute(context.Background(), rerouteRequest(true, true))
	if err != nil {
		t.Fatalf("Reroute error: %v", err)
	}
	if got := routeSeconds(resp); got != 290 {
		t.Errorf("want heading and exclusion together (290 s), got %v s", got)
	}
}

func TestReroute_FallsBackWhenNoRouteExistsWithTheConstraints(t *testing.T) {
	s := rerouteServer(t, rerouteValhalla{free: 140, noRouteWithAny: true})

	resp, err := s.Reroute(context.Background(), rerouteRequest(true, true))
	if err != nil {
		t.Fatalf("Reroute error: %v", err)
	}
	if got := routeSeconds(resp); got != 140 {
		t.Errorf("want the free route (140 s), got %v s", got)
	}
}

func TestReroute_WithoutConstraintsIsAPlainRoute(t *testing.T) {
	s := rerouteServer(t, rerouteValhalla{free: 140, extraForHeading: 74})

	resp, err := s.Reroute(context.Background(), rerouteRequest(false, false))
	if err != nil {
		t.Fatalf("Reroute error: %v", err)
	}
	if got := routeSeconds(resp); got != 140 {
		t.Errorf("want 140 s, got %v s", got)
	}
}

func TestReroute_InvalidRequest(t *testing.T) {
	s := rerouteServer(t, rerouteValhalla{free: 140})
	_, err := s.Reroute(context.Background(), &routerv1.RouteRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("want InvalidArgument, got %v", err)
	}
}
