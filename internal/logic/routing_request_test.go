package logic

import (
	"testing"

	"github.com/swayrider/grpcclients/regionclient"
	routerv1 "github.com/swayrider/protos/router/v1"
	vhtypes "github.com/swayrider/routerservice/restclients/valhalla/types"
)

func rtPtr(rt regionclient.RoadType) *regionclient.RoadType {
	return &rt
}

// TestCostingModel verifies every RoutingMode maps to the correct Valhalla model.
func TestCostingModel(t *testing.T) {
	tests := []struct {
		mode routerv1.RoutingMode
		want vhtypes.CostingModel
	}{
		{routerv1.RoutingMode_RM_CAR, vhtypes.Auto},
		{routerv1.RoutingMode_RM_MOTORSCOOTER, vhtypes.MotorScooter},
		{routerv1.RoutingMode_RM_MOTORCYCLE, vhtypes.Motorcycle},
	}
	for _, tt := range tests {
		got := costingModel(tt.mode)
		if got != tt.want {
			t.Errorf("costingModel(%v) = %q, want %q", tt.mode, got, tt.want)
		}
	}
}

// TestLocationKind verifies every LocationType maps to the correct Valhalla kind.
func TestLocationKind(t *testing.T) {
	tests := []struct {
		lt   routerv1.LocationType
		want vhtypes.LocationKind
	}{
		{routerv1.LocationType_L_THROUGH, vhtypes.Through},
		{routerv1.LocationType_L_VIA, vhtypes.Via},
		{routerv1.LocationType_L_BREAK_THROUGH, vhtypes.BreakThrough},
		{routerv1.LocationType_L_BREAK, vhtypes.Break},
	}
	for _, tt := range tests {
		got := locationKind(tt.lt)
		if got != tt.want {
			t.Errorf("locationKind(%v) = %q, want %q", tt.lt, got, tt.want)
		}
	}
}

// TestGenericRoadTypeOrder verifies the first road type in the generic ordering.
func TestGenericRoadTypeOrder(t *testing.T) {
	tests := []struct {
		name              string
		maxPrimary        bool
		highwayPref       float64
		primaryPref       float64
		wantFirst         regionclient.RoadType
	}{
		{"maxPrimary high primary", true, 0.0, 0.7, regionclient.RT_PRIMARY},
		{"maxPrimary low primary", true, 0.0, 0.3, regionclient.RT_SECONDARY},
		{"highway high pref", false, 0.8, 0.5, regionclient.RT_MOTORWAY},
		{"highway low pref", false, 0.3, 0.5, regionclient.RT_TRUNK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := genericRoadTypeOrder(tt.maxPrimary, tt.highwayPref, tt.primaryPref)
			if len(got) == 0 {
				t.Fatal("empty result")
			}
			if got[0] != tt.wantFirst {
				t.Errorf("want first=%v, got %v", tt.wantFirst, got[0])
			}
		})
	}
}

// TestFarRoadTypeOrder verifies the first road type in the far-distance ordering.
func TestFarRoadTypeOrder(t *testing.T) {
	tests := []struct {
		name        string
		maxPrimary  bool
		highwayPref float64
		primaryPref float64
		wantFirst   regionclient.RoadType
	}{
		{"maxPrimary high primary", true, 0.0, 0.7, regionclient.RT_PRIMARY},
		{"maxPrimary low primary", true, 0.0, 0.3, regionclient.RT_SECONDARY},
		{"highway high pref", false, 0.8, 0.5, regionclient.RT_MOTORWAY},
		{"highway low pref", false, 0.3, 0.5, regionclient.RT_PRIMARY},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := farRoadTypeOrder(tt.maxPrimary, tt.highwayPref, tt.primaryPref)
			if len(got) == 0 {
				t.Fatal("empty result")
			}
			if got[0] != tt.wantFirst {
				t.Errorf("want first=%v, got %v", tt.wantFirst, got[0])
			}
		})
	}
}

// TestMediumRoadTypeOrder verifies the first road type in the medium-distance ordering.
func TestMediumRoadTypeOrder(t *testing.T) {
	tests := []struct {
		name        string
		maxPrimary  bool
		highwayPref float64
		primaryPref float64
		wantFirst   regionclient.RoadType
	}{
		{"maxPrimary high primary", true, 0.0, 0.7, regionclient.RT_PRIMARY},
		{"maxPrimary low primary", true, 0.0, 0.3, regionclient.RT_SECONDARY},
		{"highway high pref", false, 0.8, 0.5, regionclient.RT_MOTORWAY},
		{"highway low pref", false, 0.3, 0.5, regionclient.RT_PRIMARY},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mediumRoadTypeOrder(tt.maxPrimary, tt.highwayPref, tt.primaryPref)
			if len(got) == 0 {
				t.Fatal("empty result")
			}
			if got[0] != tt.wantFirst {
				t.Errorf("want first=%v, got %v", tt.wantFirst, got[0])
			}
		})
	}
}

// TestCloseRoadTypeOrder covers key combinations of the close-distance ordering.
func TestCloseRoadTypeOrder(t *testing.T) {
	t.Run("both rt1>rt2 by code: higher code first", func(t *testing.T) {
		// RT_SECONDARY.ToCode()=3 > RT_TRUNK.ToCode()=1
		rt1 := rtPtr(regionclient.RT_SECONDARY)
		rt2 := rtPtr(regionclient.RT_TRUNK)
		got := closeRoadTypeOrder(rt1, rt2, false, 0.8, 0.5)
		if len(got) == 0 {
			t.Fatal("empty result")
		}
		if got[0] != regionclient.RT_SECONDARY {
			t.Errorf("want RT_SECONDARY first, got %v", got[0])
		}
	})

	t.Run("both rt1<rt2 by code: rt2 first", func(t *testing.T) {
		// RT_MOTORWAY.ToCode()=0 < RT_PRIMARY.ToCode()=2
		rt1 := rtPtr(regionclient.RT_MOTORWAY)
		rt2 := rtPtr(regionclient.RT_PRIMARY)
		got := closeRoadTypeOrder(rt1, rt2, false, 0.8, 0.5)
		if len(got) == 0 {
			t.Fatal("empty result")
		}
		if got[0] != regionclient.RT_PRIMARY {
			t.Errorf("want RT_PRIMARY first, got %v", got[0])
		}
	})

	t.Run("only rt1 known", func(t *testing.T) {
		rt1 := rtPtr(regionclient.RT_MOTORWAY)
		got := closeRoadTypeOrder(rt1, nil, false, 0.8, 0.5)
		if len(got) == 0 || got[0] != regionclient.RT_MOTORWAY {
			t.Errorf("want RT_MOTORWAY first, got %v", got)
		}
	})

	t.Run("only rt2 known", func(t *testing.T) {
		rt2 := rtPtr(regionclient.RT_PRIMARY)
		got := closeRoadTypeOrder(nil, rt2, false, 0.8, 0.5)
		if len(got) == 0 || got[0] != regionclient.RT_PRIMARY {
			t.Errorf("want RT_PRIMARY first, got %v", got)
		}
	})

	t.Run("both nil defaults to secondary first", func(t *testing.T) {
		got := closeRoadTypeOrder(nil, nil, false, 0.8, 0.5)
		if len(got) == 0 || got[0] != regionclient.RT_SECONDARY {
			t.Errorf("want RT_SECONDARY first, got %v", got)
		}
	})
}

// buildMinimalRoutingRequest creates a RoutingRequest with two Break locations.
func buildMinimalRoutingRequest() *RoutingRequest {
	req := vhtypes.NewRouteRequest(vhtypes.Auto)
	loc1 := vhtypes.NewLocation(52.0, 4.0)
	loc2 := vhtypes.NewLocation(51.0, 3.0)
	req.AddLocation(*loc1)
	req.AddLocation(*loc2)
	return &RoutingRequest{
		Region:      "nl",
		RequestData: req,
	}
}

// TestAppendBorderCrossing verifies the crossing is appended and previous last becomes Through.
func TestAppendBorderCrossing(t *testing.T) {
	rr := buildMinimalRoutingRequest()
	crossing := regionclient.Coordinate{Latitude: 50.5, Longitude: 3.5}

	if err := rr.AppendBorderCrossing(crossing); err != nil {
		t.Fatalf("AppendBorderCrossing error: %v", err)
	}

	locs := rr.RequestData.Locations
	if len(locs) != 3 {
		t.Fatalf("want 3 locations, got %d", len(locs))
	}

	// Previous last (index 1) must be Through.
	if locs[1].LocationKind == nil || *locs[1].LocationKind != vhtypes.Through {
		t.Errorf("locs[1].LocationKind: want Through, got %v", locs[1].LocationKind)
	}

	// New last (index 2) must have the crossing coordinates.
	if locs[2].Lat != crossing.Latitude || locs[2].Lon != crossing.Longitude {
		t.Errorf("locs[2]: want {%.4f,%.4f}, got {%.4f,%.4f}",
			crossing.Latitude, crossing.Longitude, locs[2].Lat, locs[2].Lon)
	}
}

// TestPrependBorderCrossing verifies the crossing is prepended and previous first becomes Through.
func TestPrependBorderCrossing(t *testing.T) {
	rr := buildMinimalRoutingRequest()
	crossing := regionclient.Coordinate{Latitude: 52.5, Longitude: 4.5}

	if err := rr.PrependBorderCrossing(crossing); err != nil {
		t.Fatalf("PrependBorderCrossing error: %v", err)
	}

	locs := rr.RequestData.Locations
	if len(locs) != 3 {
		t.Fatalf("want 3 locations, got %d", len(locs))
	}

	// New first (index 0) must have the crossing coordinates.
	if locs[0].Lat != crossing.Latitude || locs[0].Lon != crossing.Longitude {
		t.Errorf("locs[0]: want {%.4f,%.4f}, got {%.4f,%.4f}",
			crossing.Latitude, crossing.Longitude, locs[0].Lat, locs[0].Lon)
	}

	// Previous first (now index 1) must be Through.
	if locs[1].LocationKind == nil || *locs[1].LocationKind != vhtypes.Through {
		t.Errorf("locs[1].LocationKind: want Through, got %v", locs[1].LocationKind)
	}
}
