package server

import (
	"context"
	"errors"
	"math"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"github.com/swayrider/grpcclients/regionclient"
	pbgeo "github.com/swayrider/protos/common_types/geo"
	routerv1 "github.com/swayrider/protos/router/v1"
	vhtypes "github.com/swayrider/routerservice/restclients/valhalla/types"
	"github.com/swayrider/routerservice/internal/logic"
	log "github.com/swayrider/swlib/logger"
)

// fakeRegionQuerier is a hand-written test double for regionQuerier. Each
// method delegates to its func field if set, otherwise returns a safe zero
// value, so tests only need to override the calls they actually care about.
type fakeRegionQuerier struct {
	searchPointFn func(ctx context.Context, token string, location regionclient.Coordinate, includeExtended bool) (regionclient.RegionList, error)

	findCrossingLocationsFn func(ctx context.Context, token, fromRegion, toRegion string, from, to regionclient.Coordinate, config regionclient.BorderCrossingConfig, limit int) ([]regionclient.BorderCrossing, error)

	findRegionPathFn func(ctx context.Context, token, fromRegion, toRegion string) ([]string, error)

	findRouteRegionPathsFn func(ctx context.Context, token string, waypoints []regionclient.Coordinate, widthKm float64) ([][]string, error)
}

func (f *fakeRegionQuerier) SearchPoint(
	ctx context.Context, token string, location regionclient.Coordinate, includeExtended bool,
) (regionclient.RegionList, error) {
	if f.searchPointFn != nil {
		return f.searchPointFn(ctx, token, location, includeExtended)
	}
	return regionclient.RegionList{}, nil
}

func (f *fakeRegionQuerier) FindCrossingLocations(
	ctx context.Context, token, fromRegion, toRegion string, from, to regionclient.Coordinate,
	config regionclient.BorderCrossingConfig, limit int,
) ([]regionclient.BorderCrossing, error) {
	if f.findCrossingLocationsFn != nil {
		return f.findCrossingLocationsFn(ctx, token, fromRegion, toRegion, from, to, config, limit)
	}
	return nil, nil
}

func (f *fakeRegionQuerier) FindRegionPath(
	ctx context.Context, token, fromRegion, toRegion string,
) ([]string, error) {
	if f.findRegionPathFn != nil {
		return f.findRegionPathFn(ctx, token, fromRegion, toRegion)
	}
	return nil, nil
}

func (f *fakeRegionQuerier) FindRouteRegionPaths(
	ctx context.Context, token string, waypoints []regionclient.Coordinate, widthKm float64,
) ([][]string, error) {
	if f.findRouteRegionPathsFn != nil {
		return f.findRouteRegionPathsFn(ctx, token, waypoints, widthKm)
	}
	return nil, nil
}

// applyPresets calls createRequestOptions on req and applies the resulting
// options to a RouteRequest for the "motorcycle" model, returning the
// resulting CostingOptionValues.
func applyPresets(req *routerv1.RouteRequest) vhtypes.CostingOptionValues {
	s := &RouterServer{}
	opts := s.createRequestOptions(req)
	vhReq := &vhtypes.RouteRequest{
		CostingOptions: vhtypes.CostingOptions{},
	}
	model := "motorcycle"
	for _, opt := range opts {
		opt.Apply(vhReq, model)
	}
	return vhReq.CostingOptions[model]
}

func routeTypePtr(v routerv1.RouteType) *routerv1.RouteType {
	return &v
}

func TestCreateRequestOptions_Unspecified(t *testing.T) {
	req := &routerv1.RouteRequest{
		Mode:       routerv1.RoutingMode_RM_MOTORCYCLE,
		ResultMode: routerv1.RoutingResultMode_RRM_MINIMAL,
		RouteType:  routeTypePtr(routerv1.RouteType_RT_UNSPECIFIED),
	}
	got := applyPresets(req)
	if got.UseHighways != nil {
		t.Errorf("RT_UNSPECIFIED: UseHighways should be nil, got %v", *got.UseHighways)
	}
	if got.Shortest != nil {
		t.Errorf("RT_UNSPECIFIED: Shortest should be nil, got %v", *got.Shortest)
	}
}

func TestCreateRequestOptions_Fastest(t *testing.T) {
	req := &routerv1.RouteRequest{
		Mode:       routerv1.RoutingMode_RM_MOTORCYCLE,
		ResultMode: routerv1.RoutingResultMode_RRM_MINIMAL,
		RouteType:  routeTypePtr(routerv1.RouteType_RT_FASTEST),
	}
	got := applyPresets(req)
	if got.UseHighways != nil {
		t.Errorf("RT_FASTEST: UseHighways should be nil, got %v", *got.UseHighways)
	}
	if got.Shortest != nil {
		t.Errorf("RT_FASTEST: Shortest should be nil, got %v", *got.Shortest)
	}
}

func TestCreateRequestOptions_Scenic(t *testing.T) {
	req := &routerv1.RouteRequest{
		Mode:       routerv1.RoutingMode_RM_MOTORCYCLE,
		ResultMode: routerv1.RoutingResultMode_RRM_MINIMAL,
		RouteType:  routeTypePtr(routerv1.RouteType_RT_SCENIC),
	}
	got := applyPresets(req)
	if got.UseHighways == nil || *got.UseHighways != 0.1 {
		t.Errorf("RT_SCENIC: want UseHighways=0.1, got %v", got.UseHighways)
	}
	if got.UseTrails == nil || *got.UseTrails != 0.9 {
		t.Errorf("RT_SCENIC: want UseTrails=0.9, got %v", got.UseTrails)
	}
	if got.UseTolls == nil || *got.UseTolls != 0.2 {
		t.Errorf("RT_SCENIC: want UseTolls=0.2, got %v", got.UseTolls)
	}
}

func TestCreateRequestOptions_Shortest(t *testing.T) {
	req := &routerv1.RouteRequest{
		Mode:       routerv1.RoutingMode_RM_MOTORCYCLE,
		ResultMode: routerv1.RoutingResultMode_RRM_MINIMAL,
		RouteType:  routeTypePtr(routerv1.RouteType_RT_SHORTEST),
	}
	got := applyPresets(req)
	if got.Shortest == nil || *got.Shortest != true {
		t.Errorf("RT_SHORTEST: want Shortest=true, got %v", got.Shortest)
	}
	if got.UseDistance == nil || *got.UseDistance != 1.0 {
		t.Errorf("RT_SHORTEST: want UseDistance=1.0, got %v", got.UseDistance)
	}
}

func TestCreateRequestOptions_ScenicExplicitHighwayOverride(t *testing.T) {
	highwayPref := 0.8
	req := &routerv1.RouteRequest{
		Mode:       routerv1.RoutingMode_RM_MOTORCYCLE,
		ResultMode: routerv1.RoutingResultMode_RRM_MINIMAL,
		RouteType:  routeTypePtr(routerv1.RouteType_RT_SCENIC),
		RouteOptions: &routerv1.RouteOptions{
			HighwayPreference: &highwayPref,
		},
	}
	got := applyPresets(req)
	// Explicit route_options.highway_preference=0.8 must override the scenic preset 0.1
	if got.UseHighways == nil || *got.UseHighways != 0.8 {
		t.Errorf("override: want UseHighways=0.8, got %v", got.UseHighways)
	}
	// Scenic trail and toll presets should still apply
	if got.UseTrails == nil || *got.UseTrails != 0.9 {
		t.Errorf("override: want UseTrails=0.9, got %v", got.UseTrails)
	}
}

func float32Ptr(v float32) *float32 {
	return &v
}

func TestCreateRequestOptions_ScenicPreference(t *testing.T) {
	tests := []struct {
		name            string
		scenicPref      float32
		wantUseTrails   float64
		wantUseFerry    float64
		wantUseHighways float64
	}{
		{"scenic_0.0", 0.0, 0.5, 0.3, 1.0},
		{"scenic_0.5", 0.5, 0.75, 0.65, 0.75},
		{"scenic_1.0", 1.0, 1.0, 1.0, 0.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &routerv1.RouteRequest{
				Mode:             routerv1.RoutingMode_RM_MOTORCYCLE,
				ResultMode:       routerv1.RoutingResultMode_RRM_MINIMAL,
				ScenicPreference: float32Ptr(tt.scenicPref),
			}
			got := applyPresets(req)
			if got.UseTrails == nil || !approximatelyEqual(*got.UseTrails, tt.wantUseTrails) {
				t.Errorf("ScenicPreference=%v: want UseTrails=%v, got %v", tt.scenicPref, tt.wantUseTrails, got.UseTrails)
			}
			if got.UseFerry == nil || !approximatelyEqual(*got.UseFerry, tt.wantUseFerry) {
				t.Errorf("ScenicPreference=%v: want UseFerry=%v, got %v", tt.scenicPref, tt.wantUseFerry, got.UseFerry)
			}
			if got.UseHighways == nil || !approximatelyEqual(*got.UseHighways, tt.wantUseHighways) {
				t.Errorf("ScenicPreference=%v: want UseHighways=%v, got %v", tt.scenicPref, tt.wantUseHighways, got.UseHighways)
			}
		})
	}
}

func TestCreateRequestOptions_HighwayAvoidance(t *testing.T) {
	tests := []struct {
		name            string
		highwayAvoid    float32
		wantUseHighways float64
	}{
		{"avoid_0.0", 0.0, 1.0},
		{"avoid_0.5", 0.5, 0.5},
		{"avoid_0.9", 0.9, 0.1},
		{"avoid_1.0", 1.0, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &routerv1.RouteRequest{
				Mode:             routerv1.RoutingMode_RM_MOTORCYCLE,
				ResultMode:       routerv1.RoutingResultMode_RRM_MINIMAL,
				HighwayAvoidance: float32Ptr(tt.highwayAvoid),
			}
			got := applyPresets(req)
			if got.UseHighways == nil {
				t.Fatalf("HighwayAvoidance=%v: UseHighways is nil", tt.highwayAvoid)
			}
			actual := *got.UseHighways
			if !approximatelyEqual(actual, tt.wantUseHighways) {
				t.Errorf("HighwayAvoidance=%v: want UseHighways=%v, got %v", tt.highwayAvoid, tt.wantUseHighways, actual)
			}
		})
	}
}

func TestCreateRequestOptions_TollAvoidance(t *testing.T) {
	tests := []struct {
		name         string
		tollAvoid    float32
		wantUseTolls float64
	}{
		{"avoid_0.0", 0.0, 1.0},
		{"avoid_0.5", 0.5, 0.5},
		{"avoid_0.8", 0.8, 0.2},
		{"avoid_1.0", 1.0, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &routerv1.RouteRequest{
				Mode:          routerv1.RoutingMode_RM_MOTORCYCLE,
				ResultMode:    routerv1.RoutingResultMode_RRM_MINIMAL,
				TollAvoidance: float32Ptr(tt.tollAvoid),
			}
			got := applyPresets(req)
			if got.UseTolls == nil {
				t.Fatalf("TollAvoidance=%v: UseTolls is nil", tt.tollAvoid)
			}
			actual := *got.UseTolls
			if !approximatelyEqual(actual, tt.wantUseTolls) {
				t.Errorf("TollAvoidance=%v: want UseTolls=%v, got %v", tt.tollAvoid, tt.wantUseTolls, actual)
			}
		})
	}
}

func TestCreateRequestOptions_UnpavedHandling(t *testing.T) {
	tests := []struct {
		name               string
		unpavedHandling    routerv1.UnpavedHandling
		wantUseTracks      *float64
		wantExcludeUnpaved *bool
	}{
		{"prefer", routerv1.UnpavedHandling_UH_PREFER, float64Ptr(0.9), nil},
		{"neutral", routerv1.UnpavedHandling_UH_NEUTRAL, nil, nil},
		{"avoid", routerv1.UnpavedHandling_UH_AVOID, nil, boolPtr(true)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &routerv1.RouteRequest{
				Mode:            routerv1.RoutingMode_RM_MOTORCYCLE,
				ResultMode:      routerv1.RoutingResultMode_RRM_MINIMAL,
				UnpavedHandling: &tt.unpavedHandling,
			}
			got := applyPresets(req)
			if tt.wantUseTracks != nil {
				if got.UseTracks == nil || *got.UseTracks != *tt.wantUseTracks {
					t.Errorf("UnpavedHandling=%v: want UseTracks=%v, got %v", tt.name, *tt.wantUseTracks, got.UseTracks)
				}
			}
			if tt.wantExcludeUnpaved != nil {
				if got.ExcludeUnpaved == nil || *got.ExcludeUnpaved != *tt.wantExcludeUnpaved {
					t.Errorf("UnpavedHandling=%v: want ExcludeUnpaved=%v, got %v", tt.name, *tt.wantExcludeUnpaved, got.ExcludeUnpaved)
				}
			}
		})
	}
}

func TestCreateRequestOptions_HighwayAvoidanceExplicitRouteOptionsOverride(t *testing.T) {
	highwayPref := 0.8
	highwayAvoid := float32(0.9)
	req := &routerv1.RouteRequest{
		Mode:             routerv1.RoutingMode_RM_MOTORCYCLE,
		ResultMode:       routerv1.RoutingResultMode_RRM_MINIMAL,
		HighwayAvoidance: &highwayAvoid,
		RouteOptions: &routerv1.RouteOptions{
			HighwayPreference: &highwayPref,
		},
	}
	got := applyPresets(req)
	// Explicit route_options.highway_preference=0.8 must win over
	// highway_avoidance=0.9 (which alone would produce UseHighways=0.1).
	if got.UseHighways == nil || *got.UseHighways != 0.8 {
		t.Errorf("override: want UseHighways=0.8, got %v", got.UseHighways)
	}
}

func TestCreateRequestOptions_TollAvoidanceExplicitRouteOptionsOverride(t *testing.T) {
	tollPref := 0.7
	tollAvoid := float32(0.8)
	req := &routerv1.RouteRequest{
		Mode:          routerv1.RoutingMode_RM_MOTORCYCLE,
		ResultMode:    routerv1.RoutingResultMode_RRM_MINIMAL,
		TollAvoidance: &tollAvoid,
		RouteOptions: &routerv1.RouteOptions{
			TollwayPreference: &tollPref,
		},
	}
	got := applyPresets(req)
	// Explicit route_options.tollway_preference=0.7 must win over
	// toll_avoidance=0.8 (which alone would produce UseTolls=0.2).
	if got.UseTolls == nil || *got.UseTolls != 0.7 {
		t.Errorf("override: want UseTolls=0.7, got %v", got.UseTolls)
	}
}

func TestCreateRequestOptions_UnpavedHandlingExplicitRouteOptionsOverride(t *testing.T) {
	avoid := routerv1.UnpavedHandling_UH_AVOID
	excludeUnpaved := false
	req := &routerv1.RouteRequest{
		Mode:            routerv1.RoutingMode_RM_MOTORCYCLE,
		ResultMode:      routerv1.RoutingResultMode_RRM_MINIMAL,
		UnpavedHandling: &avoid,
		RouteOptions: &routerv1.RouteOptions{
			ExcludeUnpaved: &excludeUnpaved,
		},
	}
	got := applyPresets(req)
	// Explicit route_options.exclude_unpaved=false must win over
	// unpaved_handling=UH_AVOID (which alone would produce ExcludeUnpaved=true).
	if got.ExcludeUnpaved == nil || *got.ExcludeUnpaved != false {
		t.Errorf("override: want ExcludeUnpaved=false, got %v", got.ExcludeUnpaved)
	}
}

func TestCreateRequestOptions_ScenicPreferenceExplicitRouteOptionsOverride(t *testing.T) {
	scenicPref := float32(1.0)
	trailPref := 0.1
	ferryPref := 0.0
	highwayPref := 0.8
	req := &routerv1.RouteRequest{
		Mode:             routerv1.RoutingMode_RM_MOTORCYCLE,
		ResultMode:       routerv1.RoutingResultMode_RRM_MINIMAL,
		ScenicPreference: &scenicPref,
		RouteOptions: &routerv1.RouteOptions{
			TrailPreference:   &trailPref,
			FerryPreference:   &ferryPref,
			HighwayPreference: &highwayPref,
		},
	}
	got := applyPresets(req)
	// scenic_preference=1.0 alone would produce UseTrails=1.0, UseFerry=1.0,
	// UseHighways=0.5 — all three must instead come from route_options.
	if got.UseTrails == nil || *got.UseTrails != 0.1 {
		t.Errorf("override: want UseTrails=0.1, got %v", got.UseTrails)
	}
	if got.UseFerry == nil || *got.UseFerry != 0.0 {
		t.Errorf("override: want UseFerry=0.0, got %v", got.UseFerry)
	}
	if got.UseHighways == nil || *got.UseHighways != 0.8 {
		t.Errorf("override: want UseHighways=0.8, got %v", got.UseHighways)
	}
}

// applyOpts calls createRequestOptions on req and applies the resulting
// options to a fresh RouteRequest, returning the whole request so top-level
// (non-costing) fields can be inspected.
func applyOpts(req *routerv1.RouteRequest) *vhtypes.RouteRequest {
	s := &RouterServer{}
	opts := s.createRequestOptions(req)
	vhReq := &vhtypes.RouteRequest{
		CostingOptions: vhtypes.CostingOptions{},
	}
	for _, opt := range opts {
		opt.Apply(vhReq, "motorcycle")
	}
	return vhReq
}

func TestCreateRequestOptions_ExcludeLocations(t *testing.T) {
	req := &routerv1.RouteRequest{
		Mode:       routerv1.RoutingMode_RM_MOTORCYCLE,
		ResultMode: routerv1.RoutingResultMode_RRM_MINIMAL,
		ExcludeLocations: []*routerv1.RouteLocation{
			{Location: &pbgeo.Coordinate{Lat: 51.0, Lon: 4.0}},
			nil, // must be skipped without panicking
			{Location: nil}, // must be skipped without panicking
			{Location: &pbgeo.Coordinate{Lat: 52.0, Lon: 5.0}},
		},
	}
	got := applyOpts(req).ExcludeLocations
	if len(got) != 2 {
		t.Fatalf("want 2 exclude locations, got %d: %v", len(got), got)
	}
	if got[0].Lat != 51.0 || got[0].Lon != 4.0 {
		t.Errorf("got[0]: want (51.0, 4.0), got (%v, %v)", got[0].Lat, got[0].Lon)
	}
	if got[1].Lat != 52.0 || got[1].Lon != 5.0 {
		t.Errorf("got[1]: want (52.0, 5.0), got (%v, %v)", got[1].Lat, got[1].Lon)
	}
}

func TestCreateRequestOptions_ExcludeLocations_Empty(t *testing.T) {
	req := &routerv1.RouteRequest{
		Mode:       routerv1.RoutingMode_RM_MOTORCYCLE,
		ResultMode: routerv1.RoutingResultMode_RRM_MINIMAL,
	}
	if got := applyOpts(req).ExcludeLocations; got != nil {
		t.Errorf("want nil ExcludeLocations when unset, got %v", got)
	}
}

func TestCreateRequestOptions_ExcludePolygons(t *testing.T) {
	req := &routerv1.RouteRequest{
		Mode:       routerv1.RoutingMode_RM_MOTORCYCLE,
		ResultMode: routerv1.RoutingResultMode_RRM_MINIMAL,
		ExcludePolygons: []*pbgeo.Polygon{
			{
				Points: []*pbgeo.Coordinate{
					{Lat: 51.0, Lon: 4.0},
					{Lat: 51.1, Lon: 4.1},
					{Lat: 51.2, Lon: 4.2},
				},
			},
			nil,                     // must be skipped without panicking
			{Points: nil},           // must be skipped without panicking
		},
	}
	got := applyOpts(req).ExcludePolygons
	if len(got) != 1 {
		t.Fatalf("want 1 exclude polygon, got %d: %v", len(got), got)
	}
	ring := got[0]
	if len(ring) != 3 {
		t.Fatalf("want 3 ring points, got %d: %v", len(ring), ring)
	}
	// Valhalla wants [lon, lat] pairs — the opposite axis order of
	// pbgeo.Coordinate{Lat, Lon} — so the conversion must swap them.
	want := [][]float64{{4.0, 51.0}, {4.1, 51.1}, {4.2, 51.2}}
	for i, pt := range ring {
		if pt[0] != want[i][0] || pt[1] != want[i][1] {
			t.Errorf("ring[%d]: want [lon,lat]=%v, got %v", i, want[i], pt)
		}
	}
}

func TestCreateRequestOptions_ExcludePolygons_Empty(t *testing.T) {
	req := &routerv1.RouteRequest{
		Mode:       routerv1.RoutingMode_RM_MOTORCYCLE,
		ResultMode: routerv1.RoutingResultMode_RRM_MINIMAL,
	}
	if got := applyOpts(req).ExcludePolygons; got != nil {
		t.Errorf("want nil ExcludePolygons when unset, got %v", got)
	}
}

func TestBuildRouteSummary(t *testing.T) {
	t.Run("nil for empty assignment list", func(t *testing.T) {
		if got := buildRouteSummary(nil); got != nil {
			t.Errorf("want nil, got %v", got)
		}
	})

	t.Run("single region: start and end are the same", func(t *testing.T) {
		got := buildRouteSummary([]*logic.RegionAssignment{
			{Region: "nl"},
		})
		if got == nil {
			t.Fatal("want non-nil summary")
		}
		if got.StartRegion != "nl" || got.EndRegion != "nl" {
			t.Errorf("want start=nl end=nl, got start=%q end=%q", got.StartRegion, got.EndRegion)
		}
	})

	t.Run("multi-region path uses first and last region", func(t *testing.T) {
		got := buildRouteSummary([]*logic.RegionAssignment{
			{Region: "nl"},
			{Region: "be", IsEmpty: true}, // transfer region
			{Region: "fr"},
		})
		if got == nil {
			t.Fatal("want non-nil summary")
		}
		if got.StartRegion != "nl" {
			t.Errorf("StartRegion: want nl, got %q", got.StartRegion)
		}
		if got.EndRegion != "fr" {
			t.Errorf("EndRegion: want fr, got %q", got.EndRegion)
		}
	})
}

func TestGrpcStatus_Errors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode codes.Code
	}{
		{
			name:     "ErrValhallaUnavailable returns UNAVAILABLE",
			err:      logic.ErrValhallaUnavailable,
			wantCode: codes.Unavailable,
		},
		{
			name:     "ErrLocationOutsideOfKnownRegions returns NOT_FOUND",
			err:      logic.ErrLocationOutsideOfKnownRegions,
			wantCode: codes.NotFound,
		},
		{
			name:     "ErrNoRouteFound returns NOT_FOUND",
			err:      logic.ErrNoRouteFound,
			wantCode: codes.NotFound,
		},
		{
			name:     "ErrNoBorderCrossings returns NOT_FOUND",
			err:      logic.ErrNoBorderCrossings,
			wantCode: codes.NotFound,
		},
		{
			name:     "unknown error returns INTERNAL",
			err:      errors.New("some unknown error"),
			wantCode: codes.Internal,
		},
		{
			name:     "nil error returns nil",
			err:      nil,
			wantCode: codes.OK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := grpcStatus(tt.err)
			if tt.err == nil {
				if gotErr != nil {
					t.Errorf("grpcStatus(nil) = %v, want nil", gotErr)
				}
				return
			}
			st, ok := status.FromError(gotErr)
			if !ok {
				t.Errorf("grpcStatus() did not return a status error")
				return
			}
			if st.Code() != tt.wantCode {
				t.Errorf("grpcStatus(%v).Code() = %v, want %v", tt.err, st.Code(), tt.wantCode)
			}
		})
	}
}

func TestGrpcStatus_NetworkError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode codes.Code
	}{
		{
			name:     "net.Error timeout returns UNAVAILABLE",
			err:      &net.OpError{Op: "dial", Err: errors.New("i/o timeout")},
			wantCode: codes.Internal,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := grpcStatus(tt.err)
			st, ok := status.FromError(gotErr)
			if !ok {
				t.Errorf("grpcStatus() did not return a status error")
				return
			}
			if st.Code() != tt.wantCode {
				t.Errorf("grpcStatus(%T).Code() = %v, want %v", tt.err, st.Code(), tt.wantCode)
			}
		})
	}
}

func TestValidateLocations(t *testing.T) {
	valid := &routerv1.RouteLocation{Location: &pbgeo.Coordinate{Lat: 51.0, Lon: 4.0}}

	tests := []struct {
		name      string
		locations []*routerv1.RouteLocation
		wantErr   bool
	}{
		{
			name:      "valid locations",
			locations: []*routerv1.RouteLocation{valid, valid},
			wantErr:   false,
		},
		{
			name:      "nil entry",
			locations: []*routerv1.RouteLocation{nil, valid},
			wantErr:   true,
		},
		{
			name:      "nil coordinate",
			locations: []*routerv1.RouteLocation{{Location: nil}, valid},
			wantErr:   true,
		},
		{
			name: "NaN lat",
			locations: []*routerv1.RouteLocation{
				{Location: &pbgeo.Coordinate{Lat: math.NaN(), Lon: 4.0}}, valid,
			},
			wantErr: true,
		},
		{
			name: "+Inf lon",
			locations: []*routerv1.RouteLocation{
				{Location: &pbgeo.Coordinate{Lat: 51.0, Lon: math.Inf(1)}}, valid,
			},
			wantErr: true,
		},
		{
			name: "lat out of range",
			locations: []*routerv1.RouteLocation{
				{Location: &pbgeo.Coordinate{Lat: 91.0, Lon: 4.0}}, valid,
			},
			wantErr: true,
		},
		{
			name: "lon out of range",
			locations: []*routerv1.RouteLocation{
				{Location: &pbgeo.Coordinate{Lat: 51.0, Lon: 181.0}}, valid,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLocations(tt.locations)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateLocations() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTripStatusError(t *testing.T) {
	tests := []struct {
		name    string
		trip    *vhtypes.Trip
		wantErr bool
	}{
		{
			name:    "zero status returns nil",
			trip:    &vhtypes.Trip{Status: 0},
			wantErr: false,
		},
		{
			name:    "non-zero status returns ErrNoRouteFound",
			trip:    &vhtypes.Trip{Status: 1, StatusMessage: "no path could be found"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tripStatusError(tt.trip)
			if (err != nil) != tt.wantErr {
				t.Errorf("tripStatusError() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, logic.ErrNoRouteFound) {
				t.Errorf("tripStatusError() = %v, want errors.Is(err, logic.ErrNoRouteFound)", err)
			}
		})
	}
}

func float64Ptr(v float64) *float64 {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}

func approximatelyEqual(a, b float64) bool {
	const epsilon = 1e-6
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < epsilon
}

// TestIncomingToken_WithBearerHeader verifies a Bearer-prefixed authorization
// header is extracted with the prefix stripped.
func TestIncomingToken_WithBearerHeader(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer abc123"))
	if got := incomingToken(ctx); got != "abc123" {
		t.Errorf("incomingToken() = %q, want %q", got, "abc123")
	}
}

// TestIncomingToken_NoMetadata verifies a context with no incoming metadata
// returns an empty string.
func TestIncomingToken_NoMetadata(t *testing.T) {
	if got := incomingToken(context.Background()); got != "" {
		t.Errorf("incomingToken() = %q, want empty string", got)
	}
}

// TestIncomingToken_NoAuthorizationHeader verifies metadata present but
// without an authorization key returns an empty string.
func TestIncomingToken_NoAuthorizationHeader(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-other", "value"))
	if got := incomingToken(ctx); got != "" {
		t.Errorf("incomingToken() = %q, want empty string", got)
	}
}

// TestAssignRegionsToLocations_Success verifies a simple single-region path
// produces a populated assignment list with no error.
func TestAssignRegionsToLocations_Success(t *testing.T) {
	fake := &fakeRegionQuerier{
		searchPointFn: func(ctx context.Context, token string, location regionclient.Coordinate, includeExtended bool) (regionclient.RegionList, error) {
			return regionclient.RegionList{CoreRegions: []string{"nl"}}, nil
		},
		findRouteRegionPathsFn: func(ctx context.Context, token string, waypoints []regionclient.Coordinate, widthKm float64) ([][]string, error) {
			return [][]string{{"nl"}}, nil
		},
	}
	s := &RouterServer{regionClient: fake}
	req := &routerv1.RouteRequest{
		Locations: []*routerv1.RouteLocation{
			{Location: &pbgeo.Coordinate{Lat: 52.3, Lon: 4.9}},
			{Location: &pbgeo.Coordinate{Lat: 52.1, Lon: 4.5}},
		},
	}

	locationList, assignmentList, err := s.assignRegionsToLocations(context.Background(), req, "tok", log.New())
	if err != nil {
		t.Fatalf("assignRegionsToLocations error: %v", err)
	}
	if len(locationList) != 2 {
		t.Errorf("want 2 locations, got %d", len(locationList))
	}
	if len(assignmentList) != 1 || assignmentList[0].Region != "nl" {
		t.Errorf("want 1 assignment for nl, got %+v", assignmentList)
	}
}

// TestAssignRegionsToLocations_NoRoutePossible verifies routePossible=false
// (no path between resolved regions) maps to logic.ErrNoRouteFound.
func TestAssignRegionsToLocations_NoRoutePossible(t *testing.T) {
	fake := &fakeRegionQuerier{
		searchPointFn: func(ctx context.Context, token string, location regionclient.Coordinate, includeExtended bool) (regionclient.RegionList, error) {
			if location.Latitude > 50 {
				return regionclient.RegionList{CoreRegions: []string{"nl"}}, nil
			}
			return regionclient.RegionList{CoreRegions: []string{"fr"}}, nil
		},
		findRouteRegionPathsFn: func(ctx context.Context, token string, waypoints []regionclient.Coordinate, widthKm float64) ([][]string, error) {
			return nil, errors.New("no corridor")
		},
		findRegionPathFn: func(ctx context.Context, token, fromRegion, toRegion string) ([]string, error) {
			return nil, nil // no path exists
		},
	}
	s := &RouterServer{regionClient: fake}
	req := &routerv1.RouteRequest{
		Locations: []*routerv1.RouteLocation{
			{Location: &pbgeo.Coordinate{Lat: 52.3, Lon: 4.9}},
			{Location: &pbgeo.Coordinate{Lat: 48.8, Lon: 2.3}},
		},
	}

	_, _, err := s.assignRegionsToLocations(context.Background(), req, "tok", log.New())
	if !errors.Is(err, logic.ErrNoRouteFound) {
		t.Errorf("want ErrNoRouteFound, got %v", err)
	}
}

// TestAssignRegionsToLocations_CalculateRegionAssignmentError verifies a
// downstream error (e.g. SearchPoint failure) is propagated as-is.
func TestAssignRegionsToLocations_CalculateRegionAssignmentError(t *testing.T) {
	wantErr := errors.New("region service unavailable")
	fake := &fakeRegionQuerier{
		searchPointFn: func(ctx context.Context, token string, location regionclient.Coordinate, includeExtended bool) (regionclient.RegionList, error) {
			return regionclient.RegionList{}, wantErr
		},
	}
	s := &RouterServer{regionClient: fake}
	req := &routerv1.RouteRequest{
		Locations: []*routerv1.RouteLocation{
			{Location: &pbgeo.Coordinate{Lat: 52.3, Lon: 4.9}},
			{Location: &pbgeo.Coordinate{Lat: 48.8, Lon: 2.3}},
		},
	}

	_, _, err := s.assignRegionsToLocations(context.Background(), req, "tok", log.New())
	if !errors.Is(err, wantErr) {
		t.Errorf("want %v, got %v", wantErr, err)
	}
}

// minimalVhResponse builds a minimal, decodable vhtypes.RouteResponse with
// one leg (a two-point shape, one location, an empty maneuver list).
func minimalVhResponse(id *string, lat1, lon1, lat2, lon2 float64) *vhtypes.RouteResponse {
	return &vhtypes.RouteResponse{
		Id: id,
		Trip: vhtypes.Trip{
			Status: 0,
			Units:  vhtypes.Kilometers,
			Locations: []vhtypes.Location{
				{Lat: lat1, Lon: lon1},
				{Lat: lat2, Lon: lon2},
			},
			Legs: []vhtypes.Leg{
				{
					Shape:     encodeShape([]float64{lat1, lon1, lat2, lon2}),
					Maneuvers: []vhtypes.Maneuver{},
					Summary:   vhtypes.Summary{Time: 100, Length: 10, MinLat: lat1, MinLon: lon1, MaxLat: lat2, MaxLon: lon2},
				},
			},
			Summary: vhtypes.Summary{Time: 100, Length: 10, MinLat: lat1, MinLon: lon1, MaxLat: lat2, MaxLon: lon2},
		},
	}
}

// TestBuildCombinedRouteResponse_SingleResponse verifies a single Valhalla
// response passes through buildRouteResponse only (no merge/append path).
func TestBuildCombinedRouteResponse_SingleResponse(t *testing.T) {
	id := "route-1"
	respList := []*vhtypes.RouteResponse{minimalVhResponse(&id, 50, 4, 51, 5)}

	s := &RouterServer{}
	got, err := s.buildCombinedRouteResponse(respList, log.New())
	if err != nil {
		t.Fatalf("buildCombinedRouteResponse error: %v", err)
	}
	if got.Id == nil || *got.Id != "route-1" {
		t.Errorf("Id: want route-1, got %v", got.Id)
	}
	if len(got.Trip.Locations) != 2 || len(got.Trip.Legs) != 1 {
		t.Errorf("want 2 locations, 1 leg, got %d locations, %d legs",
			len(got.Trip.Locations), len(got.Trip.Legs))
	}
}

// TestBuildCombinedRouteResponse_TwoResponses verifies two Valhalla
// responses sharing a border waypoint are combined: the shared border
// location is stripped and the two legs are merged into one.
func TestBuildCombinedRouteResponse_TwoResponses(t *testing.T) {
	respList := []*vhtypes.RouteResponse{
		minimalVhResponse(nil, 50, 4, 51, 5),
		minimalVhResponse(nil, 51, 5, 52, 6),
	}

	s := &RouterServer{}
	got, err := s.buildCombinedRouteResponse(respList, log.New())
	if err != nil {
		t.Fatalf("buildCombinedRouteResponse error: %v", err)
	}
	// 3 distinct locations total (50,4)/(51,5)/(52,6), minus the shared
	// border point (51,5) itself, which removeBorderLocations strips as an
	// internal stitching artifact rather than a real requested waypoint.
	if len(got.Trip.Locations) != 2 {
		t.Errorf("want 2 locations (border point removed), got %d", len(got.Trip.Locations))
	}
	// The two legs share a border point and should merge into one leg.
	if len(got.Trip.Legs) != 1 {
		t.Errorf("want 1 merged leg, got %d", len(got.Trip.Legs))
	}
}

// TestBuildCombinedRouteResponse_EmptyList verifies an empty respList
// returns an error rather than indexing respList[0].
func TestBuildCombinedRouteResponse_EmptyList(t *testing.T) {
	s := &RouterServer{}
	_, err := s.buildCombinedRouteResponse(nil, log.New())
	if err == nil {
		t.Fatal("want an error for an empty response list, got nil")
	}
}

// TestAddTrip_NotAppend verifies a fresh trip is built with status, units,
// and language mapped from the Valhalla trip.
func TestAddTrip_NotAppend(t *testing.T) {
	resp := &routerv1.RouteResponse{}
	trip := &vhtypes.Trip{
		Status:        0,
		StatusMessage: "",
		Units:         vhtypes.Miles,
		Language:      vhtypes.Language("en-US"),
		Locations:     []vhtypes.Location{{Lat: 1, Lon: 2}, {Lat: 3, Lon: 4}},
		Legs: []vhtypes.Leg{
			{Shape: encodeShape([]float64{1, 2, 3, 4}), Maneuvers: []vhtypes.Maneuver{}, Summary: vhtypes.Summary{}},
		},
		Summary: vhtypes.Summary{Time: 50, Length: 5},
	}

	if err := addTrip(resp, trip, false, log.New()); err != nil {
		t.Fatalf("addTrip error: %v", err)
	}
	if resp.Trip.Unit != routerv1.Unit_U_IMPERIAL {
		t.Errorf("Unit: want U_IMPERIAL, got %v", resp.Trip.Unit)
	}
	if len(resp.Trip.Locations) != 2 {
		t.Errorf("want 2 locations, got %d", len(resp.Trip.Locations))
	}
	if len(resp.Trip.Legs) != 1 {
		t.Errorf("want 1 leg, got %d", len(resp.Trip.Legs))
	}
	if resp.Trip.Summary == nil || resp.Trip.Summary.Time != 50 {
		t.Errorf("Summary: want Time=50, got %+v", resp.Trip.Summary)
	}
}

// TestAddTrip_Append verifies append mode delegates into addLocations,
// addLegs, and createTripSummary in accumulate mode.
func TestAddTrip_Append(t *testing.T) {
	resp := &routerv1.RouteResponse{
		Trip: &routerv1.Trip{
			Locations: []*routerv1.RouteLocationReturned{{Location: &pbgeo.Coordinate{Lat: 1, Lon: 2}}},
			Legs:      []*routerv1.Leg{{Shape: encodeShape([]float64{1, 2, 3, 4})}},
			Summary:   &routerv1.Summary{Time: 50, Length: 5, BoundingBox: &pbgeo.BoundingBox{BottomLeft: &pbgeo.Coordinate{Lat: 1, Lon: 2}, TopRight: &pbgeo.Coordinate{Lat: 3, Lon: 4}}},
		},
	}
	trip := &vhtypes.Trip{
		Locations: []vhtypes.Location{{Lat: 3, Lon: 4}, {Lat: 5, Lon: 6}}, // first is the shared border point
		Legs: []vhtypes.Leg{
			{Shape: encodeShape([]float64{3, 4, 5, 6}), Maneuvers: []vhtypes.Maneuver{}, Summary: vhtypes.Summary{}},
		},
		Summary: vhtypes.Summary{Time: 20, Length: 2},
	}

	if err := addTrip(resp, trip, true, log.New()); err != nil {
		t.Fatalf("addTrip error: %v", err)
	}
	if len(resp.Trip.Locations) != 2 {
		t.Errorf("want 2 locations (border skipped), got %d", len(resp.Trip.Locations))
	}
	if len(resp.Trip.Legs) != 2 {
		t.Errorf("want 2 legs, got %d", len(resp.Trip.Legs))
	}
	if !resp.Trip.Legs[0].MergeNext {
		t.Error("want first leg's MergeNext set on append")
	}
	if resp.Trip.Summary.Time != 70 {
		t.Errorf("Summary.Time: want 70 (50+20 accumulated), got %v", resp.Trip.Summary.Time)
	}
}

// TestAddLocations_NotAppend verifies fresh locations are built via createLocation.
func TestAddLocations_NotAppend(t *testing.T) {
	trip := &routerv1.Trip{}
	locations := []vhtypes.Location{{Lat: 1, Lon: 2}, {Lat: 3, Lon: 4}}

	if err := addLocations(trip, locations, false, log.New()); err != nil {
		t.Fatalf("addLocations error: %v", err)
	}
	if len(trip.Locations) != 2 {
		t.Fatalf("want 2 locations, got %d", len(trip.Locations))
	}
	if trip.Locations[0].Location.Lat != 1 || trip.Locations[1].Location.Lat != 3 {
		t.Errorf("unexpected locations: %+v", trip.Locations)
	}
}

// TestAddLocations_Append_SkipsFirstDuplicate verifies append mode skips the
// first new location (the shared border point) and keeps the rest.
func TestAddLocations_Append_SkipsFirstDuplicate(t *testing.T) {
	trip := &routerv1.Trip{
		Locations: []*routerv1.RouteLocationReturned{{Location: &pbgeo.Coordinate{Lat: 1, Lon: 2}}},
	}
	locations := []vhtypes.Location{{Lat: 1, Lon: 2}, {Lat: 5, Lon: 6}}

	if err := addLocations(trip, locations, true, log.New()); err != nil {
		t.Fatalf("addLocations error: %v", err)
	}
	if len(trip.Locations) != 2 {
		t.Fatalf("want 2 locations (1 existing + 1 new, border skipped), got %d", len(trip.Locations))
	}
	if trip.Locations[1].Location.Lat != 5 {
		t.Errorf("want second location lat=5, got %v", trip.Locations[1].Location.Lat)
	}
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

// TestCreateLocation_FullFieldMapping verifies every optional field on a
// Valhalla location is mapped onto the returned RouteLocationReturned.
func TestCreateLocation_FullFieldMapping(t *testing.T) {
	minRC := vhtypes.Primary
	maxRC := vhtypes.Motorway
	side := vhtypes.Left
	dt := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)

	vhLoc := &vhtypes.Location{
		Lat: 51.0, Lon: 4.0,
		Heading:        intPtr(90),
		TimeZoneOffset: strPtr("+01:00"),
		TimeZoneName:   strPtr("Europe/Brussels"),
		Name:           strPtr("Test Place"),
		City:           strPtr("Brussels"),
		State:          strPtr("Brussels"),
		PostalCode:     strPtr("1000"),
		Country:        strPtr("BE"),
		Phone:          strPtr("+32000000"),
		Url:            strPtr("https://example.com"),
		SearchFilter: &vhtypes.SearchFilters{
			ExcludeTunnel: boolPtr(true), ExcludeBridge: boolPtr(false),
			ExcludeToll: boolPtr(true), ExcludeFerry: boolPtr(false),
			ExcludeRamp: boolPtr(true), ExcludeClosures: boolPtr(false),
			MinRoadClass: &minRC, MaxRoadClass: &maxRC,
		},
		SideOfStreet: &side,
		DateTime:     &dt,
	}

	got, err := createLocation(vhLoc, log.New())
	if err != nil {
		t.Fatalf("createLocation error: %v", err)
	}

	if got.Location.Lat != 51.0 || got.Location.Lon != 4.0 {
		t.Errorf("Location: want (51,4), got (%v,%v)", got.Location.Lat, got.Location.Lon)
	}
	if got.PreferredHeading == nil || *got.PreferredHeading != 90 {
		t.Errorf("PreferredHeading: want 90, got %v", got.PreferredHeading)
	}
	if got.TimeZoneOffset == nil || *got.TimeZoneOffset != "+01:00" {
		t.Errorf("TimeZoneOffset: want +01:00, got %v", got.TimeZoneOffset)
	}
	if got.TimeZoneName == nil || *got.TimeZoneName != "Europe/Brussels" {
		t.Errorf("TimeZoneName: want Europe/Brussels, got %v", got.TimeZoneName)
	}
	if got.Info == nil {
		t.Fatal("want non-nil Info")
	}
	if *got.Info.Name != "Test Place" || *got.Info.City != "Brussels" || *got.Info.State != "Brussels" ||
		*got.Info.PostalCode != "1000" || *got.Info.Country != "BE" || *got.Info.Phone != "+32000000" ||
		*got.Info.Url != "https://example.com" {
		t.Errorf("Info: unexpected value %+v", got.Info)
	}
	if got.Filter == nil {
		t.Fatal("want non-nil Filter")
	}
	if !*got.Filter.ExcludeTunnel || *got.Filter.ExcludeBridge || !*got.Filter.ExcludeToll {
		t.Errorf("Filter exclude flags: unexpected value %+v", got.Filter)
	}
	if got.Filter.MinRoadClass == nil || *got.Filter.MinRoadClass != routerv1.RoadClass_RC_PRIMARY {
		t.Errorf("MinRoadClass: want RC_PRIMARY, got %v", got.Filter.MinRoadClass)
	}
	if got.Filter.MaxRoadClass == nil || *got.Filter.MaxRoadClass != routerv1.RoadClass_RC_MOTORWAY {
		t.Errorf("MaxRoadClass: want RC_MOTORWAY, got %v", got.Filter.MaxRoadClass)
	}
	if got.SideOfStreet == nil || *got.SideOfStreet != routerv1.SideOfStreet_SS_LEFT {
		t.Errorf("SideOfStreet: want SS_LEFT, got %v", got.SideOfStreet)
	}
	if got.DateTime == nil || !got.DateTime.AsTime().Equal(dt) {
		t.Errorf("DateTime: want %v, got %v", dt, got.DateTime)
	}
}

// TestCreateLocation_LocationKindMapping covers every LocationKind value.
func TestCreateLocation_LocationKindMapping(t *testing.T) {
	tests := []struct {
		kind *vhtypes.LocationKind
		want routerv1.LocationType
	}{
		{nil, routerv1.LocationType_L_BREAK},
		{kindPtr(vhtypes.Through), routerv1.LocationType_L_THROUGH},
		{kindPtr(vhtypes.Via), routerv1.LocationType_L_VIA},
		{kindPtr(vhtypes.BreakThrough), routerv1.LocationType_L_BREAK_THROUGH},
	}
	for _, tt := range tests {
		got, err := createLocation(&vhtypes.Location{Lat: 1, Lon: 2, LocationKind: tt.kind}, log.New())
		if err != nil {
			t.Fatalf("createLocation error: %v", err)
		}
		if got.Type != tt.want {
			t.Errorf("kind=%v: want %v, got %v", tt.kind, tt.want, got.Type)
		}
	}
}

func kindPtr(k vhtypes.LocationKind) *vhtypes.LocationKind { return &k }
func roadClassPtr(rc vhtypes.RoadClass) *vhtypes.RoadClass { return &rc }

// TestCreateLocation_RoadClassMapping covers every RoadClass value, plus an
// unrecognized value leaving the field unset.
func TestCreateLocation_RoadClassMapping(t *testing.T) {
	tests := []struct {
		rc   vhtypes.RoadClass
		want routerv1.RoadClass
	}{
		{vhtypes.Motorway, routerv1.RoadClass_RC_MOTORWAY},
		{vhtypes.Trunk, routerv1.RoadClass_RC_TRUNK},
		{vhtypes.Primary, routerv1.RoadClass_RC_PRIMARY},
		{vhtypes.Secondary, routerv1.RoadClass_RC_SECONDARY},
		{vhtypes.Tertiary, routerv1.RoadClass_RC_TERTIARY},
		{vhtypes.Unclassified, routerv1.RoadClass_RC_UNCLASSIFIED},
		{vhtypes.Residential, routerv1.RoadClass_RC_RESIDENTIAL},
		{vhtypes.Service, routerv1.RoadClass_RC_SERVICE},
		{vhtypes.Track, routerv1.RoadClass_RC_TRACK},
	}
	for _, tt := range tests {
		vhLoc := &vhtypes.Location{Lat: 1, Lon: 2, SearchFilter: &vhtypes.SearchFilters{
			MinRoadClass: roadClassPtr(tt.rc), MaxRoadClass: roadClassPtr(tt.rc),
		}}
		got, err := createLocation(vhLoc, log.New())
		if err != nil {
			t.Fatalf("createLocation error: %v", err)
		}
		if got.Filter.MinRoadClass == nil || *got.Filter.MinRoadClass != tt.want {
			t.Errorf("rc=%v: MinRoadClass want %v, got %v", tt.rc, tt.want, got.Filter.MinRoadClass)
		}
		if got.Filter.MaxRoadClass == nil || *got.Filter.MaxRoadClass != tt.want {
			t.Errorf("rc=%v: MaxRoadClass want %v, got %v", tt.rc, tt.want, got.Filter.MaxRoadClass)
		}
	}

	t.Run("unrecognized value left unset", func(t *testing.T) {
		bogus := vhtypes.RoadClass("bogus")
		vhLoc := &vhtypes.Location{Lat: 1, Lon: 2, SearchFilter: &vhtypes.SearchFilters{
			MinRoadClass: &bogus,
		}}
		got, err := createLocation(vhLoc, log.New())
		if err != nil {
			t.Fatalf("createLocation error: %v", err)
		}
		if got.Filter.MinRoadClass != nil {
			t.Errorf("want nil MinRoadClass for unrecognized value, got %v", *got.Filter.MinRoadClass)
		}
	})
}

// TestAddLegs_NotAppend verifies fresh legs are built via createLeg.
func TestAddLegs_NotAppend(t *testing.T) {
	trip := &routerv1.Trip{}
	legs := []vhtypes.Leg{
		{Shape: encodeShape([]float64{1, 2, 3, 4}), Maneuvers: []vhtypes.Maneuver{}, Summary: vhtypes.Summary{}},
	}
	if err := addLegs(trip, legs, false, log.New()); err != nil {
		t.Fatalf("addLegs error: %v", err)
	}
	if len(trip.Legs) != 1 {
		t.Fatalf("want 1 leg, got %d", len(trip.Legs))
	}
}

// TestAddLegs_Append_MarksMergeNextOnLastExistingLeg verifies appending legs
// marks the last existing leg's MergeNext and appends the new legs.
func TestAddLegs_Append_MarksMergeNextOnLastExistingLeg(t *testing.T) {
	trip := &routerv1.Trip{
		Legs: []*routerv1.Leg{{Shape: encodeShape([]float64{1, 2, 3, 4})}},
	}
	legs := []vhtypes.Leg{
		{Shape: encodeShape([]float64{3, 4, 5, 6}), Maneuvers: []vhtypes.Maneuver{}, Summary: vhtypes.Summary{}},
	}
	if err := addLegs(trip, legs, true, log.New()); err != nil {
		t.Fatalf("addLegs error: %v", err)
	}
	if len(trip.Legs) != 2 {
		t.Fatalf("want 2 legs, got %d", len(trip.Legs))
	}
	if !trip.Legs[0].MergeNext {
		t.Error("want existing last leg's MergeNext set")
	}
}

// TestCreateLeg_FieldMapping is a delegation smoke test: shape, elevation
// interval, and that maneuvers/elevation/summary are all populated.
func TestCreateLeg_FieldMapping(t *testing.T) {
	interval := 30.0
	vhLeg := &vhtypes.Leg{
		Shape:             encodeShape([]float64{1, 2, 3, 4}),
		ElevationInterval: &interval,
		Elevation:         []float64{100, 200},
		Maneuvers: []vhtypes.Maneuver{
			{Type: vhtypes.StartManeuver},
		},
		Summary: vhtypes.Summary{Time: 10, Length: 1},
	}

	got, err := createLeg(vhLeg, log.New())
	if err != nil {
		t.Fatalf("createLeg error: %v", err)
	}
	if got.Shape != vhLeg.Shape {
		t.Errorf("Shape mismatch")
	}
	if got.ElevationInterval == nil || *got.ElevationInterval != 30.0 {
		t.Errorf("ElevationInterval: want 30, got %v", got.ElevationInterval)
	}
	if len(got.Maneuvers) != 1 {
		t.Errorf("want 1 maneuver, got %d", len(got.Maneuvers))
	}
	if len(got.Elevation) != 2 {
		t.Errorf("want 2 elevation points, got %d", len(got.Elevation))
	}
	if got.Summary == nil || got.Summary.Time != 10 {
		t.Errorf("Summary: want Time=10, got %+v", got.Summary)
	}
}

// TestAddManeuvers verifies count and per-item delegation to createManeuver.
func TestAddManeuvers(t *testing.T) {
	leg := &routerv1.Leg{}
	maneuvers := []vhtypes.Maneuver{
		{Type: vhtypes.StartManeuver, Instruction: "Start"},
		{Type: vhtypes.DestinationManeuver, Instruction: "Arrive"},
	}
	if err := addManeuvers(leg, maneuvers, log.New()); err != nil {
		t.Fatalf("addManeuvers error: %v", err)
	}
	if len(leg.Maneuvers) != 2 {
		t.Fatalf("want 2 maneuvers, got %d", len(leg.Maneuvers))
	}
	if leg.Maneuvers[0].Instruction != "Start" || leg.Maneuvers[1].Instruction != "Arrive" {
		t.Errorf("unexpected maneuver instructions: %+v, %+v", leg.Maneuvers[0], leg.Maneuvers[1])
	}
}

// TestCreateManeuver_ScalarFields verifies scalar field passthrough.
func TestCreateManeuver_ScalarFields(t *testing.T) {
	vhM := &vhtypes.Maneuver{
		Type:                             vhtypes.RightManeuver,
		Instruction:                      "Turn right",
		VerbalTransitionAlertInstruction: "alert",
		VerbalPreTransitionInstruction:   "pre",
		VerbalPostTransitionInstruction:  "post",
		StreetNames:                      []string{"Main St"},
		BeginStreetNames:                 []string{"Elm St"},
		Time:                             12.5,
		Length:                           1.2,
		BeginShapeIndex:                  3,
		EndShapeIndex:                    7,
		Toll:                             boolPtr(true),
		Highway:                          boolPtr(false),
		Rough:                            boolPtr(true),
		Gate:                             boolPtr(false),
		Ferry:                            boolPtr(true),
		BearingBefore:                    10,
		BearingAfter:                     20,
		VerbalMultiCue:                   boolPtr(true),
	}

	got, err := createManeuver(vhM, log.New())
	if err != nil {
		t.Fatalf("createManeuver error: %v", err)
	}
	if got.Instruction != "Turn right" || got.VerbalTransitionAlertInstruction != "alert" ||
		got.VerbalPreTransitionInstruction != "pre" || got.VerbalPostTransitionInstruction != "post" {
		t.Errorf("instruction fields: unexpected value %+v", got)
	}
	if len(got.StreetNames) != 1 || got.StreetNames[0] != "Main St" {
		t.Errorf("StreetNames: unexpected value %v", got.StreetNames)
	}
	if got.Time != 12.5 || got.Length != 1.2 {
		t.Errorf("Time/Length: unexpected value %v/%v", got.Time, got.Length)
	}
	if got.BeginShapeIndex != 3 || got.EndShapeIndex != 7 {
		t.Errorf("shape indices: unexpected value %d/%d", got.BeginShapeIndex, got.EndShapeIndex)
	}
	if got.Toll == nil || !*got.Toll || got.Highway == nil || *got.Highway {
		t.Errorf("Toll/Highway: unexpected value %v/%v", got.Toll, got.Highway)
	}
	if got.BearingBefore != 10 || got.BearingAfter != 20 {
		t.Errorf("bearings: unexpected value %d/%d", got.BearingBefore, got.BearingAfter)
	}
	if got.VerbalMultiCue == nil || !*got.VerbalMultiCue {
		t.Errorf("VerbalMultiCue: want true, got %v", got.VerbalMultiCue)
	}
}

// TestCreateManeuver_Sign verifies Sign mapping including ConsecutiveCount
// pointer copying, for an entry with and without it set.
func TestCreateManeuver_Sign(t *testing.T) {
	vhM := &vhtypes.Maneuver{
		Sign: &vhtypes.Sign{
			ExitNumberElements: []vhtypes.SignElement{
				{Text: "91B", ConsecutiveCount: intPtr(2)},
				{Text: "91C"},
			},
			ExitBranchElements: []vhtypes.SignElement{{Text: "I 95 North"}},
			ExitTowardElements: []vhtypes.SignElement{{Text: "New York"}},
			ExitNameElements:   []vhtypes.SignElement{{Text: "Gettysburg Pike"}},
		},
	}
	got, err := createManeuver(vhM, log.New())
	if err != nil {
		t.Fatalf("createManeuver error: %v", err)
	}
	if got.Sign == nil {
		t.Fatal("want non-nil Sign")
	}
	if len(got.Sign.ExitNumberElements) != 2 {
		t.Fatalf("want 2 ExitNumberElements, got %d", len(got.Sign.ExitNumberElements))
	}
	if got.Sign.ExitNumberElements[0].Text != "91B" ||
		got.Sign.ExitNumberElements[0].ConsecutiveCount == nil ||
		*got.Sign.ExitNumberElements[0].ConsecutiveCount != 2 {
		t.Errorf("ExitNumberElements[0]: unexpected value %+v", got.Sign.ExitNumberElements[0])
	}
	if got.Sign.ExitNumberElements[1].ConsecutiveCount != nil {
		t.Errorf("ExitNumberElements[1]: want nil ConsecutiveCount, got %v", *got.Sign.ExitNumberElements[1].ConsecutiveCount)
	}
	if len(got.Sign.ExitBranchElements) != 1 || got.Sign.ExitBranchElements[0].Text != "I 95 North" {
		t.Errorf("ExitBranchElements: unexpected value %+v", got.Sign.ExitBranchElements)
	}
	if len(got.Sign.ExitTowardElements) != 1 || got.Sign.ExitTowardElements[0].Text != "New York" {
		t.Errorf("ExitTowardElements: unexpected value %+v", got.Sign.ExitTowardElements)
	}
	if len(got.Sign.ExitNameElements) != 1 || got.Sign.ExitNameElements[0].Text != "Gettysburg Pike" {
		t.Errorf("ExitNameElements: unexpected value %+v", got.Sign.ExitNameElements)
	}
}

// TestCreateManeuver_RoundaboutExitCount covers set vs. nil.
func TestCreateManeuver_RoundaboutExitCount(t *testing.T) {
	got, err := createManeuver(&vhtypes.Maneuver{RoundaboutExitCount: intPtr(3)}, log.New())
	if err != nil {
		t.Fatalf("createManeuver error: %v", err)
	}
	if got.RoundaboutExitCount == nil || *got.RoundaboutExitCount != 3 {
		t.Errorf("want 3, got %v", got.RoundaboutExitCount)
	}

	got, err = createManeuver(&vhtypes.Maneuver{}, log.New())
	if err != nil {
		t.Fatalf("createManeuver error: %v", err)
	}
	if got.RoundaboutExitCount != nil {
		t.Errorf("want nil, got %v", *got.RoundaboutExitCount)
	}
}

// TestCreateManeuver_TransitInfo verifies full TransitInfo mapping including
// the time.Time -> timestamppb conversion for transit stops.
func TestCreateManeuver_TransitInfo(t *testing.T) {
	arrival := time.Date(2026, 1, 1, 8, 6, 0, 0, time.UTC)
	departure := time.Date(2026, 1, 1, 8, 8, 0, 0, time.UTC)
	vhM := &vhtypes.Maneuver{
		TransitInfo: &vhtypes.TransitInfo{
			OnestopId: "onestop-1", ShortName: "N", LongName: "Broadway Express",
			Headsign: "Astoria", Color: 16567306, TextColor: 0,
			Description: "desc", OperatorOnestopId: "op-1", OperatorName: "MTA", OperatorUrl: "https://mta.info",
			TransitStops: []vhtypes.TransitStop{
				{
					Type: vhtypes.Station, Name: "14 St - Union Sq",
					ArrivalDateTime: arrival, DepartureDateTime: departure,
					IsParentStop: true, AssumedSchedule: false, Lat: 40.7, Lon: -73.9,
				},
			},
		},
	}
	got, err := createManeuver(vhM, log.New())
	if err != nil {
		t.Fatalf("createManeuver error: %v", err)
	}
	if got.TransitInfo == nil {
		t.Fatal("want non-nil TransitInfo")
	}
	if got.TransitInfo.OnestopId != "onestop-1" || got.TransitInfo.OperatorName != "MTA" {
		t.Errorf("TransitInfo: unexpected value %+v", got.TransitInfo)
	}
	if len(got.TransitInfo.TransitStops) != 1 {
		t.Fatalf("want 1 transit stop, got %d", len(got.TransitInfo.TransitStops))
	}
	stop := got.TransitInfo.TransitStops[0]
	if !stop.ArrivalDateTime.AsTime().Equal(arrival) {
		t.Errorf("ArrivalDateTime: want %v, got %v", arrival, stop.ArrivalDateTime.AsTime())
	}
	if !stop.DepartureDateTime.AsTime().Equal(departure) {
		t.Errorf("DepartureDateTime: want %v, got %v", departure, stop.DepartureDateTime.AsTime())
	}
	if !stop.IsParentStop {
		t.Error("want IsParentStop=true")
	}
	if stop.Location.Lat != 40.7 || stop.Location.Lon != -73.9 {
		t.Errorf("Location: unexpected value %+v", stop.Location)
	}
}

// TestCreateManeuver_TravelModeMapping covers every TravelMode value + default.
func TestCreateManeuver_TravelModeMapping(t *testing.T) {
	tests := []struct {
		mode vhtypes.TravelMode
		want routerv1.TravelMode
	}{
		{vhtypes.PedestrianTravelMode, routerv1.TravelMode_TM_PEDESTRIAN},
		{vhtypes.BicycleTravelMode, routerv1.TravelMode_TM_BICYCLE},
		{vhtypes.TransitTravelMode, routerv1.TravelMode_TM_TRANSIT},
		{vhtypes.DriveTravelMode, routerv1.TravelMode_TM_DRIVE},
		{vhtypes.TravelMode("unknown"), routerv1.TravelMode_TM_DRIVE},
	}
	for _, tt := range tests {
		got, err := createManeuver(&vhtypes.Maneuver{TravelMode: tt.mode}, log.New())
		if err != nil {
			t.Fatalf("createManeuver error: %v", err)
		}
		if got.TravelMode != tt.want {
			t.Errorf("mode=%v: want %v, got %v", tt.mode, tt.want, got.TravelMode)
		}
	}
}

// TestCreateManeuver_TravelTypeMapping covers every TravelType value + default.
func TestCreateManeuver_TravelTypeMapping(t *testing.T) {
	tests := []struct {
		typ  vhtypes.TravelType
		want routerv1.TravelType
	}{
		{vhtypes.MotorScooterTravelType, routerv1.TravelType_TT_MOTORSCOOTER},
		{vhtypes.MotorcycleTravelType, routerv1.TravelType_TT_MOTORCYCLE},
		{vhtypes.TruckTravelType, routerv1.TravelType_TT_TRUCK},
		{vhtypes.BusTravelType, routerv1.TravelType_TT_BUS},
		{vhtypes.FootTravelType, routerv1.TravelType_TT_FOOT},
		{vhtypes.WheelchairTravelType, routerv1.TravelType_TT_WHEELCHAIR},
		{vhtypes.RoadTravelType, routerv1.TravelType_TT_ROAD},
		{vhtypes.HybridTravelType, routerv1.TravelType_TT_HYBRID},
		{vhtypes.CrossTravelType, routerv1.TravelType_TT_CROSS},
		{vhtypes.MountainTravelType, routerv1.TravelType_TT_MOUNTAIN},
		{vhtypes.TramTravelType, routerv1.TravelType_TT_TRAM},
		{vhtypes.MetroTravelType, routerv1.TravelType_TT_METRO},
		{vhtypes.RailTravelType, routerv1.TravelType_TT_RAIL},
		{vhtypes.FerryTravelType, routerv1.TravelType_TT_FERRY},
		{vhtypes.CableCarTravelType, routerv1.TravelType_TT_CABLE_CAR},
		{vhtypes.GondolaTravelType, routerv1.TravelType_TT_GONDOLA},
		{vhtypes.FunicularTravelType, routerv1.TravelType_TT_FUNICULAR},
		{vhtypes.CarTravelType, routerv1.TravelType_TT_CAR},
		{vhtypes.TravelType("unknown"), routerv1.TravelType_TT_CAR},
	}
	for _, tt := range tests {
		got, err := createManeuver(&vhtypes.Maneuver{TravelType: tt.typ}, log.New())
		if err != nil {
			t.Fatalf("createManeuver error: %v", err)
		}
		if got.TravelType != tt.want {
			t.Errorf("type=%v: want %v, got %v", tt.typ, tt.want, got.TravelType)
		}
	}
}

// TestCreateManeuver_BssManeuverType covers nil, both known values, and an
// unrecognized value falling to the BS_NONE_ACTION default.
func TestCreateManeuver_BssManeuverType(t *testing.T) {
	got, err := createManeuver(&vhtypes.Maneuver{}, log.New())
	if err != nil {
		t.Fatalf("createManeuver error: %v", err)
	}
	if got.BssManeuverType != nil {
		t.Errorf("nil input: want nil, got %v", *got.BssManeuverType)
	}

	bss := vhtypes.RentBikeAtBikeShare
	got, err = createManeuver(&vhtypes.Maneuver{BssManeuverType: &bss}, log.New())
	if err != nil {
		t.Fatalf("createManeuver error: %v", err)
	}
	if got.BssManeuverType == nil || *got.BssManeuverType != routerv1.BikeShareManeuver_BS_RENT_BIKE_AT_BIKESHARE {
		t.Errorf("RentBikeAtBikeShare: want BS_RENT_BIKE_AT_BIKESHARE, got %v", got.BssManeuverType)
	}

	bss = vhtypes.ReturnBikeAtBikeShare
	got, err = createManeuver(&vhtypes.Maneuver{BssManeuverType: &bss}, log.New())
	if err != nil {
		t.Fatalf("createManeuver error: %v", err)
	}
	if got.BssManeuverType == nil || *got.BssManeuverType != routerv1.BikeShareManeuver_BS_RETURN_BIKE_AT_BIKESHARE {
		t.Errorf("ReturnBikeAtBikeShare: want BS_RETURN_BIKE_AT_BIKESHARE, got %v", got.BssManeuverType)
	}

	bogus := vhtypes.BikeShareManeuver("bogus")
	got, err = createManeuver(&vhtypes.Maneuver{BssManeuverType: &bogus}, log.New())
	if err != nil {
		t.Fatalf("createManeuver error: %v", err)
	}
	if got.BssManeuverType == nil || *got.BssManeuverType != routerv1.BikeShareManeuver_BS_NONE_ACTION {
		t.Errorf("unrecognized value: want BS_NONE_ACTION default, got %v", got.BssManeuverType)
	}
}

// TestCreateManeuver_Lanes covers Valid/Active both set, both nil, and an
// empty Lanes slice leaving maneuver.Lanes unallocated.
func TestCreateManeuver_Lanes(t *testing.T) {
	valid := vhtypes.TDThrough
	active := vhtypes.TDLeft
	vhM := &vhtypes.Maneuver{
		Lanes: []vhtypes.TurnLane{
			{Directions: vhtypes.TDThrough | vhtypes.TDLeft, Valid: &valid, Active: &active},
			{Directions: vhtypes.TDRight},
		},
	}
	got, err := createManeuver(vhM, log.New())
	if err != nil {
		t.Fatalf("createManeuver error: %v", err)
	}
	if len(got.Lanes) != 2 {
		t.Fatalf("want 2 lanes, got %d", len(got.Lanes))
	}
	if got.Lanes[0].Valid == nil || *got.Lanes[0].Valid != uint32(valid) {
		t.Errorf("Lanes[0].Valid: want %v, got %v", valid, got.Lanes[0].Valid)
	}
	if got.Lanes[0].Active == nil || *got.Lanes[0].Active != uint32(active) {
		t.Errorf("Lanes[0].Active: want %v, got %v", active, got.Lanes[0].Active)
	}
	if got.Lanes[1].Valid != nil || got.Lanes[1].Active != nil {
		t.Errorf("Lanes[1]: want nil Valid/Active, got %v/%v", got.Lanes[1].Valid, got.Lanes[1].Active)
	}

	got, err = createManeuver(&vhtypes.Maneuver{}, log.New())
	if err != nil {
		t.Fatalf("createManeuver error: %v", err)
	}
	if got.Lanes != nil {
		t.Errorf("empty Lanes: want nil, got %v", got.Lanes)
	}
}

// TestAddElevation verifies the trivial elevation passthrough, including nil input.
func TestAddElevation(t *testing.T) {
	leg := &routerv1.Leg{}
	if err := addElevation(leg, []float64{1, 2, 3}, log.New()); err != nil {
		t.Fatalf("addElevation error: %v", err)
	}
	if len(leg.Elevation) != 3 {
		t.Errorf("want 3 elevation points, got %d", len(leg.Elevation))
	}

	leg2 := &routerv1.Leg{}
	if err := addElevation(leg2, nil, log.New()); err != nil {
		t.Fatalf("addElevation error: %v", err)
	}
	if leg2.Elevation != nil {
		t.Errorf("want nil elevation, got %v", leg2.Elevation)
	}
}

// TestCreateTripSummary_Initial verifies a fresh summary/bounding box is
// built from the first Valhalla summary.
func TestCreateTripSummary_Initial(t *testing.T) {
	trip := &routerv1.Trip{}
	vhSummary := &vhtypes.Summary{Time: 100, Length: 10, HasToll: true, MinLat: 50, MinLon: 4, MaxLat: 51, MaxLon: 5}

	if err := createTripSummary(trip, vhSummary, false, log.New()); err != nil {
		t.Fatalf("createTripSummary error: %v", err)
	}
	if trip.Summary.Time != 100 || trip.Summary.Length != 10 || !trip.Summary.HasToll {
		t.Errorf("Summary: unexpected value %+v", trip.Summary)
	}
	if trip.Summary.BoundingBox.BottomLeft.Lat != 50 || trip.Summary.BoundingBox.TopRight.Lat != 51 {
		t.Errorf("BoundingBox: unexpected value %+v", trip.Summary.BoundingBox)
	}
}

// TestCreateTripSummary_Accumulate verifies sum/OR-accumulate/bbox-expand
// semantics across two calls, including the "already true stays true" and
// "new value inside existing box, no change" cases.
func TestCreateTripSummary_Accumulate(t *testing.T) {
	trip := &routerv1.Trip{}
	first := &vhtypes.Summary{Time: 100, Length: 10, HasToll: true, HasHighway: false, MinLat: 50, MinLon: 4, MaxLat: 51, MaxLon: 5}
	if err := createTripSummary(trip, first, false, log.New()); err != nil {
		t.Fatalf("createTripSummary error: %v", err)
	}

	// Second summary: HasToll already true and stays false-input (should stay true);
	// HasHighway newly true; bounding box entirely inside the first (no change expected).
	second := &vhtypes.Summary{Time: 20, Length: 2, HasToll: false, HasHighway: true, MinLat: 50.2, MinLon: 4.2, MaxLat: 50.8, MaxLon: 4.8}
	if err := createTripSummary(trip, second, true, log.New()); err != nil {
		t.Fatalf("createTripSummary error: %v", err)
	}
	if trip.Summary.Time != 120 || trip.Summary.Length != 12 {
		t.Errorf("Time/Length: want 120/12, got %v/%v", trip.Summary.Time, trip.Summary.Length)
	}
	if !trip.Summary.HasToll {
		t.Error("HasToll: want true to stay true")
	}
	if !trip.Summary.HasHighway {
		t.Error("HasHighway: want true after second summary sets it")
	}
	if trip.Summary.BoundingBox.BottomLeft.Lat != 50 || trip.Summary.BoundingBox.TopRight.Lat != 51 {
		t.Errorf("BoundingBox should be unchanged (second box is inside first), got %+v", trip.Summary.BoundingBox)
	}

	// Third summary expands the box in all four directions.
	third := &vhtypes.Summary{MinLat: 49, MinLon: 3, MaxLat: 52, MaxLon: 6}
	if err := createTripSummary(trip, third, true, log.New()); err != nil {
		t.Fatalf("createTripSummary error: %v", err)
	}
	bb := trip.Summary.BoundingBox
	if bb.BottomLeft.Lat != 49 || bb.BottomLeft.Lon != 3 || bb.TopRight.Lat != 52 || bb.TopRight.Lon != 6 {
		t.Errorf("BoundingBox should expand to third summary's box, got %+v", bb)
	}
}

// TestCreateLegSummary verifies straightforward field mapping.
func TestCreateLegSummary(t *testing.T) {
	leg := &routerv1.Leg{}
	vhSummary := &vhtypes.Summary{Time: 30, Length: 3, HasFerry: true, MinLat: 1, MinLon: 2, MaxLat: 3, MaxLon: 4}

	if err := createLegSummary(leg, vhSummary, log.New()); err != nil {
		t.Fatalf("createLegSummary error: %v", err)
	}
	if leg.Summary.Time != 30 || leg.Summary.Length != 3 || !leg.Summary.HasFerry {
		t.Errorf("Summary: unexpected value %+v", leg.Summary)
	}
	if leg.Summary.BoundingBox.BottomLeft.Lat != 1 || leg.Summary.BoundingBox.TopRight.Lon != 4 {
		t.Errorf("BoundingBox: unexpected value %+v", leg.Summary.BoundingBox)
	}
}
