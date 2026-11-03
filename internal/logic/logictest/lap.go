// Package logictest builds roundabouts for tests: the edges and the shape that
// a map-matched lap round a roundabout would give. It is not used outside tests.
package logictest

import (
	"math"

	vhtypes "github.com/swayrider/routerservice/restclients/valhalla/types"
)

const metersPerDegree = 111320.0

// Node is a node on a lap round a roundabout: how far round from the point
// of entry it is, in degrees, and the roads that leave it. The headings of the
// roads are given relative to the tangent of the ring there: a road that
// leaves 30 degrees to the outside of the tangent, say, is 30.
type Node struct {
	Travelled float64
	Arms      []Arm
}

type Arm struct {
	Off   float64 // degrees to the outside of the tangent of the ring
	Drive string
	Class string
}

// LapAround builds what a map-matched lap round a roundabout would give: the
// edges and the shape they point into. The roundabout has the given radius in
// meters, is entered from the south heading north, and is gone round
// counter-clockwise (right-hand traffic), or clockwise when [left] is set.
// The edges end at the nodes, in order, and the last node ends the lap.
func LapAround(radius float64, left bool, nodes []Node) ([]vhtypes.TraceEdge, []vhtypes.ShapePoint) {
	const lat0, lon0 = 51.0, 4.0
	cosLat := math.Cos(lat0 * math.Pi / 180)
	dir := -1.0 // compass bearing from the centre decreases going round
	if left {
		dir = 1
	}
	bearingAt := func(travelled float64) float64 { return math.Mod(180+dir*travelled+720, 360) }
	tangentAt := func(travelled float64) int { // the heading of the ring
		return int(math.Round(math.Mod(bearingAt(travelled)+dir*90+720, 360)))
	}
	point := func(bearing float64) vhtypes.ShapePoint {
		b := bearing * math.Pi / 180
		return vhtypes.ShapePoint{
			Lat: lat0 + radius*math.Cos(b)/metersPerDegree,
			Lon: lon0 + radius*math.Sin(b)/(cosLat*metersPerDegree),
		}
	}

	// The road before the roundabout, two points south of the point of entry.
	shape := []vhtypes.ShapePoint{
		{Lat: lat0 - (radius+20)/metersPerDegree, Lon: lon0},
		{Lat: lat0 - (radius+10)/metersPerDegree, Lon: lon0},
	}
	right := !left
	entry := vhtypes.TraceEdge{BeginHeading: 0, EndHeading: 0, BeginShapeIndex: 0, EndShapeIndex: 2, DriveOnRight: &right}
	edges := []vhtypes.TraceEdge{entry}
	shape = append(shape, point(180)) // the point of entry, index 2

	travelled := 0.0
	for _, node := range nodes {
		begin := len(shape) - 1
		// A point every 10 degrees along the edge, then the node.
		for d := math.Floor(travelled/10)*10 + 10; d < node.Travelled; d += 10 {
			shape = append(shape, point(bearingAt(d)))
		}
		shape = append(shape, point(bearingAt(node.Travelled)))
		e := vhtypes.TraceEdge{
			Roundabout:      true,
			BeginHeading:    tangentAt(travelled),
			EndHeading:      tangentAt(node.Travelled),
			BeginShapeIndex: begin,
			EndShapeIndex:   len(shape) - 1,
			DriveOnRight:    &right,
		}
		if len(node.Arms) > 0 {
			e.EndNode = &vhtypes.TraceNode{}
			for _, a := range node.Arms {
				var heading int
				if left {
					heading = (e.EndHeading - int(a.Off) + 720) % 360
				} else {
					heading = (e.EndHeading + int(a.Off)) % 360
				}
				e.EndNode.IntersectingEdges = append(e.EndNode.IntersectingEdges,
					vhtypes.TraceIntersectingEdge{BeginHeading: heading, Driveability: a.Drive, RoadClass: a.Class})
			}
		}
		edges = append(edges, e)
		travelled = node.Travelled
	}
	return edges, shape
}

func ExitArm(off float64) Arm { return Arm{Off: off, Drive: "forward", Class: "secondary"} }
