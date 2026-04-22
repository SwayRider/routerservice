package server

import (
	"errors"
	"net"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
