package logic

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/paulmach/orb"
	"github.com/swayrider/grpcclients/regionclient"
	pbgeo "github.com/swayrider/protos/common_types/geo"
	routerv1 "github.com/swayrider/protos/router/v1"
	"github.com/swayrider/routerservice/restclients/valhalla"
	vhtypes "github.com/swayrider/routerservice/restclients/valhalla/types"
	log "github.com/swayrider/swlib/logger"
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

// TestSelectBorderCrossing covers the crossing-selection guard added for code
// review finding #2 (crossings[0] panicking on an empty slice).
func TestSelectBorderCrossing(t *testing.T) {
	t.Run("empty crossings returns ErrNoBorderCrossings", func(t *testing.T) {
		got, err := selectBorderCrossing(nil, regionclient.RT_MOTORWAY)
		if got != nil {
			t.Errorf("want nil result, got %v", got)
		}
		if !errors.Is(err, ErrNoBorderCrossings) {
			t.Errorf("want ErrNoBorderCrossings, got %v", err)
		}
	})

	t.Run("exact preferred-road-type match not in position 0 is selected", func(t *testing.T) {
		crossings := []regionclient.BorderCrossing{
			{RoadType: regionclient.RT_SECONDARY, Location: regionclient.Coordinate{Latitude: 1, Longitude: 1}},
			{RoadType: regionclient.RT_MOTORWAY, Location: regionclient.Coordinate{Latitude: 2, Longitude: 2}},
		}
		got, err := selectBorderCrossing(crossings, regionclient.RT_MOTORWAY)
		if err != nil {
			t.Fatalf("selectBorderCrossing error: %v", err)
		}
		if got.RoadType != regionclient.RT_MOTORWAY || got.Location.Latitude != 2 {
			t.Errorf("want the RT_MOTORWAY entry, got %+v", got)
		}
	})

	t.Run("no match falls back to first crossing", func(t *testing.T) {
		crossings := []regionclient.BorderCrossing{
			{RoadType: regionclient.RT_SECONDARY, Location: regionclient.Coordinate{Latitude: 1, Longitude: 1}},
			{RoadType: regionclient.RT_TRUNK, Location: regionclient.Coordinate{Latitude: 2, Longitude: 2}},
		}
		got, err := selectBorderCrossing(crossings, regionclient.RT_MOTORWAY)
		if err != nil {
			t.Fatalf("selectBorderCrossing error: %v", err)
		}
		if got.RoadType != regionclient.RT_SECONDARY || got.Location.Latitude != 1 {
			t.Errorf("want the first entry, got %+v", got)
		}
	})
}

// TestCreateRoutingRequests_EmptyTransferAssignment verifies that a transfer-region
// assignment (IsEmpty, FromIndex/ToIndex == -1) no longer panics on routeLocations[-1]
// and instead produces a RoutingRequest with the transfer region's name and no
// pre-populated locations (those are filled in later by AddBorderCrossings).
func TestCreateRoutingRequests_EmptyTransferAssignment(t *testing.T) {
	routeLocations := []*routerv1.RouteLocation{
		{Location: &pbgeo.Coordinate{Lat: 52.3, Lon: 4.9}, Type: routerv1.LocationType_L_BREAK},
		{Location: &pbgeo.Coordinate{Lat: 48.8, Lon: 2.3}, Type: routerv1.LocationType_L_BREAK},
	}
	assignmentList := []*RegionAssignment{
		{Region: "nl", FromIndex: 0, ToIndex: 0},
		{Region: "be", FromIndex: -1, ToIndex: -1, IsEmpty: true},
		{Region: "fr", FromIndex: 1, ToIndex: 1},
	}

	requestList, err := CreateRoutingRequests(
		nil, routerv1.RoutingMode_RM_CAR, routeLocations, assignmentList, log.New())
	if err != nil {
		t.Fatalf("CreateRoutingRequests error: %v", err)
	}
	if len(requestList) != 3 {
		t.Fatalf("want 3 requests, got %d", len(requestList))
	}

	if requestList[0].Region != "nl" || len(requestList[0].RequestData.Locations) != 1 {
		t.Errorf("requestList[0]: want region nl with 1 location, got region %q with %d locations",
			requestList[0].Region, len(requestList[0].RequestData.Locations))
	}
	if requestList[1].Region != "be" || len(requestList[1].RequestData.Locations) != 0 {
		t.Errorf("requestList[1]: want region be with 0 locations, got region %q with %d locations",
			requestList[1].Region, len(requestList[1].RequestData.Locations))
	}
	if requestList[2].Region != "fr" || len(requestList[2].RequestData.Locations) != 1 {
		t.Errorf("requestList[2]: want region fr with 1 location, got region %q with %d locations",
			requestList[2].Region, len(requestList[2].RequestData.Locations))
	}
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

// TestAppendBorderCrossing_EmptyLocations verifies appending onto a transfer-region
// request (no locations yet) doesn't panic and produces a single Break location.
func TestAppendBorderCrossing_EmptyLocations(t *testing.T) {
	rr := &RoutingRequest{
		Region:      "be",
		RequestData: vhtypes.NewRouteRequest(vhtypes.Auto),
	}
	crossing := regionclient.Coordinate{Latitude: 50.5, Longitude: 3.5}

	if err := rr.AppendBorderCrossing(crossing); err != nil {
		t.Fatalf("AppendBorderCrossing error: %v", err)
	}

	locs := rr.RequestData.Locations
	if len(locs) != 1 {
		t.Fatalf("want 1 location, got %d", len(locs))
	}
	if locs[0].Lat != crossing.Latitude || locs[0].Lon != crossing.Longitude {
		t.Errorf("locs[0]: want {%.4f,%.4f}, got {%.4f,%.4f}",
			crossing.Latitude, crossing.Longitude, locs[0].Lat, locs[0].Lon)
	}
	if locs[0].LocationKind != nil {
		t.Errorf("locs[0].LocationKind: want nil (Break), got %v", *locs[0].LocationKind)
	}
}

// TestGetRoadType_SparseLocateResponse verifies getRoadType doesn't panic when
// Valhalla's /locate response contains an edge with no "edge" details object
// (resp.Edges[0].Edge is a *EdgeDetails with omitempty, so this is legal JSON).
func TestGetRoadType_SparseLocateResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"edges":[{}]}]`))
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("failed to parse test server URL: %v", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("failed to parse test server port: %v", err)
	}

	clnt := valhalla.NewClient()
	clnt.AddRegion("be", u.Hostname(), port)

	got := getRoadType(context.Background(), time.Second, clnt, "be", orb.Point{4.0, 51.0}, false)
	if got != nil {
		t.Errorf("getRoadType() = %v, want nil for a sparse locate response", *got)
	}
}

// TestPrependBorderCrossing_EmptyLocations verifies prepending onto a transfer-region
// request (no locations yet) doesn't panic and produces a single Break location.
func TestPrependBorderCrossing_EmptyLocations(t *testing.T) {
	rr := &RoutingRequest{
		Region:      "be",
		RequestData: vhtypes.NewRouteRequest(vhtypes.Auto),
	}
	crossing := regionclient.Coordinate{Latitude: 52.5, Longitude: 4.5}

	if err := rr.PrependBorderCrossing(crossing); err != nil {
		t.Fatalf("PrependBorderCrossing error: %v", err)
	}

	locs := rr.RequestData.Locations
	if len(locs) != 1 {
		t.Fatalf("want 1 location, got %d", len(locs))
	}
	if locs[0].Lat != crossing.Latitude || locs[0].Lon != crossing.Longitude {
		t.Errorf("locs[0]: want {%.4f,%.4f}, got {%.4f,%.4f}",
			crossing.Latitude, crossing.Longitude, locs[0].Lat, locs[0].Lon)
	}
	if locs[0].LocationKind != nil {
		t.Errorf("locs[0].LocationKind: want nil (Break), got %v", *locs[0].LocationKind)
	}
}

// TestCreateRoutingRequests_MultiLocationHappyPath verifies multiple real
// assignments each spanning multiple routeLocations indices produce requests
// with the right location counts, coordinates, and region.
func TestCreateRoutingRequests_MultiLocationHappyPath(t *testing.T) {
	routeLocations := []*routerv1.RouteLocation{
		{Location: &pbgeo.Coordinate{Lat: 52.3, Lon: 4.9}, Type: routerv1.LocationType_L_BREAK},
		{Location: &pbgeo.Coordinate{Lat: 52.1, Lon: 4.5}, Type: routerv1.LocationType_L_THROUGH},
		{Location: &pbgeo.Coordinate{Lat: 48.8, Lon: 2.3}, Type: routerv1.LocationType_L_BREAK},
	}
	assignmentList := []*RegionAssignment{
		{Region: "nl", FromIndex: 0, ToIndex: 1},
		{Region: "fr", FromIndex: 2, ToIndex: 2},
	}

	requestList, err := CreateRoutingRequests(
		nil, routerv1.RoutingMode_RM_CAR, routeLocations, assignmentList, log.New())
	if err != nil {
		t.Fatalf("CreateRoutingRequests error: %v", err)
	}
	if len(requestList) != 2 {
		t.Fatalf("want 2 requests, got %d", len(requestList))
	}

	nlLocs := requestList[0].RequestData.Locations
	if requestList[0].Region != "nl" || len(nlLocs) != 2 {
		t.Fatalf("requestList[0]: want region nl with 2 locations, got region %q with %d locations",
			requestList[0].Region, len(nlLocs))
	}
	if nlLocs[0].Lat != 52.3 || nlLocs[0].Lon != 4.9 || nlLocs[1].Lat != 52.1 || nlLocs[1].Lon != 4.5 {
		t.Errorf("requestList[0] locations: unexpected coordinates %+v", nlLocs)
	}
	if nlLocs[1].LocationKind == nil || *nlLocs[1].LocationKind != vhtypes.Through {
		t.Errorf("requestList[0] locations[1]: want Through, got %v", nlLocs[1].LocationKind)
	}

	frLocs := requestList[1].RequestData.Locations
	if requestList[1].Region != "fr" || len(frLocs) != 1 {
		t.Fatalf("requestList[1]: want region fr with 1 location, got region %q with %d locations",
			requestList[1].Region, len(frLocs))
	}
	if frLocs[0].Lat != 48.8 || frLocs[0].Lon != 2.3 {
		t.Errorf("requestList[1] location: unexpected coordinates %+v", frLocs[0])
	}
}

// TestCreateRoutingRequests_IdSplitting_Single verifies a single-request list
// keeps the given id unchanged (no "#1" suffix).
func TestCreateRoutingRequests_IdSplitting_Single(t *testing.T) {
	routeLocations := []*routerv1.RouteLocation{
		{Location: &pbgeo.Coordinate{Lat: 52.3, Lon: 4.9}, Type: routerv1.LocationType_L_BREAK},
	}
	assignmentList := []*RegionAssignment{
		{Region: "nl", FromIndex: 0, ToIndex: 0},
	}
	id := "my-route"

	requestList, err := CreateRoutingRequests(
		&id, routerv1.RoutingMode_RM_CAR, routeLocations, assignmentList, log.New())
	if err != nil {
		t.Fatalf("CreateRoutingRequests error: %v", err)
	}
	if len(requestList) != 1 {
		t.Fatalf("want 1 request, got %d", len(requestList))
	}
	if requestList[0].RequestData.Id == nil || *requestList[0].RequestData.Id != "my-route" {
		t.Errorf("want id \"my-route\" unchanged, got %v", requestList[0].RequestData.Id)
	}
}

// TestCreateRoutingRequests_IdSplitting_Multiple verifies a multi-request list
// gets "#1", "#2", ... suffixes appended to the given id.
func TestCreateRoutingRequests_IdSplitting_Multiple(t *testing.T) {
	routeLocations := []*routerv1.RouteLocation{
		{Location: &pbgeo.Coordinate{Lat: 52.3, Lon: 4.9}, Type: routerv1.LocationType_L_BREAK},
		{Location: &pbgeo.Coordinate{Lat: 48.8, Lon: 2.3}, Type: routerv1.LocationType_L_BREAK},
		{Location: &pbgeo.Coordinate{Lat: 41.9, Lon: 12.5}, Type: routerv1.LocationType_L_BREAK},
	}
	assignmentList := []*RegionAssignment{
		{Region: "nl", FromIndex: 0, ToIndex: 0},
		{Region: "fr", FromIndex: 1, ToIndex: 1},
		{Region: "it", FromIndex: 2, ToIndex: 2},
	}
	id := "my-route"

	requestList, err := CreateRoutingRequests(
		&id, routerv1.RoutingMode_RM_CAR, routeLocations, assignmentList, log.New())
	if err != nil {
		t.Fatalf("CreateRoutingRequests error: %v", err)
	}
	if len(requestList) != 3 {
		t.Fatalf("want 3 requests, got %d", len(requestList))
	}
	wantIds := []string{"my-route#1", "my-route#2", "my-route#3"}
	for i, want := range wantIds {
		if requestList[i].RequestData.Id == nil || *requestList[i].RequestData.Id != want {
			t.Errorf("requestList[%d].Id: want %q, got %v", i, want, requestList[i].RequestData.Id)
		}
	}
}

// TestCreateRoutingRequests_NilId verifies no request gets an Id set when id is nil.
func TestCreateRoutingRequests_NilId(t *testing.T) {
	routeLocations := []*routerv1.RouteLocation{
		{Location: &pbgeo.Coordinate{Lat: 52.3, Lon: 4.9}, Type: routerv1.LocationType_L_BREAK},
		{Location: &pbgeo.Coordinate{Lat: 48.8, Lon: 2.3}, Type: routerv1.LocationType_L_BREAK},
	}
	assignmentList := []*RegionAssignment{
		{Region: "nl", FromIndex: 0, ToIndex: 0},
		{Region: "fr", FromIndex: 1, ToIndex: 1},
	}

	requestList, err := CreateRoutingRequests(
		nil, routerv1.RoutingMode_RM_CAR, routeLocations, assignmentList, log.New())
	if err != nil {
		t.Fatalf("CreateRoutingRequests error: %v", err)
	}
	for i, r := range requestList {
		if r.RequestData.Id != nil {
			t.Errorf("requestList[%d].Id: want nil, got %v", i, *r.RequestData.Id)
		}
	}
}

// TestCreateRoutingRequests_OptionsApplied verifies options are applied to
// every request in the returned list, not just the first.
func TestCreateRoutingRequests_OptionsApplied(t *testing.T) {
	routeLocations := []*routerv1.RouteLocation{
		{Location: &pbgeo.Coordinate{Lat: 52.3, Lon: 4.9}, Type: routerv1.LocationType_L_BREAK},
		{Location: &pbgeo.Coordinate{Lat: 48.8, Lon: 2.3}, Type: routerv1.LocationType_L_BREAK},
	}
	assignmentList := []*RegionAssignment{
		{Region: "nl", FromIndex: 0, ToIndex: 0},
		{Region: "fr", FromIndex: 1, ToIndex: 1},
	}

	requestList, err := CreateRoutingRequests(
		nil, routerv1.RoutingMode_RM_CAR, routeLocations, assignmentList, log.New(),
		LanguageOption("fr-FR"), UnitOption(routerv1.Unit_U_IMPERIAL))
	if err != nil {
		t.Fatalf("CreateRoutingRequests error: %v", err)
	}
	for i, r := range requestList {
		if r.RequestData.Language == nil || *r.RequestData.Language != vhtypes.Language("fr-FR") {
			t.Errorf("requestList[%d].Language: want fr-FR, got %v", i, r.RequestData.Language)
		}
		if r.RequestData.Units == nil || *r.RequestData.Units != vhtypes.Miles {
			t.Errorf("requestList[%d].Units: want Miles, got %v", i, r.RequestData.Units)
		}
	}
}

// TestRoutingRequestOptions covers every RoutingRequestOption constructor,
// asserting the specific field each one sets on the RouteRequest.
func TestRoutingRequestOptions(t *testing.T) {
	const model = "auto"

	tests := []struct {
		name  string
		opt   RoutingRequestOption
		check func(t *testing.T, r *vhtypes.RouteRequest)
	}{
		{"RouteDetailsOption", RouteDetailsOption(RDFull), func(t *testing.T, r *vhtypes.RouteRequest) {
			if r.DirectionsType == nil || *r.DirectionsType != vhtypes.Instructions {
				t.Errorf("DirectionsType: want Instructions, got %v", r.DirectionsType)
			}
		}},
		{"TollPreferenceOption", TollPreferenceOption(0.7), func(t *testing.T, r *vhtypes.RouteRequest) {
			v := r.CostingOptions[model].UseTolls
			if v == nil || *v != 0.7 {
				t.Errorf("UseTolls: want 0.7, got %v", v)
			}
		}},
		{"FerryPreferenceOption", FerryPreferenceOption(0.6), func(t *testing.T, r *vhtypes.RouteRequest) {
			v := r.CostingOptions[model].UseFerry
			if v == nil || *v != 0.6 {
				t.Errorf("UseFerry: want 0.6, got %v", v)
			}
		}},
		{"HighwayPreferenceOption", HighwayPreferenceOption(0.9), func(t *testing.T, r *vhtypes.RouteRequest) {
			v := r.CostingOptions[model].UseHighways
			if v == nil || *v != 0.9 {
				t.Errorf("UseHighways: want 0.9, got %v", v)
			}
		}},
		{"LivingStreetsPreferenceOption", LivingStreetsPreferenceOption(0.4), func(t *testing.T, r *vhtypes.RouteRequest) {
			v := r.CostingOptions[model].UseLivingStreets
			if v == nil || *v != 0.4 {
				t.Errorf("UseLivingStreets: want 0.4, got %v", v)
			}
		}},
		{"TracksPreferenceOption", TracksPreferenceOption(0.3), func(t *testing.T, r *vhtypes.RouteRequest) {
			v := r.CostingOptions[model].UseTracks
			if v == nil || *v != 0.3 {
				t.Errorf("UseTracks: want 0.3, got %v", v)
			}
		}},
		{"TrailsPreferenceOption", TrailsPreferenceOption(0.2), func(t *testing.T, r *vhtypes.RouteRequest) {
			v := r.CostingOptions[model].UseTrails
			if v == nil || *v != 0.2 {
				t.Errorf("UseTrails: want 0.2, got %v", v)
			}
		}},
		{"PrimaryPreferenceOption", PrimaryPreferenceOption(0.8), func(t *testing.T, r *vhtypes.RouteRequest) {
			v := r.CostingOptions[model].UsePrimary
			if v == nil || *v != 0.8 {
				t.Errorf("UsePrimary: want 0.8, got %v", v)
			}
		}},
		{"ShortestPathOption", ShortestPathOption(true), func(t *testing.T, r *vhtypes.RouteRequest) {
			v := r.CostingOptions[model].Shortest
			if v == nil || !*v {
				t.Errorf("Shortest: want true, got %v", v)
			}
		}},
		{"ShortestDistancePreferenceOption", ShortestDistancePreferenceOption(0.5), func(t *testing.T, r *vhtypes.RouteRequest) {
			v := r.CostingOptions[model].UseDistance
			if v == nil || *v != 0.5 {
				t.Errorf("UseDistance: want 0.5, got %v", v)
			}
		}},
		{"ExcludeUnpavedOption", ExcludeUnpavedOption(true), func(t *testing.T, r *vhtypes.RouteRequest) {
			v := r.CostingOptions[model].ExcludeUnpaved
			if v == nil || !*v {
				t.Errorf("ExcludeUnpaved: want true, got %v", v)
			}
		}},
		{"TopSpeedOption", TopSpeedOption(130), func(t *testing.T, r *vhtypes.RouteRequest) {
			v := r.CostingOptions[model].TopSpeed
			if v == nil || *v != 130 {
				t.Errorf("TopSpeed: want 130, got %v", v)
			}
		}},
		{"CurvynessOption", CurvynessOption(0.5), func(t *testing.T, r *vhtypes.RouteRequest) {
			v := r.CostingOptions[model].ManeuverPenalty
			if v == nil || *v != 5.0 {
				t.Errorf("ManeuverPenalty: want 5.0 (10*0.5), got %v", v)
			}
		}},
		{"UnitOption imperial", UnitOption(routerv1.Unit_U_IMPERIAL), func(t *testing.T, r *vhtypes.RouteRequest) {
			if r.Units == nil || *r.Units != vhtypes.Miles {
				t.Errorf("Units: want Miles, got %v", r.Units)
			}
		}},
		{"UnitOption default", UnitOption(routerv1.Unit_U_METRIC), func(t *testing.T, r *vhtypes.RouteRequest) {
			if r.Units == nil || *r.Units != vhtypes.Kilometers {
				t.Errorf("Units: want Kilometers, got %v", r.Units)
			}
		}},
		{"LanguageOption", LanguageOption("nl-NL"), func(t *testing.T, r *vhtypes.RouteRequest) {
			if r.Language == nil || *r.Language != vhtypes.Language("nl-NL") {
				t.Errorf("Language: want nl-NL, got %v", r.Language)
			}
		}},
		{"ExcludeLocationsOption", ExcludeLocationsOption([]vhtypes.Location{{Lat: 1, Lon: 2}}), func(t *testing.T, r *vhtypes.RouteRequest) {
			if len(r.ExcludeLocations) != 1 || r.ExcludeLocations[0].Lat != 1 || r.ExcludeLocations[0].Lon != 2 {
				t.Errorf("ExcludeLocations: unexpected value %+v", r.ExcludeLocations)
			}
		}},
		{"ExcludePolygonsOption", ExcludePolygonsOption([][][]float64{{{1, 2}, {3, 4}}}), func(t *testing.T, r *vhtypes.RouteRequest) {
			if len(r.ExcludePolygons) != 1 || len(r.ExcludePolygons[0]) != 2 {
				t.Errorf("ExcludePolygons: unexpected value %+v", r.ExcludePolygons)
			}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := vhtypes.NewRouteRequest(vhtypes.Auto)
			tt.opt.Apply(r, model)
			tt.check(t, r)
		})
	}
}

// TestAddBorderCrossings_SingleRequest_NoOp verifies a single-request list is
// a no-op: no client calls are made and no error is returned.
func TestAddBorderCrossings_SingleRequest_NoOp(t *testing.T) {
	lst := RoutingRequestList{buildMinimalRoutingRequest()}
	fake := &fakeRegionQuerier{
		findCrossingLocationsFn: func(ctx context.Context, token, fromRegion, toRegion string, from, to regionclient.Coordinate, config regionclient.BorderCrossingConfig, limit int) ([]regionclient.BorderCrossing, error) {
			t.Fatal("FindCrossingLocations should not be called for a single-request list")
			return nil, nil
		},
	}
	clnt := valhalla.NewClient()

	err := lst.AddBorderCrossings(context.Background(), fake, "tok", clnt,
		routerv1.RoutingMode_RM_CAR, 0.5, 0.5, false, time.Second, log.New())
	if err != nil {
		t.Fatalf("AddBorderCrossings error: %v", err)
	}
}

// newTestValhallaLocateServer spins up an httptest.Server serving /locate
// with a fixed classification for every request, registered under region on
// a fresh valhalla.Client.
func newTestValhallaLocateServer(t *testing.T, region, classification string) *valhalla.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"edges":[{"edge":{"classification":{"classification":"` + classification + `"}}}]}]`))
	}))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("failed to parse test server URL: %v", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("failed to parse test server port: %v", err)
	}

	clnt := valhalla.NewClient()
	clnt.AddRegion(region, u.Hostname(), port)
	return clnt
}

// newTestValhallaMultiRegionClient spins up one httptest.Server per entry in
// classifications (region -> classification) and registers them all on a
// single *valhalla.Client, for tests that call getRoadType for more than one
// region against the same client (as AddBorderCrossings does).
func newTestValhallaMultiRegionClient(t *testing.T, classifications map[string]string) *valhalla.Client {
	t.Helper()
	clnt := valhalla.NewClient()
	for region, classification := range classifications {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"edges":[{"edge":{"classification":{"classification":"` + classification + `"}}}]}]`))
		}))
		t.Cleanup(srv.Close)

		u, err := url.Parse(srv.URL)
		if err != nil {
			t.Fatalf("failed to parse test server URL: %v", err)
		}
		port, err := strconv.Atoi(u.Port())
		if err != nil {
			t.Fatalf("failed to parse test server port: %v", err)
		}
		clnt.AddRegion(region, u.Hostname(), port)
	}
	return clnt
}

// TestAddBorderCrossings_TwoRegions_Success verifies a 2-request list gets
// stitched: r1 gains an appended location and r2 gains a prepended location,
// both matching the selected crossing's coordinates.
func TestAddBorderCrossings_TwoRegions_Success(t *testing.T) {
	r1 := buildMinimalRoutingRequest()
	r1.Region = "nl"
	r2 := buildMinimalRoutingRequest()
	r2.Region = "fr"
	lst := RoutingRequestList{r1, r2}

	crossing := regionclient.Coordinate{Latitude: 50.5, Longitude: 3.5}
	var gotFrom, gotTo string
	fake := &fakeRegionQuerier{
		findCrossingLocationsFn: func(ctx context.Context, token, fromRegion, toRegion string, from, to regionclient.Coordinate, config regionclient.BorderCrossingConfig, limit int) ([]regionclient.BorderCrossing, error) {
			gotFrom, gotTo = fromRegion, toRegion
			return []regionclient.BorderCrossing{
				{FromRegion: fromRegion, ToRegion: toRegion, RoadType: regionclient.RT_MOTORWAY, Location: crossing},
			}, nil
		},
	}
	// getRoadType is called once per region using the SAME *valhalla.Client passed
	// into AddBorderCrossings, so both regions must be registered on one client.
	clnt := newTestValhallaMultiRegionClient(t, map[string]string{"nl": "motorway", "fr": "motorway"})

	err := lst.AddBorderCrossings(context.Background(), fake, "tok", clnt,
		routerv1.RoutingMode_RM_CAR, 0.5, 0.5, false, time.Second, log.New())
	if err != nil {
		t.Fatalf("AddBorderCrossings error: %v", err)
	}
	if gotFrom != "nl" || gotTo != "fr" {
		t.Errorf("FindCrossingLocations called with (%q,%q), want (nl,fr)", gotFrom, gotTo)
	}

	r1Locs := r1.RequestData.Locations
	if len(r1Locs) != 3 {
		t.Fatalf("r1: want 3 locations after append, got %d", len(r1Locs))
	}
	if r1Locs[2].Lat != crossing.Latitude || r1Locs[2].Lon != crossing.Longitude {
		t.Errorf("r1 last location: want crossing coords, got %+v", r1Locs[2])
	}

	r2Locs := r2.RequestData.Locations
	if len(r2Locs) != 3 {
		t.Fatalf("r2: want 3 locations after prepend, got %d", len(r2Locs))
	}
	if r2Locs[0].Lat != crossing.Latitude || r2Locs[0].Lon != crossing.Longitude {
		t.Errorf("r2 first location: want crossing coords, got %+v", r2Locs[0])
	}
}

// TestAddBorderCrossings_EmptyCrossings_ReturnsErrNoBorderCrossings is a
// regression test for historical bug #2: AddBorderCrossings used to
// unconditionally index crossings[0], panicking when regionservice's
// FindCrossingLocations legally returns an empty slice with a nil error.
func TestAddBorderCrossings_EmptyCrossings_ReturnsErrNoBorderCrossings(t *testing.T) {
	r1 := buildMinimalRoutingRequest()
	r1.Region = "nl"
	r2 := buildMinimalRoutingRequest()
	r2.Region = "fr"
	lst := RoutingRequestList{r1, r2}

	fake := &fakeRegionQuerier{
		findCrossingLocationsFn: func(ctx context.Context, token, fromRegion, toRegion string, from, to regionclient.Coordinate, config regionclient.BorderCrossingConfig, limit int) ([]regionclient.BorderCrossing, error) {
			return nil, nil
		},
	}
	clnt := valhalla.NewClient()

	err := lst.AddBorderCrossings(context.Background(), fake, "tok", clnt,
		routerv1.RoutingMode_RM_CAR, 0.5, 0.5, false, time.Second, log.New())
	if !errors.Is(err, ErrNoBorderCrossings) {
		t.Errorf("want ErrNoBorderCrossings, got %v", err)
	}
}

// TestAddBorderCrossings_TransferRegion is a regression test combining
// historical bug #1's fixed output shape (a transfer-region request with no
// locations of its own) with bug #2's code path: it exercises both stitches
// around a transfer request without panicking.
func TestAddBorderCrossings_TransferRegion(t *testing.T) {
	real1 := buildMinimalRoutingRequest()
	real1.Region = "nl"
	transfer := &RoutingRequest{Region: "be", RequestData: vhtypes.NewRouteRequest(vhtypes.Auto)}
	real2 := buildMinimalRoutingRequest()
	real2.Region = "fr"
	lst := RoutingRequestList{real1, transfer, real2}

	fake := &fakeRegionQuerier{
		findCrossingLocationsFn: func(ctx context.Context, token, fromRegion, toRegion string, from, to regionclient.Coordinate, config regionclient.BorderCrossingConfig, limit int) ([]regionclient.BorderCrossing, error) {
			return []regionclient.BorderCrossing{
				{FromRegion: fromRegion, ToRegion: toRegion, RoadType: regionclient.RT_MOTORWAY,
					Location: regionclient.Coordinate{Latitude: 50.0, Longitude: 4.0}},
			}, nil
		},
	}
	clnt := valhalla.NewClient()

	err := lst.AddBorderCrossings(context.Background(), fake, "tok", clnt,
		routerv1.RoutingMode_RM_CAR, 0.5, 0.5, false, time.Second, log.New())
	if err != nil {
		t.Fatalf("AddBorderCrossings error: %v", err)
	}
	if len(transfer.RequestData.Locations) != 2 {
		t.Fatalf("transfer request: want 2 locations (prepended + appended), got %d",
			len(transfer.RequestData.Locations))
	}
}

// TestAddBorderCrossings_FindCrossingLocationsError verifies a downstream
// error is propagated unwrapped.
func TestAddBorderCrossings_FindCrossingLocationsError(t *testing.T) {
	r1 := buildMinimalRoutingRequest()
	r1.Region = "nl"
	r2 := buildMinimalRoutingRequest()
	r2.Region = "fr"
	lst := RoutingRequestList{r1, r2}

	wantErr := errors.New("region service unavailable")
	fake := &fakeRegionQuerier{
		findCrossingLocationsFn: func(ctx context.Context, token, fromRegion, toRegion string, from, to regionclient.Coordinate, config regionclient.BorderCrossingConfig, limit int) ([]regionclient.BorderCrossing, error) {
			return nil, wantErr
		},
	}
	clnt := valhalla.NewClient()

	err := lst.AddBorderCrossings(context.Background(), fake, "tok", clnt,
		routerv1.RoutingMode_RM_CAR, 0.5, 0.5, false, time.Second, log.New())
	if !errors.Is(err, wantErr) {
		t.Errorf("want %v, got %v", wantErr, err)
	}
}

// TestGetRoadType_EmptyEdges verifies an empty (but present) edges array
// returns nil without panicking.
func TestGetRoadType_EmptyEdges(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"edges":[]}]`))
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	clnt := valhalla.NewClient()
	clnt.AddRegion("be", u.Hostname(), port)

	got := getRoadType(context.Background(), time.Second, clnt, "be", orb.Point{4.0, 51.0}, false)
	if got != nil {
		t.Errorf("getRoadType() = %v, want nil for empty edges", *got)
	}
}

// TestGetRoadType_LocateError verifies a Locate failure (unregistered
// region) is swallowed to nil rather than propagated as an error.
func TestGetRoadType_LocateError(t *testing.T) {
	clnt := valhalla.NewClient() // no region registered

	got := getRoadType(context.Background(), time.Second, clnt, "be", orb.Point{4.0, 51.0}, false)
	if got != nil {
		t.Errorf("getRoadType() = %v, want nil when Locate errors", *got)
	}
}

// TestGetRoadType_ClassificationBranches covers every Classification value
// crossed with maxPrimary true/false.
func TestGetRoadType_ClassificationBranches(t *testing.T) {
	tests := []struct {
		classification string
		maxPrimary     bool
		want           *regionclient.RoadType
	}{
		{"motorway", false, rtPtr(regionclient.RT_MOTORWAY)},
		{"motorway", true, nil},
		{"trunk", false, rtPtr(regionclient.RT_TRUNK)},
		{"trunk", true, nil},
		{"primary", false, rtPtr(regionclient.RT_PRIMARY)},
		{"primary", true, rtPtr(regionclient.RT_PRIMARY)},
		{"secondary", false, rtPtr(regionclient.RT_SECONDARY)},
		{"secondary", true, rtPtr(regionclient.RT_SECONDARY)},
		{"tertiary", false, nil},
		{"tertiary", true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.classification+"/maxPrimary="+strconv.FormatBool(tt.maxPrimary), func(t *testing.T) {
			clnt := newTestValhallaLocateServer(t, "be", tt.classification)
			got := getRoadType(context.Background(), time.Second, clnt, "be", orb.Point{4.0, 51.0}, tt.maxPrimary)
			if tt.want == nil {
				if got != nil {
					t.Errorf("want nil, got %v", *got)
				}
				return
			}
			if got == nil || *got != *tt.want {
				t.Errorf("want %v, got %v", *tt.want, got)
			}
		})
	}
}
