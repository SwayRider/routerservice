package server

import (
	"testing"

	pbgeo "github.com/swayrider/protos/common_types/geo"
	routerv1 "github.com/swayrider/protos/router/v1"
)

// encodeShape converts a flat [lat, lon, lat, lon...] slice into a polyline string
// using the same codec as merge.go.
func encodeShape(flatCoords []float64) string {
	encoded, err := polylineCodec.EncodeFlatCoords(nil, flatCoords)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

// TestFindMergeGroups tests group detection in leg slices.
func TestFindMergeGroups(t *testing.T) {
	t.Run("no MergeNext", func(t *testing.T) {
		legs := []*routerv1.Leg{
			{MergeNext: false},
			{MergeNext: false},
		}
		groups := findMergeGroups(legs)
		if len(groups) != 0 {
			t.Errorf("want 0 groups, got %d", len(groups))
		}
	})

	t.Run("single run in the middle", func(t *testing.T) {
		// legs[1].MergeNext=true means leg 1 and leg 2 form a group
		legs := []*routerv1.Leg{
			{MergeNext: false},
			{MergeNext: true},
			{MergeNext: false},
		}
		groups := findMergeGroups(legs)
		if len(groups) != 1 {
			t.Fatalf("want 1 group, got %d", len(groups))
		}
		if groups[0].start != 1 || groups[0].end != 2 {
			t.Errorf("want group [1,2], got [%d,%d]", groups[0].start, groups[0].end)
		}
	})

	t.Run("MergeNext on last leg clamped", func(t *testing.T) {
		legs := []*routerv1.Leg{
			{MergeNext: false},
			{MergeNext: true},
		}
		groups := findMergeGroups(legs)
		if len(groups) != 1 {
			t.Fatalf("want 1 group, got %d", len(groups))
		}
		if groups[0].start != 1 || groups[0].end != 1 {
			t.Errorf("want group [1,1], got [%d,%d]", groups[0].start, groups[0].end)
		}
	})

	t.Run("two non-adjacent runs", func(t *testing.T) {
		legs := []*routerv1.Leg{
			{MergeNext: true},  // 0
			{MergeNext: false}, // 1 — end of first group
			{MergeNext: false}, // 2
			{MergeNext: true},  // 3
			{MergeNext: false}, // 4 — end of second group
		}
		groups := findMergeGroups(legs)
		if len(groups) != 2 {
			t.Fatalf("want 2 groups, got %d: %v", len(groups), groups)
		}
		if groups[0].start != 0 || groups[0].end != 1 {
			t.Errorf("group[0]: want [0,1], got [%d,%d]", groups[0].start, groups[0].end)
		}
		if groups[1].start != 3 || groups[1].end != 4 {
			t.Errorf("group[1]: want [3,4], got [%d,%d]", groups[1].start, groups[1].end)
		}
	})
}

// TestMergeSummaries tests summary aggregation.
func TestMergeSummaries(t *testing.T) {
	t.Run("nil entries skipped", func(t *testing.T) {
		got := mergeSummaries([]*routerv1.Summary{nil, nil})
		if got.Time != 0 || got.Length != 0 {
			t.Errorf("want zero summary, got time=%v length=%v", got.Time, got.Length)
		}
	})

	t.Run("time and length accumulated", func(t *testing.T) {
		s := mergeSummaries([]*routerv1.Summary{
			{Time: 100, Length: 10},
			{Time: 200, Length: 20},
		})
		if s.Time != 300 {
			t.Errorf("Time: want 300, got %v", s.Time)
		}
		if s.Length != 30 {
			t.Errorf("Length: want 30, got %v", s.Length)
		}
	})

	t.Run("boolean flags are OR'd", func(t *testing.T) {
		s := mergeSummaries([]*routerv1.Summary{
			{HasToll: true, HasHighway: false, HasFerry: false},
			{HasToll: false, HasHighway: true, HasFerry: false},
		})
		if !s.HasToll {
			t.Error("HasToll: want true")
		}
		if !s.HasHighway {
			t.Error("HasHighway: want true")
		}
		if s.HasFerry {
			t.Error("HasFerry: want false")
		}
	})

	t.Run("bounding box expanded", func(t *testing.T) {
		s := mergeSummaries([]*routerv1.Summary{
			{
				BoundingBox: &pbgeo.BoundingBox{
					BottomLeft: &pbgeo.Coordinate{Lat: 50.0, Lon: 4.0},
					TopRight:   &pbgeo.Coordinate{Lat: 51.0, Lon: 5.0},
				},
			},
			{
				BoundingBox: &pbgeo.BoundingBox{
					BottomLeft: &pbgeo.Coordinate{Lat: 49.0, Lon: 3.0},
					TopRight:   &pbgeo.Coordinate{Lat: 52.0, Lon: 6.0},
				},
			},
		})
		bb := s.BoundingBox
		if bb == nil {
			t.Fatal("BoundingBox is nil")
		}
		if bb.BottomLeft.Lat != 49.0 || bb.BottomLeft.Lon != 3.0 {
			t.Errorf("BottomLeft: want {49,3}, got {%v,%v}", bb.BottomLeft.Lat, bb.BottomLeft.Lon)
		}
		if bb.TopRight.Lat != 52.0 || bb.TopRight.Lon != 6.0 {
			t.Errorf("TopRight: want {52,6}, got {%v,%v}", bb.TopRight.Lat, bb.TopRight.Lon)
		}
	})
}

// TestRemoveBorderLocations tests filtering of border crossing locations.
func TestRemoveBorderLocations(t *testing.T) {
	makeLocs := func(n int) []*routerv1.RouteLocationReturned {
		locs := make([]*routerv1.RouteLocationReturned, n)
		for i := range locs {
			locs[i] = &routerv1.RouteLocationReturned{}
		}
		return locs
	}

	t.Run("empty borderIdx leaves locations unchanged", func(t *testing.T) {
		trip := &routerv1.Trip{Locations: makeLocs(3)}
		removeBorderLocations(trip, map[int]bool{})
		if len(trip.Locations) != 3 {
			t.Errorf("want 3 locations, got %d", len(trip.Locations))
		}
	})

	t.Run("removes exactly the indexed locations", func(t *testing.T) {
		trip := &routerv1.Trip{Locations: makeLocs(5)}
		// Mark indices 1 and 3 as borders
		removeBorderLocations(trip, map[int]bool{1: true, 3: true})
		if len(trip.Locations) != 3 {
			t.Errorf("want 3 locations, got %d", len(trip.Locations))
		}
	})
}

// TestFlattenManeuvers verifies shape-index offsets and order.
func TestFlattenManeuvers(t *testing.T) {
	leg0 := &routerv1.Leg{
		Maneuvers: []*routerv1.Maneuver{
			{BeginShapeIndex: 0, EndShapeIndex: 3},
			{BeginShapeIndex: 3, EndShapeIndex: 5},
		},
	}
	leg1 := &routerv1.Leg{
		Maneuvers: []*routerv1.Maneuver{
			{BeginShapeIndex: 0, EndShapeIndex: 4},
		},
	}
	// leg0 starts at offset 0, leg1 starts at offset 10
	offsets := []int{0, 10}

	got := flattenManeuvers([]*routerv1.Leg{leg0, leg1}, offsets)
	if len(got) != 3 {
		t.Fatalf("want 3 maneuvers, got %d", len(got))
	}

	// leg0 maneuvers: no offset applied (offset=0)
	if got[0].BeginShapeIndex != 0 || got[0].EndShapeIndex != 3 {
		t.Errorf("m[0]: want [0,3], got [%d,%d]", got[0].BeginShapeIndex, got[0].EndShapeIndex)
	}
	if got[1].BeginShapeIndex != 3 || got[1].EndShapeIndex != 5 {
		t.Errorf("m[1]: want [3,5], got [%d,%d]", got[1].BeginShapeIndex, got[1].EndShapeIndex)
	}

	// leg1 maneuver: offset 10 added
	if got[2].BeginShapeIndex != 10 || got[2].EndShapeIndex != 14 {
		t.Errorf("m[2]: want [10,14], got [%d,%d]", got[2].BeginShapeIndex, got[2].EndShapeIndex)
	}

	// Verify cloning: mutating original must not affect the result
	leg0.Maneuvers[0].BeginShapeIndex = 99
	if got[0].BeginShapeIndex == 99 {
		t.Error("flattenManeuvers did not clone: mutating original affected result")
	}
}

// TestHandleBorderManeuvers verifies that the boundary between two legs is
// collapsed into a single M_CONTINUE maneuver.
func TestHandleBorderManeuvers(t *testing.T) {
	// Two legs, 2 maneuvers each: [start, dest] and [start, dest]
	leg0 := &routerv1.Leg{
		Maneuvers: []*routerv1.Maneuver{
			{Type: routerv1.ManeuverType_M_START, BeginShapeIndex: 0, EndShapeIndex: 5, Time: 10, Length: 1.0},
			{Type: routerv1.ManeuverType_M_DESTINATION, BeginShapeIndex: 5, EndShapeIndex: 8, Time: 5, Length: 0.5},
		},
	}
	leg1 := &routerv1.Leg{
		Maneuvers: []*routerv1.Maneuver{
			{Type: routerv1.ManeuverType_M_START, BeginShapeIndex: 8, EndShapeIndex: 12, Time: 8, Length: 0.8},
			{Type: routerv1.ManeuverType_M_DESTINATION, BeginShapeIndex: 12, EndShapeIndex: 15, Time: 3, Length: 0.3},
		},
	}

	// Build the flattened maneuver list (same offsets as flattenManeuvers would produce
	// if both legs start at offset 0 in this simplified case)
	maneuvers := []*routerv1.Maneuver{
		leg0.Maneuvers[0],
		leg0.Maneuvers[1],
		leg1.Maneuvers[0],
		leg1.Maneuvers[1],
	}

	got := handleBorderManeuvers(maneuvers, []*routerv1.Leg{leg0, leg1})

	// 4 input maneuvers − 1 removed = 3
	if len(got) != 3 {
		t.Fatalf("want 3 maneuvers after merge, got %d", len(got))
	}

	// The maneuver at the boundary (index 1 after the merge, replacing the old last-of-leg0
	// and first-of-leg1) must be M_CONTINUE with combined time and length.
	continueM := got[1]
	if continueM.Type != routerv1.ManeuverType_M_CONTINUE {
		t.Errorf("boundary maneuver type: want M_CONTINUE, got %v", continueM.Type)
	}
	if continueM.Time != leg0.Maneuvers[1].Time+leg1.Maneuvers[0].Time {
		t.Errorf("combined time: want %v, got %v",
			leg0.Maneuvers[1].Time+leg1.Maneuvers[0].Time, continueM.Time)
	}
	if continueM.Length != leg0.Maneuvers[1].Length+leg1.Maneuvers[0].Length {
		t.Errorf("combined length: want %v, got %v",
			leg0.Maneuvers[1].Length+leg1.Maneuvers[0].Length, continueM.Length)
	}
}

// TestMergeShapes verifies polyline concatenation with and without overlap.
func TestMergeShapes(t *testing.T) {
	t.Run("non-overlapping shapes concatenated", func(t *testing.T) {
		// Leg 0: A→B, Leg 1: C→D (no shared endpoint)
		shape0 := encodeShape([]float64{50.0, 4.0, 51.0, 5.0})
		shape1 := encodeShape([]float64{52.0, 6.0, 53.0, 7.0})

		legs := []*routerv1.Leg{
			{Shape: shape0},
			{Shape: shape1},
		}
		mergedShape, offsets, _, _, err := mergeShapes([]string{shape0, shape1}, legs, false)
		if err != nil {
			t.Fatalf("mergeShapes error: %v", err)
		}

		// Decode and verify 4 distinct points
		pts, _, decErr := polylineCodec.DecodeFlatCoords(nil, []byte(mergedShape))
		if decErr != nil {
			t.Fatalf("decode error: %v", decErr)
		}
		if len(pts)/2 != 4 {
			t.Errorf("want 4 points, got %d", len(pts)/2)
		}

		// leg0 starts at offset 0, leg1 starts at offset 2
		if offsets[0] != 0 {
			t.Errorf("offsets[0]: want 0, got %d", offsets[0])
		}
		if offsets[1] != 2 {
			t.Errorf("offsets[1]: want 2, got %d", offsets[1])
		}
	})

	t.Run("overlapping endpoint is not duplicated", func(t *testing.T) {
		// Leg 0: A→B, Leg 1: B'→C, where B' is a few meters from B — the two
		// legs come from independent Valhalla instances snapping to their own
		// regional road graphs, so an exact coordinate match is unrealistic.
		sharedLat, sharedLon := 51.0, 5.0
		nearbyLat, nearbyLon := 51.00001, 5.0 // ~1.1m north, within tolerance
		shape0 := encodeShape([]float64{50.0, 4.0, sharedLat, sharedLon})
		shape1 := encodeShape([]float64{nearbyLat, nearbyLon, 52.0, 6.0})

		legs := []*routerv1.Leg{
			{Shape: shape0},
			{Shape: shape1},
		}
		mergedShape, offsets, _, _, err := mergeShapes([]string{shape0, shape1}, legs, false)
		if err != nil {
			t.Fatalf("mergeShapes error: %v", err)
		}

		pts, _, decErr := polylineCodec.DecodeFlatCoords(nil, []byte(mergedShape))
		if decErr != nil {
			t.Fatalf("decode error: %v", decErr)
		}
		// A, B, C → 3 points, not 4
		if len(pts)/2 != 3 {
			t.Errorf("want 3 points (overlap deduped), got %d", len(pts)/2)
		}

		// leg1's local point 0 is the shared border point, which already
		// exists in the merged shape at index 1 (leg0's last point) — so
		// leg1's maneuver indices must offset by 1, not 2.
		if offsets[1] != 1 {
			t.Errorf("offsets[1]: want 1, got %d", offsets[1])
		}
	})

	t.Run("overlap not falsely detected beyond tolerance", func(t *testing.T) {
		// Leg 0 ends and Leg 1 starts ~20m apart — distinct points on the
		// road, not the same border crossing snapped by two instances.
		shape0 := encodeShape([]float64{50.0, 4.0, 51.0, 5.0})
		shape1 := encodeShape([]float64{51.00018, 5.0, 52.0, 6.0}) // ~20m north

		legs := []*routerv1.Leg{
			{Shape: shape0},
			{Shape: shape1},
		}
		mergedShape, offsets, _, _, err := mergeShapes([]string{shape0, shape1}, legs, false)
		if err != nil {
			t.Fatalf("mergeShapes error: %v", err)
		}

		pts, _, decErr := polylineCodec.DecodeFlatCoords(nil, []byte(mergedShape))
		if decErr != nil {
			t.Fatalf("decode error: %v", decErr)
		}
		// No dedup expected: 4 distinct points
		if len(pts)/2 != 4 {
			t.Errorf("want 4 points (no overlap), got %d", len(pts)/2)
		}
		if offsets[1] != 2 {
			t.Errorf("offsets[1]: want 2, got %d", offsets[1])
		}
	})

	t.Run("elevation merged with overlap skip", func(t *testing.T) {
		sharedLat, sharedLon := 51.0, 5.0
		shape0 := encodeShape([]float64{50.0, 4.0, sharedLat, sharedLon})
		shape1 := encodeShape([]float64{sharedLat, sharedLon, 52.0, 6.0})
		elev0 := []float64{100, 200}
		elev1 := []float64{200, 300}
		interval := float64(30)

		legs := []*routerv1.Leg{
			{Shape: shape0, Elevation: elev0, ElevationInterval: &interval},
			{Shape: shape1, Elevation: elev1, ElevationInterval: &interval},
		}
		_, _, mergedElev, _, err := mergeShapes([]string{shape0, shape1}, legs, true)
		if err != nil {
			t.Fatalf("mergeShapes error: %v", err)
		}
		// elev0=[100,200] + elev1[1:]=[300] → [100, 200, 300]
		if len(mergedElev) != 3 {
			t.Errorf("want 3 elevation points, got %d: %v", len(mergedElev), mergedElev)
		}
		if mergedElev[0] != 100 || mergedElev[1] != 200 || mergedElev[2] != 300 {
			t.Errorf("elevation: want [100 200 300], got %v", mergedElev)
		}
	})
}

// simpleLeg builds a minimal Leg with a valid two-point polyline shape and no
// maneuvers/elevation, suitable as a mergeLegGroup/mergeRouteResponse fixture.
func simpleLeg(lat1, lon1, lat2, lon2 float64, mergeNext bool) *routerv1.Leg {
	return &routerv1.Leg{
		Shape:     encodeShape([]float64{lat1, lon1, lat2, lon2}),
		Summary:   &routerv1.Summary{Time: 10, Length: 1.0},
		MergeNext: mergeNext,
	}
}

// TestMergeRouteResponse_NilOrNoTripNoop verifies nil response/trip is a no-op.
func TestMergeRouteResponse_NilOrNoTripNoop(t *testing.T) {
	if err := mergeRouteResponse(nil); err != nil {
		t.Errorf("nil response: want nil error, got %v", err)
	}
	if err := mergeRouteResponse(&routerv1.RouteResponse{Trip: nil}); err != nil {
		t.Errorf("nil trip: want nil error, got %v", err)
	}
}

// TestMergeRouteResponse_LessThanTwoLegsNoop verifies fewer than 2 legs is a no-op.
func TestMergeRouteResponse_LessThanTwoLegsNoop(t *testing.T) {
	leg := simpleLeg(50, 4, 51, 5, false)
	trip := &routerv1.Trip{Legs: []*routerv1.Leg{leg}}
	resp := &routerv1.RouteResponse{Trip: trip}

	if err := mergeRouteResponse(resp); err != nil {
		t.Fatalf("mergeRouteResponse error: %v", err)
	}
	if len(trip.Legs) != 1 || trip.Legs[0] != leg {
		t.Errorf("legs should be unchanged, got %v", trip.Legs)
	}
}

// TestMergeRouteResponse_NoMergeGroupsNoop verifies legs with no MergeNext
// flag are left unchanged.
func TestMergeRouteResponse_NoMergeGroupsNoop(t *testing.T) {
	leg0 := simpleLeg(50, 4, 51, 5, false)
	leg1 := simpleLeg(51, 5, 52, 6, false)
	trip := &routerv1.Trip{Legs: []*routerv1.Leg{leg0, leg1}}
	resp := &routerv1.RouteResponse{Trip: trip}

	if err := mergeRouteResponse(resp); err != nil {
		t.Fatalf("mergeRouteResponse error: %v", err)
	}
	if len(trip.Legs) != 2 || trip.Legs[0] != leg0 || trip.Legs[1] != leg1 {
		t.Errorf("legs should be unchanged, got %v", trip.Legs)
	}
}

// TestMergeRouteResponse_SingleGroupMerged verifies a 2-leg group collapses
// to 1 leg matching mergeLegGroup's own output on the same input.
func TestMergeRouteResponse_SingleGroupMerged(t *testing.T) {
	leg0 := simpleLeg(50, 4, 51, 5, true)
	leg1 := simpleLeg(51, 5, 52, 6, false)
	// mergeLegGroup mutates/clones its input's maneuvers but the Shape/Summary
	// values are independent of leg identity, so build a fresh pair to diff against.
	want, err := mergeLegGroup([]*routerv1.Leg{
		simpleLeg(50, 4, 51, 5, true), simpleLeg(51, 5, 52, 6, false),
	})
	if err != nil {
		t.Fatalf("mergeLegGroup error: %v", err)
	}

	trip := &routerv1.Trip{Legs: []*routerv1.Leg{leg0, leg1}}
	resp := &routerv1.RouteResponse{Trip: trip}
	if err := mergeRouteResponse(resp); err != nil {
		t.Fatalf("mergeRouteResponse error: %v", err)
	}

	if len(trip.Legs) != 1 {
		t.Fatalf("want 1 merged leg, got %d", len(trip.Legs))
	}
	if trip.Legs[0].Shape != want.Shape {
		t.Errorf("merged shape: want %q, got %q", want.Shape, trip.Legs[0].Shape)
	}
}

// TestMergeRouteResponse_MultipleGroupsProcessedInReverseOrder verifies two
// separate merge groups (legs [0,1] and [3,4], leg 2 standalone) both merge
// correctly and the reverse-order processing doesn't corrupt earlier-group
// indices.
func TestMergeRouteResponse_MultipleGroupsProcessedInReverseOrder(t *testing.T) {
	leg0 := simpleLeg(50, 4, 51, 5, true)
	leg1 := simpleLeg(51, 5, 52, 6, false)
	leg2 := simpleLeg(52, 6, 53, 7, false)
	leg3 := simpleLeg(53, 7, 54, 8, true)
	leg4 := simpleLeg(54, 8, 55, 9, false)
	trip := &routerv1.Trip{Legs: []*routerv1.Leg{leg0, leg1, leg2, leg3, leg4}}
	resp := &routerv1.RouteResponse{Trip: trip}

	if err := mergeRouteResponse(resp); err != nil {
		t.Fatalf("mergeRouteResponse error: %v", err)
	}
	if len(trip.Legs) != 3 {
		t.Fatalf("want 3 legs (2 merged pairs + 1 standalone), got %d", len(trip.Legs))
	}
	if trip.Legs[1] != leg2 {
		t.Errorf("middle leg should be the untouched standalone leg2, got %v", trip.Legs[1])
	}
	if trip.Legs[0].MergeNext || trip.Legs[2].MergeNext {
		t.Error("merged legs must have MergeNext cleared")
	}
}

// TestMergeRouteResponse_MergeLegGroupError verifies a mergeShapes decode
// error on one leg in a merge group is wrapped and surfaced.
func TestMergeRouteResponse_MergeLegGroupError(t *testing.T) {
	leg0 := simpleLeg(50, 4, 51, 5, true)
	leg1 := &routerv1.Leg{Shape: "\x00", MergeNext: false} // invalid polyline byte
	trip := &routerv1.Trip{Legs: []*routerv1.Leg{leg0, leg1}}
	resp := &routerv1.RouteResponse{Trip: trip}

	err := mergeRouteResponse(resp)
	if err == nil {
		t.Fatal("want an error for a malformed shape, got nil")
	}
}

// TestMergeLegGroup_SingleLegClearsMergeNext verifies a single-leg group
// returns that leg unchanged except for MergeNext being reset to false.
func TestMergeLegGroup_SingleLegClearsMergeNext(t *testing.T) {
	leg := simpleLeg(50, 4, 51, 5, true)
	got, err := mergeLegGroup([]*routerv1.Leg{leg})
	if err != nil {
		t.Fatalf("mergeLegGroup error: %v", err)
	}
	if got != leg {
		t.Error("want the same leg pointer returned")
	}
	if got.MergeNext {
		t.Error("want MergeNext cleared")
	}
}

// TestMergeLegGroup_EmptyGroupError verifies an empty leg group errors.
func TestMergeLegGroup_EmptyGroupError(t *testing.T) {
	_, err := mergeLegGroup(nil)
	if err == nil {
		t.Fatal("want an error for an empty leg group, got nil")
	}
}

// TestMergeLegGroup_OrchestratesSubHelpers verifies the merged leg's fields
// match independently calling the already-tested leaf helpers on the same
// input, pinning the orchestration rather than re-testing the helpers.
func TestMergeLegGroup_OrchestratesSubHelpers(t *testing.T) {
	leg0 := &routerv1.Leg{
		Shape: encodeShape([]float64{50, 4, 51, 5}),
		Maneuvers: []*routerv1.Maneuver{
			{Type: routerv1.ManeuverType_M_START, BeginShapeIndex: 0, EndShapeIndex: 1, Time: 10, Length: 1.0},
		},
		Summary:   &routerv1.Summary{Time: 10, Length: 1.0},
		MergeNext: true,
	}
	leg1 := &routerv1.Leg{
		Shape: encodeShape([]float64{51, 5, 52, 6}),
		Maneuvers: []*routerv1.Maneuver{
			{Type: routerv1.ManeuverType_M_DESTINATION, BeginShapeIndex: 0, EndShapeIndex: 1, Time: 5, Length: 0.5},
		},
		Summary: &routerv1.Summary{Time: 5, Length: 0.5},
	}
	legs := []*routerv1.Leg{leg0, leg1}

	// Independently compute the expected merge via the leaf helpers, using a
	// fresh (unmutated) copy of the input.
	leg0b := &routerv1.Leg{Shape: leg0.Shape, Maneuvers: []*routerv1.Maneuver{
		{Type: routerv1.ManeuverType_M_START, BeginShapeIndex: 0, EndShapeIndex: 1, Time: 10, Length: 1.0},
	}, Summary: leg0.Summary}
	leg1b := &routerv1.Leg{Shape: leg1.Shape, Maneuvers: []*routerv1.Maneuver{
		{Type: routerv1.ManeuverType_M_DESTINATION, BeginShapeIndex: 0, EndShapeIndex: 1, Time: 5, Length: 0.5},
	}, Summary: leg1.Summary}
	wantShape, wantOffsets, _, _, err := mergeShapes(
		[]string{leg0b.Shape, leg1b.Shape}, []*routerv1.Leg{leg0b, leg1b}, false)
	if err != nil {
		t.Fatalf("mergeShapes error: %v", err)
	}
	wantManeuvers := flattenManeuvers([]*routerv1.Leg{leg0b, leg1b}, wantOffsets)
	wantManeuvers = handleBorderManeuvers(wantManeuvers, []*routerv1.Leg{leg0b, leg1b})
	wantSummary := mergeSummaries([]*routerv1.Summary{leg0b.Summary, leg1b.Summary})

	got, err := mergeLegGroup(legs)
	if err != nil {
		t.Fatalf("mergeLegGroup error: %v", err)
	}

	if got.Shape != wantShape {
		t.Errorf("Shape: want %q, got %q", wantShape, got.Shape)
	}
	if len(got.Maneuvers) != len(wantManeuvers) {
		t.Fatalf("Maneuvers: want %d, got %d", len(wantManeuvers), len(got.Maneuvers))
	}
	for i := range wantManeuvers {
		if got.Maneuvers[i].Type != wantManeuvers[i].Type {
			t.Errorf("Maneuvers[%d].Type: want %v, got %v", i, wantManeuvers[i].Type, got.Maneuvers[i].Type)
		}
	}
	if got.Summary.Time != wantSummary.Time || got.Summary.Length != wantSummary.Length {
		t.Errorf("Summary: want %+v, got %+v", wantSummary, got.Summary)
	}
}
