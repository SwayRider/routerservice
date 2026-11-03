package logic

import (
	"math"
	"sort"

	vhtypes "github.com/swayrider/routerservice/restclients/valhalla/types"
)

// RoundaboutExits are the exits of a roundabout as angles in degrees (0-359)
// clockwise from the direction of travel on entry: 0 is straight on, 180 is the
// road the roundabout was entered from. They are in the order the roundabout
// is passed.
type RoundaboutExits struct {
	Angles []int
}

const (
	// An exit that heads the same way as the one seen from the route, within
	// this many degrees, is the same exit.
	sameExitDegrees = 10

	// The road the roundabout was entered from is not drawn as an exit: it is
	// the entry stub of the icon. Arms this close to 180 are that road.
	entryRoadDegrees = 25

	// A matched lap has to turn at least this far to count as a whole lap.
	fullLapDegrees = 260

	// After the exit taken, an exit this close to the one before it round the
	// ring is not a road of its own (a tunnel under the roundabout that meets
	// the ring, for example).
	nearbyExitDegrees = 15
)

// roundaboutArm is a road leaving a roundabout: the heading it leaves at, and
// the index of the roundabout edge whose end node it leaves from.
type roundaboutArm struct {
	angle int
	edge  int
}

// ringEdges finds the roundabout in the edges of a matched shape: the first
// run of roundabout edges, with the road before it and, when [needExit] is set,
// the road after it.
func ringEdges(edges []vhtypes.TraceEdge, needExit bool) (first, last int, ok bool) {
	first, last = -1, -1
	for i, e := range edges {
		if !e.Roundabout {
			if first >= 0 {
				break
			}
			continue
		}
		if first < 0 {
			first = i
		}
		last = i
	}
	if first < 1 || (needExit && last+1 >= len(edges)) {
		return 0, 0, false
	}
	return first, last, true
}

// outerExits returns the exits on the roundabout nodes from edge [first] to
// edge [last], in no particular order. An exit is an intersecting edge on the
// outer side of the path (the right where traffic drives on the right) that
// can be driven out of the roundabout: the rule Valhalla uses to number them.
// Where there is no edge after the last one, the path is taken to go straight
// on.
func outerExits(edges []vhtypes.TraceEdge, first, last int) []roundaboutArm {
	driveOnRight := edges[first].DriveOnRight == nil || *edges[first].DriveOnRight
	entryHeading := edges[first-1].EndHeading

	var exits []roundaboutArm
	for i := first; i <= last; i++ {
		e := edges[i]
		if e.EndNode == nil {
			continue
		}
		path := 0
		if i+1 < len(edges) {
			path = turnDegrees(e.EndHeading, edges[i+1].BeginHeading)
		}
		for _, ie := range e.EndNode.IntersectingEdges {
			if ie.Driveability != vhtypes.DriveForward && ie.Driveability != vhtypes.DriveBoth {
				continue
			}
			if !outerSide(turnDegrees(e.EndHeading, ie.BeginHeading), path, driveOnRight) {
				continue
			}
			exits = append(exits, roundaboutArm{angle: turnDegrees(entryHeading, ie.BeginHeading), edge: i})
		}
	}
	return exits
}

// sortExits puts exits in the order the roundabout is passed: by the node they
// leave from, and by the heading they leave at on the same node.
func sortExits(arms []roundaboutArm, driveOnRight bool) {
	sort.SliceStable(arms, func(i, j int) bool {
		if arms[i].edge != arms[j].edge {
			return arms[i].edge < arms[j].edge
		}
		return armOrderKey(arms[i].angle, driveOnRight) < armOrderKey(arms[j].angle, driveOnRight)
	})
}

// RoundaboutExitsFromEdges works out the exits of a roundabout up to the one
// the route takes, from the edges of the matched route: the road before the
// roundabout, the roundabout, and the road after it.
//
// The exits are found with the same rule Valhalla uses to number them, so the
// exit taken is the [exitCount]-th. When the result disagrees with Valhalla's
// count, or the exit taken is not at that position, the second result is false
// and nothing should be used. The exits after the one taken are not known from
// these edges: see [FullRoundaboutExits].
func RoundaboutExitsFromEdges(
	edges []vhtypes.TraceEdge,
	exitCount int,
) (RoundaboutExits, bool) {
	first, last, ok := ringEdges(edges, true)
	if !ok {
		return RoundaboutExits{}, false
	}
	driveOnRight := edges[first].DriveOnRight == nil || *edges[first].DriveOnRight
	entryHeading := edges[first-1].EndHeading

	exits := outerExits(edges, first, last)
	// Valhalla's count: the exit taken plus every exit passed on the way.
	if len(exits)+1 != exitCount {
		return RoundaboutExits{}, false
	}
	taken := roundaboutArm{angle: turnDegrees(entryHeading, edges[last+1].BeginHeading), edge: last}
	exits = append(exits, taken)
	sortExits(exits, driveOnRight)

	// The exit taken has to be the exitCount-th one passed.
	key := func(angle int) int { return armOrderKey(angle, driveOnRight) }
	before, same := 0, 0
	for _, x := range exits {
		switch k := key(x.angle); {
		case k < key(taken.angle):
			before++
		case k == key(taken.angle):
			same++
		}
	}
	if exitCount < before+1 || exitCount > before+same {
		return RoundaboutExits{}, false
	}

	result := RoundaboutExits{}
	for _, x := range exits {
		result.Angles = append(result.Angles, x.angle)
	}
	return result, true
}

// FullRoundaboutExits works out all exits of a roundabout from the edges of a
// shape that goes once around it, entered from the road before it. [shape] is
// the matched path, which the shape indices of the edges point into. [known]
// are the exits up to the one the route takes (see [RoundaboutExitsFromEdges]):
// the walk has to find the same ones first, by the heading they leave at, or
// it is not the same roundabout and nothing should be used.
//
// The angle of an exit is where it is round the ring, not the heading of its
// lane, which leaves almost along the ring and says little about where the
// road is: the bearing from the centre of the roundabout to the exit's node,
// relative to the heading on entry. The road the roundabout was entered from is
// not an exit of the result. After the exit taken, an exit within
// [nearbyExitDegrees] of the one before it is left out: it is not a road of
// its own. Exits up to the one taken are never left out, as Valhalla counts
// them in the exit it speaks of.
func FullRoundaboutExits(
	edges []vhtypes.TraceEdge,
	shape []vhtypes.ShapePoint,
	known RoundaboutExits,
) (RoundaboutExits, bool) {
	first, last, ok := ringEdges(edges, false)
	if !ok || len(known.Angles) == 0 {
		return RoundaboutExits{}, false
	}
	driveOnRight := edges[first].DriveOnRight == nil || *edges[first].DriveOnRight

	// The walk has to go (nearly) all the way round.
	sweep := 0
	for i := first; i <= last; i++ {
		sweep += signedDegrees(edges[i].BeginHeading, edges[i].EndHeading)
		if i < last {
			sweep += signedDegrees(edges[i].EndHeading, edges[i+1].BeginHeading)
		}
	}
	if sweep < 0 {
		sweep = -sweep
	}
	if sweep < fullLapDegrees {
		return RoundaboutExits{}, false
	}

	// The roundabout as a circle, to place the exits on.
	from, to := edges[first].BeginShapeIndex, edges[last].EndShapeIndex
	if from < 0 || to < from || to >= len(shape) {
		return RoundaboutExits{}, false
	}
	fit, ok := fitRing(shape[from : to+1])
	if !ok {
		return RoundaboutExits{}, false
	}
	entryHeading := edges[first-1].EndHeading

	arms := outerExits(edges, first, last)
	sortExits(arms, driveOnRight)
	type exit struct{ heading, position int }
	var all []exit
	for _, a := range arms {
		at := edges[a.edge].EndShapeIndex
		if at < 0 || at >= len(shape) {
			return RoundaboutExits{}, false
		}
		bearing := int(math.Round(fit.bearingTo(shape[at])))
		position := turnDegrees(entryHeading, bearing)
		if headingDistance(position, 180) <= entryRoadDegrees {
			continue
		}
		all = append(all, exit{heading: a.angle, position: position})
	}

	// The exits already known come first, and are the same ones.
	if len(all) < len(known.Angles) {
		return RoundaboutExits{}, false
	}
	var result RoundaboutExits
	for i, a := range known.Angles {
		if headingDistance(all[i].heading, a) > sameExitDegrees {
			return RoundaboutExits{}, false
		}
		result.Angles = append(result.Angles, all[i].position)
	}
	for _, x := range all[len(known.Angles):] {
		previous := result.Angles[len(result.Angles)-1]
		if headingDistance(x.position, previous) <= nearbyExitDegrees {
			continue
		}
		result.Angles = append(result.Angles, x.position)
	}
	return result, true
}

// armOrderKey orders angles in the order a roundabout is passed, starting at
// the road it was entered from: counter-clockwise where traffic drives on the
// right, clockwise where it drives on the left.
func armOrderKey(angle int, driveOnRight bool) int {
	if driveOnRight {
		return ((180-angle)%360 + 360) % 360
	}
	return ((angle-180)%360 + 360) % 360
}

// outerSide mirrors Valhalla's test for an intersecting edge on the outer side
// of the path: right of it where traffic drives on the right, left otherwise.
// [t] and [path] are turn degrees from the heading on arrival.
func outerSide(t, path int, driveOnRight bool) bool {
	if driveOnRight {
		if path > 180 {
			return t > path || t < 180
		}
		return t > path && t < 180
	}
	if path > 180 {
		return t < path && t > 180
	}
	return t < path || t > 180
}

// turnDegrees is the clockwise turn from heading [from] to heading [to].
func turnDegrees(from, to int) int {
	return ((to-from)%360 + 360) % 360
}

// signedDegrees is the turn from heading [from] to heading [to] in the range
// -180 to 179: negative is anti-clockwise.
func signedDegrees(from, to int) int {
	return ((to-from+540)%360+360)%360 - 180
}

// headingDistance is the smallest angle between two headings.
func headingDistance(a, b int) int {
	d := turnDegrees(a, b)
	if d > 180 {
		d = 360 - d
	}
	return d
}
