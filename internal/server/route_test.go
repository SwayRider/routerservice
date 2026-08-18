package server

import (
	"errors"
	"math"
	"net"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	pbgeo "github.com/swayrider/protos/common_types/geo"
	routerv1 "github.com/swayrider/protos/router/v1"
	vhtypes "github.com/swayrider/routerservice/restclients/valhalla/types"
	"github.com/swayrider/routerservice/internal/logic"
)

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
