package logic

import (
	"math"

	vhtypes "github.com/swayrider/routerservice/restclients/valhalla/types"
)

const (
	// Distance between the points that continue a roundabout round the circle.
	ringStepDegrees = 10.0

	minRingPoints   = 4
	minRingRadius   = 6.0
	maxRingRadius   = 120.0
	maxRingResidual = 0.25 // of the radius, root mean square
	minRingSweep    = 10.0 // degrees the route must go round to know the direction

	metersPerDegree = 111320.0
)

// ExtendAroundRing continues a route round the roundabout it is on. [ring] are
// the points of the route on the roundabout, in order. A circle is fitted
// through them, and the result is the points that carry on round it, in the
// direction of travel, every [ringStepDegrees], up to just before the point
// where the route entered. A route that takes its exit early is thus given the
// rest of the lap, so the roads after its exit can be looked up.
//
// It returns false when the points do not describe a roundabout well enough:
// too few of them, a radius that is not a roundabout's, or points that are not
// on a circle.
func ExtendAroundRing(ring []vhtypes.ShapePoint) ([]vhtypes.ShapePoint, bool) {
	fit, ok := fitRing(ring)
	if !ok {
		return nil, false
	}
	xs, ys, cx, cy, r := fit.xs, fit.ys, fit.cx, fit.cy, fit.r

	// How far round the route has gone, and in which direction (positive is
	// counter-clockwise).
	angle := func(i int) float64 { return math.Atan2(ys[i]-cy, xs[i]-cx) }
	var swept float64
	for i := 1; i < len(xs); i++ {
		swept += normalizeRadians(angle(i) - angle(i-1))
	}
	if math.Abs(swept) < minRingSweep*math.Pi/180 {
		return nil, false
	}
	direction := math.Copysign(1, swept)

	step := ringStepDegrees * math.Pi / 180
	remaining := 2*math.Pi - math.Abs(swept)
	last := angle(len(xs) - 1)
	var out []vhtypes.ShapePoint
	for k := 1; float64(k)*step < remaining-step/2; k++ {
		a := last + direction*float64(k)*step
		x, y := cx+r*math.Cos(a), cy+r*math.Sin(a)
		out = append(out, vhtypes.ShapePoint{
			Lat: fit.lat0 + y/metersPerDegree,
			Lon: fit.lon0 + x/(fit.cosLat*metersPerDegree),
		})
	}
	return out, true
}

// ringFit is a circle fitted through the points of a roundabout, in flat
// coordinates in meters around the first point.
type ringFit struct {
	lat0, lon0, cosLat float64
	xs, ys             []float64 // the points
	cx, cy, r          float64   // the circle
}

// fitRing fits a circle through [ring], and says whether it is a roundabout's:
// enough points, a radius that is a roundabout's, and points that are on a
// circle.
func fitRing(ring []vhtypes.ShapePoint) (ringFit, bool) {
	if len(ring) < minRingPoints {
		return ringFit{}, false
	}
	f := ringFit{lat0: ring[0].Lat, lon0: ring[0].Lon}
	f.cosLat = math.Cos(f.lat0 * math.Pi / 180)
	f.xs = make([]float64, len(ring))
	f.ys = make([]float64, len(ring))
	for i, p := range ring {
		f.xs[i] = (p.Lon - f.lon0) * f.cosLat * metersPerDegree
		f.ys[i] = (p.Lat - f.lat0) * metersPerDegree
	}

	var ok bool
	f.cx, f.cy, f.r, ok = fitCircle(f.xs, f.ys)
	if !ok || f.r < minRingRadius || f.r > maxRingRadius {
		return ringFit{}, false
	}
	var squares float64
	for i := range f.xs {
		d := math.Hypot(f.xs[i]-f.cx, f.ys[i]-f.cy) - f.r
		squares += d * d
	}
	if math.Sqrt(squares/float64(len(f.xs))) > maxRingResidual*f.r {
		return ringFit{}, false
	}
	return f, true
}

// bearingTo is the compass bearing in degrees, 0 to 360, from the centre of
// the circle to [p].
func (f ringFit) bearingTo(p vhtypes.ShapePoint) float64 {
	x := (p.Lon - f.lon0) * f.cosLat * metersPerDegree
	y := (p.Lat - f.lat0) * metersPerDegree
	b := math.Atan2(x-f.cx, y-f.cy) * 180 / math.Pi
	return math.Mod(b+360, 360)
}

// fitCircle fits a circle through the points by least squares (Kasa): it
// solves x^2 + y^2 = 2ax + 2by + c for the centre (a, b), and the radius
// follows from c.
func fitCircle(xs, ys []float64) (cx, cy, r float64, ok bool) {
	n := float64(len(xs))
	var sx, sy, sxx, syy, sxy, sz, sxz, syz float64
	for i := range xs {
		x, y := xs[i], ys[i]
		z := x*x + y*y
		sx += x
		sy += y
		sxx += x * x
		syy += y * y
		sxy += x * y
		sz += z
		sxz += x * z
		syz += y * z
	}
	// Normal equations for (2a, 2b, c).
	m := [3][4]float64{
		{sxx, sxy, sx, sxz},
		{sxy, syy, sy, syz},
		{sx, sy, n, sz},
	}
	sol, solved := solve3(m)
	if !solved {
		return 0, 0, 0, false
	}
	cx, cy = sol[0]/2, sol[1]/2
	r2 := sol[2] + cx*cx + cy*cy
	if r2 <= 0 {
		return 0, 0, 0, false
	}
	return cx, cy, math.Sqrt(r2), true
}

// solve3 solves a 3x3 linear system given as an augmented matrix, by Gaussian
// elimination with pivoting.
func solve3(m [3][4]float64) ([3]float64, bool) {
	for col := 0; col < 3; col++ {
		pivot := col
		for row := col + 1; row < 3; row++ {
			if math.Abs(m[row][col]) > math.Abs(m[pivot][col]) {
				pivot = row
			}
		}
		if math.Abs(m[pivot][col]) < 1e-9 {
			return [3]float64{}, false
		}
		m[col], m[pivot] = m[pivot], m[col]
		for row := col + 1; row < 3; row++ {
			f := m[row][col] / m[col][col]
			for k := col; k < 4; k++ {
				m[row][k] -= f * m[col][k]
			}
		}
	}
	var x [3]float64
	for row := 2; row >= 0; row-- {
		v := m[row][3]
		for k := row + 1; k < 3; k++ {
			v -= m[row][k] * x[k]
		}
		x[row] = v / m[row][row]
	}
	return x, true
}

// normalizeRadians maps an angle to the range -pi to pi.
func normalizeRadians(a float64) float64 {
	for a > math.Pi {
		a -= 2 * math.Pi
	}
	for a < -math.Pi {
		a += 2 * math.Pi
	}
	return a
}
