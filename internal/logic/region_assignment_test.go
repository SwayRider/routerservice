package logic

import (
	"math"
	"testing"

	pbgeo "github.com/swayrider/protos/common_types/geo"
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
