package logic

import (
	"math"
	"testing"

	vhtypes "github.com/swayrider/routerservice/restclients/valhalla/types"
)

const (
	testLat = 51.0
	testLon = 4.0
)

// pointsOnCircle returns points every stepDeg degrees on a circle of the given
// radius in meters, from fromDeg to toDeg (both included), counter-clockwise
// when toDeg is larger.
func pointsOnCircle(radius, fromDeg, toDeg, stepDeg float64) []vhtypes.ShapePoint {
	cosLat := math.Cos(testLat * math.Pi / 180)
	var out []vhtypes.ShapePoint
	n := int(math.Round((toDeg - fromDeg) / stepDeg))
	for i := 0; i <= int(math.Abs(float64(n))); i++ {
		a := (fromDeg + math.Copysign(1, float64(n))*float64(i)*stepDeg) * math.Pi / 180
		out = append(out, vhtypes.ShapePoint{
			Lat: testLat + radius*math.Sin(a)/metersPerDegree,
			Lon: testLon + radius*math.Cos(a)/(cosLat*metersPerDegree),
		})
	}
	return out
}

// bearingFromCentre returns the angle of p round the centre of a circle placed
// at the test origin (the points above are made around a centre of 0, 0
// meters from the first point offset: here around testLat, testLon).
func angleOf(p vhtypes.ShapePoint) float64 {
	cosLat := math.Cos(testLat * math.Pi / 180)
	return math.Atan2((p.Lat-testLat)*metersPerDegree, (p.Lon-testLon)*cosLat*metersPerDegree)
}

func radiusOf(p vhtypes.ShapePoint) float64 {
	cosLat := math.Cos(testLat * math.Pi / 180)
	return math.Hypot((p.Lat-testLat)*metersPerDegree, (p.Lon-testLon)*cosLat*metersPerDegree)
}

func TestExtendAroundRing_CarriesOnCounterClockwiseToJustBeforeTheEntry(t *testing.T) {
	ring := pointsOnCircle(30, 0, 120, 10) // 13 points, a third of the lap
	got, ok := ExtendAroundRing(ring)
	if !ok {
		t.Fatal("want ok")
	}
	// 240 degrees are left: points at 130, 140, ... 350 degrees.
	if len(got) != 23 {
		t.Fatalf("points = %d, want 23", len(got))
	}
	for i, p := range got {
		if d := math.Abs(radiusOf(p) - 30); d > 0.5 {
			t.Errorf("point %d is %.2f m off the circle", i, d)
		}
		want := (130 + 10*float64(i)) * math.Pi / 180
		if diff := math.Abs(normalizeRadians(angleOf(p) - want)); diff > 0.02 {
			t.Errorf("point %d at %.1f degrees, want %.1f", i, angleOf(p)*180/math.Pi, want*180/math.Pi)
		}
	}
}

func TestExtendAroundRing_FollowsTheDirectionOfTravel(t *testing.T) {
	ring := pointsOnCircle(30, 0, -120, 10) // clockwise
	got, ok := ExtendAroundRing(ring)
	if !ok {
		t.Fatal("want ok")
	}
	if len(got) != 23 {
		t.Fatalf("points = %d, want 23", len(got))
	}
	first := normalizeRadians(angleOf(got[0]) - (-130 * math.Pi / 180))
	if math.Abs(first) > 0.02 {
		t.Errorf("first extension point at %.1f degrees, want -130", angleOf(got[0])*180/math.Pi)
	}
}

func TestExtendAroundRing_ExitNearTheEndHasLittleToAdd(t *testing.T) {
	got, ok := ExtendAroundRing(pointsOnCircle(30, 0, 330, 10))
	if !ok {
		t.Fatal("want ok")
	}
	// 30 degrees are left: 340 and 350, just before the point of entry.
	if len(got) != 2 {
		t.Errorf("points = %d, want 2", len(got))
	}
}

func TestExtendAroundRing_AcceptsAnUnevenCircle(t *testing.T) {
	ring := pointsOnCircle(25, 0, 100, 12)
	for i := range ring { // a metre of noise
		ring[i].Lat += math.Sin(float64(i)*7) * 1 / metersPerDegree
	}
	if _, ok := ExtendAroundRing(ring); !ok {
		t.Error("a metre of noise on a 25 m circle must be fine")
	}
}

func TestExtendAroundRing_RejectsWhatIsNotARoundabout(t *testing.T) {
	line := []vhtypes.ShapePoint{
		{Lat: 51, Lon: 4}, {Lat: 51.0001, Lon: 4}, {Lat: 51.0002, Lon: 4}, {Lat: 51.0003, Lon: 4},
	}
	for name, ring := range map[string][]vhtypes.ShapePoint{
		"too few points":   pointsOnCircle(30, 0, 20, 10)[:3],
		"a straight line":  line,
		"far too large":    pointsOnCircle(400, 0, 120, 10),
		"far too small":    pointsOnCircle(2, 0, 120, 10),
		"hardly any sweep": pointsOnCircle(30, 0, 5, 1),
		"nothing":          nil,
	} {
		if got, ok := ExtendAroundRing(ring); ok {
			t.Errorf("%s: got %d points, want not ok", name, len(got))
		}
	}
}

func TestExtendAroundRing_RejectsPointsThatAreNotOnACircle(t *testing.T) {
	// A wave: it bends both ways, so no circle fits it.
	var wave []vhtypes.ShapePoint
	for i := 0; i < 12; i++ {
		x := float64(i) * 8
		y := 25 * math.Sin(x/12)
		wave = append(wave, vhtypes.ShapePoint{
			Lat: testLat + y/metersPerDegree,
			Lon: testLon + x/(math.Cos(testLat*math.Pi/180)*metersPerDegree),
		})
	}
	if got, ok := ExtendAroundRing(wave); ok {
		t.Errorf("got %d points, want not ok", len(got))
	}
}
