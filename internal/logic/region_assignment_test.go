package logic

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/swayrider/grpcclients/regionclient"
	pbgeo "github.com/swayrider/protos/common_types/geo"
	log "github.com/swayrider/swlib/logger"
)

func coord(lat, lon float64) *pbgeo.Coordinate {
	return &pbgeo.Coordinate{Lat: lat, Lon: lon}
}

// TestHaversineKm verifies great-circle distances against known city pairs.
func TestHaversineKm(t *testing.T) {
	tests := []struct {
		name    string
		a, b    *pbgeo.Coordinate
		wantKm  float64
		tolerKm float64
	}{
		{
			name:    "same point",
			a:       coord(52.3676, 4.9041),
			b:       coord(52.3676, 4.9041),
			wantKm:  0,
			tolerKm: 0.001,
		},
		{
			name:    "Amsterdam to Paris",
			a:       coord(52.3676, 4.9041),
			b:       coord(48.8566, 2.3522),
			wantKm:  430,
			tolerKm: 5,
		},
		{
			name:    "Amsterdam to Brussels",
			a:       coord(52.3676, 4.9041),
			b:       coord(50.8503, 4.3517),
			wantKm:  174,
			tolerKm: 5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := haversineKm(tt.a, tt.b)
			if math.Abs(got-tt.wantKm) > tt.tolerKm {
				t.Errorf("haversineKm() = %.2f km, want %.2f ± %.2f", got, tt.wantKm, tt.tolerKm)
			}
		})
	}
}

// TestCorridorWidth verifies clamping to [minCorridorWidthKm, maxCorridorWidthKm].
func TestCorridorWidth(t *testing.T) {
	// Very short path: total << min → clamped to 100
	shortPath := []*pbgeo.Coordinate{
		coord(52.0, 4.0),
		coord(52.001, 4.001), // ~130 m
	}
	if got := corridorWidth(shortPath); got != minCorridorWidthKm {
		t.Errorf("short path: want %v, got %v", minCorridorWidthKm, got)
	}

	// Very long path: total >> max/ratio → clamped to 400
	// 4000 km total * 0.2 = 800 → clamped to 400
	longPath := []*pbgeo.Coordinate{
		coord(0, 0),
		coord(0, 36), // ~4000 km along equator
	}
	if got := corridorWidth(longPath); got != maxCorridorWidthKm {
		t.Errorf("long path: want %v, got %v", maxCorridorWidthKm, got)
	}

	// Mid-range: ~1000 km total → 1000 * 0.2 = 200, within clamp
	midPath := []*pbgeo.Coordinate{
		coord(52.0, 4.0),
		coord(43.3, 5.4), // Marseille, ~1000 km from Amsterdam
	}
	got := corridorWidth(midPath)
	if got < minCorridorWidthKm || got > maxCorridorWidthKm {
		t.Errorf("mid path: %v out of [%v, %v]", got, minCorridorWidthKm, maxCorridorWidthKm)
	}
	total := haversineKm(midPath[0], midPath[1])
	want := total * corridorWidthRatio
	if math.Abs(got-want) > 0.001 {
		t.Errorf("mid path: want %.3f, got %.3f", want, got)
	}
}

// TestCorridorSubPath covers extraction and nil cases.
func TestCorridorSubPath(t *testing.T) {
	path := []string{"A", "B", "C", "D"}

	tests := []struct {
		name       string
		from, to   string
		wantNil    bool
		wantLen    int
		wantFirst  string
		wantLast   string
	}{
		{"normal", "A", "D", false, 4, "A", "D"},
		{"adjacent", "B", "C", false, 2, "B", "C"},
		{"from missing", "X", "D", true, 0, "", ""},
		{"to missing", "A", "X", true, 0, "", ""},
		{"reversed", "D", "A", true, 0, "", ""},
		{"same index", "B", "B", true, 0, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := corridorSubPath(path, tt.from, tt.to)
			if tt.wantNil {
				if got != nil {
					t.Errorf("want nil, got %v", got)
				}
				return
			}
			if len(got) != tt.wantLen {
				t.Errorf("want len %d, got %d (%v)", tt.wantLen, len(got), got)
			}
			if got[0] != tt.wantFirst || got[len(got)-1] != tt.wantLast {
				t.Errorf("want [%s..%s], got %v", tt.wantFirst, tt.wantLast, got)
			}
		})
	}
}

// TestMatchRegions covers the four matching strategies.
func TestMatchRegions(t *testing.T) {
	t.Run("core==core", func(t *testing.T) {
		a := &RegionResolvment{CoreRegions: []string{"NL"}, ExtendedRegions: []string{"BE"}}
		b := &RegionResolvment{CoreRegions: []string{"NL"}, ExtendedRegions: []string{}}
		rc := matchRegions(a, b)
		if rc == nil || rc.CoreRegion != "NL" {
			t.Errorf("want CoreRegion=NL, got %v", rc)
		}
	})

	t.Run("core==extended (b core in a extended)", func(t *testing.T) {
		a := &RegionResolvment{CoreRegions: []string{"NL"}, ExtendedRegions: []string{"DE"}}
		b := &RegionResolvment{CoreRegions: []string{"DE"}, ExtendedRegions: []string{"NL"}}
		rc := matchRegions(a, b)
		if rc == nil || rc.CoreRegion != "DE" {
			t.Errorf("want CoreRegion=DE, got %v", rc)
		}
	})

	t.Run("extended==core (b extended in a core)", func(t *testing.T) {
		a := &RegionResolvment{CoreRegions: []string{"NL"}, ExtendedRegions: []string{}}
		b := &RegionResolvment{CoreRegions: []string{"DE"}, ExtendedRegions: []string{"NL"}}
		rc := matchRegions(a, b)
		if rc == nil || rc.CoreRegion != "DE" || rc.ExtendsIntoRegion != "NL" {
			t.Errorf("want CoreRegion=DE ExtendsInto=NL, got %v", rc)
		}
	})

	t.Run("extended==extended", func(t *testing.T) {
		a := &RegionResolvment{CoreRegions: []string{"NL"}, ExtendedRegions: []string{"BE"}}
		b := &RegionResolvment{CoreRegions: []string{"DE"}, ExtendedRegions: []string{"BE"}}
		rc := matchRegions(a, b)
		if rc == nil || rc.ExtendsIntoRegion != "BE" {
			t.Errorf("want ExtendsIntoRegion=BE, got %v", rc)
		}
	})

	t.Run("no match", func(t *testing.T) {
		a := &RegionResolvment{CoreRegions: []string{"NL"}, ExtendedRegions: []string{"BE"}}
		b := &RegionResolvment{CoreRegions: []string{"FR"}, ExtendedRegions: []string{"ES"}}
		rc := matchRegions(a, b)
		if rc != nil {
			t.Errorf("want nil, got %v", rc)
		}
	})
}

// TestResolveCandList verifies region list resolution via forward+backward pass.
func TestResolveCandList(t *testing.T) {
	t.Run("single region throughout", func(t *testing.T) {
		cands := []*regionCandidate{
			{CoreRegion: "NL", ExtendsIntoRegion: ""},
			{CoreRegion: "NL", ExtendsIntoRegion: ""},
		}
		got := resolveCandList(cands)
		for _, r := range got {
			if r != "NL" {
				t.Errorf("want NL, got %q", r)
			}
		}
	})

	t.Run("clean two-region boundary", func(t *testing.T) {
		cands := []*regionCandidate{
			{CoreRegion: "NL", ExtendsIntoRegion: ""},
			{CoreRegion: "DE", ExtendsIntoRegion: ""},
		}
		got := resolveCandList(cands)
		if len(got) != 2 {
			t.Fatalf("want 2 elements, got %d", len(got))
		}
		if got[0] != "NL" || got[1] != "DE" {
			t.Errorf("want [NL DE], got %v", got)
		}
	})

	t.Run("extends-into collapses backward", func(t *testing.T) {
		// Waypoints go NL→NL/DE-border→DE; the border waypoint extends into DE.
		// The backward pass should assign the border to DE.
		cands := []*regionCandidate{
			{CoreRegion: "NL", ExtendsIntoRegion: ""},
			{CoreRegion: "NL", ExtendsIntoRegion: "DE"},
			{CoreRegion: "DE", ExtendsIntoRegion: ""},
		}
		got := resolveCandList(cands)
		if len(got) != 3 {
			t.Fatalf("want 3 elements, got %d", len(got))
		}
		if got[2] != "DE" {
			t.Errorf("last element: want DE, got %q", got[2])
		}
	})
}

// TestBuildRegionList_AllInFirstCore verifies the shortcut path where every
// resolvment overlaps the first location's core region: every element of the
// returned list should be that first core region, without falling through to
// resolveCandList.
func TestBuildRegionList_AllInFirstCore(t *testing.T) {
	resolveList := []*RegionResolvment{
		{CoreRegions: []string{"nl"}},
		{CoreRegions: []string{"be"}, ExtendedRegions: []string{"nl"}},
		{CoreRegions: []string{"be"}, ExtendedRegions: []string{"nl"}},
	}
	got := buildRegionList(resolveList)
	if len(got) != len(resolveList) {
		t.Fatalf("want %d elements, got %d", len(resolveList), len(got))
	}
	for i, r := range got {
		if r != "nl" {
			t.Errorf("got[%d] = %q, want nl", i, r)
		}
	}
}

// TestBuildRegionList_AllInLastCore verifies the symmetric shortcut where
// every resolvment overlaps the last location's core region.
func TestBuildRegionList_AllInLastCore(t *testing.T) {
	resolveList := []*RegionResolvment{
		{CoreRegions: []string{"nl"}, ExtendedRegions: []string{"fr"}},
		{CoreRegions: []string{"nl"}, ExtendedRegions: []string{"fr"}},
		{CoreRegions: []string{"fr"}},
	}
	got := buildRegionList(resolveList)
	if len(got) != len(resolveList) {
		t.Fatalf("want %d elements, got %d", len(resolveList), len(got))
	}
	for i, r := range got {
		if r != "fr" {
			t.Errorf("got[%d] = %q, want fr", i, r)
		}
	}
}

// TestBuildRegionList_FallsThroughToResolveCandList verifies a genuine
// multi-region path (neither shortcut applies) dispatches to resolveCandList
// and produces the same result as calling it directly on the equivalent
// candidate list, pinning the dispatch logic rather than just the sub-helper.
func TestBuildRegionList_FallsThroughToResolveCandList(t *testing.T) {
	resolveList := []*RegionResolvment{
		{CoreRegions: []string{"nl"}},
		{CoreRegions: []string{"be"}},
		{CoreRegions: []string{"fr"}},
	}
	got := buildRegionList(resolveList)

	// None of nl/be/fr overlap, so matchRegions returns nil for every pair and
	// buildRegionList falls back to each resolvment's own core region.
	wantCands := []*regionCandidate{
		{CoreRegion: "nl", ExtendsIntoRegion: ""},
		{CoreRegion: "be", ExtendsIntoRegion: ""},
		{CoreRegion: "fr", ExtendsIntoRegion: ""},
	}
	want := resolveCandList(wantCands)

	if len(got) != len(want) {
		t.Fatalf("want %d elements, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestCalculateRegionAssignment_SingleRegionHappyPath verifies that when
// every location resolves to the same region and the corridor path is a
// single element, the result is one non-empty assignment spanning all
// locations.
func TestCalculateRegionAssignment_SingleRegionHappyPath(t *testing.T) {
	locations := []*pbgeo.Coordinate{
		{Lat: 52.3, Lon: 4.9},
		{Lat: 52.1, Lon: 4.5},
	}
	fake := &fakeRegionQuerier{
		searchPointFn: func(ctx context.Context, token string, location regionclient.Coordinate, includeExtended bool) (regionclient.RegionList, error) {
			return regionclient.RegionList{CoreRegions: []string{"nl"}}, nil
		},
		findRouteRegionPathsFn: func(ctx context.Context, token string, waypoints []regionclient.Coordinate, widthKm float64) ([][]string, error) {
			return [][]string{{"nl"}}, nil
		},
	}

	assignments, possible, err := CalculateRegionAssignment(context.Background(), fake, "tok", locations, log.New())
	if err != nil {
		t.Fatalf("CalculateRegionAssignment error: %v", err)
	}
	if !possible {
		t.Fatal("want routePossible=true")
	}
	if len(assignments) != 1 {
		t.Fatalf("want 1 assignment, got %d", len(assignments))
	}
	if assignments[0].Region != "nl" || assignments[0].FromIndex != 0 || assignments[0].ToIndex != 1 {
		t.Errorf("want {nl 0 1}, got %+v", assignments[0])
	}
}

// TestCalculateRegionAssignment_MultiRegionHappyPath verifies two locations
// resolving to different regions produce two ordered assignments.
func TestCalculateRegionAssignment_MultiRegionHappyPath(t *testing.T) {
	locations := []*pbgeo.Coordinate{
		{Lat: 52.3, Lon: 4.9}, // nl
		{Lat: 48.8, Lon: 2.3}, // fr
	}
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
	}

	assignments, possible, err := CalculateRegionAssignment(context.Background(), fake, "tok", locations, log.New())
	if err != nil {
		t.Fatalf("CalculateRegionAssignment error: %v", err)
	}
	if !possible {
		t.Fatal("want routePossible=true")
	}
	if len(assignments) != 2 {
		t.Fatalf("want 2 assignments, got %d", len(assignments))
	}
	if assignments[0].Region != "nl" || assignments[1].Region != "fr" {
		t.Errorf("want [nl fr] in order, got [%s %s]", assignments[0].Region, assignments[1].Region)
	}
}

// TestCalculateRegionAssignment_TransferRegionInjected verifies a 3-element
// corridor path spanning two real assignments results in an injected IsEmpty
// middle assignment — the region-assignment-side half of the regression for
// the historical CreateRoutingRequests panic on transfer regions.
func TestCalculateRegionAssignment_TransferRegionInjected(t *testing.T) {
	locations := []*pbgeo.Coordinate{
		{Lat: 52.3, Lon: 4.9}, // nl
		{Lat: 48.8, Lon: 2.3}, // fr
	}
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
	}

	assignments, possible, err := CalculateRegionAssignment(context.Background(), fake, "tok", locations, log.New())
	if err != nil {
		t.Fatalf("CalculateRegionAssignment error: %v", err)
	}
	if !possible {
		t.Fatal("want routePossible=true")
	}
	if len(assignments) != 3 {
		t.Fatalf("want 3 assignments (nl, transfer be, fr), got %d: %+v", len(assignments), assignments)
	}
	if assignments[1].Region != "be" || !assignments[1].IsEmpty || assignments[1].FromIndex != -1 || assignments[1].ToIndex != -1 {
		t.Errorf("want injected transfer assignment {be -1 -1 true}, got %+v", assignments[1])
	}
}

// TestCalculateRegionAssignment_ResolveRegionsError verifies a SearchPoint
// failure propagates through ResolveRegions.
func TestCalculateRegionAssignment_ResolveRegionsError(t *testing.T) {
	wantErr := errors.New("search point failed")
	fake := &fakeRegionQuerier{
		searchPointFn: func(ctx context.Context, token string, location regionclient.Coordinate, includeExtended bool) (regionclient.RegionList, error) {
			return regionclient.RegionList{}, wantErr
		},
	}

	_, _, err := CalculateRegionAssignment(context.Background(), fake, "tok",
		[]*pbgeo.Coordinate{{Lat: 1, Lon: 1}}, log.New())
	if !errors.Is(err, wantErr) {
		t.Errorf("want %v, got %v", wantErr, err)
	}
}

// TestCalculateRegionAssignment_FindRouteRegionPathsError_FallsBackToFindRegionPath
// verifies the graceful-fallback behavior: when the corridor lookup fails,
// CalculateRegionAssignment still succeeds via injectTransferRegions's
// FindRegionPath fallback rather than failing the whole request.
func TestCalculateRegionAssignment_FindRouteRegionPathsError_FallsBackToFindRegionPath(t *testing.T) {
	locations := []*pbgeo.Coordinate{
		{Lat: 52.3, Lon: 4.9}, // nl
		{Lat: 48.8, Lon: 2.3}, // fr
	}
	fake := &fakeRegionQuerier{
		searchPointFn: func(ctx context.Context, token string, location regionclient.Coordinate, includeExtended bool) (regionclient.RegionList, error) {
			if location.Latitude > 50 {
				return regionclient.RegionList{CoreRegions: []string{"nl"}}, nil
			}
			return regionclient.RegionList{CoreRegions: []string{"fr"}}, nil
		},
		findRouteRegionPathsFn: func(ctx context.Context, token string, waypoints []regionclient.Coordinate, widthKm float64) ([][]string, error) {
			return nil, errors.New("valhalla region service unavailable")
		},
		findRegionPathFn: func(ctx context.Context, token, fromRegion, toRegion string) ([]string, error) {
			return []string{fromRegion, toRegion}, nil
		},
	}

	assignments, possible, err := CalculateRegionAssignment(context.Background(), fake, "tok", locations, log.New())
	if err != nil {
		t.Fatalf("CalculateRegionAssignment error: %v", err)
	}
	if !possible {
		t.Fatal("want routePossible=true despite corridor lookup failure")
	}
	if len(assignments) != 2 || assignments[0].Region != "nl" || assignments[1].Region != "fr" {
		t.Errorf("want [nl fr], got %+v", assignments)
	}
}

// TestInjectTransferRegions_NoTransferNeeded verifies a 2-element path
// injects no transfer regions.
func TestInjectTransferRegions_NoTransferNeeded(t *testing.T) {
	assignmentList := []*RegionAssignment{
		{Region: "nl", FromIndex: 0, ToIndex: 0},
		{Region: "fr", FromIndex: 1, ToIndex: 1},
	}
	fake := &fakeRegionQuerier{
		findRegionPathFn: func(ctx context.Context, token, fromRegion, toRegion string) ([]string, error) {
			return []string{fromRegion, toRegion}, nil
		},
	}

	got, possible, err := injectTransferRegions(context.Background(), fake, "tok", assignmentList, nil, log.New())
	if err != nil {
		t.Fatalf("injectTransferRegions error: %v", err)
	}
	if !possible {
		t.Fatal("want possible=true")
	}
	if len(got) != 2 {
		t.Fatalf("want 2 assignments (no injection), got %d: %+v", len(got), got)
	}
}

// TestInjectTransferRegions_SingleTransferInjected verifies a 3-element path
// injects exactly one IsEmpty entry between the two real assignments.
func TestInjectTransferRegions_SingleTransferInjected(t *testing.T) {
	assignmentList := []*RegionAssignment{
		{Region: "nl", FromIndex: 0, ToIndex: 0},
		{Region: "fr", FromIndex: 1, ToIndex: 1},
	}
	fake := &fakeRegionQuerier{
		findRegionPathFn: func(ctx context.Context, token, fromRegion, toRegion string) ([]string, error) {
			return []string{fromRegion, "be", toRegion}, nil
		},
	}

	got, possible, err := injectTransferRegions(context.Background(), fake, "tok", assignmentList, nil, log.New())
	if err != nil {
		t.Fatalf("injectTransferRegions error: %v", err)
	}
	if !possible {
		t.Fatal("want possible=true")
	}
	if len(got) != 3 {
		t.Fatalf("want 3 assignments, got %d: %+v", len(got), got)
	}
	if got[1].Region != "be" || !got[1].IsEmpty || got[1].FromIndex != -1 || got[1].ToIndex != -1 {
		t.Errorf("want injected {be -1 -1 true}, got %+v", got[1])
	}
}

// TestInjectTransferRegions_MultipleTransfersInjected verifies a 4+-element
// path injects multiple ordered IsEmpty entries.
func TestInjectTransferRegions_MultipleTransfersInjected(t *testing.T) {
	assignmentList := []*RegionAssignment{
		{Region: "nl", FromIndex: 0, ToIndex: 0},
		{Region: "es", FromIndex: 1, ToIndex: 1},
	}
	fake := &fakeRegionQuerier{
		findRegionPathFn: func(ctx context.Context, token, fromRegion, toRegion string) ([]string, error) {
			return []string{fromRegion, "be", "fr", toRegion}, nil
		},
	}

	got, possible, err := injectTransferRegions(context.Background(), fake, "tok", assignmentList, nil, log.New())
	if err != nil {
		t.Fatalf("injectTransferRegions error: %v", err)
	}
	if !possible {
		t.Fatal("want possible=true")
	}
	if len(got) != 4 {
		t.Fatalf("want 4 assignments, got %d: %+v", len(got), got)
	}
	if got[1].Region != "be" || !got[1].IsEmpty || got[2].Region != "fr" || !got[2].IsEmpty {
		t.Errorf("want injected [be fr] transfers in order, got %+v", got)
	}
}

// TestInjectTransferRegions_EmptyPath_NotPossible verifies an empty path
// (no route between two regions) sets possible=false with no error.
func TestInjectTransferRegions_EmptyPath_NotPossible(t *testing.T) {
	assignmentList := []*RegionAssignment{
		{Region: "nl", FromIndex: 0, ToIndex: 0},
		{Region: "fr", FromIndex: 1, ToIndex: 1},
	}
	fake := &fakeRegionQuerier{
		findRegionPathFn: func(ctx context.Context, token, fromRegion, toRegion string) ([]string, error) {
			return nil, nil
		},
	}

	_, possible, err := injectTransferRegions(context.Background(), fake, "tok", assignmentList, nil, log.New())
	if err != nil {
		t.Fatalf("injectTransferRegions error: %v", err)
	}
	if possible {
		t.Error("want possible=false")
	}
}

// TestInjectTransferRegions_FindRegionPathError verifies a downstream error propagates.
func TestInjectTransferRegions_FindRegionPathError(t *testing.T) {
	assignmentList := []*RegionAssignment{
		{Region: "nl", FromIndex: 0, ToIndex: 0},
		{Region: "fr", FromIndex: 1, ToIndex: 1},
	}
	wantErr := errors.New("path lookup failed")
	fake := &fakeRegionQuerier{
		findRegionPathFn: func(ctx context.Context, token, fromRegion, toRegion string) ([]string, error) {
			return nil, wantErr
		},
	}

	_, _, err := injectTransferRegions(context.Background(), fake, "tok", assignmentList, nil, log.New())
	if !errors.Is(err, wantErr) {
		t.Errorf("want %v, got %v", wantErr, err)
	}
}

// TestInjectTransferRegions_UsesCorridorSubPathWhenAvailable verifies that
// when the corridor path already contains both regions in order,
// FindRegionPath is never called.
func TestInjectTransferRegions_UsesCorridorSubPathWhenAvailable(t *testing.T) {
	assignmentList := []*RegionAssignment{
		{Region: "nl", FromIndex: 0, ToIndex: 0},
		{Region: "fr", FromIndex: 1, ToIndex: 1},
	}
	corridorPath := []string{"nl", "be", "fr"}
	fake := &fakeRegionQuerier{
		findRegionPathFn: func(ctx context.Context, token, fromRegion, toRegion string) ([]string, error) {
			t.Fatal("FindRegionPath should not be called when a corridor sub-path is available")
			return nil, nil
		},
	}

	got, possible, err := injectTransferRegions(context.Background(), fake, "tok", assignmentList, corridorPath, log.New())
	if err != nil {
		t.Fatalf("injectTransferRegions error: %v", err)
	}
	if !possible {
		t.Fatal("want possible=true")
	}
	if len(got) != 3 || got[1].Region != "be" {
		t.Errorf("want [nl be fr], got %+v", got)
	}
}
