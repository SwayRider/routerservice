package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/swayrider/grpcclients/regionclient"
	pbgeo "github.com/swayrider/protos/common_types/geo"
	routerv1 "github.com/swayrider/protos/router/v1"
	"github.com/swayrider/routerservice/internal/valhalla"
	vhtypes "github.com/swayrider/routerservice/restclients/valhalla/types"
	log "github.com/swayrider/swlib/logger"
)

// newE2ERegionServer spins up an httptest.Server that answers both /locate
// (used by getRoadType during border-crossing stitching) and /route (used by
// the main routing loop) for one synthetic Valhalla region.
func newE2ERegionServer(t *testing.T, classification string, routeResp *vhtypes.RouteResponse) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/locate":
			_, _ = w.Write([]byte(`[{"edges":[{"edge":{"classification":{"classification":"` + classification + `"}}}]}]`))
		case "/route":
			_ = json.NewEncoder(w).Encode(routeResp)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newE2EValhallaConfig builds a *valhalla.Config pointing each region at its
// own httptest.Server, with a real (non-zero) RequestTimeout — Route()
// derives a context.WithTimeout from it for every Valhalla call, so a zero
// value would make every request fail immediately.
func newE2EValhallaConfig(t *testing.T, servers map[string]*httptest.Server) *valhalla.Config {
	t.Helper()
	hosts := make(map[string]string, len(servers))
	ports := make(map[string]int, len(servers))
	for region, srv := range servers {
		u, err := url.Parse(srv.URL)
		if err != nil {
			t.Fatalf("failed to parse test server URL: %v", err)
		}
		port, err := strconv.Atoi(u.Port())
		if err != nil {
			t.Fatalf("failed to parse test server port: %v", err)
		}
		hosts[region] = u.Hostname()
		ports[region] = port
	}
	return &valhalla.Config{ValhallaHosts: hosts, ValhallaPorts: ports, RequestTimeout: 5 * time.Second}
}

// TestRoute_TwoRegionHappyPath is a straightforward two-region route: one
// location in nl, one in fr, no transfer region, a valid border crossing.
func TestRoute_TwoRegionHappyPath(t *testing.T) {
	nlSrv := newE2ERegionServer(t, "motorway", minimalVhResponse(nil, 52.3, 4.9, 50.0, 3.5))
	frSrv := newE2ERegionServer(t, "motorway", minimalVhResponse(nil, 50.0, 3.5, 48.8, 2.3))
	valhallaConfig := newE2EValhallaConfig(t, map[string]*httptest.Server{"nl": nlSrv, "fr": frSrv})

	fake := &fakeRegionQuerier{
		searchPointFn: func(ctx context.Context, token string, location regionclient.Coordinate, includeExtended bool) (regionclient.RegionList, error) {
			if location.Latitude > 50 {
				return regionclient.RegionList{CoreRegions: []string{"nl"}}, nil
			}
			return regionclient.RegionList{CoreRegions: []string{"fr"}}, nil
		},
		findRouteRegionPathsFn: func(ctx context.Context, token string, waypoints []regionclient.Coordinate, widthKm float64) ([][]string, error) {
			return [][]string{{"nl", "fr"}}, nil
		},
		findCrossingLocationsFn: func(ctx context.Context, token, fromRegion, toRegion string, from, to regionclient.Coordinate, config regionclient.BorderCrossingConfig, limit int) ([]regionclient.BorderCrossing, error) {
			return []regionclient.BorderCrossing{
				{FromRegion: fromRegion, ToRegion: toRegion, RoadType: regionclient.RT_MOTORWAY,
					Location: regionclient.Coordinate{Latitude: 50.0, Longitude: 3.5}},
			}, nil
		},
	}

	s := NewRouterServer(valhallaConfig, fake, log.New())
	req := &routerv1.RouteRequest{
		Mode: routerv1.RoutingMode_RM_CAR,
		Locations: []*routerv1.RouteLocation{
			{Location: &pbgeo.Coordinate{Lat: 52.3, Lon: 4.9}},
			{Location: &pbgeo.Coordinate{Lat: 48.8, Lon: 2.3}},
		},
	}

	resp, err := s.Route(context.Background(), req)
	if err != nil {
		t.Fatalf("Route error: %v", err)
	}
	if resp == nil {
		t.Fatal("want non-nil response")
	}
	if resp.Trip.Status != 0 {
		t.Errorf("Trip.Status: want 0, got %d", resp.Trip.Status)
	}
	if resp.Summary == nil || resp.Summary.StartRegion != "nl" || resp.Summary.EndRegion != "fr" {
		t.Errorf("Summary: want start=nl end=fr, got %+v", resp.Summary)
	}
}

// TestRoute_TransferRegionHappyPath is a regression test for the historical
// CreateRoutingRequests panic on transfer-region assignments (bug #1): a
// route from nl to fr where the region corridor passes through be as a pure
// transfer region (never itself containing a waypoint) used to panic on
// routeLocations[-1]. This exercises the whole Route() path end to end and
// asserts it completes without panicking or erroring.
func TestRoute_TransferRegionHappyPath(t *testing.T) {
	nlSrv := newE2ERegionServer(t, "motorway", minimalVhResponse(nil, 52.3, 4.9, 51.0, 4.0))
	beSrv := newE2ERegionServer(t, "motorway", minimalVhResponse(nil, 51.0, 4.0, 50.0, 3.5))
	frSrv := newE2ERegionServer(t, "motorway", minimalVhResponse(nil, 50.0, 3.5, 48.8, 2.3))
	valhallaConfig := newE2EValhallaConfig(t, map[string]*httptest.Server{"nl": nlSrv, "be": beSrv, "fr": frSrv})

	fake := &fakeRegionQuerier{
		searchPointFn: func(ctx context.Context, token string, location regionclient.Coordinate, includeExtended bool) (regionclient.RegionList, error) {
			if location.Latitude > 50 {
				return regionclient.RegionList{CoreRegions: []string{"nl"}}, nil
			}
			return regionclient.RegionList{CoreRegions: []string{"fr"}}, nil
		},
		findRouteRegionPathsFn: func(ctx context.Context, token string, waypoints []regionclient.Coordinate, widthKm float64) ([][]string, error) {
			return [][]string{{"nl", "be", "fr"}}, nil
		},
		findCrossingLocationsFn: func(ctx context.Context, token, fromRegion, toRegion string, from, to regionclient.Coordinate, config regionclient.BorderCrossingConfig, limit int) ([]regionclient.BorderCrossing, error) {
			return []regionclient.BorderCrossing{
				{FromRegion: fromRegion, ToRegion: toRegion, RoadType: regionclient.RT_MOTORWAY,
					Location: regionclient.Coordinate{Latitude: 50.5, Longitude: 3.8}},
			}, nil
		},
	}

	s := NewRouterServer(valhallaConfig, fake, log.New())
	req := &routerv1.RouteRequest{
		Mode: routerv1.RoutingMode_RM_CAR,
		Locations: []*routerv1.RouteLocation{
			{Location: &pbgeo.Coordinate{Lat: 52.3, Lon: 4.9}},
			{Location: &pbgeo.Coordinate{Lat: 48.8, Lon: 2.3}},
		},
	}

	resp, err := s.Route(context.Background(), req)
	if err != nil {
		t.Fatalf("Route error (this used to panic before the fix): %v", err)
	}
	if resp == nil {
		t.Fatal("want non-nil response")
	}
	if resp.Summary == nil || resp.Summary.StartRegion != "nl" || resp.Summary.EndRegion != "fr" {
		t.Errorf("Summary: want start=nl end=fr, got %+v", resp.Summary)
	}
}

// TestRoute_EmptyBorderCrossingsReturnsNotFound is a regression test for the
// historical AddBorderCrossings panic on an empty crossing list (bug #2):
// regionservice's FindCrossingLocations can legally return an empty slice
// with a nil error, which used to panic on crossings[0]. Route() must now
// return a clean NotFound status instead.
func TestRoute_EmptyBorderCrossingsReturnsNotFound(t *testing.T) {
	nlSrv := newE2ERegionServer(t, "motorway", minimalVhResponse(nil, 52.3, 4.9, 50.0, 3.5))
	frSrv := newE2ERegionServer(t, "motorway", minimalVhResponse(nil, 50.0, 3.5, 48.8, 2.3))
	valhallaConfig := newE2EValhallaConfig(t, map[string]*httptest.Server{"nl": nlSrv, "fr": frSrv})

	fake := &fakeRegionQuerier{
		searchPointFn: func(ctx context.Context, token string, location regionclient.Coordinate, includeExtended bool) (regionclient.RegionList, error) {
			if location.Latitude > 50 {
				return regionclient.RegionList{CoreRegions: []string{"nl"}}, nil
			}
			return regionclient.RegionList{CoreRegions: []string{"fr"}}, nil
		},
		findRouteRegionPathsFn: func(ctx context.Context, token string, waypoints []regionclient.Coordinate, widthKm float64) ([][]string, error) {
			return [][]string{{"nl", "fr"}}, nil
		},
		findCrossingLocationsFn: func(ctx context.Context, token, fromRegion, toRegion string, from, to regionclient.Coordinate, config regionclient.BorderCrossingConfig, limit int) ([]regionclient.BorderCrossing, error) {
			return nil, nil // legal empty-without-error case
		},
	}

	s := NewRouterServer(valhallaConfig, fake, log.New())
	req := &routerv1.RouteRequest{
		Mode: routerv1.RoutingMode_RM_CAR,
		Locations: []*routerv1.RouteLocation{
			{Location: &pbgeo.Coordinate{Lat: 52.3, Lon: 4.9}},
			{Location: &pbgeo.Coordinate{Lat: 48.8, Lon: 2.3}},
		},
	}

	resp, err := s.Route(context.Background(), req)
	if resp != nil {
		t.Errorf("want nil response on error, got %v", resp)
	}
	if err == nil {
		t.Fatal("want an error (this used to panic on crossings[0] before the fix), got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("want a gRPC status error, got %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Errorf("status code: want NotFound, got %v", st.Code())
	}
}
